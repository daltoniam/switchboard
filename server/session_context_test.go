package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAppSessionID_Priority(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		req  *mcpsdk.CallToolRequest
		want string
	}{
		{
			name: "context wins over headers",
			ctx:  WithAppSessionID(context.Background(), "from-ctx"),
			req: &mcpsdk.CallToolRequest{
				Extra: &mcpsdk.RequestExtra{
					Header: http.Header{
						AppSessionIDHeader: []string{"from-app-header"},
						mcpSessionIDHeader: []string{"from-mcp-header"},
					},
				},
			},
			want: "from-ctx",
		},
		{
			name: "app header wins over mcp header",
			ctx:  context.Background(),
			req: &mcpsdk.CallToolRequest{
				Extra: &mcpsdk.RequestExtra{
					Header: http.Header{
						AppSessionIDHeader: []string{"app-sess"},
						mcpSessionIDHeader: []string{"mcp-sess"},
					},
				},
			},
			want: "app-sess",
		},
		{
			name: "mcp header used when app header absent",
			ctx:  context.Background(),
			req: &mcpsdk.CallToolRequest{
				Extra: &mcpsdk.RequestExtra{
					Header: http.Header{
						mcpSessionIDHeader: []string{"mcp-only"},
					},
				},
			},
			want: "mcp-only",
		},
		{
			name: "trims whitespace on app header",
			ctx:  context.Background(),
			req: &mcpsdk.CallToolRequest{
				Extra: &mcpsdk.RequestExtra{
					Header: http.Header{
						AppSessionIDHeader: []string{"  spaced-id  "},
					},
				},
			},
			want: "spaced-id",
		},
		{
			name: "empty request falls back to default",
			ctx:  context.Background(),
			req:  nil,
			want: defaultSessionID,
		},
		{
			name: "nil session falls back to default",
			ctx:  context.Background(),
			req:  &mcpsdk.CallToolRequest{},
			want: defaultSessionID,
		},
		{
			name: "empty app header ignored",
			ctx:  context.Background(),
			req: &mcpsdk.CallToolRequest{
				Extra: &mcpsdk.RequestExtra{
					Header: http.Header{
						AppSessionIDHeader: []string{"   "},
						mcpSessionIDHeader: []string{"mcp-fallback"},
					},
				},
			},
			want: "mcp-fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveAppSessionID(tt.ctx, tt.req))
		})
	}
}

func TestAppSessionMiddleware_SetsContext(t *testing.T) {
	var got string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = AppSessionIDFromCtx(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	h := AppSessionMiddleware(inner)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(AppSessionIDHeader, "conv-abc")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "conv-abc", got)
}

func TestAppSessionMiddleware_NoHeaderLeavesEmpty(t *testing.T) {
	var got string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = AppSessionIDFromCtx(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	h := AppSessionMiddleware(inner)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Empty(t, got)
}

func TestSessionFor_UsesAppHeaderWithoutMCPSession(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})

	req := &mcpsdk.CallToolRequest{
		Extra: &mcpsdk.RequestExtra{
			Header: http.Header{AppSessionIDHeader: []string{"agent-1"}},
		},
	}
	sess := s.sessionFor(context.Background(), req)
	require.NotNil(t, sess)
	assert.Equal(t, "agent-1", sess.ID)

	// Same header, no MCP session → same store entry.
	sess2 := s.sessionFor(context.Background(), req)
	assert.Same(t, sess, sess2)

	// Different header → different session.
	reqB := &mcpsdk.CallToolRequest{
		Extra: &mcpsdk.RequestExtra{
			Header: http.Header{AppSessionIDHeader: []string{"agent-2"}},
		},
	}
	sessB := s.sessionFor(context.Background(), reqB)
	assert.Equal(t, "agent-2", sessB.ID)
	assert.NotSame(t, sess, sessB)
}

func TestSessionFor_ContextSessionWins(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	pre := newSession("preloaded")
	pre.SetContext(map[string]any{"owner": "preset"})
	ctx := withSession(context.Background(), pre)

	req := &mcpsdk.CallToolRequest{
		Extra: &mcpsdk.RequestExtra{
			Header: http.Header{AppSessionIDHeader: []string{"ignored"}},
		},
	}
	got := s.sessionFor(ctx, req)
	assert.Same(t, pre, got)
	assert.Equal(t, "preset", got.GetContext()["owner"])
}

