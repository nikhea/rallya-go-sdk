package rallya

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(t *testing.T, body any, status int, headers map[string]string) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{"Content-Type": []string{"application/json"}}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(string(raw))),
	}
}

type tokenFixture struct {
	mu       sync.Mutex
	pair     *TokenPair
	setCalls []any
}

func (f *tokenFixture) get() *TokenPair {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pair
}

func (f *tokenFixture) set(p *TokenPair) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pair = p
	f.setCalls = append(f.setCalls, p)
}

func newTokenClient(fix *tokenFixture, rt roundTripFunc, extra ...func(*Options)) *Client {
	opts := Options{
		BaseURL:   "http://localhost:8080/api/v1",
		GetTokens: fix.get,
		SetTokens: fix.set,
		HTTPClient: &http.Client{
			Transport: rt,
		},
	}
	for _, fn := range extra {
		fn(&opts)
	}
	c, err := NewClient(opts)
	if err != nil {
		panic(err)
	}
	return c
}

func TestMapsErrorEnvelopeWithStatus(t *testing.T) {
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		return jsonResp(t, map[string]string{"error": "email not verified", "code": "EMAIL_NOT_VERIFIED"}, 403, nil), nil
	})
	_, err := c.Auth.Me(context.Background())
	re, ok := err.(*RallyaError)
	if !ok {
		t.Fatalf("expected *RallyaError, got %T (%v)", err, err)
	}
	if re.Status != 403 || re.Code != "EMAIL_NOT_VERIFIED" {
		t.Fatalf("got status=%d code=%q", re.Status, re.Code)
	}
}

func TestStealth404Helper(t *testing.T) {
	if !IsNotFoundOrForbidden(&RallyaError{Status: 404, Message: "organization not found"}) {
		t.Fatal("404 should match")
	}
	if !IsNotFoundOrForbidden(&RallyaError{Status: 403, Message: "forbidden"}) {
		t.Fatal("403 should match")
	}
	if IsNotFoundOrForbidden(&RallyaError{Status: 500, Message: "boom"}) {
		t.Fatal("500 should not match")
	}
}

func TestRetryAfterOn429(t *testing.T) {
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		return jsonResp(t, map[string]string{"error": "rate limited"}, 429, map[string]string{"Retry-After": "2"}), nil
	})
	_, err := c.Auth.Me(context.Background())
	re, ok := err.(*RallyaError)
	if !ok {
		t.Fatalf("expected *RallyaError, got %T", err)
	}
	if re.RetryAfterMs == nil || *re.RetryAfterMs != 2000 {
		t.Fatalf("expected retryAfterMs=2000, got %v", re.RetryAfterMs)
	}
}

func TestRefreshRetriesOnce(t *testing.T) {
	var calls []string
	var mu sync.Mutex
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "old", RefreshToken: "r"}}
	var authFailed atomic.Bool
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.String())
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "auth/refresh") {
			return jsonResp(t, TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, 200, nil), nil
		}
		if strings.HasSuffix(r.URL.Path, "auth/me") && r.Header.Get("Authorization") != "Bearer new-access" {
			return jsonResp(t, map[string]string{"error": "unauthorized"}, 401, nil), nil
		}
		return jsonResp(t, map[string]any{"id": "u1", "email": "j@t.com", "emailVerified": true, "profile": map[string]any{}}, 200, nil), nil
	}, func(o *Options) {
		o.OnAuthFailure = func() { authFailed.Store(true) }
	})
	me, err := c.Auth.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.Email != "j@t.com" {
		t.Fatalf("got email %q", me.Email)
	}
	if got := fix.get(); got == nil || got.AccessToken != "new-access" {
		t.Fatalf("tokens not rotated: %+v", got)
	}
	if authFailed.Load() {
		t.Fatal("onAuthFailure should not fire")
	}
	refreshes := 0
	for _, c := range calls {
		if strings.Contains(c, "auth/refresh") {
			refreshes++
		}
	}
	if refreshes != 1 {
		t.Fatalf("expected 1 refresh, got %d (%v)", refreshes, calls)
	}
}

func TestRefreshRejectedClearsTokens(t *testing.T) {
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "old", RefreshToken: "stolen"}}
	var failed atomic.Bool
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		return jsonResp(t, map[string]string{"error": "unauthorized"}, 401, nil), nil
	}, func(o *Options) {
		o.OnAuthFailure = func() { failed.Store(true) }
	})
	_, err := c.Auth.Me(context.Background())
	if _, ok := err.(*RallyaError); !ok {
		t.Fatalf("expected *RallyaError, got %T (%v)", err, err)
	}
	if fix.get() != nil {
		t.Fatal("tokens should be cleared")
	}
	if !failed.Load() {
		t.Fatal("onAuthFailure should fire")
	}
}

