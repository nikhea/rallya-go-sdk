package rallya

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
)

// --- Orders ---

type OrdersService struct{ client *Client }

func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		var out [32]byte
		hex.Encode(out[:], b[:])
		return string(out[:])
	}
	return "order-req-fallback"
}

func (s *OrdersService) Create(ctx context.Context, eventIDOrSlug string, in CreateOrderInput) (Order, error) {
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = newIdempotencyKey()
	}
	return Do[Order](ctx, s.client, "events/"+Seg(eventIDOrSlug)+"/orders", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *OrdersService) ListMine(ctx context.Context, q *PageQuery) (Page[Order], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	out, err := Do[Page[Order]](ctx, s.client, "orders/mine", RequestOptions{Method: http.MethodGet, Query: query})
	if err != nil {
		return out, err
	}
	return FillPage(q, out), nil
}

func (s *OrdersService) Get(ctx context.Context, orderID string) (Order, error) {
	return Do[Order](ctx, s.client, "orders/"+Seg(orderID), RequestOptions{Method: http.MethodGet})
}

func (s *OrdersService) Cancel(ctx context.Context, orderID string) (Order, error) {
	return Do[Order](ctx, s.client, "orders/"+Seg(orderID)+"/cancel", RequestOptions{Method: http.MethodPost})
}

// --- Payments ---

type PaymentsService struct{ client *Client }

// Checkout creates a Stripe Checkout Session for a priced order and returns
// the redirect URL. 503 when Stripe is unconfigured server-side.
func (s *PaymentsService) Checkout(ctx context.Context, orderID string) (CheckoutResponse, error) {
	return Do[CheckoutResponse](ctx, s.client, "orders/"+Seg(orderID)+"/checkout", RequestOptions{Method: http.MethodPost})
}

// --- Attendees ---

type AttendeesService struct{ client *Client }

func (s *AttendeesService) ListMine(ctx context.Context, q *PageQuery) (Page[Attendee], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	out, err := Do[Page[Attendee]](ctx, s.client, "attendees/mine", RequestOptions{Method: http.MethodGet, Query: query})
	if err != nil {
		return out, err
	}
	return FillPage(q, out), nil
}

func (s *AttendeesService) GetMine(ctx context.Context, attendeeID string) (Attendee, error) {
	return Do[Attendee](ctx, s.client, "attendees/"+Seg(attendeeID), RequestOptions{Method: http.MethodGet})
}

func (s *AttendeesService) CancelMine(ctx context.Context, attendeeID string) (Attendee, error) {
	return Do[Attendee](ctx, s.client, "attendees/"+Seg(attendeeID)+"/cancel", RequestOptions{Method: http.MethodPost})
}

