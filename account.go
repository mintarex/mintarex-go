package mintarex

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
)

// AccountResource is the /account/* namespace: balances, fees, limits.
type AccountResource struct {
	client *Client
}

// BalancesParams filters the balance list.
type BalancesParams struct {
	CurrencyType CurrencyType // optional: "fiat" or "crypto"
	IncludeEmpty bool         // whether to include zero balances
}

// Balances returns balances across all currencies the account holds.
func (r *AccountResource) Balances(ctx context.Context, params BalancesParams) (*BalancesResponse, error) {
	q := map[string]string{}
	if params.CurrencyType != "" {
		q["currency_type"] = string(params.CurrencyType)
	}
	if params.IncludeEmpty {
		q["include_empty"] = "true"
	}
	out := &BalancesResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/account/balances",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

var fiat3RE = regexp.MustCompile(`^[A-Z]{3,10}$`)

// Balance returns the aggregated balance for a single currency (fiat or crypto).
func (r *AccountResource) Balance(ctx context.Context, currency string) (*SingleBalanceResponse, error) {
	var (
		c   string
		err error
	)
	if fiat3RE.MatchString(currency) {
		c, err = assertFiatCurrency(currency, "currency")
	} else {
		c, err = assertCoin(currency, "currency")
	}
	if err != nil {
		return nil, err
	}
	out := &SingleBalanceResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/account/balance/" + url.PathEscape(c),
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}


// Limits returns daily/monthly deposit + withdrawal limits for this account.
func (r *AccountResource) Limits(ctx context.Context) (*LimitsResponse, error) {
	out := &LimitsResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/account/limits",
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}
