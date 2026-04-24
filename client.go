package mintarex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SDKVersion is the current SDK version, surfaced in the User-Agent header.
const SDKVersion = "0.0.3"

const (
	defaultBaseURL       = "https://institutional.mintarex.com/v1"
	defaultStreamBaseURL = "https://institutional.mintarex.com/v1/stream"
	defaultTimeout       = 30 * time.Second
	defaultMaxRetries    = 3
	maxRetries           = 10
	maxRetryAfterMs      = 60_000
	liveKeyPrefix        = "mxn_live_"
	testKeyPrefix        = "mxn_test_"
)

// Options configure a new [Client].
type Options struct {
	APIKey    string // required, "mxn_live_..." or "mxn_test_..."
	APISecret string // required

	// Environment is inferred from APIKey prefix if empty. Must match.
	Environment Environment

	// BaseURL overrides the default API URL. HTTPS required (loopback may use HTTP).
	BaseURL string
	// StreamBaseURL overrides the default SSE base URL. Same scheme rules as BaseURL.
	StreamBaseURL string

	// Timeout for each HTTP request. Default: 30s.
	Timeout time.Duration
	// MaxRetries for 429/503 and network errors. Clamped to [0, 10]. Default: 3.
	MaxRetries int

	// HTTPClient overrides the http.Client used for requests (for tests or a proxy).
	HTTPClient *http.Client

	// UserAgent, if set, is appended to the default User-Agent.
	UserAgent string
}

// Client is the main SDK entry point. Construct via [New] and share across
// goroutines; it is safe for concurrent use.
type Client struct {
	apiKey         string
	// apiSecret is stored as a closure so reflection-based prints
	// (e.g. fmt.Sprintf("%+v", *client) on a dereferenced struct) cannot
	// observe its value — fmt prints the function pointer address instead.
	// Retrieved via c.apiSecret() inside this package.
	apiSecret      func() string
	Environment    Environment
	BaseURL        *url.URL
	StreamBaseURL  *url.URL
	timeout        time.Duration
	maxRetries     int
	http           *http.Client
	userAgentExtra string

	Account  *AccountResource
	RFQ      *RFQResource
	Trades   *TradesResource
	Crypto   *CryptoResource
	Webhooks *WebhooksResource
	Public   *PublicResource
	Streams  *StreamsResource
}