func (s *AttendeesService) ListRoster(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, q *PageQuery) (Page[Attendee], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	out, err := Do[Page[Attendee]](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/attendees", RequestOptions{Method: http.MethodGet, Query: query})
	if err != nil {
		return out, err
	}
	return FillPage(q, out), nil
}

type WalkInInput struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

func (s *AttendeesService) AddWalkIn(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, in WalkInInput) (Attendee, error) {
	return Do[Attendee](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/attendees", RequestOptions{Method: http.MethodPost, Body: in})
}

type CorrectAttendeeInput struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

func (s *AttendeesService) Correct(ctx context.Context, orgIDOrSlug, eventIDOrSlug, attendeeID string, in CorrectAttendeeInput) (Attendee, error) {
	return Do[Attendee](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/attendees/"+Seg(attendeeID), RequestOptions{Method: http.MethodPatch, Body: in})
}

// --- Check-in ---

type CheckinService struct{ client *Client }

// Scan performs a door scan. Exactly one of Code / AttendeeID.
// Refusals are 200 + outcome, never errors.
func (s *CheckinService) Scan(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, in ScanInput) (ScanResult, error) {
	return Do[ScanResult](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/checkin", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *CheckinService) ScanBatch(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, codes []string) (ScanBatchResponse, error) {
	if len(codes) > 50 {
		return ScanBatchResponse{}, errors.New("batch check-in limited to 50 codes")
	}
	return Do[ScanBatchResponse](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/checkin/batch", RequestOptions{Method: http.MethodPost, Body: map[string]any{"codes": codes}})
}

func (s *CheckinService) Revert(ctx context.Context, orgIDOrSlug, eventIDOrSlug, attendeeID string) (ScanResult, error) {
	return Do[ScanResult](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/checkin/revert", RequestOptions{Method: http.MethodPost, Body: map[string]string{"attendeeId": attendeeID}})
}

func (s *CheckinService) Stats(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) (CheckinStats, error) {
	return Do[CheckinStats](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/checkin/stats", RequestOptions{Method: http.MethodGet})
}

// --- Audit / Admin / Health ---

type AuditService struct{ client *Client }

func (s *AuditService) ListOrg(ctx context.Context, orgIDOrSlug string, q *AuditQuery) (Page[AuditEvent], error) {
	out, err := Do[Page[AuditEvent]](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/audit", RequestOptions{Method: http.MethodGet, Query: auditQuery(q)})
	if err != nil {
		return out, err
	}
	return FillPage(auditPageQuery(q), out), nil
}

func (s *AuditService) ListPlatform(ctx context.Context, q *AuditQuery) (Page[AuditEvent], error) {
	out, err := Do[Page[AuditEvent]](ctx, s.client, "admin/audit", RequestOptions{Method: http.MethodGet, Query: auditQuery(q)})
	if err != nil {
		return out, err
	}
	return FillPage(auditPageQuery(q), out), nil
}

type AdminService struct{ client *Client }

func (s *AdminService) ListOrgs(ctx context.Context, q *PageQuery) (Page[AdminOrg], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	out, err := Do[Page[AdminOrg]](ctx, s.client, "admin/orgs", RequestOptions{Method: http.MethodGet, Query: query})
	if err != nil {
		return out, err
	}
	return FillPage(q, out), nil
}

func (s *AdminService) GetOrg(ctx context.Context, id string) (map[string]any, error) {
	return Do[map[string]any](ctx, s.client, "admin/orgs/"+Seg(id), RequestOptions{Method: http.MethodGet})
}

func (s *AdminService) ReseedOrgPolicies(ctx context.Context, id string) (PolicyDiff, error) {
	return Do[PolicyDiff](ctx, s.client, "admin/orgs/"+Seg(id)+"/policies/reseed", RequestOptions{Method: http.MethodPost})
}

type UserSearchQuery struct {
	PageQuery
	Q string
}

func (s *AdminService) SearchUsers(ctx context.Context, q *UserSearchQuery) (Page[map[string]any], error) {
	query := map[string]string{}
	if q != nil {
		for k, v := range pageQuery(q.PageQuery) {
			query[k] = v
		}
		if q.Q != "" {
			query["q"] = q.Q
		}
	}
	out, err := Do[Page[map[string]any]](ctx, s.client, "admin/users", RequestOptions{Method: http.MethodGet, Query: query})
	if err != nil {
		return out, err
	}
	var pq *PageQuery
	if q != nil {
		pq = &q.PageQuery
	}
	return FillPage(pq, out), nil
}

func (s *AdminService) GetUser(ctx context.Context, id string) (map[string]any, error) {
	return Do[map[string]any](ctx, s.client, "admin/users/"+Seg(id), RequestOptions{Method: http.MethodGet})
}

func (s *AdminService) ListUserOrders(ctx context.Context, id string, q *PageQuery) (Page[map[string]any], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	out, err := Do[Page[map[string]any]](ctx, s.client, "admin/users/"+Seg(id)+"/orders", RequestOptions{Method: http.MethodGet, Query: query})
	if err != nil {
		return out, err
	}
	return FillPage(q, out), nil
}

func (s *AdminService) SyncUserPolicies(ctx context.Context, id string) (PolicyDiff, error) {
	return Do[PolicyDiff](ctx, s.client, "admin/users/"+Seg(id)+"/policies/sync", RequestOptions{Method: http.MethodPost})
}

type HealthService struct{ client *Client }

func (s *HealthService) Live(ctx context.Context) (HealthStatus, error) {
	// /health sits outside /api/v1 — resolve against the origin, no auth.
	origin := s.client.BaseURL
	// Strip everything after the host: scheme://host.
	rest := origin
	if i := indexOf(rest, "://"); i >= 0 {
		slash := indexOf(rest[i+3:], "/")
		if slash >= 0 {
			rest = rest[:i+3+slash]
		}
	} else if i := indexOf(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return AbsoluteGet[HealthStatus](ctx, s.client, rest+"/health")
}

func (s *HealthService) Hello(ctx context.Context) (map[string]any, error) {
	return Do[map[string]any](ctx, s.client, "hello", RequestOptions{Method: http.MethodGet, NoAuth: true})
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
