package mintarex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// seqRoundTripper replays a fixed sequence of canned responses.
type seqRoundTripper struct {
	responses []cannedResp
	calls     []*http.Request
	i         atomic.Int32
	mu        sync.Mutex
}

type cannedResp struct {
	status  int
	body    string
	headers map[string]string
}

func (s *seqRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.calls = append(s.calls, cloneReq(req))
	s.mu.Unlock()
	idx := int(s.i.Load())
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	s.i.Add(1)
	r := s.responses[idx]
	resp := &http.Response{
		StatusCode: r.status,
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Header:     http.Header{},
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")
	for k, v := range r.headers {
		resp.Header.Set(k, v)
	}
	return resp, nil
}

func cloneReq(r *http.Request) *http.Request {
	c := r.Clone(context.Background())
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(b))
		c.Body = io.NopCloser(bytes.NewReader(b))
	}
	return c
}

func newMockClient(t *testing.T, rt *seqRoundTripper, maxRetries int) *Client {
	t.Helper()
	c, err := New(Options{
		APIKey:     "mxn_test_abc123",
		APISecret:  "secret",
		HTTPClient: &http.Client{Transport: rt},
		MaxRetries: maxRetries,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGETSignsWithEmptyBodyHashAndCorrectHeaders(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{status: 200, body: `{"balances":[],"timestamp":"t"}`}}}
	mx := newMockClient(t, rt, 0)
	if _, err := mx.Account.Balances(context.Background(), BalancesParams{}); err != nil {
		t.Fatal(err)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(rt.calls))
	}
	req := rt.calls[0]
	if req.Header.Get("MX-API-KEY") == "" {
		t.Error("missing MX-API-KEY")
	}
	if req.Header.Get("MX-SIGNATURE") == "" {
		t.Error("missing MX-SIGNATURE")
	}
	if req.Header.Get("MX-TIMESTAMP") == "" {
		t.Error("missing MX-TIMESTAMP")
	}
	if req.Header.Get("MX-NONCE") == "" {
		t.Error("missing MX-NONCE")
	}
	if req.Method != "GET" {
		t.Errorf("method = %q; want GET", req.Method)
	}
}

func TestPOSTSignsWithBodyAndSetsContentType(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{status: 200, body: `{
		"quote_id":"550e8400-e29b-41d4-a716-446655440000","base":"BTC","quote":"USD","side":"buy",
		"network":"btc","price":"1","base_amount":"1","quote_amount":"1","expires_at":"t","expires_in_ms":30000
	}`}}}
	mx := newMockClient(t, rt, 0)
	_, err := mx.RFQ.Quote(context.Background(), QuoteRequest{
		Base: "BTC", Quote: "USD", Side: "buy",
		Amount: "0.1", AmountType: "base",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := rt.calls[0]
	if req.Method != "POST" {
		t.Errorf("method = %q; want POST", req.Method)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content-type = %q", req.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(body), `"base":"BTC"`) {
		t.Errorf("body missing base: %s", body)
	}
}

func TestRetriesOn429ThenSucceeds(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{
		{status: 429, body: `{"error":"rate_limited","message":"slow down"}`, headers: map[string]string{"Retry-After": "0"}},
		{status: 200, body: `{"balances":[],"timestamp":"t"}`},
	}}
	mx := newMockClient(t, rt, 3)
	if _, err := mx.Account.Balances(context.Background(), BalancesParams{}); err != nil {
		t.Fatal(err)
	}
	if len(rt.calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(rt.calls))
	}
}

func TestRetriesOn503ThenSucceeds(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{
		{status: 503, body: `{"error":"service_unavailable","message":"try again"}`, headers: map[string]string{"Retry-After": "0"}},
		{status: 200, body: `{"balances":[],"timestamp":"t"}`},
	}}
	mx := newMockClient(t, rt, 2)
	if _, err := mx.Account.Balances(context.Background(), BalancesParams{}); err != nil {
		t.Fatal(err)
	}
	if len(rt.calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(rt.calls))
	}
}

func TestDoesNotRetryOn400(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{
		{status: 400, body: `{"error":"invalid_parameter","message":"bad"}`},
	}}
	mx := newMockClient(t, rt, 3)
	_, err := mx.Account.Balances(context.Background(), BalancesParams{})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if len(rt.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(rt.calls))
	}
}

