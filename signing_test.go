package mintarex

import (
	"regexp"
	"strings"
	"testing"
)

func TestEmptyBodySHA256MatchesSHAOfEmptyString(t *testing.T) {
	if got := SHA256Hex(nil); got != EmptyBodySHA256 {
		t.Errorf("SHA256Hex(nil) = %q; want %q", got, EmptyBodySHA256)
	}
	if got := SHA256Hex([]byte{}); got != EmptyBodySHA256 {
		t.Errorf("SHA256Hex([]byte{}) = %q; want %q", got, EmptyBodySHA256)
	}
}

func TestCanonicalStringFormat(t *testing.T) {
	s := BuildCanonicalString("GET", "/v1/account/balances", "1712582345",
		"550e8400-e29b-41d4-a716-446655440000", EmptyBodySHA256)
	want := "GET\n/v1/account/balances\n1712582345\n550e8400-e29b-41d4-a716-446655440000\n" + EmptyBodySHA256
	if s != want {
		t.Errorf("canonical = %q;\nwant   = %q", s, want)
	}
}

func TestCanonicalStringUppercasesMethod(t *testing.T) {
	s := BuildCanonicalString("post", "/v1/rfq", "1", "n", "h")
	if !strings.HasPrefix(s, "POST\n") {
		t.Errorf("expected POST prefix, got %q", s)
	}
}

func TestHMACSignReturns64CharLowercaseHex(t *testing.T) {
	sig := HMACSign("secret", "hello")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sig) {
		t.Errorf("sig not 64-char lowercase hex: %q", sig)
	}
}

func TestSignProducesAllFourRequiredHeaders(t *testing.T) {
	h := Sign(SignParams{
		APIKey:    "mxn_live_abc",
		APISecret: "deadbeef",
		Method:    "GET",
		Path:      "/v1/account/fees",
		Timestamp: "1712582345",
		Nonce:     "550e8400-e29b-41d4-a716-446655440000",
	})
	if h.APIKey != "mxn_live_abc" {
		t.Errorf("APIKey mismatch: %q", h.APIKey)
	}
	if h.Timestamp != "1712582345" {
		t.Errorf("Timestamp mismatch: %q", h.Timestamp)
	}
	if h.Nonce != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("Nonce mismatch: %q", h.Nonce)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(h.Signature) {
		t.Errorf("Signature not hex-64: %q", h.Signature)
	}
}

func TestSignDeterministicForSameInputs(t *testing.T) {
	p := SignParams{
		APIKey: "mxn_live_abc", APISecret: "secret",
		Method: "POST", Path: "/v1/rfq",
		Body:      []byte(`{"base":"BTC","quote":"USD"}`),
		Timestamp: "1000", Nonce: "nnn",
	}
	if Sign(p).Signature != Sign(p).Signature {
		t.Error("sign not deterministic")
	}
}

func TestSignDiffersWhenAnySignedInputChanges(t *testing.T) {
	base := SignParams{
		APIKey: "mxn_live_abc", APISecret: "s",
		Method: "POST", Path: "/v1/rfq", Body: []byte("x"),
		Timestamp: "1", Nonce: "n",
	}
	variants := []SignParams{
		base,
		{APIKey: base.APIKey, APISecret: base.APISecret, Method: "GET", Path: base.Path, Body: base.Body, Timestamp: base.Timestamp, Nonce: base.Nonce},
		{APIKey: base.APIKey, APISecret: base.APISecret, Method: base.Method, Path: "/v1/other", Body: base.Body, Timestamp: base.Timestamp, Nonce: base.Nonce},
		{APIKey: base.APIKey, APISecret: base.APISecret, Method: base.Method, Path: base.Path, Body: base.Body, Timestamp: "2", Nonce: base.Nonce},
		{APIKey: base.APIKey, APISecret: base.APISecret, Method: base.Method, Path: base.Path, Body: base.Body, Timestamp: base.Timestamp, Nonce: "n2"},
		{APIKey: base.APIKey, APISecret: base.APISecret, Method: base.Method, Path: base.Path, Body: []byte("y"), Timestamp: base.Timestamp, Nonce: base.Nonce},
		{APIKey: base.APIKey, APISecret: "s2", Method: base.Method, Path: base.Path, Body: base.Body, Timestamp: base.Timestamp, Nonce: base.Nonce},
	}
	sigs := make(map[string]struct{}, len(variants))
	for _, v := range variants {
		sigs[Sign(v).Signature] = struct{}{}
	}
	if len(sigs) != len(variants) {
		t.Errorf("expected %d unique signatures, got %d", len(variants), len(sigs))
	}
}

func TestSignWithEmptyBodyUsesEmptyBodySHA256(t *testing.T) {
	canonical := BuildCanonicalString("GET", "/v1/account/fees", "1", "n", EmptyBodySHA256)
	want := HMACSign("secret", canonical)
	got := Sign(SignParams{
		APIKey: "k", APISecret: "secret",
		Method: "GET", Path: "/v1/account/fees",
		Timestamp: "1", Nonce: "n",
	}).Signature
	if got != want {
		t.Errorf("sig mismatch: got %q want %q", got, want)
	}
}

func TestSignAutoGeneratesTimestampAndNonce(t *testing.T) {
	h := Sign(SignParams{
		APIKey: "k", APISecret: "s",
		Method: "GET", Path: "/v1/x",
	})
	if h.Timestamp == "" {
		t.Error("timestamp not auto-generated")
	}
	if len(h.Nonce) != 36 { // UUID v4 "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
		t.Errorf("nonce not UUID: %q", h.Nonce)
	}
}
