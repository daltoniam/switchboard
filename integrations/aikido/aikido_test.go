package aikido

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClientID     = "AIK_CLIENT_test"
	testClientSecret = "test-secret"
)

// fakeAikido is an Aikido app with the OAuth token endpoint and a handler
// for public API paths.
type fakeAikido struct {
	*httptest.Server
	tokens   atomic.Int32
	mu       sync.Mutex
	requests []*recorded
	api      http.HandlerFunc
	token    func() string
}

type recorded struct {
	method, path, query, body, auth string
}

func newFake(t *testing.T, api http.HandlerFunc) *fakeAikido {
	t.Helper()
	f := &fakeAikido{api: api}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path == tokenPath {
			want := "Basic " + base64.StdEncoding.EncodeToString([]byte(testClientID+":"+testClientSecret))
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != want || !strings.Contains(string(body), `"grant_type":"client_credentials"`) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"invalid_client","error_description":"The provided client credentials are invalid"}`)
				return
			}
			n := f.tokens.Add(1)
			tok := "tok-" + string(rune('0'+n))
			if f.token != nil {
				tok = f.token()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "expires_in": 3600, "token_type": "bearer"})
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok-") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"invalid token"}`)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, &recorded{r.Method, strings.TrimPrefix(r.URL.EscapedPath(), apiPath), r.URL.RawQuery, string(body), r.Header.Get("Authorization")})
		f.mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		f.api(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeAikido) last(t *testing.T) *recorded {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.requests)
	return f.requests[len(f.requests)-1]
}

func configured(t *testing.T, f *fakeAikido) *aikido {
	t.Helper()
	a := New().(*aikido)
	require.NoError(t, a.Configure(t.Context(), mcp.Credentials{"client_id": testClientID, "client_secret": testClientSecret, "base_url": f.URL}))
	return a
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "aikido", i.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		origin  string
	}{
		{"defaults to eu", mcp.Credentials{"client_id": "id", "client_secret": "s"}, "", "https://app.aikido.dev"},
		{"us region", mcp.Credentials{"client_id": "id", "client_secret": "s", "region": "US"}, "", "https://app.us.aikido.dev"},
		{"au region", mcp.Credentials{"client_id": "id", "client_secret": "s", "region": "au"}, "", "https://app.au.aikido.dev"},
		{"me region", mcp.Credentials{"client_id": "id", "client_secret": "s", "region": "me"}, "", "https://app.me.aikido.dev"},
		{"base url wins over region", mcp.Credentials{"client_id": "id", "client_secret": "s", "region": "us", "base_url": "https://app.au.aikido.dev"}, "", "https://app.au.aikido.dev"},
		{"terraform style base url", mcp.Credentials{"client_id": "id", "client_secret": "s", "base_url": "https://app.us.aikido.dev/api"}, "", "https://app.us.aikido.dev"},
		{"public api base url", mcp.Credentials{"client_id": "id", "client_secret": "s", "base_url": "https://app.us.aikido.dev/api/public/v1/"}, "", "https://app.us.aikido.dev"},
		{"loopback http allowed", mcp.Credentials{"client_id": "id", "client_secret": "s", "base_url": "http://127.0.0.1:8080"}, "", "http://127.0.0.1:8080"},
		{"missing client id", mcp.Credentials{"client_secret": "s"}, "client_id is required", ""},
		{"missing client secret", mcp.Credentials{"client_id": "id"}, "client_secret is required", ""},
		{"client id with spaces", mcp.Credentials{"client_id": "a b", "client_secret": "s"}, "client_id is required", ""},
		{"unknown region", mcp.Credentials{"client_id": "id", "client_secret": "s", "region": "mars"}, "region must be eu, us, au or me", ""},
		{"plain http remote", mcp.Credentials{"client_id": "id", "client_secret": "s", "base_url": "http://app.aikido.dev"}, "base_url must be an HTTPS URL", ""},
		{"base url with credentials", mcp.Credentials{"client_id": "id", "client_secret": "s", "base_url": "https://u:p@app.aikido.dev"}, "base_url must be an HTTPS URL", ""},
		{"base url with query", mcp.Credentials{"client_id": "id", "client_secret": "s", "base_url": "https://app.aikido.dev?x=1"}, "base_url must be an HTTPS URL", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New().(*aikido)
			err := a.Configure(t.Context(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				_, ok := a.snapshot()
				assert.False(t, ok, "a failed Configure must leave the adapter unconfigured")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.origin, a.conn.origin)
		})
	}
}