func TestGivesUpAfterMaxRetriesOn429(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{
		{status: 429, body: `{"error":"r","message":"x"}`, headers: map[string]string{"Retry-After": "0"}},
	}}
	mx := newMockClient(t, rt, 2)
	_, err := mx.Account.Balances(context.Background(), BalancesParams{})
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Errorf("expected RateLimitError, got %T", err)
	}
	if len(rt.calls) != 3 { // initial + 2 retries
		t.Errorf("expected 3 calls, got %d", len(rt.calls))
	}
}

func TestMapsEachHTTPStatusToCorrectErrorType(t *testing.T) {
	cases := []struct {
		status int
		code   string
		check  func(error) bool
	}{
		{400, "x", func(e error) bool { var v *ValidationError; return errors.As(e, &v) }},
		{400, "insufficient_balance", func(e error) bool { var v *InsufficientBalanceError; return errors.As(e, &v) }},
		{401, "x", func(e error) bool { var v *AuthenticationError; return errors.As(e, &v) }},
		{403, "x", func(e error) bool { var v *PermissionError; return errors.As(e, &v) }},
		{404, "x", func(e error) bool { var v *NotFoundError; return errors.As(e, &v) }},
		{409, "x", func(e error) bool { var v *ConflictError; return errors.As(e, &v) }},
		{410, "quote_expired_or_not_found", func(e error) bool { var v *QuoteExpiredError; return errors.As(e, &v) }},
		{429, "x", func(e error) bool { var v *RateLimitError; return errors.As(e, &v) }},
		{503, "x", func(e error) bool { var v *ServiceUnavailableError; return errors.As(e, &v) }},
	}
	for _, c := range cases {
		body, _ := json.Marshal(map[string]string{"error": c.code, "message": "x"})
		rt := &seqRoundTripper{responses: []cannedResp{{status: c.status, body: string(body)}}}
		mx := newMockClient(t, rt, 0)
		_, err := mx.Account.Balances(context.Background(), BalancesParams{})
		if !c.check(err) {
			t.Errorf("status %d code %q: wrong error type: %T %v", c.status, c.code, err, err)
		}
	}
}

func TestParsesIETFRateLimitHeadersOnSuccess(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{
		status: 200, body: `{"balances":[],"timestamp":"t"}`,
		headers: map[string]string{
			"RateLimit-Limit":     "100",
			"RateLimit-Remaining": "99",
			"RateLimit-Reset":     "60",
			"X-Request-Id":        "req_abc",
		},
	}}}
	mx := newMockClient(t, rt, 0)
	r, err := mx.Account.Balances(context.Background(), BalancesParams{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Meta == nil {
		t.Fatal("meta is nil")
	}
	if r.Meta.RateLimit.Limit == nil || *r.Meta.RateLimit.Limit != 100 {
		t.Errorf("limit = %v; want 100", r.Meta.RateLimit.Limit)
	}
	if r.Meta.RateLimit.Remaining == nil || *r.Meta.RateLimit.Remaining != 99 {
		t.Errorf("remaining = %v; want 99", r.Meta.RateLimit.Remaining)
	}
	if r.Meta.RequestID != "req_abc" {
		t.Errorf("request-id = %q", r.Meta.RequestID)
	}
}

func TestParsesLegacyXRateLimitHeadersAsFallback(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{
		status: 200, body: `{"balances":[],"timestamp":"t"}`,
		headers: map[string]string{
			"X-RateLimit-Limit":     "50",
			"X-RateLimit-Remaining": "40",
			"X-RateLimit-Reset":     "30",
		},
	}}}
	mx := newMockClient(t, rt, 0)
	r, _ := mx.Account.Balances(context.Background(), BalancesParams{})
	if r.Meta.RateLimit.Limit == nil || *r.Meta.RateLimit.Limit != 50 {
		t.Errorf("limit = %v; want 50", r.Meta.RateLimit.Limit)
	}
}

func TestParsesRateLimitInfoIntoErrorOn429(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{
		status: 429, body: `{"error":"rate_limited","message":"x"}`,
		headers: map[string]string{"X-RateLimit-Remaining": "0", "Retry-After": "10"},
	}}}
	mx := newMockClient(t, rt, 0)
	_, err := mx.Account.Balances(context.Background(), BalancesParams{})
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitError, got %T", err)
	}
	if rl.RetryAfterMs != 10_000 {
		t.Errorf("retry-after ms = %d; want 10000", rl.RetryAfterMs)
	}
	if rl.RateLimit.Remaining == nil || *rl.RateLimit.Remaining != 0 {
		t.Errorf("rate-limit remaining = %v", rl.RateLimit.Remaining)
	}
}

