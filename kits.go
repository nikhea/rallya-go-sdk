package rallya

import (
	"context"
	"net/http"
)

// --- Kits ---

type KitsService struct{ client *Client }

// CreateKit defines one named kit type for an event (quantityTotal >= 1).
func (s *KitsService) CreateKit(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, in CreateKitInput) (KitType, error) {
	return Do[KitType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kits", RequestOptions{Method: http.MethodPost, Body: in})
}

// ListKits returns kit types with live pending/collected/voided/remaining tallies.
func (s *KitsService) ListKits(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) ([]KitType, error) {
	out, err := Do[[]KitType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kits", RequestOptions{Method: http.MethodGet})
	if out == nil && err == nil {
		return []KitType{}, nil
	}
	return out, err
}

// UpdateKit patches name/description/quantity. QuantityTotal cannot drop
// below the collected count (422).
func (s *KitsService) UpdateKit(ctx context.Context, orgIDOrSlug, eventIDOrSlug, kitID string, in UpdateKitInput) (KitType, error) {
	return Do[KitType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kits/"+Seg(kitID), RequestOptions{Method: http.MethodPatch, Body: in})
}

// DeleteKit removes a kit type. Refused while active collections exist (422).
func (s *KitsService) DeleteKit(ctx context.Context, orgIDOrSlug, eventIDOrSlug, kitID string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kits/"+Seg(kitID), RequestOptions{Method: http.MethodDelete})
	return err
}

// Collect hands a kit to a checked-in attendee (422 otherwise). Collects
// immediately unless Reserve holds a PENDING unit. Pass IdempotencyKey to
// make tablet retries safe.
func (s *KitsService) Collect(ctx context.Context, orgIDOrSlug, eventIDOrSlug, kitID string, in CollectKitInput) (KitCollection, error) {
	return Do[KitCollection](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kits/"+Seg(kitID)+"/collect", RequestOptions{Method: http.MethodPost, Body: in})
}

// MarkCollected flips a reserved (PENDING) handout to COLLECTED.
func (s *KitsService) MarkCollected(ctx context.Context, orgIDOrSlug, eventIDOrSlug, collectionID string) (KitCollection, error) {
	return Do[KitCollection](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kit-collections/"+Seg(collectionID)+"/collect", RequestOptions{Method: http.MethodPost})
}

// Void cancels a PENDING or COLLECTED handout (frees re-issue).
func (s *KitsService) Void(ctx context.Context, orgIDOrSlug, eventIDOrSlug, collectionID string) (KitCollection, error) {
	return Do[KitCollection](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kit-collections/"+Seg(collectionID)+"/void", RequestOptions{Method: http.MethodPost})
}

// ListCollections lists handouts, filterable by kit/status/attendee.
func (s *KitsService) ListCollections(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, f *CollectionFilter) (CollectionList, error) {
	var query map[string]string
	if f != nil {
		query = map[string]string{}
		if f.KitID != "" {
			query["kitId"] = f.KitID
		}
		if f.Status != "" {
			query["status"] = string(f.Status)
		}
		if f.AttendeeID != "" {
			query["attendeeId"] = f.AttendeeID
		}
	}
	return Do[CollectionList](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kit-collections", RequestOptions{Method: http.MethodGet, Query: query})
}

// ListKitCollections lists handouts for one kit, with optional status filter.
func (s *KitsService) ListKitCollections(ctx context.Context, orgIDOrSlug, eventIDOrSlug, kitID string, status CollectionStatus) (CollectionList, error) {
	var query map[string]string
	if status != "" {
		query = map[string]string{"status": string(status)}
	}
	return Do[CollectionList](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/kits/"+Seg(kitID)+"/collections", RequestOptions{Method: http.MethodGet, Query: query})
}
