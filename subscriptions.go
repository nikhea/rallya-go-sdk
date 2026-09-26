package rallya

import (
	"context"
	"net/http"
)

// --- Subscriptions (org tiers; attendee checkout stays one-off) ---

type SubscriptionsService struct{ client *Client }

// ListPlans returns the public catalog: tiers with limits, features, prices.
func (s *SubscriptionsService) ListPlans(ctx context.Context) ([]SubscriptionTier, error) {
	out, err := Do[[]SubscriptionTier](ctx, s.client, "subscription/plans", RequestOptions{Method: http.MethodGet, NoAuth: true})
	if out == nil && err == nil {
		return []SubscriptionTier{}, nil
	}
	return out, err
}

// Get returns one org's billing state. Absent subscription reads FREE.
func (s *SubscriptionsService) Get(ctx context.Context, orgIDOrSlug string) (Subscription, error) {
	return Do[Subscription](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/subscription", RequestOptions{Method: http.MethodGet})
}

// Checkout starts a Stripe subscription-mode Checkout for PRO/SCALE (OWNER).
// Returns the hosted redirect URL — fulfillment lands via webhook.
// 503 when billing is unconfigured server-side.
func (s *SubscriptionsService) Checkout(ctx context.Context, orgIDOrSlug string, plan SubscriptionPlan) (CheckoutSession, error) {
	return Do[CheckoutSession](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/subscription/checkout", RequestOptions{Method: http.MethodPost, Body: map[string]string{"plan": string(plan)}})
}

// Portal opens the Stripe Customer Portal for self-serve manage/cancel
// (OWNER). Downgrades land at period end.
func (s *SubscriptionsService) Portal(ctx context.Context, orgIDOrSlug string) (PortalSession, error) {
	return Do[PortalSession](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/subscription/portal", RequestOptions{Method: http.MethodPost})
}
