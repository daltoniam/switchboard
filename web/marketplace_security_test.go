package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/daltoniam/switchboard/marketplace"
	"github.com/daltoniam/switchboard/pluginoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mutationLoader struct {
	loads     int
	unloads   int
	unloadErr error
}

func (l *mutationLoader) LoadPlugin(context.Context, string, string) error {
	l.loads++
	return nil
}

func (l *mutationLoader) UnloadPlugin(context.Context, string) error {
	l.unloads++
	return l.unloadErr
}

func marketplaceRequest(path, form string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3847"+path, strings.NewReader(form))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestMarketplaceMutationsRejectUnsafeRequests(t *testing.T) {
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = rw.Write([]byte("untrusted wasm"))
	}))
	defer server.Close()
	for _, path := range []string{"install", "install-url", "upload", "load-path", "uninstall", "update", "check-updates", "auto-update", "add-manifest", "remove-manifest"} {
		t.Run(path, func(t *testing.T) {
			for _, attack := range []struct {
				name   string
				origin string
				site   string
				host   string
				peer   string
			}{
				{name: "foreign origin", origin: "https://evil.example"},
				{name: "null origin", origin: "null"},
				{name: "wrong origin port", origin: "http://127.0.0.1:3848"},
				{name: "cross site", site: "cross-site"},
				{name: "same site", site: "same-site"},
				{name: "matching origin cross site", origin: "http://127.0.0.1:3847", site: "cross-site"},
				{name: "rebound host", host: "evil.example:3847", origin: "http://evil.example:3847"},
				{name: "wrong host port", host: "127.0.0.1:3848"},
				{name: "nonlocal peer", peer: "192.0.2.1:12345"},
				{name: "localhost mismatched origin", host: "localhost:3847", origin: "http://127.0.0.1:3847"},
			} {
				t.Run(attack.name, func(t *testing.T) {
					w, _, _ := setupTestWeb()
					w.marketplace = marketplace.NewManager(marketplace.Config{}, t.TempDir(), func(marketplace.Config) error { return nil })
					ip, err := w.marketplace.InstallFromBytes("primer", []byte("trusted wasm"))
					require.NoError(t, err)
					loader := &mutationLoader{}
					w.wasmLoader = loader
					before := w.marketplace.Config()
					r := marketplaceRequest("/plugins/"+path, url.Values{"name": {"primer"}, "url": {server.URL + "/primer.wasm"}, "path": {ip.Path}, "enabled": {"true"}}.Encode())
					if path == "upload" {
						r = uploadPlugin(t, "primer", []byte("untrusted wasm"))
						r.Host, r.RemoteAddr = "127.0.0.1:3847", "127.0.0.1:12345"
					}
					r.Header.Set("Origin", attack.origin)
					r.Header.Set("Sec-Fetch-Site", attack.site)
					if attack.host != "" {
						r.Host = attack.host
					}
					if attack.peer != "" {
						r.RemoteAddr = attack.peer
					}
					rr := httptest.NewRecorder()
					w.Handler().ServeHTTP(rr, r)
					assert.Equal(t, http.StatusForbidden, rr.Code)
					assert.Zero(t, loader.loads)
					assert.Zero(t, loader.unloads)
					assert.Equal(t, before, w.marketplace.Config())
					data, err := os.ReadFile(ip.Path)
					require.NoError(t, err)
					assert.Equal(t, "trusted wasm", string(data))
				})
			}
		})
	}
	assert.Zero(t, downloads.Load())
}

func TestMarketplaceLocalRequestsWork(t *testing.T) {
	for _, tc := range []struct{ name, host, origin, site, peer string }{
		{name: "native", host: "127.0.0.1:3847", peer: "127.0.0.1:12345"},
		{name: "UI", host: "127.0.0.1:3847", origin: "http://127.0.0.1:3847", site: "same-origin", peer: "127.0.0.1:12345"},
		{name: "localhost UI", host: "localhost:3847", origin: "http://localhost:3847", site: "same-origin", peer: "[::1]:12345"},
		{name: "native none", host: "localhost:3847", site: "none", peer: "127.0.0.1:12345"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _, _ := setupTestWeb()
			loader := &mutationLoader{}
			w.wasmLoader = loader
			r := marketplaceRequest("/plugins/load-path", "path=/tmp/primer.wasm")
			r.Host, r.RemoteAddr = tc.host, tc.peer
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			rr := httptest.NewRecorder()
			w.Handler().ServeHTTP(rr, r)
			assert.Equal(t, http.StatusSeeOther, rr.Code)
			assert.Contains(t, rr.Header().Get("Location"), "success=")
			assert.Equal(t, 1, loader.loads)
		})
	}
}

func TestPluginUninstallPreservesArtifactOnFlushFailure(t *testing.T) {
	w, _, _ := setupTestWeb()
	w.marketplace = marketplace.NewManager(marketplace.Config{}, t.TempDir(), func(marketplace.Config) error { return nil })
	ip, err := w.marketplace.InstallFromBytes("primer", []byte("trusted wasm"))
	require.NoError(t, err)
	loader := &mutationLoader{unloadErr: pluginoauth.ErrPersistence}
	w.wasmLoader = loader
	rr := httptest.NewRecorder()
	w.Handler().ServeHTTP(rr, marketplaceRequest("/plugins/uninstall", "name=primer"))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error=")
	assert.NotContains(t, rr.Header().Get("Location"), "success=")
	assert.FileExists(t, ip.Path)
	assert.Len(t, w.marketplace.InstalledPlugins(), 1)
	assert.Equal(t, 1, loader.unloads)
	loader.unloadErr = nil
	rr = httptest.NewRecorder()
	w.Handler().ServeHTTP(rr, marketplaceRequest("/plugins/uninstall", "name=primer"))
	assert.Contains(t, rr.Header().Get("Location"), "success=")
	assert.NoFileExists(t, ip.Path)
	assert.Empty(t, w.marketplace.InstalledPlugins())
}
