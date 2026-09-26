package rallya

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

func pageQuery(q PageQuery) map[string]string {
	m := map[string]string{}
	if q.Page > 0 {
		m["page"] = strconv.Itoa(q.Page)
	}
	if q.PerPage > 0 {
		m["perPage"] = strconv.Itoa(q.PerPage)
	}
	return m
}

func eventFilterQuery(f *EventFilter) map[string]string {
	m := map[string]string{}
	if f == nil {
		return m
	}
	for k, v := range pageQuery(f.PageQuery) {
		m[k] = v
	}
	if f.Status != "" {
		m["status"] = string(f.Status)
	}
	if f.From != "" {
		m["from"] = f.From
	}
	if f.To != "" {
		m["to"] = f.To
	}
	if f.Q != "" {
		m["q"] = f.Q
	}
	if f.Sort != "" {
		m["sort"] = f.Sort
	}
	return m
}

func auditQuery(q *AuditQuery) map[string]string {
	m := map[string]string{}
	if q == nil {
		return m
	}
	for k, v := range pageQuery(q.PageQuery) {
		m[k] = v
	}
	if q.Action != "" {
		m["action"] = q.Action
	}
	if q.ObjectType != "" {
		m["objectType"] = q.ObjectType
	}
	if q.ObjectID != "" {
		m["objectId"] = q.ObjectID
	}
	if q.Actor != "" {
		m["actor"] = q.Actor
	}
	if q.Org != "" {
		m["org"] = q.Org
	}
	if q.Since != "" {
		m["since"] = q.Since
	}
	if q.Until != "" {
		m["until"] = q.Until
	}
	return m
}

// --- Auth ---

type AuthService struct{ client *Client }

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	return Do[TokenPair](ctx, s.client, "auth/register", RequestOptions{Method: http.MethodPost, Body: in, NoAuth: true})
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (TokenPair, error) {
	return Do[TokenPair](ctx, s.client, "auth/login", RequestOptions{Method: http.MethodPost, Body: in, NoAuth: true})
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	return Do[TokenPair](ctx, s.client, "auth/refresh", RequestOptions{
		Method: http.MethodPost, Body: map[string]string{"refreshToken": refreshToken}, NoAuth: true, NoRetry: true,
	})
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "auth/verify-email", RequestOptions{Method: http.MethodPost, Body: map[string]string{"token": token}, NoAuth: true})
}

func (s *AuthService) VerifyEmailLink(ctx context.Context, token string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "auth/verify-email", RequestOptions{Method: http.MethodGet, Query: map[string]string{"token": token}, NoAuth: true})
}

func (s *AuthService) VerifyCode(ctx context.Context, email, code string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "auth/verify-code", RequestOptions{Method: http.MethodPost, Body: map[string]string{"email": email, "code": code}, NoAuth: true})
}

func (s *AuthService) ResendVerification(ctx context.Context, email string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "auth/resend-verification", RequestOptions{Method: http.MethodPost, Body: map[string]string{"email": email}, NoAuth: true})
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) (MessageResponse, error) {
	// Always 200 (no account enumeration).
	return Do[MessageResponse](ctx, s.client, "auth/forgot-password", RequestOptions{Method: http.MethodPost, Body: map[string]string{"email": email}, NoAuth: true})
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "auth/reset-password", RequestOptions{Method: http.MethodPost, Body: map[string]string{"token": token, "newPassword": newPassword}, NoAuth: true})
}

func (s *AuthService) Logout(ctx context.Context) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "auth/logout", RequestOptions{Method: http.MethodPost})
}

func (s *AuthService) Me(ctx context.Context) (Me, error) {
	return Do[Me](ctx, s.client, "auth/me", RequestOptions{Method: http.MethodGet})
}

// --- Orgs ---

type OrgsService struct{ client *Client }

func (s *OrgsService) Create(ctx context.Context, in CreateOrgInput) (Org, error) {
	return Do[Org](ctx, s.client, "orgs", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *OrgsService) ListMine(ctx context.Context) ([]Org, error) {
	out, err := Do[[]Org](ctx, s.client, "orgs", RequestOptions{Method: http.MethodGet})
	if out == nil && err == nil {
		return []Org{}, nil
	}
	return out, err
}

func (s *OrgsService) Get(ctx context.Context, idOrSlug string) (Org, error) {
	return Do[Org](ctx, s.client, "orgs/"+Seg(idOrSlug), RequestOptions{Method: http.MethodGet})
}

func (s *OrgsService) Update(ctx context.Context, idOrSlug string, in UpdateOrgInput) (Org, error) {
	return Do[Org](ctx, s.client, "orgs/"+Seg(idOrSlug), RequestOptions{Method: http.MethodPatch, Body: in})
}

func (s *OrgsService) Remove(ctx context.Context, idOrSlug string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug), RequestOptions{Method: http.MethodDelete})
	return err
}

