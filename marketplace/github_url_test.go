package marketplace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGitHubURL(t *testing.T) {
	cases := []struct {
		name          string
		in            string
		wantURL       string
		wantRawAccept bool
	}{
		{
			name:          "blob_html_url_rewritten_to_contents_api",
			in:            "https://github.com/teamcurri/switchboard_plugins/blob/main/manifest.json",
			wantURL:       "https://api.github.com/repos/teamcurri/switchboard_plugins/contents/manifest.json?ref=main",
			wantRawAccept: true,
		},
		{
			name:          "raw_html_url_rewritten_to_contents_api",
			in:            "https://github.com/teamcurri/switchboard_plugins/raw/main/dist/curri.wasm",
			wantURL:       "https://api.github.com/repos/teamcurri/switchboard_plugins/contents/dist/curri.wasm?ref=main",
			wantRawAccept: true,
		},
		{
			name:          "blob_url_master_branch",
			in:            "https://github.com/teamcurri/switchboard_plugins/blob/master/manifest.json",
			wantURL:       "https://api.github.com/repos/teamcurri/switchboard_plugins/contents/manifest.json?ref=master",
			wantRawAccept: true,
		},
		{
			name:          "blob_url_nested_path",
			in:            "https://github.com/o/r/blob/dev/sub/dir/file.json",
			wantURL:       "https://api.github.com/repos/o/r/contents/sub/dir/file.json?ref=dev",
			wantRawAccept: true,
		},
		{
			name:          "raw_cdn_left_untouched",
			in:            "https://raw.githubusercontent.com/teamcurri/switchboard_plugins/main/manifest.json",
			wantURL:       "https://raw.githubusercontent.com/teamcurri/switchboard_plugins/main/manifest.json",
			wantRawAccept: false,
		},
		{
			name:          "contents_api_left_untouched",
			in:            "https://api.github.com/repos/o/r/contents/manifest.json?ref=main",
			wantURL:       "https://api.github.com/repos/o/r/contents/manifest.json?ref=main",
			wantRawAccept: false,
		},
		{
			name:          "non_github_url_passthrough",
			in:            "https://example.com/foo/manifest.json",
			wantURL:       "https://example.com/foo/manifest.json",
			wantRawAccept: false,
		},
		{
			name:          "github_release_download_passthrough",
			in:            "https://github.com/o/r/releases/download/v1/plugin.wasm",
			wantURL:       "https://github.com/o/r/releases/download/v1/plugin.wasm",
			wantRawAccept: false,
		},
		{
			name:          "malformed_url_passthrough",
			in:            "::not-a-url",
			wantURL:       "::not-a-url",
			wantRawAccept: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotRaw := normalizeGitHubURL(tc.in)
			assert.Equal(t, tc.wantURL, gotURL)
			assert.Equal(t, tc.wantRawAccept, gotRaw)
		})
	}
}

// TestFetchManifest_RewritesBlobURL verifies a user-pasted GitHub HTML
// /blob/ URL is rewritten to the Contents API and accompanied by the bearer
// token and raw-bytes Accept header so private-repo manifests fetch successfully.
func TestFetchManifest_RewritesBlobURL(t *testing.T) {
	var (
		gotPath   string
		gotAuth   string
		gotAccept string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":1,"name":"private-mf","plugins":[]}`))
	}))
	t.Cleanup(srv.Close)

	// Hijack the api.github.com host by pointing the manager's HTTP client
	// at our test server. Use a custom transport rather than a global rewrite.
	mgr := NewManager(Config{}, t.TempDir(), nil, WithTokenFunc(func(host string) string {
		if host == "api.github.com" {
			return "ghp_secret"
		}
		return ""
	}))
	mgr.client = srv.Client()
	mgr.client.Transport = &rewriteTransport{
		match:   "api.github.com",
		target:  srv.URL,
		wrapped: http.DefaultTransport,
	}

	manifest, err := mgr.FetchManifest(context.Background(),
		"https://github.com/teamcurri/switchboard_plugins/blob/main/manifest.json")
	require.NoError(t, err)
	assert.Equal(t, "private-mf", manifest.Name)
	assert.Equal(t, "/repos/teamcurri/switchboard_plugins/contents/manifest.json?ref=main", gotPath)
	assert.Equal(t, "Bearer ghp_secret", gotAuth)
	assert.Equal(t, "application/vnd.github.raw", gotAccept)
}

// TestDownloadWasm_RewritesBlobURL ensures the same normalization applies to
// WASM downloads, so a /raw/ HTML URL in manifest.json works for private repos.
func TestDownloadWasm_RewritesBlobURL(t *testing.T) {
	wasmBytes := []byte("\x00asm\x01\x00\x00\x00")
	var (
		gotAuth   string
		gotAccept string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write(wasmBytes)
	}))
	t.Cleanup(srv.Close)

	mgr := NewManager(Config{}, t.TempDir(), nil, WithTokenFunc(func(host string) string {
		if host == "api.github.com" {
			return "ghp_secret"
		}
		return ""
	}))
	mgr.client = srv.Client()
	mgr.client.Transport = &rewriteTransport{
		match:   "api.github.com",
		target:  srv.URL,
		wrapped: http.DefaultTransport,
	}

	data, err := mgr.downloadWasm(context.Background(),
		"https://github.com/teamcurri/switchboard_plugins/raw/main/dist/here.wasm")
	require.NoError(t, err)
	assert.Equal(t, wasmBytes, data)
	assert.Equal(t, "Bearer ghp_secret", gotAuth)
	assert.Equal(t, "application/vnd.github.raw", gotAccept)
}

// rewriteTransport rewrites requests whose Host matches `match` to point at
// `target` (a test server URL), so we can intercept api.github.com calls.
type rewriteTransport struct {
	match   string
	target  string
	wrapped http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == t.match {
		targetURL, err := req.URL.Parse(t.target)
		if err != nil {
			return nil, err
		}
		req = req.Clone(req.Context())
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host
	}
	return t.wrapped.RoundTrip(req)
}
