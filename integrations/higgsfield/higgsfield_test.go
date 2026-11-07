package higgsfield

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureToken = "fixture-token"

type upstreamCall struct {
	name string
	args map[string]any
	auth string
}

// hostedFixture is a fake Higgsfield hosted MCP server. It serves the given
// upstream tool names at /mcp for the fixture bearer token and records calls.
type hostedFixture struct {
	server *httptest.Server
	mu     sync.Mutex
	paths  []string
	calls  []upstreamCall
}

func newHostedFixture(t *testing.T, results map[string]string, extra ...string) *hostedFixture {
	t.Helper()
	f := &hostedFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-higgsfield", Version: "1"}, nil)
	names := make([]string, 0, len(dispatch)+len(extra))
	for _, upstream := range dispatch {
		names = append(names, upstream)
	}
	names = append(names, extra...)
	for _, name := range names {
		text := results[name]
		if text == "" {
			text = `{"tool":"` + name + `"}`
		}
		server.AddTool(&mcpsdk.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			f.mu.Lock()
			f.calls = append(f.calls, upstreamCall{name: req.Params.Name, args: args, auth: "Bearer " + fixtureToken})
			f.mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *hostedFixture) snapshot() []upstreamCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]upstreamCall(nil), f.calls...)
}

func newIntegration(t *testing.T) mcp.Integration {
	t.Helper()
	integration := New()
	closer, ok := integration.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { assert.NoError(t, closer.Close()) })
	return integration
}

func configured(t *testing.T, baseURL, token string) mcp.Integration {
	t.Helper()
	integration := newIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": baseURL, "mcp_access_token": token}))
	return integration
}

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "higgsfield", integration.Name())
	assert.Equal(t, defaultBaseURL, MCPServerURL(integration))
	assert.Empty(t, MCPServerURL(nil))
}

func TestCredentialKeys(t *testing.T) {
	integration := New()
	plain, ok := integration.(mcp.PlainTextCredentials)
	require.True(t, ok)
	assert.Equal(t, []string{"base_url"}, plain.PlainTextKeys())
	optional, ok := integration.(mcp.OptionalCredentials)
	require.True(t, ok)
	assert.Equal(t, []string{"base_url"}, optional.OptionalKeys())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "missing token", creds: mcp.Credentials{}, wantErr: "mcp_access_token is required"},
		{name: "blank token", creds: mcp.Credentials{"mcp_access_token": "  "}, wantErr: "mcp_access_token is required"},
		{name: "bad scheme", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "ftp://mcp.higgsfield.ai"}, wantErr: "invalid base_url"},
		{name: "no host", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "https://"}, wantErr: "invalid base_url"},
		{name: "default url", creds: mcp.Credentials{"mcp_access_token": "t"}, wantURL: defaultBaseURL},
		{name: "full endpoint", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "https://example.test/mcp/"}, wantURL: "https://example.test"},
		{name: "origin", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": " https://example.test/ "}, wantURL: "https://example.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := newIntegration(t)
			err := integration.Configure(t.Context(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, MCPServerURL(integration))
		})
	}
}

func TestTools_Metadata(t *testing.T) {
	tools := New().Tools()
	require.NotEmpty(t, tools)
	seen := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "higgsfield_"), "%s lacks prefix", tool.Name)
		assert.NotEmpty(t, tool.Description, "%s has no description", tool.Name)
		assert.True(t, tool.Access.Declared(), "%s does not declare Access", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		for _, required := range tool.Required {
			_, ok := tool.Parameters[required]
			assert.True(t, ok, "%s requires undocumented parameter %s", tool.Name, required)
		}
	}
}

func TestTools_StartHere(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "higgsfield_balance" {
			assert.Contains(t, tool.Description, "Start here")
			return
		}
	}
	t.Fatal("higgsfield_balance not found")
}