func TestOrderCreateAutoIdempotencyKey(t *testing.T) {
	var sent map[string]any
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &sent)
		return jsonResp(t, map[string]string{"id": "o1"}, 200, nil), nil
	})
	_, err := c.Orders.Create(context.Background(), "event-slug", CreateOrderInput{TicketTypeID: "t1", Quantity: 2})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := sent["idempotencyKey"].(string)
	if key == "" {
		t.Fatalf("expected auto idempotency key, sent %v", sent)
	}
}

func TestBatchCheckinLimitWithoutNetwork(t *testing.T) {
	called := false
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		called = true
		return jsonResp(t, map[string]any{}, 200, nil), nil
	})
	codes := make([]string, 51)
	for i := range codes {
		codes[i] = "c"
	}
	if _, err := c.Checkin.ScanBatch(context.Background(), "o", "e", codes); err == nil || !strings.Contains(err.Error(), "50") {
		t.Fatalf("expected 50-limit error, got %v", err)
	}
	if called {
		t.Fatal("network should not be hit")
	}
}

func TestOversizedCoverWithoutNetwork(t *testing.T) {
	called := false
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		called = true
		return jsonResp(t, map[string]any{}, 200, nil), nil
	})
	big := make([]byte, 5*1024*1024+1)
	if _, err := c.Events.UploadCover(context.Background(), "o", "e", big, "cover.png", "image/png"); err == nil || !strings.Contains(err.Error(), "5 MB") {
		t.Fatalf("expected 5 MB error, got %v", err)
	}
	if called {
		t.Fatal("network should not be hit")
	}
}

func TestEncodesSegmentsAndTrailingSlashBaseURL(t *testing.T) {
	var seen string
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	opts := Options{
		BaseURL:   "http://localhost:8080/api/v1/",
		GetTokens: fix.get,
		SetTokens: fix.set,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = r.URL.String()
			return jsonResp(t, map[string]string{"id": "x"}, 200, nil), nil
		})},
	}
	c, err := NewClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Orgs.Get(context.Background(), "Acme Inc"); err != nil {
		t.Fatal(err)
	}
	if seen != "http://localhost:8080/api/v1/orgs/Acme%20Inc" {
		t.Fatalf("got %q", seen)
	}
}

func TestPublicRoutesWithoutAuthorization(t *testing.T) {
	var auth string
	seen := false
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		auth = r.Header.Get("Authorization")
		seen = true
		return jsonResp(t, map[string]any{"items": []any{}, "total": 0}, 200, nil), nil
	})
	if _, err := c.Events.ListPublic(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !seen || auth != "" {
		t.Fatalf("expected no Authorization header, got %q", auth)
	}
}

func newKeyClient(apiKey string, fn func() string, rt roundTripFunc, onFail func()) *Client {
	opts := Options{BaseURL: "http://localhost:8080/api/v1", APIKey: apiKey, APIKeyFunc: fn, HTTPClient: &http.Client{Transport: rt}, OnAuthFailure: onFail}
	c, err := NewClient(opts)
	if err != nil {
		panic(err)
	}
	return c
}

func TestAPIKeySendsHeaderWithoutRefresh(t *testing.T) {
	var gotKey, gotAuth string
	calls := 0
	var failed bool
	c := newKeyClient("rk_live_testsecret", nil, func(r *http.Request) (*http.Response, error) {
		calls++
		gotKey = r.Header.Get("X-Api-Key")
		gotAuth = r.Header.Get("Authorization")
		return jsonResp(t, map[string]string{"error": "unauthorized"}, 401, nil), nil
	}, func() { failed = true })
	_, err := c.Orgs.Get(context.Background(), "acme")
	if _, ok := err.(*RallyaError); !ok {
		t.Fatalf("expected *RallyaError, got %T", err)
	}
	if gotKey != "rk_live_testsecret" {
		t.Fatalf("got key %q", gotKey)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization, got %q", gotAuth)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no refresh), got %d", calls)
	}
	if !failed {
		t.Fatal("onAuthFailure should fire")
	}
}

func TestAPIKeySuppressedOnPublicRoutes(t *testing.T) {
	var gotKey string
	seen := false
	c := newKeyClient("rk_live_testsecret", nil, func(r *http.Request) (*http.Response, error) {
		gotKey = r.Header.Get("X-Api-Key")
		seen = true
		return jsonResp(t, map[string]any{"items": []any{}, "total": 0}, 200, nil), nil
	}, nil)
	if _, err := c.Events.ListPublic(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !seen || gotKey != "" {
		t.Fatalf("expected no X-API-Key on public route, got %q", gotKey)
	}
}

func TestAPIKeyFuncProvider(t *testing.T) {
	var gotKey string
	c := newKeyClient("", func() string { return "rk_live_from_fn" }, func(r *http.Request) (*http.Response, error) {
		gotKey = r.Header.Get("X-Api-Key")
		return jsonResp(t, map[string]string{"id": "x"}, 200, nil), nil
	}, nil)
	if _, err := c.Orgs.Get(context.Background(), "acme"); err != nil {
		t.Fatal(err)
	}
	if gotKey != "rk_live_from_fn" {
		t.Fatalf("got %q", gotKey)
	}
}

func TestRejectsAmbiguousOrMissingAuth(t *testing.T) {
	fix := &tokenFixture{}
	if _, err := NewClient(Options{BaseURL: "http://localhost:8080/api/v1", APIKey: "rk_live_x", GetTokens: fix.get, SetTokens: fix.set}); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("expected not-both error, got %v", err)
	}
	if _, err := NewClient(Options{BaseURL: "http://localhost:8080/api/v1"}); err == nil || !strings.Contains(err.Error(), "provide apiKey or TokenStore") {
		t.Fatalf("expected missing-auth error, got %v", err)
	}
	if _, err := NewClient(Options{BaseURL: "http://localhost:8080/api/v1", GetTokens: fix.get}); err == nil || !strings.Contains(err.Error(), "required together") {
		t.Fatalf("expected together error, got %v", err)
	}
}

