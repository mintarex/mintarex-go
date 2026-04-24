package mintarex

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
)

// CryptoResource is the /crypto/* namespace: deposits, withdrawals, addresses.
type CryptoResource struct {
	client    *Client
	Addresses *CryptoAddressesResource
}

// CryptoAddressesResource is the /crypto/withdrawal-addresses subresource.
type CryptoAddressesResource struct {
	client *Client
}

// DepositAddressParams are the inputs to [CryptoResource.DepositAddress].
type DepositAddressParams struct {
	Coin    string // required
	Network string // optional
}

// DepositAddress returns a deposit address for the given coin.
func (r *CryptoResource) DepositAddress(ctx context.Context, p DepositAddressParams) (*DepositAddress, error) {
	coin, err := assertCoin(p.Coin, "Coin")
	if err != nil {
		return nil, err
	}
	q := map[string]string{"coin": coin}
	if p.Network != "" {
		n, err := assertNetwork(p.Network, "Network")
		if err != nil {
			return nil, err
		}
		q["network"] = n
	}
	out := &DepositAddress{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/crypto/deposit-address",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// DepositListParams filters the crypto deposit list.
type DepositListParams struct {
	Coin   string
	Status string
	From   string
	To     string
	Limit  int
	Offset int
}

// Deposits lists detected / confirmed crypto deposits.
func (r *CryptoResource) Deposits(ctx context.Context, p DepositListParams) (*CryptoDepositsList, error) {
	q, err := buildCryptoListQuery(p.Coin, p.Status, p.From, p.To, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	out := &CryptoDepositsList{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/crypto/deposits",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// WithdrawRequest is the input to [CryptoResource.Withdraw].
type WithdrawRequest struct {
	Coin           string // required
	Network        string // required
	Amount         string // required, decimal string
	Address        string // required (must be on allowlist)
	AddressTag     string // optional (memo / destination tag / etc.)
	IdempotencyKey string // optional — auto-generated if empty
}

// Withdraw submits a crypto withdrawal.
func (r *CryptoResource) Withdraw(ctx context.Context, req WithdrawRequest) (*CryptoWithdrawal, error) {
	coin, err := assertCoin(req.Coin, "Coin")
	if err != nil {
		return nil, err
	}
	network, err := assertNetwork(req.Network, "Network")
	if err != nil {
		return nil, err
	}
	amount, err := assertAmount(req.Amount, "Amount")
	if err != nil {
		return nil, err
	}
	address, err := assertAddress(req.Address, "Address")
	if err != nil {
		return nil, err
	}
	key := req.IdempotencyKey
	if key == "" {
		key = uuid.NewString()
	} else if _, err := assertIdempotencyKey(key, "idempotency_key"); err != nil {
		return nil, err
	}

	body := map[string]string{
		"coin":            coin,
		"network":         network,
		"amount":          amount,
		"address":         address,
		"idempotency_key": key,
	}
	if req.AddressTag != "" {
		tag, err := assertAddressTag(req.AddressTag, "AddressTag")
		if err != nil {
			return nil, err
		}
		body["address_tag"] = tag
	}

	retry := true
	out := &CryptoWithdrawal{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method:              http.MethodPost,
		Path:                "/crypto/withdraw",
		Body:                body,
		RetryOnNetworkError: &retry,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// WithdrawalListParams filters the crypto withdrawal list.
type WithdrawalListParams struct {
	Coin   string
	Status string
	From   string
	To     string
	Limit  int
	Offset int
}

// Withdrawals lists crypto withdrawals.
func (r *CryptoResource) Withdrawals(ctx context.Context, p WithdrawalListParams) (*CryptoWithdrawalsList, error) {
	q, err := buildCryptoListQuery(p.Coin, p.Status, p.From, p.To, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	out := &CryptoWithdrawalsList{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/crypto/withdrawals",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// GetWithdrawal fetches a single crypto withdrawal by UUID.
func (r *CryptoResource) GetWithdrawal(ctx context.Context, withdrawalUUID string) (*CryptoWithdrawal, error) {
	id, err := assertUUID(withdrawalUUID, "withdrawal_uuid")
	if err != nil {
		return nil, err
	}
	out := &CryptoWithdrawal{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/crypto/withdrawals/" + url.PathEscape(id),
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// AddressListParams filters the withdrawal-address list.
type AddressListParams struct {
	Currency string
	Network  string
	Status   string
	Limit    int
	Offset   int
}

// List returns withdrawal addresses on file.
func (r *CryptoAddressesResource) List(ctx context.Context, p AddressListParams) (*WithdrawalAddressesList, error) {
	q := map[string]string{}
	if p.Currency != "" {
		v, err := assertCoin(p.Currency, "Currency")
		if err != nil {
			return nil, err
		}
		q["currency"] = v
	}
	if p.Network != "" {
		v, err := assertNetwork(p.Network, "Network")
		if err != nil {
			return nil, err
		}
		q["network"] = v
	}
	if p.Status != "" {
		q["status"] = p.Status
	}
	if p.Limit > 0 {
		q["limit"] = strconv.Itoa(clampInt(p.Limit, 1, 200))
	}
	if p.Offset > 0 {
		q["offset"] = strconv.Itoa(clampInt(p.Offset, 0, 2_000_000))
	}
	out := &WithdrawalAddressesList{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/crypto/withdrawal-addresses",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// AddressAddRequest is the input to [CryptoAddressesResource.Add].
type AddressAddRequest struct {
	Currency   string
	Network    string
	Address    string
	Label      string
	AddressTag string // optional
}

// Add adds a withdrawal address (requires email confirmation).
func (r *CryptoAddressesResource) Add(ctx context.Context, req AddressAddRequest) (*AddressAddResponse, error) {
	currency, err := assertCoin(req.Currency, "Currency")
	if err != nil {
		return nil, err
	}
	network, err := assertNetwork(req.Network, "Network")
	if err != nil {
		return nil, err
	}
	address, err := assertAddress(req.Address, "Address")
	if err != nil {
		return nil, err
	}
	label, err := assertLabel(req.Label, "Label")
	if err != nil {
		return nil, err
	}
	body := map[string]string{
		"currency": currency,
		"network":  network,
		"address":  address,
		"label":    label,
	}
	if req.AddressTag != "" {
		tag, err := assertAddressTag(req.AddressTag, "AddressTag")
		if err != nil {
			return nil, err
		}
		body["address_tag"] = tag
	}
	out := &AddressAddResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodPost,
		Path:   "/crypto/withdrawal-addresses",
		Body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// Remove revokes a withdrawal address by UUID.
func (r *CryptoAddressesResource) Remove(ctx context.Context, addressUUID string) (*AddressRemoveResponse, error) {
	id, err := assertUUID(addressUUID, "address_uuid")
	if err != nil {
		return nil, err
	}
	out := &AddressRemoveResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodDelete,
		Path:   "/crypto/withdrawal-addresses/" + url.PathEscape(id),
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

func buildCryptoListQuery(coin, status, from, to string, limit, offset int) (map[string]string, error) {
	q := map[string]string{}
	if coin != "" {
		v, err := assertCoin(coin, "Coin")
		if err != nil {
			return nil, err
		}
		q["coin"] = v
	}
	if status != "" {
		q["status"] = status
	}
	if from != "" {
		q["from"] = from
	}
	if to != "" {
		q["to"] = to
	}
	if limit > 0 {
		q["limit"] = strconv.Itoa(clampInt(limit, 1, 200))
	}
	if offset > 0 {
		q["offset"] = strconv.Itoa(clampInt(offset, 0, 2_000_000))
	}
	return q, nil
}
