# rallya-go-sdk

Go SDK for the Rallya event platform API (`/api/v1`). Port of `@rallya/sdk` (TypeScript).

- Zero third-party runtime dependencies (stdlib only).
- Typed resources over every API domain: auth, orgs, events, tickets, orders, payments, attendees, check-in, audit, admin, health.
- Automatic `401 → refresh → retry` (single-flight) for user sessions.
- API-key mode (`X-API-Key`) for server-to-server callers.
- Stripe webhook signature verifier (`webhooks` package, stdlib `crypto/hmac`).

## Install

```bash
go get github.com/nikhea/rallya-go-sdk
```

```go
import rallya "github.com/nikhea/rallya-go-sdk"
import "github.com/nikhea/rallya-go-sdk/webhooks"
```

## Auth: user sessions

Short-lived access JWT + rotating refresh token, managed through callbacks
you provide (memory, file, secure store — your choice).

```go
store := rallya.NewMemoryTokenStore(nil)
client, err := rallya.NewClient(rallya.Options{
    BaseURL:   "http://localhost:8080/api/v1",
    GetTokens: store.GetTokens,
    SetTokens: store.SetTokens,
    OnAuthFailure: func() { /* refresh rejected: signed out */ },
})

pair, err := client.Auth.Login(ctx, rallya.LoginInput{Email: "j@test.com", Password: "secret123"})
_ = pair
me, err := client.Auth.Me(ctx)
_, err = client.Auth.Logout(ctx)
store.SetTokens(nil)
```

`BaseURL` may include a trailing slash; org/event IDs accept UUID or slug
(`rallya.Seg` encodes segments for you).

Auth methods (`client.Auth`): `Register`, `Login`, `Refresh` (manual;
auto-refresh already happens on 401), `VerifyEmail` / `VerifyEmailLink`,
`VerifyCode` / `ResendVerification`, `ForgotPassword` (always 200),
`ResetPassword`, `Logout`, `Me`.

## Auth: API keys (server-to-server)

Keys are per-organization, long-lived, revocable, sent as `X-API-Key`
(no refresh cycle, no token store). Pass exactly one of API key / token
store — both together is a construction error.

```go
client, err := rallya.NewClient(rallya.Options{
    BaseURL: "http://localhost:8080/api/v1",
    APIKey:  os.Getenv("RALLYA_API_KEY"), // rk_live_* ...
})
// Or dynamic: APIKeyFunc: func() string { return vault.Read("rallya/api-key") }

res, err := client.Checkin.Scan(ctx, "acme", "fest-2026", rallya.ScanInput{Code: &code})
```

## Resources

```go
// Orgs
org, err := client.Orgs.Create(ctx, rallya.CreateOrgInput{Name: "Acme Inc", Slug: "acme"})
mine, err := client.Orgs.ListMine(ctx)
err = client.Orgs.Remove(ctx, "acme")

// Events (public discovery sends no auth header)
page, err := client.Events.ListPublic(ctx, &rallya.EventFilter{Q: "jazz", Status: rallya.EventStatusPublished})
ev, err := client.Events.Create(ctx, "acme", rallya.CreateEventInput{Title: "Fest"})
ev, err = client.Events.Publish(ctx, "acme", "fest-2026")
_, err = client.Events.UploadCover(ctx, "acme", "fest-2026", pngBytes, "cover.png", "image/png") // ≤ 5 MB

// Tickets / Orders / Payments
tt, err := client.Tickets.Create(ctx, "acme", "fest-2026", rallya.CreateTicketInput{Name: "GA", PriceCents: 2000, QuantityTotal: 100})
order, err := client.Orders.Create(ctx, "fest-2026", rallya.CreateOrderInput{TicketTypeID: "tick_123", Quantity: 2}) // idempotency key auto-generated
checkout, err := client.Payments.Checkout(ctx, order.ID) // Stripe redirect URL; 503 when unconfigured

// Attendees / Check-in (refusals are 200 + outcome, never errors)
batch, err := client.Checkin.ScanBatch(ctx, "acme", "fest-2026", codes) // ≤ 50 codes
stats, err := client.Checkin.Stats(ctx, "acme", "fest-2026")

// Kits (named kit types + per-attendee collections; CHECKED_IN required)
kit, err := client.Kits.CreateKit(ctx, "acme", "fest-2026", rallya.CreateKitInput{Name: "VIP pack", QuantityTotal: 100})
collected, err := client.Kits.Collect(ctx, "acme", "fest-2026", kit.ID,
    rallya.CollectKitInput{AttendeeID: attendeeID, IdempotencyKey: "kit-req-001"}) // Reserve: true holds PENDING
picked, err := client.Kits.MarkCollected(ctx, "acme", "fest-2026", collected.ID)   // PENDING → COLLECTED
_, err = client.Kits.Void(ctx, "acme", "fest-2026", collected.ID)                  // frees re-issue

// Subscriptions (org tiers FREE/PRO/SCALE; attendee checkout stays one-off)
tiers, err := client.Subscriptions.ListPlans(ctx) // public catalog with limits + prices
sub, err := client.Subscriptions.Get(ctx, "acme") // absent subscription reads FREE
checkout, err := client.Subscriptions.Checkout(ctx, "acme", rallya.SubscriptionPlanPro) // OWNER
portal, err := client.Subscriptions.Portal(ctx, "acme") // OWNER self-serve manage/cancel

// Audit / Admin / Health
audit, err := client.Audit.ListOrg(ctx, "acme", nil)
orgs, err := client.Admin.ListOrgs(ctx, nil)
live, err := client.Health.Live(ctx) // GET <origin>/health (outside /api/v1)
hello, err := client.Health.Hello(ctx)
```

List routes take `*rallya.PageQuery` (server defaults 1/20, max 100) and
return `rallya.Page[T]` (`{Items, Total, Page, PerPage}`).

Custom transport (tests, proxies, tracing):

```go
client, err := rallya.NewClient(rallya.Options{BaseURL: u, APIKey: k,
    HTTPClient: &http.Client{Transport: myRoundTripper}})
```

Low-level typed requests remain available: `rallya.Do[T](ctx, client, "orgs/acme/api-keys", rallya.RequestOptions{...})`.

## Errors

Every API failure returns `*rallya.RallyaError` (`Status`, `Code`,
`Message`, `Details`, `RetryAfterMs`). Stealth `404`s mean "not found
**or** no access" — use the helper:

```go
_, err = client.Orgs.Get(ctx, "acme")
if re, ok := err.(*rallya.RallyaError); ok && re.IsRateLimited() {
    time.Sleep(time.Duration(*re.RetryAfterMs) * time.Millisecond) // 429 carries Retry-After
}
if rallya.IsNotFoundOrForbidden(err) { /* missing or no access */ }
```

## Stripe webhooks

Verify signatures over the **raw** request body (handlers must not
JSON-parse first):

```go
res := webhooks.VerifySignature(rawBody, r.Header.Get("Stripe-Signature"), os.Getenv("STRIPE_WEBHOOK_SIGNING_SECRET"))
if !res.Verified { /* 400 */ }
_ = res.EventID
```

## Development

```bash
gofmt -l . && go vet ./... && go build ./... && go test ./... # gates
go test -race ./... # full suite is race-clean
```
