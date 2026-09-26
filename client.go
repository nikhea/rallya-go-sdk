package rallya

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TokenStore callbacks. Exactly one auth mode: TokenStore OR API key.
type TokenGetter func() *TokenPair
type TokenSetter func(*TokenPair)

// Options configures the client. Provide APIKey/APIKeyFunc (server-to-server)
// OR GetTokens+SetTokens (user session) — never both, never neither.
type Options struct {
	BaseURL string
	// User sessions.
	GetTokens TokenGetter
	SetTokens TokenSetter
	// Server-to-server org key (rk_live_*). Static or provider.
	APIKey     string
	APIKeyFunc func() string
	// Called when the session/key is dead (refresh rejected, 401 after retry,
	// revoked key). The client already clears stored tokens before calling it.
	OnAuthFailure func()
	// Injectable transport; defaults to http.DefaultClient. Tests pass a
	// client with a stub RoundTripper.
	HTTPClient *http.Client
}

// Upload is a single multipart file part (cover/images).
type Upload struct {
	Field       string
	Filename    string
	Content     []byte
	ContentType string
}

// RequestOptions mirrors the TS RequestOptions. Auth defaults to true;
// set NoAuth for public routes and refresh. NoRetry skips the 401→refresh
// cycle (used internally by refresh itself).
type RequestOptions struct {
	Method  string
	Query   map[string]string
	Body    any
	Upload  *Upload
	Headers map[string]string
	NoAuth  bool
	NoRetry bool
}

// Client is the Rallya API client. Prefer the typed resource accessors.
type Client struct {
	BaseURL string

	getTokens TokenGetter
	setTokens TokenSetter
	apiKey    string
	apiKeyFn  func() string

	onAuthFailure func()
	http          *http.Client

	Auth      *AuthService
	Orgs      *OrgsService
	Events    *EventsService
	Tickets   *TicketsService
	Orders    *OrdersService
	Payments  *PaymentsService
	Attendees *AttendeesService
	Checkin   *CheckinService
	Audit     *AuditService
	Admin     *AdminService
	Health    *HealthService

	mu              sync.Mutex
	refreshInflight chan refreshResult
}

type refreshResult struct {
	pair TokenPair
	err  error
}

