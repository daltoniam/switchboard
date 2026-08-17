package marketplace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubTokenFunc(t *testing.T) {
	fn := GitHubTokenFunc(func() string { return "ghp_test123" })

	tests := []struct {
		host string
		want string
	}{
		{"raw.githubusercontent.com", "ghp_test123"},
		{"github.com", "ghp_test123"},
		{"objects.githubusercontent.com", "ghp_test123"},
		{"api.github.com", "ghp_test123"},
		{"example.com", ""},
		{"notagithub.com", ""},
		{"raw.githubusercontent.com:443", "ghp_test123"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := fn(tt.host)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGitHubTokenFunc_EmptyToken(t *testing.T) {
	fn := GitHubTokenFunc(func() string { return "" })
	assert.Equal(t, "", fn("raw.githubusercontent.com"))
}

func TestFetchManifest_WithTokenFunc(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":1,"name":"test","plugins":[]}`))
	}))
	t.Cleanup(srv.Close)

	mgr := NewManager(Config{}, t.TempDir(), nil, WithTokenFunc(func(host string) string {
		return "my-secret-token"
	}))

	manifest, err := mgr.FetchManifest(context.Background(), srv.URL+"/manifest.json")
	require.NoError(t, err)
	assert.Equal(t, "test", manifest.Name)
	assert.Equal(t, "Bearer my-secret-token", gotAuth)
}

func TestFetchManifest_NoTokenFunc(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":1,"name":"test","plugins":[]}`))
	}))
	t.Cleanup(srv.Close)

	mgr := NewManager(Config{}, t.TempDir(), nil)

	_, err := mgr.FetchManifest(context.Background(), srv.URL+"/manifest.json")
	require.NoError(t, err)
	assert.Equal(t, "", gotAuth)
}

func TestFetchManifest_TokenFuncReturnsEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":1,"name":"test","plugins":[]}`))
	}))
	t.Cleanup(srv.Close)

	mgr := NewManager(Config{}, t.TempDir(), nil, WithTokenFunc(func(host string) string {
		return ""
	}))

	_, err := mgr.FetchManifest(context.Background(), srv.URL+"/manifest.json")
	require.NoError(t, err)
	assert.Equal(t, "", gotAuth)
}

func TestDownloadWasm_WithTokenFunc(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("fake-wasm-bytes"))
	}))
	t.Cleanup(srv.Close)

	mgr := NewManager(Config{}, t.TempDir(), nil, WithTokenFunc(func(host string) string {
		return "download-token"
	}))

	data, err := mgr.downloadWasm(context.Background(), srv.URL+"/plugin.wasm")
	require.NoError(t, err)
	assert.Equal(t, []byte("fake-wasm-bytes"), data)
	assert.Equal(t, "Bearer download-token", gotAuth)
}
