package pganalyze

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flakyMCPServer fails every request with 401 until healthy is set, then
// delegates to the regular mock MCP server.
func flakyMCPServer(t *testing.T, healthy *atomic.Bool, requests *atomic.Int64) *httptest.Server {
	t.Helper()
	good := newMCPServer(t)
	t.Cleanup(good.Close)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if !healthy.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`upstream unavailable`))
			return
		}
		good.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// recorder collects values written by httptest handler goroutines.
type recorder[T any] struct {
	mu     sync.Mutex
	values []T
}

func (r *recorder[T]) add(v T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = append(r.values, v)
}

func (r *recorder[T]) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = nil
}

func (r *recorder[T]) all() []T {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]T(nil), r.values...)
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newFlakyPganalyze(t *testing.T, healthy *atomic.Bool, requests *atomic.Int64) (*pganalyze, *fakeClock) {
	t.Helper()
	ts := flakyMCPServer(t, healthy, requests)
	clock := &fakeClock{now: time.Unix(1_800_000_000, 0)}
	p := &pganalyze{apiKey: "test-key", mcpURL: ts.URL, client: ts.Client(), now: clock.Now}
	return p, clock
}

func TestExecute_RecoversAfterFailedInit(t *testing.T) {
	var healthy atomic.Bool
	var requests atomic.Int64
	p, clock := newFlakyPganalyze(t, &healthy, &requests)

	result, err := p.Execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "proxy not initialized")

	healthy.Store(true)
	clock.Advance(initRetryInterval + time.Second)

	result, err = p.Execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "srv-1")
}

func TestExecute_SurfacesInitError(t *testing.T) {
	var healthy atomic.Bool
	var requests atomic.Int64
	p, _ := newFlakyPganalyze(t, &healthy, &requests)

	result, err := p.Execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "401")
	assert.Contains(t, result.Data, "upstream unavailable")
}

func TestExecute_DoesNotRetryInitWithinBackoff(t *testing.T) {
	var healthy atomic.Bool
	var requests atomic.Int64
	p, clock := newFlakyPganalyze(t, &healthy, &requests)

	_, _ = p.Execute(context.Background(), "pganalyze_list_servers", nil)
	attempts := requests.Load()
	require.Positive(t, attempts)

	clock.Advance(initRetryInterval / 2)
	_, _ = p.Execute(context.Background(), "pganalyze_list_servers", nil)
	assert.Equal(t, attempts, requests.Load(), "init must not be retried inside the backoff window")
	assert.False(t, p.Healthy(context.Background()))
	assert.Equal(t, attempts, requests.Load(), "Healthy must respect the backoff window too")
}

func TestHealthy_RecoversAfterFailedInit(t *testing.T) {
	var healthy atomic.Bool
	var requests atomic.Int64
	p, clock := newFlakyPganalyze(t, &healthy, &requests)

	assert.False(t, p.Healthy(context.Background()))

	healthy.Store(true)
	clock.Advance(initRetryInterval + time.Second)
	assert.True(t, p.Healthy(context.Background()))
}

func TestConfigure_CredentialChangeResetsProxy(t *testing.T) {
	var seenKeys recorder[string]
	good := newMCPServer(t)
	t.Cleanup(good.Close)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKeys.add(r.Header.Get("Authorization"))
		r.Header.Set("Authorization", "Bearer test-key")
		good.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)

	p := &pganalyze{client: ts.Client()}
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"api_key": "key-a", "base_url": ts.URL}))
	require.True(t, p.Healthy(context.Background()))

	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"api_key": "key-b", "base_url": ts.URL}))
	seenKeys.reset()
	result, err := p.Execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.NotEmpty(t, seenKeys.all())
	for _, key := range seenKeys.all() {
		assert.Equal(t, "Bearer key-b", key)
	}
}

func TestConfigure_ClearsFailedInitBackoff(t *testing.T) {
	var healthy atomic.Bool
	var requests atomic.Int64
	ts := flakyMCPServer(t, &healthy, &requests)

	p := &pganalyze{client: ts.Client()}
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"api_key": "bad", "base_url": ts.URL}))
	assert.False(t, p.Healthy(context.Background()))

	healthy.Store(true)
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"api_key": "test-key", "base_url": ts.URL}))
	assert.True(t, p.Healthy(context.Background()), "reconfiguring must retry immediately")
}

