package mintarex

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// TradesResource is the /trades namespace: trade history.
type TradesResource struct {
	client *Client
}

// TradeListParams filters the trade list.
type TradeListParams struct {
	Limit  int    // 1..200; 0 = omit
	Offset int    // 0..2_000_000; 0 = omit from query
	Sort   string // "asc" or "desc"
	Base   string
	Quote  string
	Side   string // "buy" | "sell"
	Status string // "filled" | "pending" | "cancelled" | "failed" | "expired"
	From   string // ISO-8601 or any server-accepted timestamp
	To     string
}

// List returns historical trades.
func (r *TradesResource) List(ctx context.Context, p TradeListParams) (*TradesList, error) {
	q := map[string]string{}
	if p.Limit > 0 {
		q["limit"] = strconv.Itoa(clampInt(p.Limit, 1, 200))
	}
	if p.Offset > 0 {
		q["offset"] = strconv.Itoa(clampInt(p.Offset, 0, 2_000_000))
	}
	if p.Sort != "" {
		if p.Sort != "asc" {
			q["sort"] = "desc"
		} else {
			q["sort"] = "asc"
		}
	}
	if p.Base != "" {
		v, err := assertCurrencyCode(p.Base, "Base")
		if err != nil {
			return nil, err
		}
		q["base"] = v
	}
	if p.Quote != "" {
		v, err := assertCurrencyCode(p.Quote, "Quote")
		if err != nil {
			return nil, err
		}
		q["quote"] = v
	}
	if p.Side != "" {
		v, err := assertSide(p.Side, "Side")
		if err != nil {
			return nil, err
		}
		q["side"] = v
	}
	if p.Status != "" {
		q["status"] = p.Status
	}
	if p.From != "" {
		q["from"] = p.From
	}
	if p.To != "" {
		q["to"] = p.To
	}
	out := &TradesList{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/trades",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// Get fetches a single trade by its UUID.
func (r *TradesResource) Get(ctx context.Context, tradeUUID string) (*Trade, error) {
	id, err := assertUUID(tradeUUID, "trade_uuid")
	if err != nil {
		return nil, err
	}
	out := &Trade{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/trades/" + url.PathEscape(id),
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