func TestTools_Metadata(t *testing.T) {
	a := New()
	seen := map[mcp.ToolName]bool{}
	for _, tool := range a.Tools() {
		assert.True(t, strings.HasPrefix(string(tool.Name), "aikido_"), tool.Name)
		assert.NotEmpty(t, tool.Description, tool.Name)
		assert.True(t, tool.Access.Declared(), "%s has no access class", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		assert.NotContains(t, tool.Description, "\u2014", "%s uses an em dash", tool.Name)
		seen[tool.Name] = true
		for _, required := range tool.Required {
			assert.Contains(t, tool.Parameters, required, "%s requires undeclared %s", tool.Name, required)
		}
	}
	assert.Contains(t, dispatch["aikido_list_issue_groups"].desc, "Start here")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch entry", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch entry %s has no tool definition", name)
	}
}

// Access follows what a call can change: reads never mutate, and deleting
// a resource is never classified as a plain read.
func TestEndpoints_AccessMatchesMethod(t *testing.T) {
	for _, ep := range endpoints {
		switch ep.method {
		case http.MethodGet:
			assert.Equal(t, mcp.AccessRead, ep.access, "%s is a GET but not read", ep.tool)
			assert.True(t, strings.HasSuffix(ep.scope, ":read"), "%s scope %q", ep.tool, ep.scope)
		default:
			assert.NotEqual(t, mcp.AccessRead, ep.access, "%s mutates but is marked read", ep.tool)
			assert.True(t, strings.HasSuffix(ep.scope, ":write"), "%s scope %q", ep.tool, ep.scope)
		}
		if strings.HasPrefix(string(ep.tool), "aikido_delete_") {
			assert.Equal(t, mcp.AccessDestructive, ep.access, ep.tool)
		}
	}
}

func TestEndpoints_PathParamsDeclared(t *testing.T) {
	for _, ep := range endpoints {
		declared := map[string]bool{}
		for _, p := range ep.params {
			if p.in == inPath {
				declared[p.name] = true
				assert.True(t, p.required, "%s path param %s must be required", ep.tool, p.name)
			}
		}
		required := dispatch[ep.tool].definition().Required
		for _, m := range pathParam.FindAllStringSubmatch(ep.path, -1) {
			assert.True(t, declared[m[1]], "%s path {%s} has no parameter", ep.tool, m[1])
			assert.Contains(t, required, m[1], "%s does not advertise path parameter %s as required", ep.tool, m[1])
		}
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	result, err := New().Execute(t.Context(), "aikido_nope", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_Unconfigured(t *testing.T) {
	result, err := New().Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestHealthy(t *testing.T) {
	assert.False(t, New().Healthy(t.Context()), "unconfigured adapter is unhealthy")

	f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == apiPath+"/workspace" {
			jsonHandler(`{"id":1,"name":"Acme"}`)(w, r)
			return
		}
		http.NotFound(w, r)
	})
	assert.True(t, configured(t, f).Healthy(t.Context()))

	bad := New().(*aikido)
	require.NoError(t, bad.Configure(t.Context(), mcp.Credentials{"client_id": testClientID, "client_secret": "wrong", "base_url": f.URL}))
	assert.False(t, bad.Healthy(t.Context()))
}

func TestToken_CachedAcrossCalls(t *testing.T) {
	f := newFake(t, jsonHandler(`{"id":1}`))
	a := configured(t, f)
	for range 3 {
		result, err := a.Execute(t.Context(), "aikido_get_workspace", nil)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
	}
	assert.EqualValues(t, 1, f.tokens.Load(), "the token is fetched once and reused")
}

func TestToken_RefreshedBeforeExpiry(t *testing.T) {
	f := newFake(t, jsonHandler(`{"id":1}`))
	a := configured(t, f)
	clock := time.Now()
	a.now = func() time.Time { return clock }
	_, err := a.Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	clock = clock.Add(time.Hour - 30*time.Second)
	_, err = a.Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 2, f.tokens.Load(), "a token within the refresh margin is replaced")
}

func TestToken_RevokedTokenRetriedOnce(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer tok-1" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"token revoked"}`)
			return
		}
		jsonHandler(`{"id":1}`)(w, r)
	})
	a := configured(t, f)
	result, err := a.Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.EqualValues(t, 2, f.tokens.Load())
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 2)
	assert.Equal(t, "Bearer tok-1", f.requests[0].auth)
	assert.Equal(t, "Bearer tok-2", f.requests[1].auth, "the retry uses the newly issued token")
}

