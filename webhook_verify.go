package mintarex

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultToleranceSeconds is the default allowed clock skew for webhook
// timestamp verification.
const DefaultToleranceSeconds = 300

const (
	signaturePrefix   = "v1="
	expectedSigHexLen = 64
)

// VerifyParams are the inputs to [VerifyWebhook].
type VerifyParams struct {
	// Body is the exact raw request body bytes (NOT parsed JSON).
	Body []byte
	// Headers are the HTTP request headers. Use http.Header directly or build
	// one with http.Header{"X-Mintarex-Signature": {"v1=..."}, ...}.
	Headers http.Header
	// Secret is the endpoint's signing secret (whsec_...).
	Secret string
	// ToleranceSeconds overrides the default ±300s clock skew window.
	// Zero = use [DefaultToleranceSeconds]. Negative = error.
	ToleranceSeconds int
	// Now, if non-zero, is used instead of time.Now() (for tests).
	Now time.Time
}

// VerifyWebhook verifies a webhook signature and returns the parsed event.
// Uses constant-time comparison and rejects stale timestamps.
//
// Returns a [*WebhookSignatureError] on any failure.
func VerifyWebhook(p VerifyParams) (*WebhookEvent, error) {
	if p.Secret == "" {
		return nil, &WebhookSignatureError{Message: "secret is required"}
	}
	if p.ToleranceSeconds < 0 {
		return nil, &WebhookSignatureError{Message: "tolerance_seconds must be >= 0"}
	}
	tolerance := p.ToleranceSeconds
	if tolerance == 0 {
		tolerance = DefaultToleranceSeconds
	}

	sigHeader := p.Headers.Get("X-Mintarex-Signature")
	tsHeader := p.Headers.Get("X-Mintarex-Timestamp")
	eventType := p.Headers.Get("X-Mintarex-Event-Type")
	eventID := p.Headers.Get("X-Mintarex-Event-Id")
	deliveryID := p.Headers.Get("X-Mintarex-Delivery-Id")

	if sigHeader == "" {
		return nil, &WebhookSignatureError{Message: "missing X-Mintarex-Signature header"}
	}
	if tsHeader == "" {
		return nil, &WebhookSignatureError{Message: "missing X-Mintarex-Timestamp header"}
	}
	if eventType == "" {
		return nil, &WebhookSignatureError{Message: "missing X-Mintarex-Event-Type header"}
	}
	if eventID == "" {
		return nil, &WebhookSignatureError{Message: "missing X-Mintarex-Event-Id header"}
	}
	if deliveryID == "" {
		return nil, &WebhookSignatureError{Message: "missing X-Mintarex-Delivery-Id header"}
	}

	sig, err := parseSignature(sigHeader)
	if err != nil {
		return nil, err
	}
	ts, err := parseTimestamp(tsHeader)
	if err != nil {
		return nil, err
	}

	nowSec := time.Now().Unix()
	if !p.Now.IsZero() {
		nowSec = p.Now.Unix()
	}
	if diff := abs64(nowSec - ts); diff > int64(tolerance) {
		return nil, &WebhookSignatureError{
			Message: "timestamp outside tolerance window (±" + strconv.Itoa(tolerance) + "s)",
		}
	}

	// HMAC-SHA256(secret, "<timestamp>.<body>") hex.
	h := hmac.New(sha256.New, []byte(p.Secret))
	h.Write([]byte(tsHeader))
	h.Write([]byte("."))
	h.Write(p.Body)
	expected := hex.EncodeToString(h.Sum(nil))

	if !constantTimeHexEqual(expected, sig) {
		return nil, &WebhookSignatureError{Message: "signature mismatch"}
	}

	// Parse body — must be a JSON object.
	var payload map[string]any
	if err := json.Unmarshal(p.Body, &payload); err != nil {
		return nil, &WebhookSignatureError{Message: "body is not valid JSON", Cause: err}
	}
	if payload == nil {
		return nil, &WebhookSignatureError{Message: "body is not a JSON object"}
	}

	// Lift timestamp + sandbox into structured fields; rest is event data.
	bodyTimestamp := ""
	if ts, ok := payload["timestamp"].(string); ok {
		bodyTimestamp = ts
	}
	delete(payload, "timestamp")
	sandbox := false
	if s, ok := payload["sandbox"].(bool); ok {
		sandbox = s
	}
	delete(payload, "sandbox")

	return &WebhookEvent{
		EventType:    eventType,
		EventID:      eventID,
		DeliveryUUID: deliveryID,
		Timestamp:    bodyTimestamp,
		Sandbox:      sandbox,
		Data:         payload,
	}, nil
}

func parseSignature(header string) (string, error) {
	trimmed := strings.TrimSpace(header)
	if !strings.HasPrefix(trimmed, signaturePrefix) {
		return "", &WebhookSignatureError{Message: `signature must start with "v1="`}
	}
	hexPart := trimmed[len(signaturePrefix):]
	if len(hexPart) != expectedSigHexLen {
		return "", &WebhookSignatureError{Message: "signature is not a 64-char hex string"}
	}
	for _, r := range hexPart {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return "", &WebhookSignatureError{Message: "signature is not a 64-char hex string"}
		}
	}
	return strings.ToLower(hexPart), nil
}

func parseTimestamp(header string) (int64, error) {
	t, err := strconv.ParseInt(header, 10, 64)
	if err != nil || t < 0 {
		return 0, &WebhookSignatureError{Message: "timestamp header is not a valid Unix seconds integer"}
	}
	return t, nil
}

func constantTimeHexEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	aBytes, err := hex.DecodeString(a)
	if err != nil {
		return false
	}
	bBytes, err := hex.DecodeString(b)
	if err != nil {
		return false
	}
	return hmac.Equal(aBytes, bBytes)
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
