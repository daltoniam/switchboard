package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func getTailnetSettings(ctx context.Context, t *tailscale, _ map[string]any) (*mcp.ToolResult, error) {
	return rawResult(t.doJSON(ctx, http.MethodGet, t.tailnetPath("/settings"), nil))
}

func updateTailnetSettings(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	settings := r.Map("settings")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if len(settings) == 0 {
		return mcp.ErrResult(fmt.Errorf("settings is required: object of tailnet setting fields to change (e.g. {\"devicesApprovalOn\": true})"))
	}
	return rawResult(t.doJSON(ctx, http.MethodPatch, t.tailnetPath("/settings"), settings))
}

func webhookPath(id, suffix string) string {
	return "/webhooks/" + url.PathEscape(id) + suffix
}

// listWebhooks strips signing secrets. Scripts bypass compaction, so the
// handler is the only place that guarantees secrets never leave the adapter.
func listWebhooks(ctx context.Context, t *tailscale, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := t.doJSON(ctx, http.MethodGet, t.tailnetPath("/webhooks"), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("tailscale: decode webhooks response: %w", err))
	}
	hooks, _ := doc["webhooks"].([]any)
	for _, h := range hooks {
		if hook, ok := h.(map[string]any); ok {
			delete(hook, "secret")
		}
	}
	return mcp.JSONResult(doc)
}

func createWebhook(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	endpointURL := r.Str("endpoint_url")
	subscriptions := r.StrSlice("subscriptions")
	providerType := r.Str("provider_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("endpoint_url", endpointURL); err != nil {
		return mcp.ErrResult(err)
	}
	if len(subscriptions) == 0 {
		return mcp.ErrResult(fmt.Errorf("subscriptions is required: list of event types such as nodeCreated, nodeNeedsApproval, policyUpdate"))
	}
	body := map[string]any{
		"endpointUrl":   endpointURL,
		"subscriptions": subscriptions,
	}
	if providerType != "" {
		body["providerType"] = providerType
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, t.tailnetPath("/webhooks"), body))
}

func deleteWebhook(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return webhookAction(ctx, t, args, http.MethodDelete, "")
}

func testWebhook(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return webhookAction(ctx, t, args, http.MethodPost, "/test")
}

func webhookAction(ctx context.Context, t *tailscale, args map[string]any, method, suffix string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("webhook_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("webhook_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, method, webhookPath(id, suffix), nil))
}

func getContacts(ctx context.Context, t *tailscale, _ map[string]any) (*mcp.ToolResult, error) {
	return rawResult(t.doJSON(ctx, http.MethodGet, t.tailnetPath("/contacts"), nil))
}