// New constructs a Client. Returns a [ConfigurationError] if APIKey/APISecret
// are missing, if Environment doesn't match the key prefix, or if BaseURL is
// invalid.
func New(opts Options) (*Client, error) {
	if opts.APIKey == "" {
		return nil, &ConfigurationError{Message: "APIKey is required"}
	}
	if opts.APISecret == "" {
		return nil, &ConfigurationError{Message: "APISecret is required"}
	}

	env := opts.Environment
	if env == "" {
		switch {
		case strings.HasPrefix(opts.APIKey, liveKeyPrefix):
			env = EnvLive
		case strings.HasPrefix(opts.APIKey, testKeyPrefix):
			env = EnvSandbox
		default:
			return nil, &ConfigurationError{
				Message: "APIKey must start with mxn_live_ or mxn_test_ (or set Environment explicitly)",
			}
		}
	}
	if env != EnvLive && env != EnvSandbox {
		return nil, &ConfigurationError{Message: fmt.Sprintf("invalid Environment: %q", env)}
	}
	keyPrefixOK := (env == EnvLive && strings.HasPrefix(opts.APIKey, liveKeyPrefix)) ||
		(env == EnvSandbox && strings.HasPrefix(opts.APIKey, testKeyPrefix))
	if !keyPrefixOK {
		return nil, &ConfigurationError{
			Message: fmt.Sprintf(`APIKey prefix does not match environment %q; `+
				`live keys start with mxn_live_, sandbox keys with mxn_test_`, env),
		}
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURLParsed, err := parseBaseURL(baseURL, "BaseURL")
	if err != nil {
		return nil, err
	}

	streamURL := opts.StreamBaseURL
	if streamURL == "" {
		streamURL = defaultStreamBaseURL
	}
	streamURLParsed, err := parseBaseURL(streamURL, "StreamBaseURL")
	if err != nil {
		return nil, err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	retries := opts.MaxRetries
	switch {
	case retries < 0:
		retries = defaultMaxRetries
	case retries > maxRetries:
		retries = maxRetries
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		// Redirect disabled so we never follow a 3xx away from the configured host.
		httpClient = &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	uaExtra := ""
	if opts.UserAgent != "" {
		uaExtra = " " + opts.UserAgent
	}

	c := &Client{
		apiKey:         opts.APIKey,
		apiSecret:      newSecretAccessor(opts.APISecret),
		Environment:    env,
		BaseURL:        baseURLParsed,
		StreamBaseURL:  streamURLParsed,
		timeout:        timeout,
		maxRetries:     retries,
		http:           httpClient,
		userAgentExtra: uaExtra,
	}
	c.Account = &AccountResource{client: c}
	c.RFQ = &RFQResource{client: c}
	c.Trades = &TradesResource{client: c}
	c.Crypto = &CryptoResource{client: c, Addresses: &CryptoAddressesResource{client: c}}
	c.Webhooks = &WebhooksResource{client: c}
	c.Public = &PublicResource{client: c}
	c.Streams = &StreamsResource{client: c}
	return c, nil
}

// newSecretAccessor closes over the secret so it lives only in the closure's
// captured scope, never as a directly-readable struct field. Reflection-based
// prints (fmt.Sprintf("%+v", *client)) see only the function pointer.
func newSecretAccessor(secret string) func() string {
	return func() string { return secret }
}

// String returns a redacted representation of the Client so that printing it
// (e.g. fmt.Sprintf("%v", client) or %s/%+v on the pointer) cannot leak
// APISecret into logs. Combined with [secretString], dereferenced struct
// values are also masked when their fields are walked via reflection.
func (c *Client) String() string {
	if c == nil {
		return "<nil mintarex.Client>"
	}
	base := ""
	if c.BaseURL != nil {
		base = c.BaseURL.String()
	}
	return fmt.Sprintf(
		"mintarex.Client{APIKey: %q, APISecret: \"[REDACTED]\", Environment: %q, BaseURL: %q}",
		c.apiKey, c.Environment, base,
	)
}

// GoString covers fmt.Sprintf("%#v", client). Same redaction guarantee as
// [Client.String].
func (c *Client) GoString() string {
	return c.String()
}

// RequestOptions control a single signed request.
type RequestOptions struct {
	Method string            // "GET", "POST", "DELETE", ...
	Path   string            // must start with "/"
	Query  map[string]string // nil-safe; values already stringified (bools → "true"/"false")
	Body   any               // will be JSON-marshaled; use nil for empty

	// MaxRetries overrides Client.maxRetries for this request. Zero (the
	// struct zero value) means "use client default". Set to a positive
	// number to override; if you need to DISABLE retries for a single
	// request, set Client.MaxRetries = 0 globally instead.
	MaxRetries int
	// RetryOnNetworkError overrides the default retry policy. nil = use defaults
	// (GET/DELETE always retry; POST/PUT/PATCH only if body has idempotency_key).
	RetryOnNetworkError *bool
}

// Request executes a signed request. The caller is expected to pass a
// context with a timeout that is ≥ the per-request timeout, or context.Background().
// Returns the parsed JSON body in dst (must be a pointer) plus *ResponseMeta.
func (c *Client) Request(ctx context.Context, opts RequestOptions, dst any) (*ResponseMeta, error) {
	if !strings.HasPrefix(opts.Path, "/") {
		return nil, &ConfigurationError{Message: `Path must start with "/"`}
	}
	method := strings.ToUpper(opts.Method)

	// Build URL.
	u := *c.BaseURL
	u.Path = joinPath(c.BaseURL.Path, opts.Path)
	u.Fragment = ""
	if len(opts.Query) > 0 {
		q := u.Query()
		for k, v := range opts.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	// Encode body once (same bytes used for signing and for sending).
	var bodyBytes []byte
	if opts.Body != nil && method != http.MethodGet && method != http.MethodDelete {
		var err error
		bodyBytes, err = json.Marshal(opts.Body)
		if err != nil {
			return nil, &ConfigurationError{
				Message: fmt.Sprintf("request body is not JSON-serializable: %v", err),
			}
		}
	}

	// Canonical path for signing includes query string.
	canonicalPath := u.Path
	if u.RawQuery != "" {
		canonicalPath = u.Path + "?" + u.RawQuery
	}

	retries := c.maxRetries
	// Zero-valued RequestOptions.MaxRetries means "inherit from client";
	// a positive value overrides, clamped to maxRetries (const 10).
	if opts.MaxRetries > 0 {
		retries = opts.MaxRetries
		if retries > maxRetries {
			retries = maxRetries
		}
	}
	retryNet := resolveRetryOnNetworkError(opts, method)

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		meta, bodyRaw, httpStatus, retryAfterMs, ok, netErr := c.executeOnce(ctx, method, &u, canonicalPath, bodyBytes)

		if netErr != nil {
			lastErr = netErr
			if retryNet && attempt < retries {
				if !sleepForBackoff(ctx, backoffDuration(attempt, 0)) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, netErr
		}

		if ok {
			if dst != nil && len(bodyRaw) > 0 {
				if err := json.Unmarshal(bodyRaw, dst); err != nil {
					return meta, &NetworkError{
						Message: fmt.Sprintf("failed to unmarshal response body: %v", err),
						Cause:   err,
					}
				}
			}
			return meta, nil
		}

		// Non-2xx; build typed error with retry-after populated.
		apiErr := parseErrorBody(bodyRaw, httpStatus, meta)
		apiErr.RetryAfterMs = retryAfterMs
		typed := errorFromResponse(apiErr)
		if (httpStatus == http.StatusTooManyRequests || httpStatus == http.StatusServiceUnavailable) && attempt < retries {
			delay := backoffDuration(attempt, retryAfterMs)
			if !sleepForBackoff(ctx, delay) {
				return nil, ctx.Err()
			}
			continue
		}
		return meta, typed
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &NetworkError{Message: "retry limit exceeded"}
}

// executeOnce performs a single attempt. Returns (meta, rawBody, status, retryAfterMs, ok, netErr).
func (c *Client) executeOnce(
	ctx context.Context,
	method string,
	u *url.URL,
	canonicalPath string,
	bodyBytes []byte,
) (*ResponseMeta, []byte, int, int, bool, error) {
	hdrs := Sign(SignParams{
		APIKey:    c.apiKey,
		APISecret: c.apiSecret(),
		Method:    method,
		Path:      canonicalPath,
		Body:      bodyBytes,
	})

	var reqBody io.Reader
	if len(bodyBytes) > 0 {
		reqBody = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, nil, 0, 0, false, &NetworkError{Message: "build request failed", Cause: err}
	}
	for k, v := range hdrs.Map() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("mintarex-go/%s (go %s)%s",
		SDKVersion, runtime.Version(), c.userAgentExtra))
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, 0, 0, false, err
		}
		return nil, nil, 0, 0, false, &NetworkError{Message: err.Error(), Cause: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, resp.StatusCode, 0, false, &NetworkError{
			Message: "failed to read response body",
			Cause:   err,
		}
	}

	meta := &ResponseMeta{
		RequestID: resp.Header.Get("X-Request-Id"),
		RateLimit: readRateLimitHeaders(resp.Header),
		Status:    resp.StatusCode,
	}
	retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	return meta, raw, resp.StatusCode, retryAfter, ok, nil
}

// parseErrorBody extracts an APIError from a non-2xx response body.
func parseErrorBody(raw []byte, status int, meta *ResponseMeta) APIError {
	base := APIError{
		Status:    status,
		Code:      "unknown_error",
		Message:   fmt.Sprintf("HTTP %d", status),
		RequestID: meta.RequestID,
		RateLimit: meta.RateLimit,
	}
	if len(raw) == 0 {
		return base
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		base.ResponseBody = parsed
		if s, ok := parsed["error"].(string); ok {
			base.Code = s
		}
		if s, ok := parsed["message"].(string); ok {
			base.Message = s
		}
	} else {
		base.ResponseBody = string(raw)
	}
	// Retry-After header in outer scope — hard to get here without the response;
	// the caller fills this in via the meta+header path below.
	return base
}

// parseRetryAfterHeader parses a Retry-After header value into milliseconds,
// clamped to [0, maxRetryAfterMs]. Returns 0 if not parseable.
func parseRetryAfterHeader(v string) int {
	if v == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		ms := int(math.Max(0, math.Min(float64(maxRetryAfterMs), secs*1000)))
		return ms
	}
	if ts, err := http.ParseTime(v); err == nil {
		delta := time.Until(ts).Milliseconds()
		if delta < 0 {
			delta = 0
		}
		if delta > maxRetryAfterMs {
			delta = maxRetryAfterMs
		}
		return int(delta)
	}
	return 0
}

// readRateLimitHeaders parses IETF RateLimit-* headers (RFC 9331) with
// legacy X-RateLimit-* fallback.
func readRateLimitHeaders(h http.Header) RateLimitInfo {
	return RateLimitInfo{
		Limit:     parseIntHeader(h, "RateLimit-Limit", "X-RateLimit-Limit"),
		Remaining: parseIntHeader(h, "RateLimit-Remaining", "X-RateLimit-Remaining"),
		Reset:     parseIntHeader(h, "RateLimit-Reset", "X-RateLimit-Reset"),
	}
}

func parseIntHeader(h http.Header, name, fallback string) *int {
	v := h.Get(name)
	if v == "" && fallback != "" {
		v = h.Get(fallback)
	}
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	n := int(f)
	return &n
}

func joinPath(a, b string) string {
	left := strings.TrimRight(a, "/")
	right := b
	if !strings.HasPrefix(right, "/") {
		right = "/" + right
	}
	return left + right
}

func parseBaseURL(input, name string) (*url.URL, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, &ConfigurationError{Message: fmt.Sprintf("invalid %s: %s (%v)", name, input, err)}
	}
	if u.Scheme == "https" {
		return u, nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return u, nil
	}
	if u.Scheme == "http" {
		return nil, &ConfigurationError{
			Message: fmt.Sprintf("invalid %s: %s (http:// is only permitted for loopback)", name, input),
		}
	}
	return nil, &ConfigurationError{
		Message: fmt.Sprintf("invalid %s: %s (protocol must be https://)", name, input),
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || host == "::1" || host == "[::1]" {
		return true
	}
	return strings.HasPrefix(host, "127.")
}

func resolveRetryOnNetworkError(opts RequestOptions, method string) bool {
	if opts.RetryOnNetworkError != nil {
		return *opts.RetryOnNetworkError
	}
	if method == http.MethodGet || method == http.MethodDelete {
		return true
	}
	if body, ok := opts.Body.(map[string]any); ok {
		if _, present := body["idempotency_key"]; present {
			return true
		}
	}
	// Struct bodies with idempotency_key are opaque at reflect-time here; be
	// conservative (no retry). Callers should pass RetryOnNetworkError: ptr(true)
	// if they've included an idempotency_key in a struct body.
	return false
}

func backoffDuration(attempt int, retryAfterMs int) time.Duration {
	if retryAfterMs > 0 {
		capped := retryAfterMs
		if capped > maxRetryAfterMs {
			capped = maxRetryAfterMs
		}
		jitterCap := float64(capped) * 0.1
		if jitterCap > 5000 {
			jitterCap = 5000
		}
		jitter := (rand.Float64()*2 - 1) * jitterCap
		final := int(math.Max(0, float64(capped)+jitter))
		return time.Duration(final) * time.Millisecond
	}
	base := 500 * (1 << attempt)
	if base > 15_000 {
		base = 15_000
	}
	jitter := rand.IntN(251)
	total := base + jitter
	if total > 15_000 {
		total = 15_000
	}
	return time.Duration(total) * time.Millisecond
}

func sleepForBackoff(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
