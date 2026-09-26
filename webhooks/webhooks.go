// Package webhooks holds server-side Stripe webhook helpers.
// Import it separately so browser/WASM-style consumers only pull the core
// client: import "github.com/nikhea/rallya-go-sdk/webhooks".
package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Result is the outcome of VerifySignature.
type Result struct {
	Verified bool
	EventID  string
}

// VerifySignature verifies a Stripe webhook signature over the RAW request
// body (handlers must not JSON-parse first). Needs the webhook signing secret.
func VerifySignature(rawBody []byte, signatureHeader, webhookSecret string, toleranceSecs ...int) Result {
	tolerance := 300
	if len(toleranceSecs) > 0 {
		tolerance = toleranceSecs[0]
	}
	parts := map[string]string{}
	for _, p := range strings.Split(signatureHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}
	ts, err := strconv.ParseInt(parts["t"], 10, 64)
	if err != nil {
		return Result{}
	}
	expected := parts["v1"]
	if expected == "" {
		return Result{}
	}
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + string(rawBody)))
	digest := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(digest), []byte(expected)) != 1 {
		return Result{}
	}
	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(tolerance) {
		return Result{}
	}
	var evt struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rawBody, &evt); err != nil {
		return Result{Verified: true}
	}
	return Result{Verified: true, EventID: evt.ID}
}
