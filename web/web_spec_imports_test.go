package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const specImportOpenAPI = `{
  "openapi": "3.0.0",
  "info": {"title": "Demo API"},
  "servers": [{"url": "https://api.example.com/v1"}],
  "paths": {"/users": {"get": {"operationId": "listUsers"}}}
}`

func TestHandleSpecImportSave_ValidPersists(t *testing.T) {
	ws, _, cfg := setupTestWeb()

	form := url.Values{}
	form.Set("name", "Demo API")
	form.Set("kind", "openapi")
	form.Set("spec", specImportOpenAPI)

	req := httptest.NewRequest(http.MethodPost, "/api/spec-imports/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()

	ws.handleSpecImportSave(rw, req)

	assert.Equal(t, http.StatusSeeOther, rw.Code)
	require.Len(t, cfg.cfg.SpecImports, 1)
	assert.Equal(t, "Demo API", cfg.cfg.SpecImports[0].Name)
	assert.True(t, cfg.cfg.SpecImports[0].Enabled)
	assert.Contains(t, rw.Header().Get("Location"), "success=")
}

func TestHandleSpecImportSave_InvalidRejected(t *testing.T) {
	ws, _, cfg := setupTestWeb()

	form := url.Values{}
	form.Set("name", "Bad")
	form.Set("kind", "openapi")
	form.Set("spec", "not valid json")

	req := httptest.NewRequest(http.MethodPost, "/api/spec-imports/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()

	ws.handleSpecImportSave(rw, req)

	assert.Equal(t, http.StatusSeeOther, rw.Code)
	assert.Empty(t, cfg.cfg.SpecImports, "invalid spec must not be persisted")
	assert.Contains(t, rw.Header().Get("Location"), "error=")
}

func TestHandleSpecImportSave_Upserts(t *testing.T) {
	ws, _, cfg := setupTestWeb()
	cfg.cfg.SpecImports = []mcp.SpecImportConfig{
		{Name: "Demo API", Kind: "openapi", Spec: specImportOpenAPI, Enabled: true},
	}

	form := url.Values{}
	form.Set("name", "demo-api") // sanitizes to same "demo_api"
	form.Set("kind", "openapi")
	form.Set("spec", specImportOpenAPI)
	form.Set("api_key", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/spec-imports/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()

	ws.handleSpecImportSave(rw, req)

	require.Len(t, cfg.cfg.SpecImports, 1, "same sanitized name must replace, not append")
	assert.Equal(t, "secret", cfg.cfg.SpecImports[0].Credentials["api_key"])
}

func TestHandleSpecImportDelete_Removes(t *testing.T) {
	ws, _, cfg := setupTestWeb()
	cfg.cfg.SpecImports = []mcp.SpecImportConfig{
		{Name: "Demo API", Kind: "openapi", Spec: specImportOpenAPI, Enabled: true},
	}

	form := url.Values{}
	form.Set("name", "Demo API")

	req := httptest.NewRequest(http.MethodPost, "/api/spec-imports/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()

	ws.handleSpecImportDelete(rw, req)

	assert.Equal(t, http.StatusSeeOther, rw.Code)
	assert.Empty(t, cfg.cfg.SpecImports)
}

func TestHandleSpecImports_RendersPage(t *testing.T) {
	ws, _, cfg := setupTestWeb()
	cfg.cfg.SpecImports = []mcp.SpecImportConfig{
		{Name: "Demo API", Kind: "openapi", Spec: specImportOpenAPI, Enabled: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/spec-imports", nil)
	rw := httptest.NewRecorder()

	ws.handleSpecImports(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	body := rw.Body.String()
	assert.Contains(t, body, "Spec Imports")
	assert.Contains(t, body, "Demo API")
	assert.Contains(t, body, "Add a Spec Import")
}

func TestHandleSpecImports_FlashRenderedOnce(t *testing.T) {
	ws, _, _ := setupTestWeb()

	req := httptest.NewRequest(http.MethodGet, "/spec-imports?error=spec+import+needs+a+spec+or+path", nil)
	rw := httptest.NewRecorder()

	ws.handleSpecImports(rw, req)

	body := rw.Body.String()
	// The base layout owns flash rendering from the query string; the page
	// must not render it a second time. Count the message text, which only
	// appears inside the rendered flash div (not in CSS).
	assert.Equal(t, 1, strings.Count(body, "spec import needs a spec or path"),
		"flash message must render exactly once, not duplicated by the page")
}
