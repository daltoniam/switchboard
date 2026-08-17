package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mcpProtocol20260728 = "2026-07-28"
	mcpProtocol20251125 = "2025-11-25"
	mcpProtocolVersionH = "Mcp-Protocol-Version"
	mcpMethodH          = "Mcp-Method"
	mcpNameH            = "Mcp-Name"
	mcpSessionIDH       = "Mcp-Session-Id"
	mcpMetaProtocolKey  = "io.modelcontextprotocol/protocolVersion"
	mcpMetaClientInfo   = "io.modelcontextprotocol/clientInfo"
	mcpMetaClientCaps   = "io.modelcontextprotocol/clientCapabilities"
)

type echoToolIn struct {
	Message string `json:"message"`
}

type echoToolOut struct {
	Echo string `json:"echo"`
}

// TestMCPSDKFeasibility is the fail-closed Phase 1 spike. It must prove the
// pinned Go MCP SDK implements the complete 2026-07-28 surface this plan needs:
// server/discover, stateless modern + legacy compatibility, typed structured
// tool output, dynamic resource add/delete, opaque-cursor paging, caller-
// controlled private cache scope, subscriptions/listen, resource-specific
// updates, and disconnect/reconnect.
func TestMCPSDKFeasibility(t *testing.T) {
	t.Run("Discover", testSDKFeasibilityDiscover)
	t.Run("StatelessModernAndLegacy", testSDKFeasibilityStatelessModernAndLegacy)
	t.Run("StructuredToolOutput", testSDKFeasibilityStructuredToolOutput)
	t.Run("DynamicResourceAddDelete", testSDKFeasibilityDynamicResources)
	t.Run("OpaqueCursorPaging", testSDKFeasibilityOpaqueCursorPaging)
	t.Run("PrivateCacheScope", testSDKFeasibilityPrivateCacheScope)
	t.Run("SubscriptionsListen", testSDKFeasibilitySubscriptionsListen)
	t.Run("ResourceSpecificUpdates", testSDKFeasibilityResourceUpdates)
	t.Run("DisconnectReconnect", testSDKFeasibilityDisconnectReconnect)
}

func testSDKFeasibilityDiscover(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{pageSize: 10})
	t.Cleanup(httpSrv.Close)
	_ = srv

	status, headers, body := postJSONRPC(t, httpSrv.URL, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "server/discover",
		Params: map[string]any{
			"_meta": modernRequestMeta(),
		},
	}, modernHTTPHeaders("server/discover", ""))
	require.Equal(t, http.StatusOK, status, "discover HTTP status; body=%v", body)
	assert.Empty(t, headers.Get(mcpSessionIDH), "modern discover must not mint Mcp-Session-Id")

	result := jsonRPCResult(t, body)
	supported, _ := result["supportedVersions"].([]any)
	require.NotEmpty(t, supported, "discover must advertise supportedVersions")
	assert.Contains(t, supported, mcpProtocol20260728)

	assert.Equal(t, "complete", result["resultType"], "discover resultType")
	_, hasTTL := result["ttlMs"]
	assert.True(t, hasTTL, "discover must include ttlMs")
	_, hasScope := result["cacheScope"]
	assert.True(t, hasScope, "discover must include cacheScope")

	caps, _ := result["capabilities"].(map[string]any)
	require.NotNil(t, caps, "discover must advertise capabilities")
	_, hasLogging := caps["logging"]
	assert.False(t, hasLogging, "modern discover must not advertise deprecated logging")
	tools, _ := caps["tools"].(map[string]any)
	require.NotNil(t, tools)
	// omitempty encodes listChanged=false as absence; both mean "no listChanged".
	if v, ok := tools["listChanged"]; ok {
		assert.Equal(t, false, v)
	}
}

