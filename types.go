package rallya

// Hand-friendly types mapped 1:1 from the Go DTOs of the Rallya server
// and from src/types.ts of @rallya/sdk.

type MemberRole string

const (
	MemberRoleOwner  MemberRole = "OWNER"
	MemberRoleAdmin  MemberRole = "ADMIN"
	MemberRoleMember MemberRole = "MEMBER"
)

type EventStatus string

const (
	EventStatusDraft     EventStatus = "DRAFT"
	EventStatusPublished EventStatus = "PUBLISHED"
	EventStatusCancelled EventStatus = "CANCELLED"
)

type TicketStatus string

const (
	TicketStatusDraft  TicketStatus = "DRAFT"
	TicketStatusActive TicketStatus = "ACTIVE"
	TicketStatusPaused TicketStatus = "PAUSED"
)

type OrderStatus string

const (
	OrderStatusPending        OrderStatus = "PENDING"
	OrderStatusPendingPayment OrderStatus = "PENDING_PAYMENT"
	OrderStatusConfirmed      OrderStatus = "CONFIRMED"
	OrderStatusCancelled      OrderStatus = "CANCELLED"
	OrderStatusExpired        OrderStatus = "EXPIRED"
)

type AttendeeStatus string

const (
	AttendeeStatusRegistered AttendeeStatus = "REGISTERED"
	AttendeeStatusCheckedIn  AttendeeStatus = "CHECKED_IN"
	AttendeeStatusCancelled  AttendeeStatus = "CANCELLED"
)

type CheckinOutcome string

const (
	CheckinOutcomeCheckedIn        CheckinOutcome = "CHECKED_IN"
	CheckinOutcomeAlreadyCheckedIn CheckinOutcome = "ALREADY_CHECKED_IN"
	CheckinOutcomeInvalidCode      CheckinOutcome = "INVALID_CODE"
	CheckinOutcomeCancelled        CheckinOutcome = "CANCELLED"
	CheckinOutcomeWrongEvent       CheckinOutcome = "WRONG_EVENT"
	CheckinOutcomeReverted         CheckinOutcome = "REVERTED"
)

type CheckinMethod string

const (
	CheckinMethodQR     CheckinMethod = "qr"
	CheckinMethodManual CheckinMethod = "manual"
)

// TokenPair is a short-lived access JWT plus its rotating refresh token.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// PageQuery is pagination input. Server defaults page=1 perPage=20 max 100.
type PageQuery struct {
	Page    int
	PerPage int
}

// Page is a normalized paginated response.
type Page[T any] struct {
	Items   []T `json:"items"`
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
}

// --- Auth ---

type RegisterInput struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OrgMembership struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Joined string `json:"joinedAt,omitempty"`
}

type Me struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	Profile       struct {
		FirstName string `json:"firstName,omitempty"`
		LastName  string `json:"lastName,omitempty"`
	} `json:"profile"`
	Organizations []OrgMembership `json:"organizations,omitempty"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

// --- Organizations ---

type CreateOrgInput struct {
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
	Logo string `json:"logo,omitempty"`
}

type UpdateOrgInput struct {
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
	Logo string `json:"logo,omitempty"`
}

type Org struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	LogoURL   string `json:"logoUrl,omitempty"`
	Role      string `json:"role,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type OrgMember struct {
	UserID        string `json:"userId"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"emailVerified"`
	Role          string `json:"role"`
	JoinedAt      string `json:"joinedAt"`
}

type OrgInvite struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

type Permission struct {
	Object string `json:"object"`
	Action string `json:"action"`
}

type CustomRole struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
	Holders     int          `json:"holders"`
	CreatedAt   string       `json:"createdAt"`
}

// --- Events ---

type CreateEventInput struct {
	Title       string `json:"title"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Venue       string `json:"venue,omitempty"`
	Location    string `json:"location,omitempty"`
	StartsAt    string `json:"startsAt,omitempty"`
	EndsAt      string `json:"endsAt,omitempty"`
	Capacity    *int   `json:"capacity,omitempty"`
}

type UpdateEventInput struct {
	Title       *string `json:"title,omitempty"`
	Slug        *string `json:"slug,omitempty"`
	Description *string `json:"description,omitempty"`
	Venue       *string `json:"venue,omitempty"`
	Location    *string `json:"location,omitempty"`
	StartsAt    *string `json:"startsAt,omitempty"`
	EndsAt      *string `json:"endsAt,omitempty"`
	Capacity    *int    `json:"capacity,omitempty"`
	ClearCover  *bool   `json:"clearCover,omitempty"`
}

type EventFilter struct {
	PageQuery
	Status EventStatus `json:"-"`
	From   string      `json:"-"`
	To     string      `json:"-"`
	Q      string      `json:"-"`
	Sort   string      `json:"-"`
}

type RallyaEvent struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Description string      `json:"description,omitempty"`
	Venue       string      `json:"venue,omitempty"`
	Location    string      `json:"location,omitempty"`
	StartsAt    string      `json:"startsAt,omitempty"`
	EndsAt      string      `json:"endsAt,omitempty"`
	Capacity    *int        `json:"capacity,omitempty"`
	CoverURL    string      `json:"coverUrl,omitempty"`
	Status      EventStatus `json:"status"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
}

