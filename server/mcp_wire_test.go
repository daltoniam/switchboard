package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPWire_20260728_DiscoverOnProductionMux(t *testing.T) {
	s := setupTestServer()
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	status, headers, body := postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "server/discover",
		Params: map[string]any{
			"_meta": modernRequestMeta(),
		},
	}, modernHTTPHeaders("server/discover", ""))
	require.Equal(t, http.StatusOK, status, "discover body=%v", body)
	assert.Empty(t, headers.Get(mcpSessionIDH), "modern discover must not mint Mcp-Session-Id")

	result := jsonRPCResult(t, body)
	supported, _ := result["supportedVersions"].([]any)
	require.Contains(t, supported, mcpProtocol20260728)
	assert.Equal(t, "complete", result["resultType"])
	_, hasTTL := result["ttlMs"]
	assert.True(t, hasTTL)
	_, hasScope := result["cacheScope"]
	assert.True(t, hasScope)

	caps, _ := result["capabilities"].(map[string]any)
	require.NotNil(t, caps)
	_, hasLogging := caps["logging"]
	assert.False(t, hasLogging, "modern discover must not advertise deprecated logging")
	tools, _ := caps["tools"].(map[string]any)
	require.NotNil(t, tools)
	if v, ok := tools["listChanged"]; ok {
		assert.Equal(t, false, v)
	}
}

func TestMCPWire_20260728_StatelessToolsList(t *testing.T) {
	s := setupTestServer()
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	for i, id := range []int{11, 12} {
		status, headers, body := postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
			JSONRPC: "2.0",
			ID:      id,
			Method:  "tools/list",
			Params: map[string]any{
				"_meta": modernRequestMeta(),
			},
		}, modernHTTPHeaders("tools/list", ""))
		require.Equal(t, http.StatusOK, status, "modern tools/list #%d body=%v", i+1, body)
		assert.Empty(t, headers.Get(mcpSessionIDH))
		tools := resultArray(t, jsonRPCResult(t, body), "tools")
		assert.True(t, toolNamesContain(tools, "search"))
		assert.True(t, toolNamesContain(tools, "execute"))
	}
}

func TestMCPWire_20260728_LegacyInitializeSearch(t *testing.T) {
	s := setupTestServer()
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	status, _, body := postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": mcpProtocol20251125,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "legacy-test", "version": "0"},
		},
	}, legacyHTTPHeaders("initialize", ""))
	require.Equal(t, http.StatusOK, status, "legacy initialize; body=%v", body)
	assert.Equal(t, mcpProtocol20251125, jsonRPCResult(t, body)["protocolVersion"])

	status, _, body = postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]any{},
	}, legacyHTTPHeaders("notifications/initialized", ""))
	require.True(t, status == http.StatusOK || status == http.StatusAccepted, "legacy initialized status=%d body=%v", status, body)

	status, _, body = postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "search",
			"arguments": map[string]any{"limit": 1},
		},
	}, legacyHTTPHeaders("tools/call", "search"))
	require.Equal(t, http.StatusOK, status, "legacy search; body=%v", body)
	result := jsonRPCResult(t, body)
	assert.NotEqual(t, true, result["isError"])
}

func TestMCPWire_20260728_InvalidProtocolMetadata(t *testing.T) {
	s := setupTestServer()
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	headers := modernHTTPHeaders("server/discover", "")
	headers.Set(mcpProtocolVersionH, "1999-01-01")
	payload, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      99,
		Method:  "server/discover",
		Params: map[string]any{
			"_meta": map[string]any{
				mcpMetaProtocolKey: "1999-01-01",
				mcpMetaClientInfo:  map[string]any{"name": "bad", "version": "0"},
				mcpMetaClientCaps:  map[string]any{},
			},
		},
	})
	require.NoError(t, err)
	httpReq, err := http.NewRequest(http.MethodPost, httpSrv.URL+"/mcp", bytes.NewReader(payload))
	require.NoError(t, err)
	for key, values := range headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}
	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// The SDK rejects unsupported Mcp-Protocol-Version before JSON-RPC
	// dispatch. That is the protocol error: do not reinterpret as another
	// tenant, app session, or project.
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "unsupported protocol body=%q", raw)
	assert.Contains(t, string(raw), "Unsupported protocol version")
}

func TestMCPWire_20260728_AppSessionWithoutMCPSession(t *testing.T) {
	s := setupTestServer()
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	setHeaders := modernHTTPHeaders("tools/call", "session")
	setHeaders.Set(AppSessionIDHeader, "wire-sess-1")
	status, headers, body := postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
		Params: map[string]any{
			"_meta": modernRequestMeta(),
			"name":  "session",
			"arguments": map[string]any{
				"action":  "set",
				"context": map[string]any{"owner": "acme"},
			},
		},
	}, setHeaders)
	require.Equal(t, http.StatusOK, status, "session set body=%v", body)
	assert.Empty(t, headers.Get(mcpSessionIDH))

	getHeaders := modernHTTPHeaders("tools/call", "session")
	getHeaders.Set(AppSessionIDHeader, "wire-sess-1")
	status, _, body = postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "tools/call",
		Params: map[string]any{
			"_meta":     modernRequestMeta(),
			"name":      "session",
			"arguments": map[string]any{"action": "get"},
		},
	}, getHeaders)
	require.Equal(t, http.StatusOK, status, "session get body=%v", body)
	result := jsonRPCResult(t, body)
	text := firstTextContent(t, result)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &payload))
	ctx, _ := payload["context"].(map[string]any)
	require.Equal(t, "acme", ctx["owner"])
}

func TestMCPWire_20260728_OfficialClientSearchWithoutInitialize(t *testing.T) {
	s := setupTestServer()
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "wire-client", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpSrv.URL + "/mcp",
		HTTPClient:           httpSrv.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	require.Equal(t, mcpProtocol20260728, session.InitializeResult().ProtocolVersion)

	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "search",
		Arguments: map[string]any{"limit": 1},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
}

func firstTextContent(t *testing.T, result map[string]any) string {
	t.Helper()
	contents := resultArray(t, result, "content")
	require.NotEmpty(t, contents)
	obj, _ := contents[0].(map[string]any)
	require.NotNil(t, obj)
	text, _ := obj["text"].(string)
	require.NotEmpty(t, text)
	return text
}

func TestBuildHTTPMux_OmitsCatalogWhenNil(t *testing.T) {
	called := false
	mux := BuildHTTPMux(HTTPMuxConfig{
		MCP: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}),
	})
	req := httptest.NewRequest(http.MethodPost, "/project-catalog/mcp", bytes.NewReader(nil))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.False(t, called)
}
