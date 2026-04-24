package mintarex

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// markerRoundTripper returns a distinctive error so tests can tell the
// difference between "validator rejected" (*ValidationError) and "validator
// accepted, network was reached" (wrapped markerErr).
type markerRoundTripper struct{}

type markerErr struct{}

func (markerErr) Error() string { return "validator_passed_marker" }

func (markerRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, markerErr{}
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(Options{
		APIKey:     "mxn_test_abc123",
		APISecret:  "secret",
		HTTPClient: &http.Client{Transport: markerRoundTripper{}},
		MaxRetries: 0, // don't retry so we see the error immediately
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// assertValidatorPassed confirms the validator accepted the input and the
// request attempt reached (and failed at) the mock transport.
func assertValidatorPassed(t *testing.T, err error, ctx string) {
	t.Helper()
	if err == nil {
		return // 2xx from a perfect mock — also counts as "validator passed"
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Errorf("%s: expected validator to pass, got ValidationError: %v", ctx, err)
	}
}

func assertValidationError(t *testing.T, err error, ctx string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected ValidationError, got nil", ctx)
		return
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("%s: expected *ValidationError, got %T: %v", ctx, err, err)
	}
}

func TestAmountRegexRejectsBadInputs(t *testing.T) {
	mx := newTestClient(t)
	ctx := context.Background()
	bad := []string{"-1", "+1", "1e3", "1.1234567890123456789", "abc", "", "01"}
	for _, b := range bad {
		_, err := mx.RFQ.Quote(ctx, QuoteRequest{
			Base: "BTC", Quote: "USD", Side: "buy",
			Amount: b, AmountType: "base",
		})
		assertValidationError(t, err, "amount="+b)
	}
}

func TestCoinRegexRejectsBadInputs(t *testing.T) {
	mx := newTestClient(t)
	ctx := context.Background()
	bad := []string{"btc", "B", "TOOLONGCOIN123", "", "BT-C", "BTC_ETH"}
	for _, b := range bad {
		_, err := mx.Crypto.DepositAddress(ctx, DepositAddressParams{Coin: b})
		assertValidationError(t, err, "coin="+b)
	}
}

func TestCoinRegexAcceptsDigitLeadingTickers(t *testing.T) {
	// Accepts 1INCH, 2Z, BTC, USDT, WBTC — validator passes; request reaches mock.
	mx := newTestClient(t)
	ctx := context.Background()
	good := []string{"1INCH", "2Z", "BTC", "USDT", "WBTC"}
	for _, g := range good {
		_, err := mx.Crypto.DepositAddress(ctx, DepositAddressParams{Coin: g})
		assertValidatorPassed(t, err, "coin="+g)
	}
}

func TestNetworkRegexRejectsBadInputs(t *testing.T) {
	// Note: empty string is the Go zero value meaning "not provided" — the
	// resource method skips validation when Network is "". Pass-through is
	// correct behavior (matches Node/Python "undefined means no filter").
	mx := newTestClient(t)
	ctx := context.Background()
	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 41 chars
	bad := []string{"BTC", "btc/eth", long}
	for _, b := range bad {
		_, err := mx.Crypto.DepositAddress(ctx, DepositAddressParams{Coin: "BTC", Network: b})
		assertValidationError(t, err, "network="+b)
	}
}

func TestAddressRegexRejectsBadInputs(t *testing.T) {
	mx := newTestClient(t)
	ctx := context.Background()
	long := ""
	for range 256 {
		long += "a"
	}
	bad := []string{"abc", long, "has space", "has\n", ""}
	for _, b := range bad {
		_, err := mx.Crypto.Withdraw(ctx, WithdrawRequest{
			Coin: "BTC", Network: "btc", Amount: "0.1",
			Address: b, IdempotencyKey: "k1",
		})
		assertValidationError(t, err, "address rejects")
	}
}

func TestUUIDRegexRejectsNonUUID(t *testing.T) {
	mx := newTestClient(t)
	ctx := context.Background()
	bad := []string{"not-a-uuid", "12345678-1234-1234-1234-123456789012x", "", "../../etc/passwd"}
	for _, b := range bad {
		_, err := mx.RFQ.Accept(ctx, b, AcceptOptions{})
		assertValidationError(t, err, "uuid="+b)
	}
}

func TestWebhookURLRejectsHTTPAndCredentials(t *testing.T) {
	mx := newTestClient(t)
	ctx := context.Background()
	long := "https://"
	for range 3000 {
		long += "a"
	}
	bad := []string{
		"http://example.com",
		"https://user:pass@example.com/hook",
		"not a url",
		long,
	}
	for _, b := range bad {
		_, err := mx.Webhooks.Create(ctx, WebhookCreateRequest{
			URL: b, Events: []string{"trade.executed"}, Label: "x",
		})
		assertValidationError(t, err, "url="+b)
	}
}

func TestWebhookEventsValidation(t *testing.T) {
	mx := newTestClient(t)
	ctx := context.Background()
	_, err := mx.Webhooks.Create(ctx, WebhookCreateRequest{
		URL: "https://example.com", Events: []string{}, Label: "x",
	})
	assertValidationError(t, err, "empty events")
	_, err = mx.Webhooks.Create(ctx, WebhookCreateRequest{
		URL: "https://example.com", Events: []string{"BAD"}, Label: "x",
	})
	assertValidationError(t, err, "bad event")
}

func TestNewRejectsMissingKeys(t *testing.T) {
	cases := []struct {
		name    string
		opts    Options
		wantSub string
	}{
		{"missing APIKey", Options{APISecret: "s"}, "APIKey is required"},
		{"missing APISecret", Options{APIKey: "mxn_test_x"}, "APISecret is required"},
		{"unknown prefix", Options{APIKey: "k", APISecret: "s"}, "must start with"},
		{"prefix mismatch", Options{
			APIKey: "mxn_live_abc", APISecret: "s", Environment: EnvSandbox,
		}, "prefix does not match"},
	}
	for _, c := range cases {
		_, err := New(c.opts)
		if err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
			continue
		}
		var ce *ConfigurationError
		if !errors.As(err, &ce) {
			t.Errorf("%s: expected *ConfigurationError, got %T", c.name, err)
			continue
		}
		if !contains(ce.Error(), c.wantSub) {
			t.Errorf("%s: error %q missing substring %q", c.name, ce.Error(), c.wantSub)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
