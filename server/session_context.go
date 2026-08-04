package server

import (
	"context"
	"net/http"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AppSessionIDHeader is the HTTP header clients should send to identify a
// Switchboard app session (pin / context / history). It is independent of the
// MCP transport session (Mcp-Session-Id), which go-sdk 1.7+ Stateless mode
// no longer honors.
//
// Clients that manage conversation state (e.g. Crush) should mint a UUID per
// conversation and send it on every MCP request. Legacy clients that still
// receive Mcp-Session-Id continue to work via the fallback below.
const AppSessionIDHeader = "X-Switchboard-Session-Id"

// mcpSessionIDHeader is the transport-level session header from the MCP
// streamable HTTP transport. Kept as a named constant so resolution stays in
// one place when go-sdk renames or deprecates it.
const mcpSessionIDHeader = "Mcp-Session-Id"

type sessionContextKey struct{}
type appSessionIDKey struct{}

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, s)
}

func sessionFromCtx(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionContextKey{}).(*Session)
	return s
}

// WithAppSessionID stores an app session id on the context. Used by
// AppSessionMiddleware and by tests that bypass HTTP.
func WithAppSessionID(ctx context.Context, id string) context.Context {
	id = strings.TrimSpace(id)
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, appSessionIDKey{}, id)
}

// AppSessionIDFromCtx returns the app session id previously stored with
// WithAppSessionID, or "" if none is set.
func AppSessionIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(appSessionIDKey{}).(string)
	return id
}

const defaultSessionID = "default"

// AppSessionMiddleware copies X-Switchboard-Session-Id from the HTTP request
// onto the request context so tool handlers can resolve app sessions even
// when the MCP transport session id is empty (go-sdk Stateless mode).
//
// Wrap StatelessHandler / Handler with this when serving over HTTP. The
// go-sdk streamable transport propagates req.Context() into tool handlers.
func AppSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := strings.TrimSpace(r.Header.Get(AppSessionIDHeader)); id != "" {
			r = r.WithContext(WithAppSessionID(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

// resolveAppSessionID picks the app session key for a tool call.
//
// Priority:
//  1. Context value set by AppSessionMiddleware / WithAppSessionID
//  2. X-Switchboard-Session-Id on the HTTP request (via req.Extra.Header)
//  3. Mcp-Session-Id header (legacy clients / go-sdk ≤1.6 Stateless)
//  4. MCP ServerSession.ID() (stateful transport)
//  5. "default" (single-shot scripts; shared across callers — not for multi-agent)
func resolveAppSessionID(ctx context.Context, req *mcpsdk.CallToolRequest) string {
	if id := AppSessionIDFromCtx(ctx); id != "" {
		return id
	}
	if req != nil && req.Extra != nil && req.Extra.Header != nil {
		if id := strings.TrimSpace(req.Extra.Header.Get(AppSessionIDHeader)); id != "" {
			return id
		}
		if id := strings.TrimSpace(req.Extra.Header.Get(mcpSessionIDHeader)); id != "" {
			return id
		}
	}
	if req != nil {
		return sessionIDFromMCPSession(req.Session)
	}
	return defaultSessionID
}

// sessionIDFromMCPSession reads the transport session id. Empty when the
// server is in Stateless mode on go-sdk 1.7+.
func sessionIDFromMCPSession(ss *mcpsdk.ServerSession) string {
	if ss == nil {
		return defaultSessionID
	}
	if id := ss.ID(); id != "" {
		return id
	}
	return defaultSessionID
}

// sessionFor returns the Session bound to this tool call, creating it if needed.
func (s *Server) sessionFor(ctx context.Context, req *mcpsdk.CallToolRequest) *Session {
	if sess := sessionFromCtx(ctx); sess != nil {
		return sess
	}
	return s.sessionStore.GetOrCreate(resolveAppSessionID(ctx, req))
}
