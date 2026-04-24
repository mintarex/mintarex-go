package mintarex

import (
	"context"
	"net/http"
)

// PublicResource exposes public reference data: instruments, networks, fees.
type PublicResource struct {
	client *Client
}

// Instruments returns all tradable instruments.
func (r *PublicResource) Instruments(ctx context.Context) (*InstrumentsResponse, error) {
	out := &InstrumentsResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/instruments",
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// NetworksParams optionally filters the network list to a single coin.
type NetworksParams struct {
	Coin string
}

// Networks returns supported networks.
func (r *PublicResource) Networks(ctx context.Context, p NetworksParams) (*NetworksResponse, error) {
	q := map[string]string{}
	if p.Coin != "" {
		v, err := assertCoin(p.Coin, "Coin")
		if err != nil {
			return nil, err
		}
		q["coin"] = v
	}
	out := &NetworksResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/networks",
		Query:  q,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// Fees returns the public fee schedule (not account-specific).
func (r *PublicResource) Fees(ctx context.Context) (*PublicFees, error) {
	out := &PublicFees{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/fees",
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}
