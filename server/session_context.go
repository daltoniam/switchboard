package server

import "context"

type sessionContextKey struct{}

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, s)
}

func sessionFromCtx(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionContextKey{}).(*Session)
	return s
}