// Paid generation tools must be write access and tell the model how to
// preflight the cost, so a caller never spends credits by accident.
func TestTools_PaidToolsWarnAboutCredits(t *testing.T) {
	paid := []mcp.ToolName{
		"higgsfield_generate_image",
		"higgsfield_generate_video",
		"higgsfield_generate_audio",
		"higgsfield_upscale_image",
		"higgsfield_remove_background",
	}
	byName := map[mcp.ToolName]mcp.ToolDefinition{}
	for _, tool := range New().Tools() {
		byName[tool.Name] = tool
	}
	for _, name := range paid {
		t.Run(string(name), func(t *testing.T) {
			tool, ok := byName[name]
			require.True(t, ok)
			assert.Equal(t, mcp.AccessWrite, tool.Access)
			assert.Contains(t, tool.Description, "credits")
		})
	}
}

func TestTools_ReturnsCopy(t *testing.T) {
	integration := New()
	tools := integration.Tools()
	tools[0].Parameters["injected"] = "x"
	tools[0].Description = "changed"
	again := integration.Tools()
	assert.NotEqual(t, "changed", again[0].Description)
	_, injected := again[0].Parameters["injected"]
	assert.False(t, injected)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch entry", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name, upstream := range dispatch {
		assert.True(t, names[name], "dispatch entry %s has no tool definition", name)
		assert.Equal(t, "higgsfield_"+upstream, string(name), "exposed name must be the prefixed upstream name")
	}
}

func TestExecute_Unconfigured(t *testing.T) {
	integration := newIntegration(t)
	assert.False(t, integration.Healthy(t.Context()))
	result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
}

func TestExecute_UnknownTool(t *testing.T) {
	fixture := newHostedFixture(t, nil, "delete_project")
	integration := configured(t, fixture.server.URL, fixtureToken)
	for _, name := range []mcp.ToolName{"higgsfield_nope", "higgsfield_delete_project", "balance", "other_balance"} {
		t.Run(string(name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), name, nil)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
		})
	}
	assert.Empty(t, fixture.snapshot(), "unknown tools must never reach the hosted server")
}