// NewClient validates auth config and wires resources.
func NewClient(opts Options) (*Client, error) {
	hasStore := opts.GetTokens != nil || opts.SetTokens != nil
	hasKey := opts.APIKey != "" || opts.APIKeyFunc != nil
	if hasStore && hasKey {
		return nil, errors.New("rallya: use apiKey OR TokenStore (GetTokens/SetTokens), not both")
	}
	if hasStore && (opts.GetTokens == nil || opts.SetTokens == nil) {
		return nil, errors.New("rallya: GetTokens and SetTokens are required together")
	}
	if !hasStore && !hasKey {
		return nil, errors.New("rallya: provide apiKey or TokenStore (GetTokens/SetTokens)")
	}
	if strings.TrimSpace(opts.BaseURL) == "" {
		return nil, errors.New("rallya: BaseURL is required")
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	c := &Client{
		BaseURL:       strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		getTokens:     opts.GetTokens,
		setTokens:     opts.SetTokens,
		apiKey:        opts.APIKey,
		apiKeyFn:      opts.APIKeyFunc,
		onAuthFailure: opts.OnAuthFailure,
		http:          httpClient,
	}
	c.Auth = &AuthService{client: c}
	c.Orgs = &OrgsService{client: c}
	c.Events = &EventsService{client: c}
	c.Tickets = &TicketsService{client: c}
	c.Orders = &OrdersService{client: c}
	c.Payments = &PaymentsService{client: c}
	c.Attendees = &AttendeesService{client: c}
	c.Checkin = &CheckinService{client: c}
	c.Audit = &AuditService{client: c}
	c.Admin = &AdminService{client: c}
	c.Health = &HealthService{client: c}
	return c, nil
}

// Seg escapes a UUID-or-slug path segment.
func Seg(value string) string { return url.PathEscape(value) }

func (c *Client) buildURL(path string, query map[string]string) string {
	p := strings.TrimLeft(path, "/")
	u := c.BaseURL + "/" + p
	if len(query) == 0 {
		return u
	}
	v := url.Values{}
	for k, val := range query {
		if val == "" {
			continue
		}
		v.Set(k, val)
	}
	if enc := v.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

// Do performs a typed request. Prefer resource methods over calling it directly.
func Do[T any](ctx context.Context, c *Client, path string, opts RequestOptions) (T, error) {
	if c.apiKey != "" || c.apiKeyFn != nil {
		return doAPIKey[T](ctx, c, path, opts)
	}
	return doToken[T](ctx, c, path, opts)
}

func (c *Client) apiKeyValue() string {
	if c.apiKeyFn != nil {
		return c.apiKeyFn()
	}
	return c.apiKey
}

func doAPIKey[T any](ctx context.Context, c *Client, path string, opts RequestOptions) (T, error) {
	var zero T
	key := c.apiKeyValue()
	res, resBody, err := c.roundTrip(ctx, path, opts, func(h http.Header) {
		if key != "" && !opts.NoAuth {
			h.Set("X-API-Key", key)
		}
	})
	if err != nil {
		return zero, err
	}
	return decodeBody[T](res, resBody, c.handleAPIKey401(opts))
}

func (c *Client) handleAPIKey401(opts RequestOptions) func(*http.Response) {
	return func(res *http.Response) {
		if res.StatusCode == http.StatusUnauthorized && !opts.NoAuth && c.onAuthFailure != nil {
			c.onAuthFailure()
		}
	}
}

func doToken[T any](ctx context.Context, c *Client, path string, opts RequestOptions) (T, error) {
	var zero T
	var tokens *TokenPair
	if !opts.NoAuth {
		if c.getTokens == nil {
			return zero, errors.New("rallya: token store not configured")
		}
		tokens = c.getTokens()
	}
	res, body, err := c.roundTrip(ctx, path, opts, func(h http.Header) {
		if tokens != nil && tokens.AccessToken != "" && !opts.NoAuth {
			h.Set("Authorization", "Bearer "+tokens.AccessToken)
		}
	})
	if err != nil {
		return zero, err
	}
	if res.StatusCode == http.StatusUnauthorized && !opts.NoAuth && !opts.NoRetry && tokens != nil && tokens.RefreshToken != "" {
		rotated, rerr := c.refreshSingleFlight(ctx, tokens.RefreshToken)
		if rerr != nil {
			var re *RallyaError
			if asRallyaError(rerr, &re) && (re.Status == 401 || re.Status == 404) {
				c.failAuth()
			}
			return zero, rerr
		}
		res2, body2, err2 := c.roundTrip(ctx, path, opts, func(h http.Header) {
			h.Set("Authorization", "Bearer "+rotated.AccessToken)
		})
		if err2 != nil {
			return zero, err2
		}
		if res2.StatusCode == http.StatusUnauthorized {
			c.failAuth()
			return zero, newRallyaError(res2, body2)
		}
		return decodeBody[T](res2, body2, nil)
	}
	return decodeBody[T](res, body, nil)
}

func (c *Client) roundTrip(ctx context.Context, path string, opts RequestOptions, authn func(http.Header)) (*http.Response, []byte, error) {
	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}
	var bodyReader io.Reader
	var contentType string
	if opts.Upload != nil {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		ct := opts.Upload.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		part, err := w.CreatePart(textPartHeader(opts.Upload.Field, opts.Upload.Filename, ct))
		if err != nil {
			return nil, nil, err
		}
		if _, err := part.Write(opts.Upload.Content); err != nil {
			return nil, nil, err
		}
		if err := w.Close(); err != nil {
			return nil, nil, err
		}
		bodyReader = &buf
		contentType = w.FormDataContentType()
	} else if opts.Body != nil {
		raw, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = bytes.NewReader(raw)
		contentType = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path, opts.Query), bodyReader)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if authn != nil {
		authn(req.Header)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, nil, err
	}
	return res, raw, nil
}