func TestHandleSession_AppHeaderIsolatesContext(t *testing.T) {
	s := setupTestServer(&mockIntegration{name: "test", healthy: true})
	ctx := context.Background()

	reqA := sessionRequest(map[string]any{
		"action":  "set",
		"context": map[string]any{"owner": "alpha"},
	})
	reqA.Extra = &mcpsdk.RequestExtra{
		Header: http.Header{AppSessionIDHeader: []string{"sess-a"}},
	}
	result, err := s.handleSession(ctx, reqA)
	require.NoError(t, err)
	resp := parseSessionResponse(t, result)
	assert.Equal(t, "sess-a", resp["session_id"])
	assert.Equal(t, "alpha", resp["context"].(map[string]any)["owner"])

	// Different app session must not see alpha's context.
	reqB := sessionRequest(map[string]any{"action": "get"})
	reqB.Extra = &mcpsdk.RequestExtra{
		Header: http.Header{AppSessionIDHeader: []string{"sess-b"}},
	}
	result, err = s.handleSession(ctx, reqB)
	require.NoError(t, err)
	resp = parseSessionResponse(t, result)
	assert.Equal(t, "sess-b", resp["session_id"])
	assert.Empty(t, resp["context"])

	// Same app session without MCP transport id still recovers context.
	reqA2 := sessionRequest(map[string]any{"action": "get"})
	reqA2.Extra = &mcpsdk.RequestExtra{
		Header: http.Header{AppSessionIDHeader: []string{"sess-a"}},
	}
	result, err = s.handleSession(ctx, reqA2)
	require.NoError(t, err)
	resp = parseSessionResponse(t, result)
	assert.Equal(t, "alpha", resp["context"].(map[string]any)["owner"])
}

func TestHandleExecute_AppHeaderPinsAcrossCalls(t *testing.T) {
	mi := &mockIntegration{
		name:    "echo",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        "echo_ping",
				Description: "Returns pong",
				Parameters:  map[string]string{"message": "string"},
			},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
			data, _ := json.Marshal(map[string]any{"pong": args["message"]})
			return &mcp.ToolResult{Data: string(data)}, nil
		},
	}
	s := setupTestServer(mi)
	ctx := context.Background()
	appHeader := http.Header{AppSessionIDHeader: []string{"pin-sess"}}

	execReq := executeRequest("echo_ping", map[string]any{"message": "hello"})
	execReq.Extra = &mcpsdk.RequestExtra{Header: appHeader}
	result, err := s.handleExecute(ctx, execReq)
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Pin list for this app session should show $1; default session must stay empty.
	listReq := pinRequest(map[string]any{"action": "list"})
	listReq.Extra = &mcpsdk.RequestExtra{Header: appHeader}
	listResult, err := s.handlePin(ctx, listReq)
	require.NoError(t, err)
	listResp := parseSessionResponse(t, listResult)
	assert.EqualValues(t, 1, listResp["pinned_count"])

	defaultList := pinRequest(map[string]any{"action": "list"})
	defaultResult, err := s.handlePin(ctx, defaultList)
	require.NoError(t, err)
	defaultResp := parseSessionResponse(t, defaultResult)
	assert.EqualValues(t, 0, defaultResp["pinned_count"])
}

func TestAppSessionIDHeaderConstant(t *testing.T) {
	assert.Equal(t, "X-Switchboard-Session-Id", AppSessionIDHeader)
}

func TestNormalizeAppSessionID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"abc-123", "abc-123"},
		{"uuid-like_ok.here:1", "uuid-like_ok.here:1"},
		{"has space", ""},
		{"bad!", ""},
		{strings.Repeat("a", maxAppSessionIDLen), strings.Repeat("a", maxAppSessionIDLen)},
		{strings.Repeat("a", maxAppSessionIDLen+1), ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, normalizeAppSessionID(tt.in), "in=%q", tt.in)
	}
}

func TestResolveAppSessionID_RejectsInvalidHeader(t *testing.T) {
	req := &mcpsdk.CallToolRequest{
		Extra: &mcpsdk.RequestExtra{
			Header: http.Header{
				AppSessionIDHeader: []string{"not valid!!"},
				mcpSessionIDHeader: []string{"good-fallback"},
			},
		},
	}
	assert.Equal(t, "good-fallback", resolveAppSessionID(context.Background(), req))
}