func TestNew_HTTPClientHasTimeout(t *testing.T) {
	p, ok := New().(*pganalyze)
	require.True(t, ok)
	assert.Positive(t, p.client.Timeout)
}

// --- Streamable HTTP transport ---

func TestProxy_AcceptsJSONAndEventStream(t *testing.T) {
	var accept recorder[string]
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept.add(r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	_, err := proxy.send(context.Background(), "initialize", map[string]any{})
	require.NoError(t, err)
	require.Len(t, accept.all(), 1)
	assert.Contains(t, accept.all()[0], "application/json")
	assert.Contains(t, accept.all()[0], "text/event-stream")
}

func TestProxy_ParsesEventStreamResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, ": keepalive\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"tools\":[{\"name\":\"list_servers\"}]}}\n\n", req.ID)
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	require.NoError(t, proxy.fetchTools(context.Background()))
	require.Len(t, proxy.tools, 1)
	assert.Equal(t, "list_servers", proxy.tools[0].Name)
}

func TestProxy_EchoesSessionID(t *testing.T) {
	var sessionHeaders recorder[string]
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "session-123")
		} else {
			sessionHeaders.add(r.Header.Get("Mcp-Session-Id"))
		}
		if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[]}}`, req.ID)
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	require.NoError(t, proxy.initialize(context.Background()))
	require.NoError(t, proxy.fetchTools(context.Background()))
	require.Len(t, sessionHeaders.all(), 2)
	for _, h := range sessionHeaders.all() {
		assert.Equal(t, "session-123", h)
	}
}

func TestProxy_InitializedNotificationHasNoID(t *testing.T) {
	var notifications recorder[map[string]any]
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		if raw["method"] == "notifications/initialized" {
			notifications.add(raw)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{}}`, raw["id"])
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	require.NoError(t, proxy.initialize(context.Background()))
	require.Len(t, notifications.all(), 1)
	_, hasID := notifications.all()[0]["id"]
	assert.False(t, hasID, "JSON-RPC notifications must not carry an id")
}

// --- Static fallback must match the upstream schema ---

func TestStaticTools_UseUpstreamParameterNames(t *testing.T) {
	for _, tool := range staticTools {
		for param := range tool.Parameters {
			assert.False(t, strings.Contains(param, "_"),
				"%s: parameter %q is snake_case; the pganalyze MCP server uses camelCase", tool.Name, param)
		}
		for _, req := range tool.Required {
			_, ok := tool.Parameters[req]
			assert.True(t, ok, "%s: required parameter %q is not declared", tool.Name, req)
		}
	}
}

func TestStaticTools_MatchUpstreamTools(t *testing.T) {
	byName := make(map[mcp.ToolName]mcp.ToolDefinition, len(staticTools))
	for _, tool := range staticTools {
		byName[tool.Name] = tool
	}

	_, hasSamples := byName["pganalyze_get_query_samples"]
	assert.False(t, hasSamples, "get_query_samples does not exist upstream")

	tests := []struct {
		name     mcp.ToolName
		required []string
	}{
		{"pganalyze_get_query_stats", []string{"databaseId"}},
		{"pganalyze_get_query_details", []string{"databaseId", "queryId"}},
		{"pganalyze_get_query_explains", []string{"databaseId", "queryId"}},
		{"pganalyze_get_query_explain", []string{"databaseId", "explainId"}},
		{"pganalyze_get_table", []string{"databaseId", "schemaTableId"}},
		{"pganalyze_get_log_lines", []string{"serverId"}},
		{"pganalyze_get_system_stats", []string{"serverId", "startTs", "endTs"}},
	}
	for _, tc := range tests {
		t.Run(string(tc.name), func(t *testing.T) {
			tool, ok := byName[tc.name]
			require.True(t, ok, "missing static tool %s", tc.name)
			assert.ElementsMatch(t, tc.required, tool.Required)
		})
	}
}