func testSDKFeasibilityStatelessModernAndLegacy(t *testing.T) {
	_, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{pageSize: 10})
	t.Cleanup(httpSrv.Close)

	// Two independent modern POSTs must succeed without initialize.
	for i, id := range []int{11, 12} {
		status, headers, body := postJSONRPC(t, httpSrv.URL, jsonRPCRequest{
			JSONRPC: "2.0",
			ID:      id,
			Method:  "tools/list",
			Params: map[string]any{
				"_meta": modernRequestMeta(),
			},
		}, modernHTTPHeaders("tools/list", ""))
		require.Equal(t, http.StatusOK, status, "modern tools/list #%d", i+1)
		assert.Empty(t, headers.Get(mcpSessionIDH))
		result := jsonRPCResult(t, body)
		tools := resultArray(t, result, "tools")
		assert.True(t, toolNamesContain(tools, "echo"), "modern tools/list #%d missing echo", i+1)
	}

	// Legacy initialize/initialized still works on the same endpoint.
	status, _, body := postJSONRPC(t, httpSrv.URL, jsonRPCRequest{
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
	initResult := jsonRPCResult(t, body)
	assert.Equal(t, mcpProtocol20251125, initResult["protocolVersion"])

	status, _, body = postJSONRPC(t, httpSrv.URL, jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]any{},
	}, legacyHTTPHeaders("notifications/initialized", ""))
	require.True(t, status == http.StatusOK || status == http.StatusAccepted, "legacy initialized status=%d body=%v", status, body)

	status, _, body = postJSONRPC(t, httpSrv.URL, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "tools/list",
		Params:  map[string]any{},
	}, legacyHTTPHeaders("tools/list", ""))
	require.Equal(t, http.StatusOK, status, "legacy tools/list; body=%v", body)
	tools := resultArray(t, jsonRPCResult(t, body), "tools")
	assert.True(t, toolNamesContain(tools, "echo"))
}

func testSDKFeasibilityStructuredToolOutput(t *testing.T) {
	_, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{pageSize: 10})
	t.Cleanup(httpSrv.Close)

	client := newModernMCPClient(t, httpSrv.URL)
	t.Cleanup(func() { _ = client.Close() })

	res, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "echo",
		Arguments: echoToolIn{Message: "catalog"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotNil(t, res.StructuredContent, "typed tools must return structuredContent")

	encoded, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out echoToolOut
	require.NoError(t, json.Unmarshal(encoded, &out))
	assert.Equal(t, "catalog", out.Echo)
}

func testSDKFeasibilityDynamicResources(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{pageSize: 10})
	t.Cleanup(httpSrv.Close)

	listed := listResourceURIs(t, httpSrv.URL)
	assert.Equal(t, []string{"feasibility://alpha"}, listed)

	srv.AddResource(&mcpsdk.Resource{
		URI:      "feasibility://beta",
		Name:     "beta",
		MIMEType: "application/json",
	}, staticResourceHandler(`{"n":"beta"}`))

	listed = listResourceURIs(t, httpSrv.URL)
	assert.Equal(t, []string{"feasibility://alpha", "feasibility://beta"}, listed)

	srv.RemoveResources("feasibility://alpha")
	listed = listResourceURIs(t, httpSrv.URL)
	assert.Equal(t, []string{"feasibility://beta"}, listed)
}

func testSDKFeasibilityOpaqueCursorPaging(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{pageSize: 2})
	t.Cleanup(httpSrv.Close)
	for _, name := range []string{"beta", "gamma", "delta"} {
		srv.AddResource(&mcpsdk.Resource{
			URI:      "feasibility://" + name,
			Name:     name,
			MIMEType: "application/json",
		}, staticResourceHandler(`{"n":"`+name+`"}`))
	}

	page1 := listResourcesRaw(t, httpSrv.URL, "")
	uris1 := resourceURIs(t, page1)
	require.Len(t, uris1, 2)
	next, _ := page1["nextCursor"].(string)
	require.NotEmpty(t, next, "first page must return an opaque nextCursor")

	page2 := listResourcesRaw(t, httpSrv.URL, next)
	uris2 := resourceURIs(t, page2)
	require.NotEmpty(t, uris2)
	for _, uri := range uris1 {
		assert.NotContains(t, uris2, uri, "pages must not overlap")
	}

	// Malformed cursor is invalid params, not a silent restart.
	status, _, body := postJSONRPC(t, httpSrv.URL, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      40,
		Method:  "resources/list",
		Params: map[string]any{
			"_meta":  modernRequestMeta(),
			"cursor": "not-a-real-cursor",
		},
	}, modernHTTPHeaders("resources/list", ""))
	require.True(t, status == http.StatusOK || status == http.StatusBadRequest, "malformed cursor HTTP status=%d body=%v", status, body)
	rpcErr, _ := body["error"].(map[string]any)
	require.NotNil(t, rpcErr, "malformed cursor must be a JSON-RPC error")
	assert.EqualValues(t, -32602, rpcErr["code"])
}