func TestKitsURLsAndBodies(t *testing.T) {
	var seen []string
	var bodies []string
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		var raw []byte
		if r.Body != nil {
			raw, _ = io.ReadAll(r.Body)
		}
		seen = append(seen, r.Method+" "+r.URL.String())
		bodies = append(bodies, string(raw))
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/kits") {
			return jsonResp(t, map[string]any{"id": "k1", "remaining": 2}, 201, nil), nil
		}
		if strings.HasSuffix(r.URL.Path, "/collect") {
			return jsonResp(t, map[string]any{"id": "c1", "status": "COLLECTED"}, 201, nil), nil
		}
		return jsonResp(t, map[string]any{"items": []any{}, "total": 0}, 200, nil), nil
	})
	if _, err := c.Kits.CreateKit(context.Background(), "acme", "fest-2026", CreateKitInput{Name: "VIP pack", QuantityTotal: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Kits.Collect(context.Background(), "acme", "fest-2026", "k1", CollectKitInput{AttendeeID: "a1", IdempotencyKey: "k-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Kits.ListCollections(context.Background(), "acme", "fest-2026", &CollectionFilter{Status: CollectionStatusCollected}); err != nil {
		t.Fatal(err)
	}
	if got := seen[0]; got != "POST http://localhost:8080/api/v1/orgs/acme/events/fest-2026/kits" {
		t.Fatalf("create URL: %q", got)
	}
	if !strings.HasSuffix(seen[1], "/kits/k1/collect") || !strings.Contains(bodies[1], `"idempotencyKey":"k-1"`) {
		t.Fatalf("collect: %q %q", seen[1], bodies[1])
	}
	if !strings.Contains(seen[2], "/kit-collections?status=COLLECTED") {
		t.Fatalf("list filter: %q", seen[2])
	}
}

func TestSubscriptionsURLsAndCodes(t *testing.T) {
	var seen []string
	var bodies []string
	var authHeaders []string
	fix := &tokenFixture{pair: &TokenPair{AccessToken: "a", RefreshToken: "r"}}
	c := newTokenClient(fix, func(r *http.Request) (*http.Response, error) {
		var raw []byte
		if r.Body != nil {
			raw, _ = io.ReadAll(r.Body)
		}
		seen = append(seen, r.Method+" "+r.URL.String())
		bodies = append(bodies, string(raw))
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		if strings.HasSuffix(r.URL.Path, "/subscription/plans") {
			return jsonResp(t, []any{
				map[string]any{"plan": "FREE"},
				map[string]any{"plan": "PRO"},
				map[string]any{"plan": "SCALE"},
			}, 200, nil), nil
		}
		if strings.HasSuffix(r.URL.Path, "/portal") {
			return jsonResp(t, map[string]string{"error": "subscription billing unavailable", "code": "BILLING_UNAVAILABLE"}, 503, nil), nil
		}
		return jsonResp(t, map[string]string{"url": "https://checkout/session", "sessionId": "cs_1"}, 201, nil), nil
	})
	tiers, err := c.Subscriptions.ListPlans(context.Background())
	if err != nil || len(tiers) != 3 {
		t.Fatalf("plans: %v %+v", err, tiers)
	}
	if authHeaders[0] != "" {
		t.Fatalf("catalog must send no auth, got %q", authHeaders[0])
	}
	if _, err := c.Subscriptions.Checkout(context.Background(), "acme", SubscriptionPlanPro); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen[1], "/orgs/acme/subscription/checkout") || !strings.Contains(bodies[1], `"plan":"PRO"`) {
		t.Fatalf("checkout: %q %q", seen[1], bodies[1])
	}
	_, err = c.Subscriptions.Portal(context.Background(), "acme")
	re, ok := err.(*RallyaError)
	if !ok || re.Status != 503 || re.Code != "BILLING_UNAVAILABLE" {
		t.Fatalf("portal: want 503 BILLING_UNAVAILABLE, got %v", err)
	}
}