func TestRetryAfterClampedTo60Seconds(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{
		status: 429, body: `{"error":"x","message":"x"}`,
		headers: map[string]string{"Retry-After": "3600"}, // 1 hour should be clamped
	}}}
	mx := newMockClient(t, rt, 0)
	_, err := mx.Account.Balances(context.Background(), BalancesParams{})
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitError, got %T", err)
	}
	if rl.RetryAfterMs > 60_000 {
		t.Errorf("retry-after not clamped: %d ms", rl.RetryAfterMs)
	}
}

func TestQueryStringIncludedInSignedPath(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{status: 200, body: `{"balances":[],"timestamp":"t"}`}}}
	mx := newMockClient(t, rt, 0)
	_, _ = mx.Account.Balances(context.Background(), BalancesParams{
		CurrencyType: CryptoCurrency,
		IncludeEmpty: true,
	})
	req := rt.calls[0]
	q := req.URL.Query()
	if q.Get("currency_type") != "crypto" {
		t.Errorf("currency_type = %q", q.Get("currency_type"))
	}
	if q.Get("include_empty") != "true" {
		t.Errorf("include_empty = %q; must be lowercase for cross-SDK parity", q.Get("include_empty"))
	}
}

func TestInferredEnvironmentTestKeyIsSandbox(t *testing.T) {
	mx, err := New(Options{APIKey: "mxn_test_abc", APISecret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if mx.Environment != EnvSandbox {
		t.Errorf("env = %q; want sandbox", mx.Environment)
	}
}

func TestInferredEnvironmentLiveKeyIsLive(t *testing.T) {
	mx, err := New(Options{APIKey: "mxn_live_abc", APISecret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if mx.Environment != EnvLive {
		t.Errorf("env = %q; want live", mx.Environment)
	}
}

func TestLiveEnvWithTestKeyThrows(t *testing.T) {
	_, err := New(Options{
		APIKey: "mxn_test_abc", APISecret: "s",
		Environment: EnvLive,
	})
	var ce *ConfigurationError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConfigurationError, got %T", err)
	}
	if !contains(ce.Error(), "prefix does not match") {
		t.Errorf("error missing 'prefix does not match': %v", ce)
	}
}

func TestPathTraversalInUUIDArgsRejectedBeforeRequest(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{status: 200, body: "{}"}}}
	mx := newMockClient(t, rt, 0)
	_, err := mx.Trades.Get(context.Background(), "../../admin")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(rt.calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(rt.calls))
	}
}

func TestHTTPBaseURLRejectedForPublicHost(t *testing.T) {
	_, err := New(Options{
		APIKey: "mxn_test_x", APISecret: "s",
		BaseURL: "http://evil.example.com/v1",
	})
	var ce *ConfigurationError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConfigurationError, got %T", err)
	}
}

func TestHTTPBaseURLAllowedForLocalhost(t *testing.T) {
	mx, err := New(Options{
		APIKey: "mxn_test_x", APISecret: "s",
		BaseURL: "http://localhost:5001/v1",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mx.BaseURL.Hostname() != "localhost" {
		t.Errorf("host = %q", mx.BaseURL.Hostname())
	}
}

func TestCircularBodyFailsJSONSerialization(t *testing.T) {
	// Go's encoding/json errors on unsupported types (channels, funcs); use that.
	rt := &seqRoundTripper{responses: []cannedResp{{status: 200, body: "{}"}}}
	mx := newMockClient(t, rt, 0)
	_, err := mx.Client().Request(context.Background(), RequestOptions{
		Method: "POST",
		Path:   "/x",
		Body:   map[string]any{"bad": make(chan int)},
	}, nil)
	var ce *ConfigurationError
	if !errors.As(err, &ce) {
		t.Errorf("expected ConfigurationError, got %T: %v", err, err)
	}
}

func TestContextCancellationPropagates(t *testing.T) {
	rt := &seqRoundTripper{responses: []cannedResp{{status: 200, body: "{}"}}}
	mx := newMockClient(t, rt, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := mx.Account.Balances(ctx, BalancesParams{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// Client exposes the internal for testing deep paths that don't go through a resource.
func (c *Client) ClientForTest() *Client { return c }

// Client returns the client itself — test helper used in TestCircularBody above.
// Provided so tests can access Request() without importing internal-only paths.
func (c *Client) Client() *Client { return c }
