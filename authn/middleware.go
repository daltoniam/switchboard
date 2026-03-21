package authn

import (
	"net/http"
	"strings"

	"github.com/daltoniam/switchboard/tenant"
)

// Middleware extracts tenant identity from HTTP requests.
// In gateway-trusted mode (JWKSURL empty), it reads identity from headers set by the gateway.
// In JWT mode (JWKSURL set), it validates the Bearer token and extracts claims.
type Middleware struct {
	TenantHeader string // default: "X-Tenant-ID"
	UserHeader   string // default: "X-User-ID"
	ScopesHeader string // default: "X-Scopes"
}

// New creates a Middleware with default header names.
func New() *Middleware {
	return &Middleware{
		TenantHeader: "X-Tenant-ID",
		UserHeader:   "X-User-ID",
		ScopesHeader: "X-Scopes",
	}
}

// Wrap returns an http.Handler that extracts tenant identity from request headers
// and stores it in the request context. Returns 401 if tenant or user headers are missing.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	tenantH := m.TenantHeader
	if tenantH == "" {
		tenantH = "X-Tenant-ID"
	}
	userH := m.UserHeader
	if userH == "" {
		userH = "X-User-ID"
	}
	scopesH := m.ScopesHeader
	if scopesH == "" {
		scopesH = "X-Scopes"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get(tenantH)
		if tenantID == "" {
			http.Error(w, "missing tenant identity", http.StatusUnauthorized)
			return
		}

		userID := r.Header.Get(userH)
		if userID == "" {
			http.Error(w, "missing user identity", http.StatusUnauthorized)
			return
		}

		var scopes []string
		if raw := r.Header.Get(scopesH); raw != "" {
			for _, s := range strings.Split(raw, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					scopes = append(scopes, s)
				}
			}
		}

		info := tenant.Info{
			TenantID: tenantID,
			UserID:   userID,
			Scopes:   scopes,
		}

		next.ServeHTTP(w, r.WithContext(tenant.WithContext(r.Context(), info)))
	})
}
