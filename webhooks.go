package mintarex

import (
	"context"
	"net/http"
	"net/url"
)

// WebhooksResource is the /webhooks namespace: endpoint CRUD.
type WebhooksResource struct {
	client *Client
}

// WebhookCreateRequest is the input to [WebhooksResource.Create].
type WebhookCreateRequest struct {
	URL    string   // required, https only, no credentials
	Events []string // required, non-empty
	Label  string   // optional (max 100 printable ASCII chars)
}

// Create registers a new webhook endpoint.
func (r *WebhooksResource) Create(ctx context.Context, req WebhookCreateRequest) (*WebhookCreateResponse, error) {
	u, err := assertHTTPSURL(req.URL, "URL")
	if err != nil {
		return nil, err
	}
	events, err := assertEvents(req.Events, "Events")
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"url":    u,
		"events": events,
	}
	if req.Label != "" {
		label, err := assertLabel(req.Label, "Label")
		if err != nil {
			return nil, err
		}
		body["label"] = label
	}
	out := &WebhookCreateResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodPost,
		Path:   "/webhooks",
		Body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// List returns all registered webhook endpoints.
func (r *WebhooksResource) List(ctx context.Context) (*WebhooksListResponse, error) {
	out := &WebhooksListResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodGet,
		Path:   "/webhooks",
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}

// Remove deletes a webhook endpoint (may require email confirmation).
func (r *WebhooksResource) Remove(ctx context.Context, endpointUUID string) (*WebhookRemoveResponse, error) {
	id, err := assertUUID(endpointUUID, "endpoint_uuid")
	if err != nil {
		return nil, err
	}
	out := &WebhookRemoveResponse{}
	meta, err := r.client.Request(ctx, RequestOptions{
		Method: http.MethodDelete,
		Path:   "/webhooks/" + url.PathEscape(id),
	}, out)
	if err != nil {
		return nil, err
	}
	out.Meta = meta
	return out, nil
}