// A request that snapshotted the old configuration before Configure ran
// must keep using the old origin's own token, never the new one, and must
// not fill the new configuration's cache.
func TestToken_BoundToConfiguration(t *testing.T) {
	oldFake := newFake(t, jsonHandler(`{"id":1}`))
	newFake := newFake(t, jsonHandler(`{"id":2}`))
	newFake.token = func() string { return "tok-new" }
	a := configured(t, oldFake)
	stale, ok := a.snapshot()
	require.True(t, ok)
	require.NoError(t, a.Configure(t.Context(), mcp.Credentials{"client_id": testClientID, "client_secret": testClientSecret, "base_url": newFake.URL}))
	require.True(t, a.Healthy(t.Context()))

	tok, err := a.token(t.Context(), stale)
	require.NoError(t, err)
	assert.Equal(t, "tok-1", tok, "the stale request gets a token from its own origin")
	assert.EqualValues(t, 1, oldFake.tokens.Load())
	current, _ := a.snapshot()
	assert.Equal(t, "tok-new", current.accessToken, "the new configuration's cache is untouched")
}

func TestToken_PersistentUnauthorizedIsAnError(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"nope"}`)
	})
	result, err := configured(t, f).Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "401")
	assert.Contains(t, result.Data, "REST API")
	assert.EqualValues(t, 2, f.tokens.Load(), "exactly one retry with a fresh token")
}

func TestToken_InvalidClientExplainsWhereToFix(t *testing.T) {
	f := newFake(t, jsonHandler(`{}`))
	a := New().(*aikido)
	require.NoError(t, a.Configure(t.Context(), mcp.Credentials{"client_id": testClientID, "client_secret": "wrong", "base_url": f.URL}))
	result, err := a.Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid_client")
	assert.Contains(t, result.Data, "Settings > Integrations > REST API")
	assert.NotContains(t, result.Data, "wrong")
}

func TestRequest_Building(t *testing.T) {
	tests := []struct {
		name      string
		tool      mcp.ToolName
		args      map[string]any
		method    string
		path      string
		query     map[string]string
		body      string
		wantError string
	}{
		{
			name: "path, defaults and fixed format", tool: "aikido_list_issues",
			args:   map[string]any{"filter_severities": []any{"critical", "high"}, "filter_code_repo_id": float64(12)},
			method: http.MethodGet, path: "/issues/export",
			query: map[string]string{"format": "json", "page": "0", "per_page": "50", "filter_severities": "critical,high", "filter_code_repo_id": "12"},
		},
		{
			name: "path parameter escaped", tool: "aikido_get_cve",
			args:   map[string]any{"cve_id": "CVE-2021/44228"},
			method: http.MethodGet, path: "/cve/CVE-2021%2F44228",
		},
		{
			name: "body fields for put", tool: "aikido_snooze_issue",
			args:   map[string]any{"issue_id": "77", "snooze_until": float64(1900000000), "reason": "waiting on vendor", "unrelated": "ignored"},
			method: http.MethodPut, path: "/issues/77/snooze",
			body: `{"reason":"waiting on vendor","snooze_until":1900000000}`,
		},
		{
			name: "query flag on post", tool: "aikido_scan_code_repo",
			args:   map[string]any{"code_repo_id": float64(5), "include_sast_scan": true},
			method: http.MethodPost, path: "/repositories/code/5/scan",
			query: map[string]string{"include_sast_scan": "true"}, body: `{}`,
		},
		{
			name: "boolean sent as integer", tool: "aikido_list_users",
			args:   map[string]any{"include_inactive": true},
			method: http.MethodGet, path: "/users",
			query: map[string]string{"include_inactive": "1"},
		},
		{
			name: "sbom format override", tool: "aikido_export_code_repo_sbom",
			args:   map[string]any{"code_repo_id": float64(3), "format": "sbom_spdx"},
			method: http.MethodGet, path: "/repositories/code/3/licenses/export",
			query: map[string]string{"format": "sbom_spdx"},
		},
		{
			name: "integer list", tool: "aikido_list_cloud_assets",
			args:   map[string]any{"cloud_id": []any{float64(1), "2"}},
			method: http.MethodGet, path: "/clouds/assets",
			query: map[string]string{"cloud_id": "1,2"},
		},
		{
			name: "selector in path", tool: "aikido_get_compliance_overview",
			args:   map[string]any{"framework": "soc2"},
			method: http.MethodGet, path: "/report/soc2/overview",
		},
		{name: "missing required", tool: "aikido_snooze_issue", args: map[string]any{"issue_id": float64(1)}, wantError: "missing required argument(s): snooze_until"},
		{name: "missing path id", tool: "aikido_get_issue", args: map[string]any{}, wantError: "missing required argument(s): issue_id"},
		{name: "enum checked", tool: "aikido_adjust_issue_severity", args: map[string]any{"issue_id": float64(1), "adjusted_severity": "urgent", "reason": "x"}, wantError: "adjusted_severity must be one of critical, high, medium, low"},
		{name: "enum list checked", tool: "aikido_list_issues", args: map[string]any{"filter_severities": "high,urgent"}, wantError: `got "urgent"`},
		{name: "per_page bounded", tool: "aikido_list_code_repos", args: map[string]any{"per_page": float64(5)}, wantError: "per_page must be between 10 and 200"},
		{name: "wrong type", tool: "aikido_get_issue", args: map[string]any{"issue_id": "abc"}, wantError: "cannot convert"},
		{name: "bad integer list", tool: "aikido_list_cloud_assets", args: map[string]any{"cloud_id": "1,x"}, wantError: "cloud_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, jsonHandler(`{"ok":true}`))
			result, err := configured(t, f).Execute(t.Context(), tt.tool, tt.args)
			require.NoError(t, err)
			if tt.wantError != "" {
				require.True(t, result.IsError)
				assert.Contains(t, result.Data, tt.wantError)
				f.mu.Lock()
				assert.Empty(t, f.requests, "an invalid call must not reach Aikido")
				f.mu.Unlock()
				return
			}
			require.False(t, result.IsError, result.Data)
			got := f.last(t)
			assert.Equal(t, tt.method, got.method)
			assert.Equal(t, tt.path, got.path)
			q := parseQuery(t, got.query)
			for k, v := range tt.query {
				assert.Equal(t, v, q[k], "query %s", k)
			}
			if tt.body != "" {
				assert.JSONEq(t, tt.body, got.body)
			}
		})
	}
}

func parseQuery(t *testing.T, raw string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, pair := range strings.Split(raw, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		k = unescape(t, k)
		out[k] = unescape(t, v)
	}
	return out
}

func unescape(t *testing.T, s string) string {
	t.Helper()
	r := strings.NewReplacer("%2C", ",", "%2F", "/", "%3A", ":", "+", " ")
	return r.Replace(s)
}

func TestPagedResults(t *testing.T) {
	tests := []struct {
		name    string
		tool    mcp.ToolName
		args    map[string]any
		header  string
		body    string
		want    string
		hasNext bool
	}{
		{"header true", "aikido_list_issue_groups", nil, "true", `[{"id":1}]`, `[{"id":1}]`, true},
		{"header false", "aikido_list_issue_groups", nil, "false", `[{"id":1}]`, `[{"id":1}]`, false},
		{"full page without header", "aikido_list_activity_log", map[string]any{"per_page": float64(2)}, "", `[{"id":1},{"id":2}]`, `[{"id":1},{"id":2}]`, true},
		{"short page without header", "aikido_list_activity_log", map[string]any{"per_page": float64(2)}, "", `[{"id":1}]`, `[{"id":1}]`, false},
		{"unwrapped object", "aikido_list_zen_users", map[string]any{"app_id": float64(4)}, "", `{"users":[{"id":"u1"}]}`, `[{"id":"u1"}]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.header != "" {
					w.Header().Set("X-Has-Next-Page", tt.header)
				}
				_, _ = io.WriteString(w, tt.body)
			})
			result, err := configured(t, f).Execute(t.Context(), tt.tool, tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var got struct {
				Results     json.RawMessage `json:"results"`
				Page        int             `json:"page"`
				PerPage     int             `json:"per_page"`
				HasNextPage bool            `json:"has_next_page"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
			assert.JSONEq(t, tt.want, string(got.Results))
			assert.Equal(t, tt.hasNext, got.HasNextPage)
			assert.Equal(t, 0, got.Page)
			assert.Positive(t, got.PerPage)
		})
	}
}

func TestDataLeakPasswordsRedacted(t *testing.T) {
	f := newFake(t, jsonHandler(`{"dataleaks":[{"id":"l1","email":"dev@example.com","password":"hunter2","status":"open"},{"id":"l2","email":"ops@example.com","password":null}]}`))
	result, err := configured(t, f).Execute(t.Context(), "aikido_list_domain_data_leaks", map[string]any{"domain_id": float64(9)})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.NotContains(t, result.Data, "hunter2")
	assert.Contains(t, result.Data, "[redacted]")
	assert.Contains(t, result.Data, "dev@example.com")
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		header     map[string]string
		body       string
		wantErr    string
		retryable  bool
		retryAfter time.Duration
		wantData   string
	}{
		{name: "forbidden names the scope", status: http.StatusForbidden, body: `{"error":"missing scope"}`, wantErr: "needs the issues:read scope"},
		{name: "not found", status: http.StatusNotFound, body: `{"error":"Issue not found"}`, wantErr: "aikido API error (404) on GET /issues/5: Issue not found"},
		{name: "rate limited", status: http.StatusTooManyRequests, header: map[string]string{"Retry-After": "7"}, body: `{"error":"You have reached the maximum number of calls per minute. (20 calls per minute)"}`, retryable: true, retryAfter: 7 * time.Second},
		{name: "server error", status: http.StatusBadGateway, body: `oops`, retryable: true},
		{name: "redirect not followed", status: http.StatusFound, header: map[string]string{"Location": "https://evil.example/steal"}, wantErr: "redirect"},
		{name: "empty success", status: http.StatusOK, body: ``, wantData: `{"ok":true,"status":200}`},
		{name: "long error truncated", status: http.StatusBadRequest, body: strings.Repeat("x", 3000), wantErr: "(truncated)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			result, err := configured(t, f).Execute(t.Context(), "aikido_get_issue", map[string]any{"issue_id": float64(5)})
			if tt.retryable {
				require.Error(t, err)
				var retry *mcp.RetryableError
				require.True(t, errors.As(err, &retry))
				assert.Equal(t, tt.status, retry.StatusCode)
				assert.Equal(t, tt.retryAfter, retry.RetryAfter)
				return
			}
			require.NoError(t, err)
			if tt.wantErr != "" {
				require.True(t, result.IsError)
				assert.Contains(t, result.Data, tt.wantErr)
				return
			}
			require.False(t, result.IsError, result.Data)
			assert.JSONEq(t, tt.wantData, result.Data)
		})
	}
}

func TestNonJSONExportReturnedAsText(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = io.WriteString(w, "package,license\nlodash,MIT\n")
	})
	result, err := configured(t, f).Execute(t.Context(), "aikido_export_code_repo_sbom", map[string]any{"code_repo_id": float64(1), "format": "csv"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, "package,license\nlodash,MIT\n", result.Data)
}

func TestClose(t *testing.T) {
	f := newFake(t, jsonHandler(`{"id":1}`))
	a := configured(t, f)
	require.True(t, a.Healthy(t.Context()))
	require.NoError(t, a.Close())
	require.NoError(t, a.Close(), "Close is safe to call twice")
	assert.False(t, a.Healthy(t.Context()))
	result, err := a.Execute(t.Context(), "aikido_get_workspace", nil)
	require.NoError(t, err)
	assert.Contains(t, result.Data, "not configured")

	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"client_id": testClientID, "client_secret": testClientSecret, "base_url": f.URL}))
	assert.True(t, a.Healthy(t.Context()), "a closed adapter can be configured again")
	assert.EqualValues(t, 2, f.tokens.Load(), "Close drops the cached token")
}

func TestReconfigureDropsToken(t *testing.T) {
	f := newFake(t, jsonHandler(`{"id":1}`))
	a := configured(t, f)
	require.True(t, a.Healthy(t.Context()))
	require.NoError(t, a.Configure(t.Context(), mcp.Credentials{"client_id": testClientID, "client_secret": testClientSecret, "base_url": f.URL}))
	require.True(t, a.Healthy(t.Context()))
	assert.EqualValues(t, 2, f.tokens.Load(), "new credentials never reuse the previous token")
}
