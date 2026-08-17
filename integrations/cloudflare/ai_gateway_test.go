package cloudflare

import (
	"context"
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAIGateways(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai-gateway/gateways")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"my-gw","slug":"my-gw","collect_logs":true}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_ai_gateways", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-gw")
}

func TestGetAIGateway(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai-gateway/gateways/my-gw")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"my-gw","cache_ttl":3600,"rate_limiting_limit":100}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_ai_gateway", map[string]any{"gateway_id": "my-gw"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cache_ttl")
}

func TestListAIGatewayLogs_PassesFilters(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai-gateway/gateways/my-gw/logs")
		q := r.URL.Query()
		assert.Equal(t, "openai", q.Get("provider"))
		assert.Equal(t, "gpt-4o", q.Get("model"))
		assert.Equal(t, "true", q.Get("success"))
		assert.Equal(t, "2024-01-01T00:00:00Z", q.Get("start_date"))
		assert.Equal(t, "cost", q.Get("order_by"))
		assert.Equal(t, "desc", q.Get("order_by_direction"))
		assert.Equal(t, "50", q.Get("per_page"))
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"log1","provider":"openai","model":"gpt-4o","tokens_in":120,"tokens_out":45,"cost":0.0012,"cached":false}]}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_list_ai_gateway_logs", map[string]any{
		"gateway_id":         "my-gw",
		"provider":           "openai",
		"model":              "gpt-4o",
		"success":            "true",
		"start_date":         "2024-01-01T00:00:00Z",
		"order_by":           "cost",
		"order_by_direction": "desc",
		"per_page":           50,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "tokens_in")
	assert.Contains(t, result.Data, "cost")
}

func TestGetAIGatewayLog(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai-gateway/gateways/my-gw/logs/log1")
		assert.NotContains(t, r.URL.Path, "/request")
		assert.NotContains(t, r.URL.Path, "/response")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"log1","tokens_in":120,"tokens_out":45,"cost":0.0012,"duration":850}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_ai_gateway_log", map[string]any{
		"gateway_id": "my-gw",
		"log_id":     "log1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"tokens_in":120`)
}

func TestGetAIGatewayLogRequest(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai-gateway/gateways/my-gw/logs/log1/request")
		_, _ = w.Write([]byte(`{"success":true,"result":{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_ai_gateway_log_request", map[string]any{
		"gateway_id": "my-gw",
		"log_id":     "log1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "messages")
}

func TestGetAIGatewayLogResponse(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai-gateway/gateways/my-gw/logs/log1/response")
		_, _ = w.Write([]byte(`{"success":true,"result":{"choices":[{"message":{"content":"hello"}}],"usage":{"total_tokens":12}}}`))
	}))
	defer ts.Close()

	result, err := c.Execute(context.Background(), "cloudflare_get_ai_gateway_log_response", map[string]any{
		"gateway_id": "my-gw",
		"log_id":     "log1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "total_tokens")
}

// Ensure compaction spec exists for the new list tools (matches existing pattern).
func TestFieldCompactionSpec_AIGatewayLists(t *testing.T) {
	c := &cloudflare{}
	for _, name := range []mcp.ToolName{"cloudflare_list_ai_gateways", "cloudflare_list_ai_gateway_logs"} {
		fields, ok := c.CompactSpec(name)
		require.True(t, ok, "%s should have field compaction spec", name)
		assert.NotEmpty(t, fields, "%s spec should not be empty", name)
	}
}
