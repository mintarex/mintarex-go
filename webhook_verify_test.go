package mintarex

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const webhookTestSecret = "mtxhook_test_fixture_key_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func signWebhookBody(ts, body, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(ts + "." + body))
	return "v1=" + hex.EncodeToString(h.Sum(nil))
}

func realHeaders(ts, sig string) http.Header {
	h := http.Header{}
	h.Set("X-Mintarex-Signature", sig)
	h.Set("X-Mintarex-Timestamp", ts)
	h.Set("X-Mintarex-Event-Type", "trade.executed")
	h.Set("X-Mintarex-Event-Id", "evt_abc")
	h.Set("X-Mintarex-Delivery-Id", "dlv_xyz")
	return h
}

func assertWebhookError(t *testing.T, err error, wantSub string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected WebhookSignatureError containing %q, got nil", wantSub)
		return
	}
	var we *WebhookSignatureError
	if !errors.As(err, &we) {
		t.Errorf("expected *WebhookSignatureError, got %T: %v", err, err)
		return
	}
	if wantSub != "" && !contains(we.Error(), wantSub) {
		t.Errorf("error %q missing substring %q", we.Error(), wantSub)
	}
}

func TestVerifyWebhookAcceptsValidSignatureAndReadsMetadata(t *testing.T) {
	body := []byte(`{"timestamp":"2026-01-01T00:00:00Z","trade_id":"t_123","base":"BTC","quote":"USD"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	ev, err := VerifyWebhook(VerifyParams{
		Body:    body,
		Headers: realHeaders(ts, signWebhookBody(ts, string(body), webhookTestSecret)),
		Secret:  webhookTestSecret,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ev.EventType != "trade.executed" || ev.EventID != "evt_abc" || ev.DeliveryUUID != "dlv_xyz" {
		t.Errorf("header metadata not parsed: %+v", ev)
	}
	if ev.Timestamp != "2026-01-01T00:00:00Z" {
		t.Errorf("body timestamp not lifted: %q", ev.Timestamp)
	}
	if ev.Sandbox {
		t.Error("sandbox should be false")
	}
	if ev.Data["trade_id"] != "t_123" {
		t.Errorf("data missing trade_id: %v", ev.Data)
	}
}

func TestVerifyWebhookSurfacesSandboxFlag(t *testing.T) {
	body := []byte(`{"timestamp":"2026-01-01T00:00:00Z","sandbox":true,"trade_id":"t_999"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	ev, err := VerifyWebhook(VerifyParams{
		Body:    body,
		Headers: realHeaders(ts, signWebhookBody(ts, string(body), webhookTestSecret)),
		Secret:  webhookTestSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ev.Sandbox {
		t.Error("expected sandbox=true")
	}
}

func TestVerifyWebhookRejectsTamperedBody(t *testing.T) {
	body := `{"timestamp":"2026-01-01T00:00:00Z","trade_id":"t_1"}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	tampered := []byte(`{"timestamp":"2026-01-01T00:00:00Z","trade_id":"t_2"}`)
	_, err := VerifyWebhook(VerifyParams{
		Body: tampered, Headers: realHeaders(ts, sig), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "signature mismatch")
}

func TestVerifyWebhookRejectsWrongSecret(t *testing.T) {
	body := `{"timestamp":"t","trade_id":"x"}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signWebhookBody(ts, body, "mtxhook_test_other_key_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, sig), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "signature mismatch")
}

func TestVerifyWebhookRejectsStaleTimestamp(t *testing.T) {
	body := `{"timestamp":"t"}`
	ts := strconv.FormatInt(time.Now().Unix()-600, 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, sig), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "tolerance")
}

func TestVerifyWebhookRejectsFutureTimestamp(t *testing.T) {
	body := `{"timestamp":"t"}`
	ts := strconv.FormatInt(time.Now().Unix()+600, 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, sig), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "tolerance")
}

func TestVerifyWebhookRespectsCustomTolerance(t *testing.T) {
	body := `{"timestamp":"t"}`
	ts := strconv.FormatInt(time.Now().Unix()-900, 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	ev, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, sig),
		Secret: webhookTestSecret, ToleranceSeconds: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ev.EventType != "trade.executed" {
		t.Errorf("bad event type: %q", ev.EventType)
	}
}

func TestVerifyWebhookRejectsMissingHeaders(t *testing.T) {
	body := "{}"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	required := []string{
		"X-Mintarex-Signature",
		"X-Mintarex-Timestamp",
		"X-Mintarex-Event-Type",
		"X-Mintarex-Event-Id",
		"X-Mintarex-Delivery-Id",
	}
	for _, drop := range required {
		h := realHeaders(ts, sig)
		h.Del(drop)
		_, err := VerifyWebhook(VerifyParams{
			Body: []byte(body), Headers: h, Secret: webhookTestSecret,
		})
		assertWebhookError(t, err, "missing "+drop)
	}
}

func TestVerifyWebhookRejectsSigWithoutV1Prefix(t *testing.T) {
	body := "{}"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	badSig := ""
	for range 64 {
		badSig += "a"
	}
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, badSig), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "v1=")
}

func TestVerifyWebhookRejectsNonHexSignature(t *testing.T) {
	body := "{}"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonHex := "v1="
	for range 64 {
		nonHex += "z"
	}
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, nonHex), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "not a 64-char hex")
}

func TestVerifyWebhookRejectsInvalidJSONBody(t *testing.T) {
	body := "not json"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, sig), Secret: webhookTestSecret,
	})
	assertWebhookError(t, err, "not valid JSON")
}

func TestVerifyWebhookRejectsArrayBody(t *testing.T) {
	body := "[1,2,3]"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signWebhookBody(ts, body, webhookTestSecret)
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, sig), Secret: webhookTestSecret,
	})
	if err == nil {
		t.Fatal("expected error for array body")
	}
}

func TestVerifyWebhookRejectsEmptySecret(t *testing.T) {
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte("{}"), Headers: http.Header{}, Secret: "",
	})
	assertWebhookError(t, err, "secret is required")
}

func TestVerifyWebhookConstantTimeAgainstShorterSignature(t *testing.T) {
	body := "{}"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	_, err := VerifyWebhook(VerifyParams{
		Body: []byte(body), Headers: realHeaders(ts, "v1=abc"),
		Secret: webhookTestSecret,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
