<p align="center">
  <a href="https://mintarex.com">
    <img src="https://mintarex.com/mintarex.svg" alt="Mintarex" width="320" />
  </a>
</p>

<h1 align="center">mintarex-go</h1>

<p align="center">
  Official Go SDK for the <a href="https://developers.mintarex.com">Mintarex Corporate OTC API</a>.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/mintarex/mintarex-go"><img src="https://pkg.go.dev/badge/github.com/mintarex/mintarex-go.svg" alt="Go Reference" /></a>
  <a href="https://github.com/mintarex/mintarex-go/releases"><img src="https://img.shields.io/github/v/tag/mintarex/mintarex-go?label=version&style=flat-square" alt="version" /></a>
  <a href="https://github.com/mintarex/mintarex-go/blob/main/LICENSE"><img src="https://img.shields.io/github/license/mintarex/mintarex-go?style=flat-square" alt="MIT License" /></a>
  <a href="https://go.dev"><img src="https://img.shields.io/github/go-mod/go-version/mintarex/mintarex-go?style=flat-square" alt="Go version" /></a>
</p>

---

- HMAC-SHA256 request signing (automatic)
- Typed errors per API error code — unwrap with `errors.As`
- RFQ trading, crypto deposits/withdrawals, webhooks, real-time SSE streams
- Webhook signature verification helper
- Built for Go 1.24+ using only stdlib + `github.com/google/uuid`
- Context-aware: every request accepts `context.Context`

## Installation

```bash
go get github.com/mintarex/mintarex-go
```

Go 1.24+ required.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/mintarex/mintarex-go"
)

func main() {
    mx, err := mintarex.New(mintarex.Options{
        APIKey:    os.Getenv("MINTAREX_API_KEY"),    // mxn_live_... or mxn_test_...
        APISecret: os.Getenv("MINTAREX_API_SECRET"),
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Request a quote
    quote, err := mx.RFQ.Quote(ctx, mintarex.QuoteRequest{
        Base:       "BTC",
        Quote:      "USD",
        Side:       "buy",
        Amount:     "0.5",
        AmountType: "base",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("quote_id=%s price=%s\n", quote.QuoteID, quote.Price)

    // Accept — idempotency_key auto-generated if AcceptOptions.IdempotencyKey is empty
    trade, err := mx.RFQ.Accept(ctx, quote.QuoteID, mintarex.AcceptOptions{})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("filled: %s %s\n", trade.TradeID, trade.Status)
}
```

## Environments

The environment is inferred from the key prefix:

| Key prefix | Environment |
|-----------|-------------|
| `mxn_live_...` | `EnvLive` |
| `mxn_test_...` | `EnvSandbox` |

Override explicitly via `Options.Environment`.

## Error handling

All SDK errors satisfy the `error` interface; use `errors.As` to extract the
concrete type and inspect fields like `RetryAfterMs` or `RateLimit`.

```go
import "errors"

trade, err := mx.RFQ.Accept(ctx, quoteID, mintarex.AcceptOptions{})

var rl *mintarex.RateLimitError
if errors.As(err, &rl) {
    fmt.Printf("rate limited, retry after %d ms (remaining=%v)\n",
        rl.RetryAfterMs, rl.RateLimit.Remaining)
}

var ib *mintarex.InsufficientBalanceError
if errors.As(err, &ib) {
    fmt.Println("top up the account")
}

var qe *mintarex.QuoteExpiredError
if errors.As(err, &qe) {
    // re-quote and retry
}
```

Error taxonomy (all wrap `*APIError`): `AuthenticationError` (401),
`PermissionError` (403), `ValidationError` (400), `InsufficientBalanceError`,
`NotFoundError` (404), `ConflictError` (409), `QuoteExpiredError` (410),
`RateLimitError` (429), `ServerError` (5xx), `ServiceUnavailableError` (503).
Plus `NetworkError` (no HTTP response), `WebhookSignatureError`, and
`ConfigurationError`.

## Streaming (SSE)

```go
stream, err := mx.Streams.Prices(ctx, mintarex.StreamOptions{AutoReconnect: true})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for {
    msg, err := stream.Next(ctx)
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%s: %v\n", msg.Event, msg.Data)
}
```

Reconnect on transient errors is automatic; a watchdog fires if no data
arrives within 2× the heartbeat interval. Call `stream.Close()` to stop.

## Webhook verification

```go
import (
    "io"
    "net/http"

    "github.com/mintarex/mintarex-go"
)

func handler(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "", http.StatusBadRequest)
        return
    }
    event, err := mintarex.VerifyWebhook(mintarex.VerifyParams{
        Body:    body,                    // exact raw bytes, NOT parsed JSON
        Headers: r.Header,
        Secret:  os.Getenv("MINTAREX_WEBHOOK_SECRET"),
    })
    if err != nil {
        http.Error(w, "bad signature", http.StatusBadRequest)
        return
    }
    if event.EventType == "trade.executed" {
        // handle event.Data
    }
    w.WriteHeader(http.StatusNoContent)
}
```

## Configuration

```go
mx, err := mintarex.New(mintarex.Options{
    APIKey:        "...",
    APISecret:     "...",
    Environment:   mintarex.EnvSandbox, // optional — inferred from key prefix
    BaseURL:       "https://institutional.mintarex.com/v1",   // optional
    StreamBaseURL: "https://institutional.mintarex.com/v1/stream",
    Timeout:       30 * time.Second, // per-request timeout
    MaxRetries:    3,                 // for 429/503 + network errors
    UserAgent:     "my-app/1.0",      // appended to default UA
    HTTPClient:    nil,               // provide a custom *http.Client if you need a proxy
})
```

`http://` URLs are permitted only for `localhost` / `127.0.0.1` / `::1`
(dev and test scenarios). Public hosts must use HTTPS.

## Resources

| Namespace | Methods |
|-----------|---------|
| `mx.Account` | `Balances`, `Balance`, `Limits` |
| `mx.RFQ` | `Quote`, `Accept` |
| `mx.Trades` | `List`, `Get` |
| `mx.Crypto` | `DepositAddress`, `Deposits`, `Withdraw`, `Withdrawals`, `GetWithdrawal`, `Addresses.{List,Add,Remove}` |
| `mx.Webhooks` | `Create`, `List`, `Remove` |
| `mx.Streams` | `Prices`, `Account` |
| `mx.Public` | `Instruments`, `Networks`, `Fees` |

## Support

- **API Docs**: https://developers.mintarex.com
- **Issues**: https://github.com/mintarex/mintarex-go/issues
- **Contact**: support@mintarex.com

## License

MIT © [Mintarex](https://mintarex.com)

<p align="center">
  <img src="https://mintarex.com/ICON-512X512.png" alt="Mintarex" width="64" />
</p>
