package mintarex

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EmptyBodySHA256 is the SHA-256 hex digest of the empty string. Used as the
// body hash for GET/DELETE requests and POSTs without a body.
const EmptyBodySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// SignedHeaders are the four auth headers required by the Mintarex API.
type SignedHeaders struct {
	APIKey    string // MX-API-KEY
	Signature string // MX-SIGNATURE
	Timestamp string // MX-TIMESTAMP
	Nonce     string // MX-NONCE
}

// Map returns the headers as a map ready to attach to an HTTP request.
func (s SignedHeaders) Map() map[string]string {
	return map[string]string{
		"MX-API-KEY":   s.APIKey,
		"MX-SIGNATURE": s.Signature,
		"MX-TIMESTAMP": s.Timestamp,
		"MX-NONCE":     s.Nonce,
	}
}

// SignParams are the inputs to [Sign].
//
// Leave Timestamp and Nonce zero-valued in production; they will be
// auto-generated (Unix seconds, UUID v4). Set them explicitly for tests.
type SignParams struct {
	APIKey    string
	APISecret string
	Method    string // "GET", "POST", etc.
	Path      string // must include query string if any (e.g. "/v1/trades?limit=10")
	Body      []byte // nil or empty ⇒ uses [EmptyBodySHA256]
	Timestamp string // optional; override for tests
	Nonce     string // optional; override for tests
}

// BuildCanonicalString assembles the canonical string fed into the HMAC. The
// format matches the Mintarex gateway verifier exactly:
//
//	METHOD\nPATH\nTIMESTAMP\nNONCE\nSHA256_HEX(body)
func BuildCanonicalString(method, path, timestamp, nonce, bodyHash string) string {
	var b strings.Builder
	b.Grow(len(method) + len(path) + len(timestamp) + len(nonce) + len(bodyHash) + 4)
	b.WriteString(strings.ToUpper(method))
	b.WriteByte('\n')
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString(timestamp)
	b.WriteByte('\n')
	b.WriteString(nonce)
	b.WriteByte('\n')
	b.WriteString(bodyHash)
	return b.String()
}

// SHA256Hex returns the lowercase hex SHA-256 digest of body.
func SHA256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// HMACSign returns the lowercase hex HMAC-SHA256 of canonical under secret.
func HMACSign(secret, canonical string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(canonical))
	return hex.EncodeToString(h.Sum(nil))
}

// Sign produces the four auth headers for a request. Caller is responsible
// for ensuring path includes any query string and body matches the exact
// bytes being sent on the wire.
func Sign(p SignParams) SignedHeaders {
	ts := p.Timestamp
	if ts == "" {
		ts = strconv.FormatInt(time.Now().Unix(), 10)
	}
	nonce := p.Nonce
	if nonce == "" {
		nonce = uuid.NewString()
	}

	bodyHash := EmptyBodySHA256
	if len(p.Body) > 0 {
		bodyHash = SHA256Hex(p.Body)
	}

	canonical := BuildCanonicalString(p.Method, p.Path, ts, nonce, bodyHash)
	sig := HMACSign(p.APISecret, canonical)

	return SignedHeaders{
		APIKey:    p.APIKey,
		Signature: sig,
		Timestamp: ts,
		Nonce:     nonce,
	}
}
