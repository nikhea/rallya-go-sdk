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
```

## Quick start (user session)

```go
store := rallya.NewMemoryTokenStore(nil)
client, err := rallya.NewClient(rallya.Options{
    BaseURL: "http://localhost:8080/api/v1",
    GetTokens: store.GetTokens,
    SetTokens: store.SetTokens,
})
if err != nil { /* ... */ }

pair, err := client.Auth.Login(ctx, rallya.LoginInput{Email: "j@test.com", Password: "secret123"})
_ = pair
me, err := client.Auth.Me(ctx)
_ = me
```

## API key mode

```go
client, err := rallya.NewClient(rallya.Options{
    BaseURL: "http://localhost:8080/api/v1",
    APIKey: "rk_live_...",
})
res, err := client.Checkin.Scan(ctx, "acme", "fest-2026", rallya.ScanInput{Code: ptr("qr-payload")})
```

See `example_test.go` and the TypeScript README for full semantics.