func testSDKFeasibilityPrivateCacheScope(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{
		pageSize:          10,
		privateCacheScope: true,
		listTTLMs:         10_000,
		revisionTTLMs:     3_600_000,
	})
	t.Cleanup(httpSrv.Close)

	list := listResourcesRaw(t, httpSrv.URL, "")
	assert.Equal(t, "private", list["cacheScope"], "resources/list cacheScope")
	assert.EqualValues(t, 10_000, list["ttlMs"], "resources/list ttlMs")

	templates := listResourceTemplatesRaw(t, httpSrv.URL)
	assert.Equal(t, "private", templates["cacheScope"], "templates/list cacheScope")
	assert.EqualValues(t, 10_000, templates["ttlMs"], "templates/list ttlMs")

	read := readResourceRaw(t, httpSrv.URL, "feasibility://alpha")
	assert.Equal(t, "private", read["cacheScope"], "resources/read cacheScope")
	assert.EqualValues(t, 10_000, read["ttlMs"], "current-read ttlMs")

	srv.AddResource(&mcpsdk.Resource{
		URI:      "feasibility://rev/sha256:abc",
		Name:     "revision",
		MIMEType: "application/json",
	}, staticResourceHandler(`{"revision":"sha256:abc"}`))
	// The feasibility server treats URIs containing "/rev/" as immutable.
	rev := readResourceRaw(t, httpSrv.URL, "feasibility://rev/sha256:abc")
	assert.Equal(t, "private", rev["cacheScope"])
	assert.EqualValues(t, 3_600_000, rev["ttlMs"], "immutable revision ttlMs")
}

func testSDKFeasibilitySubscriptionsListen(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{
		pageSize:   10,
		subscribe:  true,
		listChange: true,
	})
	t.Cleanup(httpSrv.Close)

	gotListChanged := make(chan struct{}, 1)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "feasibility-sub", Version: "0"}, &mcpsdk.ClientOptions{
		ResourceListChangedHandler: func(context.Context, *mcpsdk.ResourceListChangedRequest) {
			select {
			case gotListChanged <- struct{}{}:
			default:
			}
		},
	})
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpSrv.URL,
		HTTPClient:           httpSrv.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	require.Equal(t, mcpProtocol20260728, session.InitializeResult().ProtocolVersion,
		"subscription client must negotiate 2026-07-28 via discover")

	// Give the listen stream time to be acknowledged before mutating.
	time.Sleep(50 * time.Millisecond)
	srv.AddResource(&mcpsdk.Resource{
		URI:      "feasibility://from-listen",
		Name:     "from-listen",
		MIMEType: "application/json",
	}, staticResourceHandler(`{"n":"from-listen"}`))

	select {
	case <-gotListChanged:
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive resources/list_changed on subscriptions/listen")
	}
}

