package tenant

import "context"

// Info holds the identity of the authenticated tenant and user for a request.
// In hosted mode, this is populated from JWT claims or gateway headers.
// In local mode, this is absent from the context (FromContext returns false).
type Info struct {
	TenantID string
	UserID   string
	Scopes   []string
}

type contextKey struct{}

// WithContext returns a new context carrying the tenant info.
func WithContext(ctx context.Context, t Info) context.Context {
	return context.WithValue(ctx, contextKey{}, t)
}

// FromContext extracts tenant info from the context.
// Returns false if no tenant info is present (i.e. local mode).
func FromContext(ctx context.Context) (Info, bool) {
	t, ok := ctx.Value(contextKey{}).(Info)
	return t, ok
}

// HasScope reports whether the tenant info includes the given scope.
func (t Info) HasScope(scope string) bool {
	for _, s := range t.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
