package mintarex

import (
	"context"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

// RFQResource is the /rfq namespace: request-for-quote + accept.
type RFQResource struct {
	client *Client
}

// Quote requests a short-lived quote (30s validity).
//
// Quote can be fiat (crypto-fiat trade) or crypto (crypto-crypto swap). The
// SDK only validates the code format; the server classifies the pair and
// rejects unsupported combinations with a specific error.
func (r *RFQResource) Quote(ctx context.Context, req QuoteRequest) (*Quote, error) {
	base, err := assertCoin(req.Base, "Base")
	if err != nil {
		return nil, err
	}
	quote, err := assertCurrencyCode(req.Quote, "Quote")
	if err != nil {
		return nil, err
	}
	side, err := assertSide(req.Side, "Side")
	if err != nil {
		return nil, err
	}
	amount, err := assertAmount(req.Amount, "Amount")
	if err != nil {
		return nil, err
	}
	amountType, err := assertAmountType(req.AmountType, "AmountType")
	if err != nil {
		return nil, err
	}

	body := map[string]string{
		"base":        base,
		"quote":       quote,
		"side":        side,
		"amount":      amount,
		"amount_type": amountType,
	}
	if req.Network != "" {
		v, err := assertNetwork(req.Network, "Network")
		if err != nil {
			return nil, err
		}
		body["network"] = v
	}
	if req.FromNetwork != "" {
		v, err := assertNetwork(req.FromNetwork, "FromNetwork")
		if err != nil {
			return nil, err
		}
		body["from_network"] = v
	}
	if req.ToNetwork != "" {
		v, err := assertNetwork(req.ToNetwork, "ToNetwork")
		if err != nil {
			return nil, err
		}
		body["to_network"] = v
	}

	out := &Quote{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodPost,
		Path:   "/rfq",
		Body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// AcceptOptions optionally supplies an idempotency key when accepting a
// quote. If IdempotencyKey is empty, a UUIDv4 is auto-generated so callers
// get safe retry semantics on network errors.
type AcceptOptions struct {
	IdempotencyKey string
}

// Accept executes a quote. See [AcceptOptions] for idempotency semantics.
func (r *RFQResource) Accept(ctx context.Context, quoteID string, opts AcceptOptions) (*TradeExecution, error) {
	qid, err := assertUUID(quoteID, "quote_id")
	if err != nil {
		return nil, err
	}
	key := opts.IdempotencyKey
	if key == "" {
		key = uuid.NewString()
	} else if _, err := assertIdempotencyKey(key, "idempotency_key"); err != nil {
		return nil, err
	}
	body := map[string]string{"idempotency_key": key}

	// We want network-error retries even though the body is a typed map[string]string
	// (not map[string]any, which is what resolveRetryOnNetworkError sniffs). Force it.
	retry := true
	out := &TradeExecution{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method:              http.MethodPost,
		Path:                "/rfq/" + url.PathEscape(qid) + "/accept",
		Body:                body,
		RetryOnNetworkError: &retry,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}
