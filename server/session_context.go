package server

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type sessionContextKey struct{}

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, s)
}

func sessionFromCtx(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionContextKey{}).(*Session)
	return s
}

const defaultSessionID = "default"

func sessionIDFromReq(ss *mcpsdk.ServerSession) string {
	if ss == nil {
		return defaultSessionID
	}
	if id := ss.ID(); id != "" {
		return id
	}
	return defaultSessionID
}
