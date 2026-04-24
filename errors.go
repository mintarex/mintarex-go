package mintarex

import (
	"errors"
	"fmt"
)

// RateLimitInfo is the parsed IETF RateLimit-* response headers (RFC 9331).
// Any field may be nil if the server did not provide it.
type RateLimitInfo struct {
	Limit     *int
	Remaining *int
	Reset     *int
}

// APIError is returned for any non-2xx HTTP response from the API.
//
// The most specific typed error is returned via [errors.As]; for example:
//
//	var rl *RateLimitError
//	if errors.As(err, &rl) {
//	    time.Sleep(time.Duration(rl.RetryAfterMs) * time.Millisecond)
//	}
type APIError struct {
	Status       int
	Code         string // API error code, e.g. "insufficient_balance"
	Message      string
	RequestID    string
	RetryAfterMs int // clamped to [0, 60000]; 0 if not provided
	RateLimit    RateLimitInfo
	ResponseBody any // raw parsed JSON (or string fallback)
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("mintarex: HTTP %d %s: %s", e.Status, e.Code, e.Message)
}

// AuthenticationError: 401 — API key not recognized or signature invalid.
type AuthenticationError struct{ APIError }

func (e *AuthenticationError) Unwrap() error { return &e.APIError }

// PermissionError: 403 — API key is valid but lacks required scope.
type PermissionError struct{ APIError }

func (e *PermissionError) Unwrap() error { return &e.APIError }

// ValidationError: 400 — request validation failed (malformed params, etc.)
// Also used for client-side validation errors (Status == 0).
type ValidationError struct{ APIError }

func (e *ValidationError) Unwrap() error { return &e.APIError }

// InsufficientBalanceError: 400 with code "insufficient_balance".
type InsufficientBalanceError struct{ APIError }

func (e *InsufficientBalanceError) Unwrap() error { return &e.APIError }

// NotFoundError: 404 — resource not found.
type NotFoundError struct{ APIError }

func (e *NotFoundError) Unwrap() error { return &e.APIError }

// ConflictError: 409 — idempotency-key conflict, quote already consumed, etc.
type ConflictError struct{ APIError }

func (e *ConflictError) Unwrap() error { return &e.APIError }

// QuoteExpiredError: 410 — RFQ quote expired (issued more than 30s ago).
type QuoteExpiredError struct{ APIError }

func (e *QuoteExpiredError) Unwrap() error { return &e.APIError }

// RateLimitError: 429 — rate limit or concurrency cap exceeded.
type RateLimitError struct{ APIError }

func (e *RateLimitError) Unwrap() error { return &e.APIError }

// ServerError: 500 — server-side error.
type ServerError struct{ APIError }

func (e *ServerError) Unwrap() error { return &e.APIError }

// ServiceUnavailableError: 503 — service temporarily unavailable; inspect RetryAfterMs.
type ServiceUnavailableError struct{ APIError }

func (e *ServiceUnavailableError) Unwrap() error { return &e.APIError }

// NetworkError is returned for network-layer failures (DNS, TCP reset, TLS,
// timeout). No HTTP response was received.
type NetworkError struct {
	Message string
	Cause   error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("mintarex: network error: %s: %v", e.Message, e.Cause)
	}
	return "mintarex: network error: " + e.Message
}

func (e *NetworkError) Unwrap() error { return e.Cause }

// WebhookSignatureError is returned when webhook verification fails — bad
// signature, missing headers, or stale timestamp.
type WebhookSignatureError struct {
	Message string
	Cause   error
}

func (e *WebhookSignatureError) Error() string {
	return "mintarex: webhook signature error: " + e.Message
}

func (e *WebhookSignatureError) Unwrap() error { return e.Cause }

// ConfigurationError is returned when the SDK is mis-configured (missing
// APIKey/APISecret, invalid BaseURL, etc.).
type ConfigurationError struct {
	Message string
}

func (e *ConfigurationError) Error() string {
	return "mintarex: configuration error: " + e.Message
}

// errorFromResponse maps an HTTP status + API error code into the most
// specific typed error. Preferring the API's code over the HTTP status when
// the code pinpoints a narrower case (e.g. insufficient_balance within 400).
func errorFromResponse(base APIError) error {
	switch base.Code {
	case "insufficient_balance":
		return &InsufficientBalanceError{APIError: base}
	case "quote_expired_or_not_found":
		return &QuoteExpiredError{APIError: base}
	}
	switch {
	case base.Status == 400:
		return &ValidationError{APIError: base}
	case base.Status == 401:
		return &AuthenticationError{APIError: base}
	case base.Status == 403:
		return &PermissionError{APIError: base}
	case base.Status == 404:
		return &NotFoundError{APIError: base}
	case base.Status == 409:
		return &ConflictError{APIError: base}
	case base.Status == 410:
		return &QuoteExpiredError{APIError: base}
	case base.Status == 429:
		return &RateLimitError{APIError: base}
	case base.Status == 503:
		return &ServiceUnavailableError{APIError: base}
	case base.Status >= 500:
		return &ServerError{APIError: base}
	}
	return &base
}

// IsRetryable reports whether err represents a transient API condition that
// the caller could retry (429 or 503). Useful if you want to handle retries
// yourself instead of letting the Client retry automatically.
func IsRetryable(err error) bool {
	var rl *RateLimitError
	if errors.As(err, &rl) {
		return true
	}
	var su *ServiceUnavailableError
	return errors.As(err, &su)
}
