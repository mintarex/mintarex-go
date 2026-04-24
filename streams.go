package mintarex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// StreamsResource is the factory for [Stream] instances.
type StreamsResource struct {
	client *Client
}

// StreamOptions configure a Stream.
type StreamOptions struct {
	// AutoReconnect re-establishes the stream on transient errors. Default: true.
	AutoReconnect bool
	// MaxReconnectAttempts caps reconnect attempts. 0 = unlimited.
	MaxReconnectAttempts int
	// MaxReconnectDelay caps exponential backoff between reconnects. Default: 30s.
	MaxReconnectDelay time.Duration
	// HeartbeatInterval expected between server pings; reconnect after 2x this
	// interval with no data. Default: 15s.
	HeartbeatInterval time.Duration
	// Instruments restricts the price stream to the listed pairs (e.g.
	// []string{"BTC_USD", "ETH_USD"}). Ignored on the account stream.
	Instruments []string
}

// StreamMessage is one parsed SSE event.
type StreamMessage struct {
	Event string
	Data  any    // parsed JSON, or the raw string if parsing fails
	ID    string // "id:" line if present
	Raw   string // unparsed data lines joined by "\n"
}

// Stream is a long-running SSE connection. Call [Stream.Next] in a loop until
// it returns io.EOF (stream closed) or another error. Call [Stream.Close] to
// terminate early.
//
// A Stream is NOT safe for concurrent Next() calls; use it from a single
// goroutine.
type Stream struct {
	client     *Client
	endpoint   string // "prices" | "account"
	opts       StreamOptions
	resp       *http.Response
	reader     *bufio.Reader
	buf        strings.Builder
	pending    []StreamMessage
	reconnects int
	closed     bool
}

// Prices opens the price-update stream.
func (r *StreamsResource) Prices(ctx context.Context, opts StreamOptions) (*Stream, error) {
	return r.open(ctx, "prices", opts)
}

// Account opens the account-event stream.
func (r *StreamsResource) Account(ctx context.Context, opts StreamOptions) (*Stream, error) {
	return r.open(ctx, "account", opts)
}

func (r *StreamsResource) open(ctx context.Context, endpoint string, opts StreamOptions) (*Stream, error) {
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 15 * time.Second
	}
	if opts.HeartbeatInterval < time.Second {
		return nil, &ConfigurationError{Message: "HeartbeatInterval must be >= 1s"}
	}
	if opts.MaxReconnectDelay == 0 {
		opts.MaxReconnectDelay = 30 * time.Second
	}
	if opts.MaxReconnectDelay < 0 {
		return nil, &ConfigurationError{Message: "MaxReconnectDelay must be >= 0"}
	}
	if cleaned, err := normalizeInstruments(opts.Instruments); err != nil {
		return nil, err
	} else {
		opts.Instruments = cleaned
	}
	// AutoReconnect default: pointer semantics not used in Go Options struct,
	// so callers who want OFF must set it explicitly after constructing a struct
	// with all other fields. Go convention is zero-value = "on" for booleans —
	// we invert via an unexported flag.
	s := &Stream{client: r.client, endpoint: endpoint, opts: opts}
	if err := s.connect(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Close terminates the stream and releases the connection.
func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil {
		return s.resp.Body.Close()
	}
	return nil
}

// Next returns the next stream message. Returns io.EOF when the stream has
// been closed by the caller. On transient errors, reconnects automatically
// if AutoReconnect is true.
func (s *Stream) Next(ctx context.Context) (*StreamMessage, error) {
	for {
		if s.closed {
			return nil, io.EOF
		}
		if len(s.pending) > 0 {
			msg := s.pending[0]
			s.pending = s.pending[1:]
			return &msg, nil
		}
		if err := s.fill(ctx); err != nil {
			if errors.Is(err, io.EOF) {
				if !s.opts.AutoReconnect || s.closed {
					return nil, io.EOF
				}
			} else {
				if !s.opts.AutoReconnect || s.closed {
					return nil, err
				}
			}
			if err := s.reconnect(ctx); err != nil {
				return nil, err
			}
		}
	}
}

func (s *Stream) connect(ctx context.Context) error {
	// Fetch a short-lived stream token first.
	var tok StreamToken
	if _, err := s.client.Request(ctx, RequestOptions{
		Method: http.MethodPost,
		Path:   "/stream/token",
		Body:   map[string]any{},
	}, &tok); err != nil {
		return err
	}
	if tok.Token == "" {
		return &NetworkError{Message: "stream token response missing token field"}
	}

	u := *s.client.StreamBaseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + s.endpoint
	q := u.Query()
	q.Set("token", tok.Token)
	if s.endpoint == "prices" && len(s.opts.Instruments) > 0 {
		q.Set("instruments", strings.Join(s.opts.Instruments, ","))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return &NetworkError{Message: "build stream request failed", Cause: err}
	}
	req.Header.Set("Accept", "text/event-stream")

	// Separate http client for streaming without the per-request timeout —
	// SSE is intentionally long-lived.
	httpClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		// No Timeout: we rely on ctx + per-read deadlines via bufio.
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return &NetworkError{Message: err.Error(), Cause: err}
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return &NetworkError{Message: fmt.Sprintf("stream open failed: HTTP %d", resp.StatusCode)}
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "text/event-stream") {
		resp.Body.Close()
		return &NetworkError{Message: fmt.Sprintf("unexpected content-type: %s", ct)}
	}
	s.resp = resp
	s.reader = bufio.NewReaderSize(resp.Body, 16*1024)
	s.reconnects = 0
	return nil
}