func testSDKFeasibilityResourceUpdates(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{
		pageSize:  10,
		subscribe: true,
	})
	t.Cleanup(httpSrv.Close)

	gotUpdate := make(chan string, 1)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "feasibility-upd", Version: "0"}, &mcpsdk.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, req *mcpsdk.ResourceUpdatedNotificationRequest) {
			if req != nil && req.Params != nil {
				select {
				case gotUpdate <- req.Params.URI:
				default:
				}
			}
		},
	})
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpSrv.URL,
		HTTPClient:           httpSrv.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	require.NoError(t, session.Subscribe(context.Background(), &mcpsdk.SubscribeParams{
		URI: "feasibility://alpha",
	}))
	time.Sleep(50 * time.Millisecond)

	require.NoError(t, srv.ResourceUpdated(context.Background(), &mcpsdk.ResourceUpdatedNotificationParams{
		URI: "feasibility://alpha",
	}))

	select {
	case uri := <-gotUpdate:
		assert.Equal(t, "feasibility://alpha", uri)
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive resources/updated for subscribed URI")
	}
}

func testSDKFeasibilityDisconnectReconnect(t *testing.T) {
	srv, httpSrv := newFeasibilityHTTPServer(t, feasibilityOptions{
		pageSize:   10,
		subscribe:  true,
		listChange: true,
	})
	t.Cleanup(httpSrv.Close)

	first := make(chan struct{}, 1)
	client1 := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "feasibility-d1", Version: "0"}, &mcpsdk.ClientOptions{
		ResourceListChangedHandler: func(context.Context, *mcpsdk.ResourceListChangedRequest) {
			select {
			case first <- struct{}{}:
			default:
			}
		},
	})
	session1, err := client1.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpSrv.URL,
		HTTPClient:           httpSrv.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	require.NoError(t, session1.Close())

	// Mutation while disconnected must not be replayed onto a later listen.
	srv.AddResource(&mcpsdk.Resource{
		URI:      "feasibility://offline",
		Name:     "offline",
		MIMEType: "application/json",
	}, staticResourceHandler(`{"n":"offline"}`))
	select {
	case <-first:
		t.Fatal("closed subscription must not receive replayed list_changed")
	case <-time.After(200 * time.Millisecond):
	}

	second := make(chan struct{}, 1)
	client2 := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "feasibility-d2", Version: "0"}, &mcpsdk.ClientOptions{
		ResourceListChangedHandler: func(context.Context, *mcpsdk.ResourceListChangedRequest) {
			select {
			case second <- struct{}{}:
			default:
			}
		},
	})
	session2, err := client2.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpSrv.URL,
		HTTPClient:           httpSrv.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session2.Close() })

	listed, err := session2.ListResources(context.Background(), nil)
	require.NoError(t, err)
	var found bool
	for _, r := range listed.Resources {
		if r.URI == "feasibility://offline" {
			found = true
		}
	}
	assert.True(t, found, "reconnect must reread current resources rather than rely on replay")

	time.Sleep(50 * time.Millisecond)
	srv.AddResource(&mcpsdk.Resource{
		URI:      "feasibility://online",
		Name:     "online",
		MIMEType: "application/json",
	}, staticResourceHandler(`{"n":"online"}`))
	select {
	case <-second:
	case <-time.After(3 * time.Second):
		t.Fatal("new subscription did not receive subsequent list_changed")
	}
}

type feasibilityOptions struct {
	pageSize          int
	privateCacheScope bool
	listTTLMs         int
	revisionTTLMs     int
	subscribe         bool
	listChange        bool
}