func textPartHeader(field, filename, contentType string) map[string][]string {
	escField := strings.ReplaceAll(field, `"`, "_")
	escFile := strings.ReplaceAll(filename, `"`, "_")
	if escFile == "" {
		escFile = "upload"
	}
	return map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escField, escFile)},
		"Content-Type":        {contentType},
	}
}

func decodeBody[T any](res *http.Response, raw []byte, onError func(*http.Response)) (T, error) {
	var zero T
	if onError != nil {
		// Fire for error observability (e.g. revoked API keys).
		if res.StatusCode == http.StatusUnauthorized {
			onError(res)
		}
	}
	if res.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(raw)) == 0 {
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return zero, nil
		}
		return zero, newRallyaError(res, raw)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return zero, newRallyaError(res, raw)
	}
	// Server may return `null` for empty pages — treat as zero value.
	if string(bytes.TrimSpace(raw)) == "null" {
		return zero, nil
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, &RallyaError{Status: res.StatusCode, Message: fmt.Sprintf("decode response: %v", err)}
	}
	return out, nil
}

func newRallyaError(res *http.Response, raw []byte) *RallyaError {
	var payload struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &payload)
	msg := payload.Error
	if msg == "" {
		msg = payload.Message
	}
	if msg == "" {
		msg = fmt.Sprintf("request failed with status %d", res.StatusCode)
	}
	var retry *int64
	if res.StatusCode == 429 {
		retry = parseRetryAfter(res.Header.Get("Retry-After"))
	}
	return &RallyaError{
		Status:       res.StatusCode,
		Code:         payload.Code,
		Message:      msg,
		Details:      payloadOrRaw(payload, raw),
		RetryAfterMs: retry,
	}
}

func payloadOrRaw(payload any, raw []byte) any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	_ = payload
	return v
}

func parseRetryAfter(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil {
		ms := int64(secs * 1000)
		return &ms
	}
	if t, err := http.ParseTime(raw); err == nil {
		ms := t.Sub(time.Now()).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		return &ms
	}
	return nil
}

func (c *Client) refreshSingleFlight(ctx context.Context, refreshToken string) (TokenPair, error) {
	c.mu.Lock()
	if c.refreshInflight != nil {
		ch := c.refreshInflight
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return TokenPair{}, ctx.Err()
		case r := <-ch:
			return r.pair, r.err
		}
	}
	ch := make(chan refreshResult, 1)
	c.refreshInflight = ch
	c.mu.Unlock()

	pair, err := Do[TokenPair](ctx, c, "auth/refresh", RequestOptions{
		Method:  http.MethodPost,
		Body:    map[string]string{"refreshToken": refreshToken},
		NoAuth:  true,
		NoRetry: true,
	})
	if err == nil && c.setTokens != nil {
		cp := pair
		c.setTokens(&cp)
	}
	c.mu.Lock()
	c.refreshInflight = nil
	c.mu.Unlock()
	ch <- refreshResult{pair: pair, err: err}
	close(ch)
	return pair, err
}

func (c *Client) failAuth() {
	if c.setTokens != nil {
		c.setTokens(nil)
	}
	if c.onAuthFailure != nil {
		c.onAuthFailure()
	}
}

// AbsoluteGet performs an absolute-URL GET (routes outside BaseURL, e.g. /health).
func AbsoluteGet[T any](ctx context.Context, c *Client, rawURL string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return zero, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return zero, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return zero, err
	}
	return decodeBody[T](res, raw, nil)
}

// MemoryTokenStore is an in-memory TokenStore for CLIs, tests, and examples.
type MemoryTokenStore struct {
	mu    sync.Mutex
	pair  *TokenPair
	calls int
}

func NewMemoryTokenStore(initial *TokenPair) *MemoryTokenStore {
	return &MemoryTokenStore{pair: initial}
}

func (s *MemoryTokenStore) GetTokens() *TokenPair {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pair
}

func (s *MemoryTokenStore) SetTokens(p *TokenPair) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pair = p
	s.calls++
}

func (s *MemoryTokenStore) SetCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}