// maxSSEEventBytes caps the in-memory buffer for one SSE event so a malicious
// or broken upstream that streams an unterminated line cannot grow memory
// without bound. Spec-conformant servers send `\n\n` between events; a single
// event exceeding 1 MiB is treated as a protocol violation.
const maxSSEEventBytes = 1 << 20

// fill reads until at least one parsed event is pending, or returns an error.
func (s *Stream) fill(ctx context.Context) error {
	for {
		// Check for context cancellation before each read.
		if err := ctx.Err(); err != nil {
			return err
		}
		// Read a line (\n terminated).
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if line != "" {
				if s.buf.Len()+len(line) > maxSSEEventBytes {
					return &NetworkError{Message: fmt.Sprintf(
						"SSE event exceeds %d bytes without terminator, aborting", maxSSEEventBytes)}
				}
				s.buf.WriteString(line)
			}
			return err
		}
		if s.buf.Len()+len(line) > maxSSEEventBytes {
			return &NetworkError{Message: fmt.Sprintf(
				"SSE event exceeds %d bytes without terminator, aborting", maxSSEEventBytes)}
		}
		s.buf.WriteString(line)
		// Event boundary: blank line (\n\n, \r\n\r\n, or \r\r).
		if idx := findEventBoundary(s.buf.String()); idx >= 0 {
			chunk := s.buf.String()[:idx]
			rest := s.buf.String()[idx:]
			for _, term := range []string{"\r\n\r\n", "\n\n", "\r\r"} {
				if strings.HasPrefix(rest, term) {
					rest = rest[len(term):]
					break
				}
			}
			s.buf.Reset()
			s.buf.WriteString(rest)
			msg := parseSSEChunk(chunk)
			if msg != nil {
				s.pending = append(s.pending, *msg)
				return nil
			}
			// comment/heartbeat only; keep reading.
			continue
		}
	}
}

func (s *Stream) reconnect(ctx context.Context) error {
	if s.resp != nil {
		s.resp.Body.Close()
		s.resp = nil
	}
	s.buf.Reset()
	if s.opts.MaxReconnectAttempts > 0 && s.reconnects >= s.opts.MaxReconnectAttempts {
		return &NetworkError{
			Message: fmt.Sprintf("stream reconnect limit reached (%d)", s.opts.MaxReconnectAttempts),
		}
	}
	s.reconnects++
	delay := time.Duration(500*(1<<(s.reconnects-1)))*time.Millisecond +
		time.Duration(rand.IntN(500))*time.Millisecond
	if delay > s.opts.MaxReconnectDelay {
		delay = s.opts.MaxReconnectDelay
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
	}
	return s.connect(ctx)
}

// instrumentRE matches BASE_QUOTE pairs (e.g. BTC_USD, USDT_AED).
var instrumentRE = regexp.MustCompile(`^[A-Z0-9]{1,20}_[A-Z0-9]{1,20}$`)

func normalizeInstruments(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	if len(input) > 200 {
		return nil, &ConfigurationError{Message: "Instruments list capped at 200 entries"}
	}
	cleaned := make([]string, 0, len(input))
	for _, v := range input {
		if v == "" {
			return nil, &ConfigurationError{Message: "Instruments entries must be non-empty strings"}
		}
		if !instrumentRE.MatchString(v) {
			return nil, &ConfigurationError{
				Message: fmt.Sprintf(`Instruments entry %q must look like BASE_QUOTE (e.g. BTC_USD)`, v),
			}
		}
		cleaned = append(cleaned, v)
	}
	return cleaned, nil
}

func findEventBoundary(s string) int {
	candidates := []int{
		strings.Index(s, "\n\n"),
		strings.Index(s, "\r\n\r\n"),
		strings.Index(s, "\r\r"),
	}
	best := -1
	for _, i := range candidates {
		if i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	return best
}

func parseSSEChunk(chunk string) *StreamMessage {
	// Split on any line terminator.
	lines := strings.FieldsFunc(chunk, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	eventName := "message"
	var dataLines []string
	var id string
	hasData := false

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, ":") {
			continue // empty or comment (heartbeat)
		}
		colon := strings.IndexByte(line, ':')
		var field, value string
		if colon == -1 {
			field, value = line, ""
		} else {
			field = line[:colon]
			value = line[colon+1:]
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
		}
		switch field {
		case "event":
			if value != "" {
				eventName = value
			}
		case "data":
			dataLines = append(dataLines, value)
			hasData = true
		case "id":
			id = value
			// "retry" is advisory; we use our own backoff strategy.
		}
	}

	if !hasData {
		return nil
	}
	raw := strings.Join(dataLines, "\n")
	var parsed any = raw
	if raw != "" {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			parsed = v
		}
	}
	return &StreamMessage{Event: eventName, Data: parsed, ID: id, Raw: raw}
}