func newFeasibilityHTTPServer(t *testing.T, opts feasibilityOptions) (*mcpsdk.Server, *httptest.Server) {
	t.Helper()
	if opts.pageSize <= 0 {
		opts.pageSize = 10
	}
	caps := &mcpsdk.ServerCapabilities{
		Tools: &mcpsdk.ToolCapabilities{ListChanged: false},
		Resources: &mcpsdk.ResourceCapabilities{
			ListChanged: opts.listChange,
			Subscribe:   opts.subscribe,
		},
	}
	serverOpts := &mcpsdk.ServerOptions{
		Instructions: "feasibility spike",
		PageSize:     opts.pageSize,
		Capabilities: caps,
	}
	if opts.subscribe {
		serverOpts.SubscribeHandler = func(context.Context, *mcpsdk.SubscribeRequest) error { return nil }
		serverOpts.UnsubscribeHandler = func(context.Context, *mcpsdk.UnsubscribeRequest) error { return nil }
	}
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "switchboard-feasibility", Version: "test"}, serverOpts)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "echo",
		Description: "Echo a message",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in echoToolIn) (*mcpsdk.CallToolResult, echoToolOut, error) {
		return nil, echoToolOut{Echo: in.Message}, nil
	})
	srv.AddResource(&mcpsdk.Resource{
		URI:      "feasibility://alpha",
		Name:     "alpha",
		MIMEType: "application/json",
	}, staticResourceHandler(`{"n":"alpha"}`))
	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "feasibility://{name}",
		Name:        "named",
		MIMEType:    "application/json",
	}, staticResourceHandler(`{"n":"template"}`))

	if opts.privateCacheScope {
		applyPrivateCacheMiddleware(srv, opts.listTTLMs, opts.revisionTTLMs)
	}

	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return srv
	}, &mcpsdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	return srv, httptest.NewServer(handler)
}

func applyPrivateCacheMiddleware(srv *mcpsdk.Server, listTTLMs, revisionTTLMs int) {
	srv.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			res, err := next(ctx, method, req)
			if err != nil || res == nil {
				return res, err
			}
			switch method {
			case "resources/list", "resources/templates/list", "server/discover":
				setCacheable(res, "private", listTTLMs)
			case "resources/read":
				ttl := listTTLMs
				if readReq, ok := req.(*mcpsdk.ReadResourceRequest); ok && readReq.Params != nil && strings.Contains(readReq.Params.URI, "/rev/") {
					ttl = revisionTTLMs
				}
				setCacheable(res, "private", ttl)
			}
			return res, nil
		}
	})
}

func setCacheable(res mcpsdk.Result, scope string, ttlMs int) {
	type cacheSetter interface {
		GetTTLMs() int
		GetCacheScope() string
	}
	// Prefer exported fields on known result types.
	switch v := res.(type) {
	case *mcpsdk.ListResourcesResult:
		v.CacheScope = scope
		v.TTLMs = ttlMs
	case *mcpsdk.ListResourceTemplatesResult:
		v.CacheScope = scope
		v.TTLMs = ttlMs
	case *mcpsdk.ReadResourceResult:
		v.CacheScope = scope
		v.TTLMs = ttlMs
	case cacheSetter:
		// Best-effort for DiscoverResult and future cacheable types via json round-trip.
		raw, err := json.Marshal(v)
		if err != nil {
			return
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return
		}
		obj["cacheScope"] = scope
		obj["ttlMs"] = ttlMs
		updated, err := json.Marshal(obj)
		if err != nil {
			return
		}
		_ = json.Unmarshal(updated, v)
	}
}

func staticResourceHandler(body string) mcpsdk.ResourceHandler {
	return func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     body,
			}},
		}, nil
	}
}

func newModernMCPClient(t *testing.T, endpoint string) *mcpsdk.ClientSession {
	t.Helper()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "feasibility-client", Version: "0"}, &mcpsdk.ClientOptions{
		Capabilities: &mcpsdk.ClientCapabilities{},
	})
	session, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:             endpoint,
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	return session
}

func listResourceURIs(t *testing.T, endpoint string) []string {
	t.Helper()
	return resourceURIs(t, listResourcesRaw(t, endpoint, ""))
}

func resourceURIs(t *testing.T, result map[string]any) []string {
	t.Helper()
	items := resultArray(t, result, "resources")
	out := make([]string, 0, len(items))
	for _, item := range items {
		obj, _ := item.(map[string]any)
		if obj == nil {
			continue
		}
		if uri, _ := obj["uri"].(string); uri != "" {
			out = append(out, uri)
		}
	}
	return out
}