func (s *OrgsService) ListMembers(ctx context.Context, idOrSlug string, q *PageQuery) (Page[OrgMember], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	return Do[Page[OrgMember]](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/members", RequestOptions{Method: http.MethodGet, Query: query})
}

type AddMemberInput struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

func (s *OrgsService) AddMember(ctx context.Context, idOrSlug string, in AddMemberInput) (OrgMember, error) {
	return Do[OrgMember](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/members", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *OrgsService) UpdateMemberRole(ctx context.Context, idOrSlug, userID, role string) (OrgMember, error) {
	return Do[OrgMember](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/members/"+Seg(userID), RequestOptions{Method: http.MethodPatch, Body: map[string]string{"role": role}})
}

func (s *OrgsService) RemoveMember(ctx context.Context, idOrSlug, userID string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/members/"+Seg(userID), RequestOptions{Method: http.MethodDelete})
	return err
}

func (s *OrgsService) UpdateMyPreferences(ctx context.Context, idOrSlug string, notifyEvents *bool) error {
	body := map[string]any{}
	if notifyEvents != nil {
		body["notifyEvents"] = *notifyEvents
	}
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/members/me", RequestOptions{Method: http.MethodPatch, Body: body})
	return err
}

type InviteInput struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

func (s *OrgsService) Invite(ctx context.Context, idOrSlug string, in InviteInput) (OrgInvite, error) {
	return Do[OrgInvite](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/invites", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *OrgsService) ListInvites(ctx context.Context, idOrSlug string, q *PageQuery) (Page[OrgInvite], error) {
	var query map[string]string
	if q != nil {
		query = pageQuery(*q)
	}
	return Do[Page[OrgInvite]](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/invites", RequestOptions{Method: http.MethodGet, Query: query})
}

func (s *OrgsService) RevokeInvite(ctx context.Context, idOrSlug, inviteID string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/invites/"+Seg(inviteID), RequestOptions{Method: http.MethodDelete})
	return err
}

func (s *OrgsService) AcceptInvite(ctx context.Context, token string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "orgs/invites/accept", RequestOptions{Method: http.MethodPost, Body: map[string]string{"token": token}})
}

func (s *OrgsService) DeclineInvite(ctx context.Context, token string) (MessageResponse, error) {
	return Do[MessageResponse](ctx, s.client, "orgs/invites/decline", RequestOptions{Method: http.MethodPost, Body: map[string]string{"token": token}})
}

func (s *OrgsService) ListRoles(ctx context.Context, idOrSlug string) ([]CustomRole, error) {
	out, err := Do[[]CustomRole](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/roles", RequestOptions{Method: http.MethodGet})
	if out == nil && err == nil {
		return []CustomRole{}, nil
	}
	return out, err
}

type DefineRoleInput struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}

func (s *OrgsService) DefineRole(ctx context.Context, idOrSlug string, in DefineRoleInput) (CustomRole, error) {
	return Do[CustomRole](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/roles", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *OrgsService) UpdateRole(ctx context.Context, idOrSlug, role string, permissions []Permission) (CustomRole, error) {
	return Do[CustomRole](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/roles/"+Seg(role), RequestOptions{Method: http.MethodPatch, Body: map[string]any{"permissions": permissions}})
}

func (s *OrgsService) DeleteRole(ctx context.Context, idOrSlug, role string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/roles/"+Seg(role), RequestOptions{Method: http.MethodDelete})
	return err
}

func (s *OrgsService) AssignRole(ctx context.Context, idOrSlug, role, userID string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/roles/"+Seg(role)+"/assign", RequestOptions{Method: http.MethodPost, Body: map[string]string{"userId": userID}})
	return err
}

func (s *OrgsService) UnassignRole(ctx context.Context, idOrSlug, role, userID string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(idOrSlug)+"/roles/"+Seg(role)+"/unassign", RequestOptions{Method: http.MethodPost, Body: map[string]string{"userId": userID}})
	return err
}

// --- Events ---

type EventsService struct{ client *Client }

const maxCoverBytes = 5 * 1024 * 1024

// ListPublic is public discovery (published only, no auth sent).
func (s *EventsService) ListPublic(ctx context.Context, f *EventFilter) (Page[RallyaEvent], error) {
	return Do[Page[RallyaEvent]](ctx, s.client, "events", RequestOptions{Method: http.MethodGet, Query: eventFilterQuery(f), NoAuth: true})
}

func (s *EventsService) GetPublic(ctx context.Context, idOrSlug string) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "events/"+Seg(idOrSlug), RequestOptions{Method: http.MethodGet, NoAuth: true})
}

func (s *EventsService) ListOrg(ctx context.Context, orgIDOrSlug string, f *EventFilter) (Page[RallyaEvent], error) {
	return Do[Page[RallyaEvent]](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events", RequestOptions{Method: http.MethodGet, Query: eventFilterQuery(f)})
}

func (s *EventsService) Create(ctx context.Context, orgIDOrSlug string, in CreateEventInput) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *EventsService) Get(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug), RequestOptions{Method: http.MethodGet})
}

func (s *EventsService) Update(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, in UpdateEventInput) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug), RequestOptions{Method: http.MethodPatch, Body: in})
}