type EventImage struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	PublicID  string `json:"publicId"`
	Format    string `json:"format,omitempty"`
	Bytes     int64  `json:"bytes"`
	Width     *int   `json:"width,omitempty"`
	Height    *int   `json:"height,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// --- Tickets ---

type CreateTicketInput struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	PriceCents    int    `json:"priceCents"`
	Currency      string `json:"currency,omitempty"`
	QuantityTotal int    `json:"quantityTotal"`
	MaxPerOrder   *int   `json:"maxPerOrder,omitempty"`
	SaleStartsAt  string `json:"saleStartsAt,omitempty"`
	SaleEndsAt    string `json:"saleEndsAt,omitempty"`
}

type UpdateTicketInput struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	PriceCents    *int    `json:"priceCents,omitempty"`
	Currency      *string `json:"currency,omitempty"`
	QuantityTotal *int    `json:"quantityTotal,omitempty"`
	MaxPerOrder   *int    `json:"maxPerOrder,omitempty"`
	SaleStartsAt  *string `json:"saleStartsAt,omitempty"`
	SaleEndsAt    *string `json:"saleEndsAt,omitempty"`
}

type TicketType struct {
	ID               string       `json:"id"`
	EventID          string       `json:"eventId"`
	Name             string       `json:"name"`
	Description      string       `json:"description,omitempty"`
	PriceCents       int          `json:"priceCents"`
	Currency         string       `json:"currency"`
	QuantityTotal    int          `json:"quantityTotal"`
	QuantitySold     int          `json:"quantitySold"`
	Remaining        int          `json:"remaining"`
	ForSale          bool         `json:"forSale"`
	UnavailableReason string      `json:"unavailableReason,omitempty"`
	MaxPerOrder      *int         `json:"maxPerOrder,omitempty"`
	SaleStartsAt     string       `json:"saleStartsAt,omitempty"`
	SaleEndsAt       string       `json:"saleEndsAt,omitempty"`
	Status           TicketStatus `json:"status"`
	SoldOut          bool         `json:"soldOut"`
	CreatedAt        string       `json:"createdAt"`
}

// --- Orders ---

type CreateOrderInput struct {
	TicketTypeID   string `json:"ticketTypeId"`
	Quantity       int    `json:"quantity"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type Order struct {
	ID           string      `json:"id"`
	EventID      string      `json:"eventId"`
	TicketTypeID string      `json:"ticketTypeId"`
	Quantity     int         `json:"quantity"`
	PriceCents   int         `json:"priceCents"`
	Currency     string      `json:"currency"`
	Status       OrderStatus `json:"status"`
	ExpiresAt    string      `json:"expiresAt,omitempty"`
	CreatedAt    string      `json:"createdAt"`
}

type CheckoutResponse struct {
	URL       string `json:"url"`
	SessionID string `json:"sessionId"`
}

// --- Attendees ---

type Attendee struct {
	ID          string         `json:"id"`
	OrderID     string         `json:"orderId,omitempty"`
	UnitIndex   int            `json:"unitIndex"`
	EventID     string         `json:"eventId"`
	UserID      string         `json:"userId,omitempty"`
	Email       string         `json:"email"`
	Name        string         `json:"name,omitempty"`
	Status      AttendeeStatus `json:"status"`
	QRPayload   string         `json:"qrPayload,omitempty"`
	CheckedInAt string         `json:"checkedInAt,omitempty"`
	CreatedAt   string         `json:"createdAt"`
}

// --- Check-in ---

type ScanInput struct {
	Code       *string `json:"code,omitempty"`
	AttendeeID *string `json:"attendeeId,omitempty"`
}

type ScanResult struct {
	Outcome     CheckinOutcome `json:"outcome"`
	Method      CheckinMethod  `json:"method"`
	AttendeeID  string         `json:"attendeeId,omitempty"`
	CheckedInAt string         `json:"checkedInAt,omitempty"`
}

type ScanBatchResponse struct {
	Results []ScanResult `json:"results"`
}

type CheckinStats struct {
	Registered int `json:"registered"`
	CheckedIn  int `json:"checkedIn"`
	Cancelled  int `json:"cancelled"`
	Total      int `json:"total"`
}

// --- Audit / Admin ---

type AuditEvent struct {
	ID         string `json:"id"`
	OrgID      string `json:"orgId,omitempty"`
	ActorID    string `json:"actorId,omitempty"`
	Action     string `json:"action"`
	ObjectType string `json:"objectType"`
	ObjectID   string `json:"objectId,omitempty"`
	Before     string `json:"before,omitempty"`
	After      string `json:"after,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

type PolicyDiff struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Total   int `json:"total"`
}

type AuditQuery struct {
	PageQuery
	Action     string
	ObjectType string
	ObjectID   string
	Actor      string
	Org        string
	Since      string
	Until      string
}

type AdminOrg struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Members   int    `json:"members"`
	CreatedAt string `json:"createdAt"`
}

type HealthStatus struct {
	Status string `json:"status"`
	App    string `json:"app"`
}