func listResourcesRaw(t *testing.T, endpoint, cursor string) map[string]any {
	t.Helper()
	params := map[string]any{"_meta": modernRequestMeta()}
	if cursor != "" {
		params["cursor"] = cursor
	}
	status, _, body := postJSONRPC(t, endpoint, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  "resources/list",
		Params:  params,
	}, modernHTTPHeaders("resources/list", ""))
	require.Equal(t, http.StatusOK, status, "resources/list body=%v", body)
	return jsonRPCResult(t, body)
}

func listResourceTemplatesRaw(t *testing.T, endpoint string) map[string]any {
	t.Helper()
	status, _, body := postJSONRPC(t, endpoint, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  "resources/templates/list",
		Params:  map[string]any{"_meta": modernRequestMeta()},
	}, modernHTTPHeaders("resources/templates/list", ""))
	require.Equal(t, http.StatusOK, status, "resources/templates/list body=%v", body)
	return jsonRPCResult(t, body)
}

func readResourceRaw(t *testing.T, endpoint, uri string) map[string]any {
	t.Helper()
	status, _, body := postJSONRPC(t, endpoint, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  "resources/read",
		Params: map[string]any{
			"_meta": modernRequestMeta(),
			"uri":   uri,
		},
	}, modernHTTPHeaders("resources/read", uri))
	require.Equal(t, http.StatusOK, status, "resources/read %s body=%v", uri, body)
	return jsonRPCResult(t, body)
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func modernRequestMeta() map[string]any {
	return map[string]any{
		mcpMetaProtocolKey: mcpProtocol20260728,
		mcpMetaClientInfo:  map[string]any{"name": "switchboard-test", "version": "0"},
		mcpMetaClientCaps:  map[string]any{},
	}
}

func modernHTTPHeaders(method, name string) http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json, text/event-stream")
	h.Set(mcpProtocolVersionH, mcpProtocol20260728)
	h.Set(mcpMethodH, method)
	if name != "" {
		h.Set(mcpNameH, name)
	}
	return h
}

func legacyHTTPHeaders(method, name string) http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json, text/event-stream")
	h.Set(mcpProtocolVersionH, mcpProtocol20251125)
	h.Set(mcpMethodH, method)
	if name != "" {
		h.Set(mcpNameH, name)
	}
	return h
}

func postJSONRPC(t *testing.T, endpoint string, req jsonRPCRequest, headers http.Header) (int, http.Header, map[string]any) {
	t.Helper()
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
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
	parsed := parseMCPHTTPBody(t, resp.Header.Get("Content-Type"), raw)
	return resp.StatusCode, resp.Header.Clone(), parsed
}

func parseMCPHTTPBody(t *testing.T, contentType string, raw []byte) map[string]any {
	t.Helper()
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}
	}
	if strings.Contains(contentType, "text/event-stream") || bytes.Contains(raw, []byte("data:")) {
		var last map[string]any
		for _, block := range strings.Split(string(raw), "\n\n") {
			for _, line := range strings.Split(block, "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" || data == "[DONE]" {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(data), &obj); err == nil {
					last = obj
				}
			}
		}
		if last != nil {
			return last
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("decode MCP body %q: %v", string(raw), err)
	}
	return obj
}

func jsonRPCResult(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	if rpcErr, ok := body["error"]; ok {
		t.Fatalf("JSON-RPC error: %v", rpcErr)
	}
	result, ok := body["result"].(map[string]any)
	require.True(t, ok, "missing JSON-RPC result: %v", body)
	return result
}

func resultArray(t *testing.T, result map[string]any, key string) []any {
	t.Helper()
	raw, ok := result[key]
	require.True(t, ok, "missing %s in %v", key, result)
	arr, ok := raw.([]any)
	require.True(t, ok, "%s is not an array: %T", key, raw)
	return arr
}

func toolNamesContain(tools []any, name string) bool {
	for _, tool := range tools {
		obj, _ := tool.(map[string]any)
		if obj != nil && obj["name"] == name {
			return true
		}
	}
	return false
}

// Silence unused import if a future edit drops sync usage in this file.
var _ = sync.Mutex{}
var _ = fmt.Sprintf