func (s *EventsService) Remove(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug), RequestOptions{Method: http.MethodDelete})
	return err
}

func (s *EventsService) Publish(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/publish", RequestOptions{Method: http.MethodPost})
}

func (s *EventsService) Unpublish(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/unpublish", RequestOptions{Method: http.MethodPost})
}

func (s *EventsService) Cancel(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) (RallyaEvent, error) {
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/cancel", RequestOptions{Method: http.MethodPost})
}

func (s *EventsService) UploadCover(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, content []byte, filename, contentType string) (RallyaEvent, error) {
	if len(content) > maxCoverBytes {
		return RallyaEvent{}, fmt.Errorf("file exceeds 5 MB cover limit (%d bytes)", len(content))
	}
	return Do[RallyaEvent](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/cover", RequestOptions{
		Method: http.MethodPost, Upload: &Upload{Field: "cover", Filename: filename, Content: content, ContentType: contentType},
	})
}

func (s *EventsService) UploadImages(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, content []byte, filename, contentType string) (EventImage, error) {
	if len(content) > maxCoverBytes {
		return EventImage{}, fmt.Errorf("file exceeds 5 MB cover limit (%d bytes)", len(content))
	}
	return Do[EventImage](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/images", RequestOptions{
		Method: http.MethodPost, Upload: &Upload{Field: "images", Filename: filename, Content: content, ContentType: contentType},
	})
}

func (s *EventsService) ListImages(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) (Page[EventImage], error) {
	return Do[Page[EventImage]](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/images", RequestOptions{Method: http.MethodGet})
}

// --- Tickets ---

type TicketsService struct{ client *Client }

func (s *TicketsService) ListPublic(ctx context.Context, eventIDOrSlug string) ([]TicketType, error) {
	out, err := Do[[]TicketType](ctx, s.client, "events/"+Seg(eventIDOrSlug)+"/tickets", RequestOptions{Method: http.MethodGet, NoAuth: true})
	if out == nil && err == nil {
		return []TicketType{}, nil
	}
	return out, err
}

func (s *TicketsService) List(ctx context.Context, orgIDOrSlug, eventIDOrSlug string) ([]TicketType, error) {
	out, err := Do[[]TicketType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets", RequestOptions{Method: http.MethodGet})
	if out == nil && err == nil {
		return []TicketType{}, nil
	}
	return out, err
}

func (s *TicketsService) Create(ctx context.Context, orgIDOrSlug, eventIDOrSlug string, in CreateTicketInput) (TicketType, error) {
	return Do[TicketType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets", RequestOptions{Method: http.MethodPost, Body: in})
}

func (s *TicketsService) Get(ctx context.Context, orgIDOrSlug, eventIDOrSlug, ticketID string) (TicketType, error) {
	return Do[TicketType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets/"+Seg(ticketID), RequestOptions{Method: http.MethodGet})
}

func (s *TicketsService) Update(ctx context.Context, orgIDOrSlug, eventIDOrSlug, ticketID string, in UpdateTicketInput) (TicketType, error) {
	return Do[TicketType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets/"+Seg(ticketID), RequestOptions{Method: http.MethodPatch, Body: in})
}

func (s *TicketsService) Remove(ctx context.Context, orgIDOrSlug, eventIDOrSlug, ticketID string) error {
	_, err := Do[any](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets/"+Seg(ticketID), RequestOptions{Method: http.MethodDelete})
	return err
}

func (s *TicketsService) Activate(ctx context.Context, orgIDOrSlug, eventIDOrSlug, ticketID string) (TicketType, error) {
	return Do[TicketType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets/"+Seg(ticketID)+"/activate", RequestOptions{Method: http.MethodPost})
}

func (s *TicketsService) Pause(ctx context.Context, orgIDOrSlug, eventIDOrSlug, ticketID string) (TicketType, error) {
	return Do[TicketType](ctx, s.client, "orgs/"+Seg(orgIDOrSlug)+"/events/"+Seg(eventIDOrSlug)+"/tickets/"+Seg(ticketID)+"/pause", RequestOptions{Method: http.MethodPost})
}
