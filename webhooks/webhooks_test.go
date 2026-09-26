package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestVerifiesWellFormedSignature(t *testing.T) {
	secret := "whsec_test"
	raw, _ := json.Marshal(map[string]string{"id": "evt_1"})
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, raw)))
	sig := hex.EncodeToString(mac.Sum(nil))
	res := VerifySignature(raw, fmt.Sprintf("t=%d,v1=%s", ts, sig), secret)
	if !res.Verified || res.EventID != "evt_1" {
		t.Fatalf("got %+v", res)
	}
}

func TestRejectsTamperedSignature(t *testing.T) {
	res := VerifySignature([]byte("{}"), "t=123,v1=deadbeef", "whsec_test")
	if res.Verified {
		t.Fatal("expected rejection")
	}
}
