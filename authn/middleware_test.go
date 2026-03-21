package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daltoniam/switchboard/tenant"
)

func TestMiddleware_ValidHeaders(t *testing.T) {
	m := New()

	var gotInfo tenant.Info
	var gotOK bool
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotInfo, gotOK = tenant.FromContext(r.Context())
	})

	handler := m.Wrap(inner)

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("X-Tenant-ID", "acme-corp")
	req.Header.Set("X-User-ID", "alice")
	req.Header.Set("X-Scopes", "read, write, admin")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !gotOK {
		t.Fatal("expected tenant info in context")
	}
	if gotInfo.TenantID != "acme-corp" {
		t.Errorf("TenantID = %q, want %q", gotInfo.TenantID, "acme-corp")
	}
	if gotInfo.UserID != "alice" {
		t.Errorf("UserID = %q, want %q", gotInfo.UserID, "alice")
	}
	if len(gotInfo.Scopes) != 3 || gotInfo.Scopes[0] != "read" || gotInfo.Scopes[1] != "write" || gotInfo.Scopes[2] != "admin" {
		t.Errorf("Scopes = %v, want [read write admin]", gotInfo.Scopes)
	}
}

func TestMiddleware_MissingTenantID(t *testing.T) {
	m := New()
	handler := m.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("X-User-ID", "alice")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestMiddleware_MissingUserID(t *testing.T) {
	m := New()
	handler := m.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("X-Tenant-ID", "acme-corp")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestMiddleware_NoScopes(t *testing.T) {
	m := New()

	var gotInfo tenant.Info
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotInfo, _ = tenant.FromContext(r.Context())
	})

	handler := m.Wrap(inner)

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("X-Tenant-ID", "acme-corp")
	req.Header.Set("X-User-ID", "alice")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(gotInfo.Scopes) != 0 {
		t.Errorf("Scopes = %v, want empty", gotInfo.Scopes)
	}
}

func TestMiddleware_CustomHeaders(t *testing.T) {
	m := &Middleware{
		TenantHeader: "X-Org-ID",
		UserHeader:   "X-Account-ID",
		ScopesHeader: "X-Permissions",
	}

	var gotInfo tenant.Info
	var gotOK bool
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotInfo, gotOK = tenant.FromContext(r.Context())
	})

	handler := m.Wrap(inner)

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("X-Org-ID", "custom-tenant")
	req.Header.Set("X-Account-ID", "custom-user")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !gotOK {
		t.Fatal("expected tenant info in context")
	}
	if gotInfo.TenantID != "custom-tenant" {
		t.Errorf("TenantID = %q, want %q", gotInfo.TenantID, "custom-tenant")
	}
	if gotInfo.UserID != "custom-user" {
		t.Errorf("UserID = %q, want %q", gotInfo.UserID, "custom-user")
	}
}