func TestExecute_ForwardsToHostedMCP(t *testing.T) {
	fixture := newHostedFixture(t, map[string]string{"balance": `{"credits":47.1,"subscription_plan_type":"free"}`})
	integration := configured(t, fixture.server.URL+"/mcp", fixtureToken)
	assert.True(t, integration.Healthy(t.Context()))

	result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"credits":47.1,"subscription_plan_type":"free"}`, result.Data)

	args := map[string]any{"params": map[string]any{"model": "soul_2", "prompt": "a cat", "get_cost": true}}
	result, err = integration.Execute(t.Context(), "higgsfield_generate_image", args)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)

	calls := fixture.snapshot()
	require.Len(t, calls, 2)
	assert.Equal(t, "balance", calls[0].name)
	assert.Empty(t, calls[0].args)
	assert.Equal(t, "generate_image", calls[1].name)
	assert.Equal(t, args, calls[1].args)
}

func TestHealthy_RejectedToken(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, "wrong-token")
	assert.False(t, integration.Healthy(t.Context()))
}

func TestConfigure_ReplacesRemoteWhenBaseURLChanges(t *testing.T) {
	first := newHostedFixture(t, nil)
	second := newHostedFixture(t, nil)
	integration := configured(t, first.server.URL, fixtureToken)
	_, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": second.server.URL, "mcp_access_token": fixtureToken}))
	assert.Equal(t, second.server.URL, MCPServerURL(integration))
	result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Len(t, first.snapshot(), 1)
	assert.Len(t, second.snapshot(), 1)
}

func TestClose_Idempotent(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, fixtureToken)
	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": fixture.server.URL, "mcp_access_token": fixtureToken}))
	result, err = integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no tool", name)
	}
}

func TestFieldCompactionSpecs_ReadToolsOnly(t *testing.T) {
	access := map[mcp.ToolName]mcp.ToolAccess{}
	for _, tool := range New().Tools() {
		access[tool.Name] = tool.Access
	}
	for name := range fieldCompactionSpecs {
		assert.Equal(t, mcp.AccessRead, access[name], "%s: only read tools get compaction specs", name)
	}
}

// Shapes below are trimmed copies of real hosted responses (2026-11-07).
func TestFieldCompactionSpecs_Shapes(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		raw     string
		keep    []string
		dropped []string
	}{
		{
			tool: "higgsfield_show_generations",
			raw: `{"items":[{"id":"613b2315","type":"image","status":"completed","model":"nano_banana_flash",
				"params":{"prompt":"icon glyph","aspect_ratio":"1:1","medias":[],"width":1024,"height":1024,"batch_size":1,"is_inpaint":false,"reference_elements":[]},
				"results":{"rawUrl":"https://cdn.test/a.png","minUrl":"https://cdn.test/a_min.webp"},"createdAt":1793842039.7,"liked":false}],
				"next_cursor":1793842039.25}`,
			keep:    []string{"613b2315", "completed", "nano_banana_flash", "icon glyph", "1:1", "https://cdn.test/a.png", "1793842039.25"},
			dropped: []string{"a_min.webp", "is_inpaint", "reference_elements", "batch_size"},
		},
		{
			tool: "higgsfield_list_voices",
			raw: `{"voices":[{"voice_id":"e2a2d2e6","voice_type":"preset","name":"Grady","gender":"male",
				"preview_url":"https://cdn.test/preview.mp3","logo_url":"https://cdn.test/thumb.webp"}],"has_more":true,"next_cursor":":1"}`,
			keep:    []string{"e2a2d2e6", "preset", "Grady", "male", "https://cdn.test/preview.mp3", ":1"},
			dropped: []string{"thumb.webp"},
		},
		{
			tool: "higgsfield_show_plans_and_credits",
			raw: `{"plans":[{"id":"plus","name":"PLUS","tagline":"For creators","credits":1000,"monthly_final_price_cents":4900,"annual_final_price_cents":3900,
				"monthly_url":"https://checkout.test/plus-monthly","pricing":{"monthly":{"mainFeatures":[{"id":"concurrent","text":"Parallel generations"}]}},"trial_checklist":[{"text":"All models"}]}],
				"topups":[],"workspace_kind":"individual","free_trial":{"headline":"3-Day Free Plus Trial","compliance_note":"Card required; renews at $49/mo."},
				"purchase_links":[{"kind":"plan_monthly","label":"PLUS monthly","url":"https://checkout.test/plus-monthly","price_cents":4900,"credits":1000,"description":"1,000 credits per month"}],
				"assistant_response":"Long sales copy"}`,
			keep:    []string{"plus", "PLUS", "1000", "4900", "3900", "individual", "3-Day Free Plus Trial", "Card required", "plan_monthly", "https://checkout.test/plus-monthly"},
			dropped: []string{"mainFeatures", "trial_checklist", "Long sales copy", "For creators"},
		},
		{
			tool:    "higgsfield_transactions",
			raw:     `{"items":[{"display_name":"Nano Banana 2","credits":-1.5,"action":"spend","created_at":"2026-11-05T01:27:19Z","internal":"x-debug"}],"next_cursor":1}`,
			keep:    []string{"Nano Banana 2", "-1.5", "spend", "2026-11-05T01:27:19Z", "next_cursor"},
			dropped: []string{"x-debug"},
		},
	}
	integration := New().(mcp.FieldCompactionIntegration)
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			fields, ok := integration.CompactSpec(tt.tool)
			require.True(t, ok)
			parsed, err := mcp.ParseJSONValue([]byte(tt.raw))
			require.NoError(t, err)
			out, err := mcp.EncodeJSONValue(parsed, mcp.JSONOptions{Fields: fields})
			require.NoError(t, err)
			for _, want := range tt.keep {
				assert.Contains(t, string(out), want)
			}
			for _, gone := range tt.dropped {
				assert.NotContains(t, string(out), gone)
			}
		})
	}
}
