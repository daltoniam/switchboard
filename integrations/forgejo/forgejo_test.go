package forgejo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func configured(t *testing.T, h http.HandlerFunc) (mcp.Integration, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	f := New()
	require.NoError(t, f.Configure(context.Background(), mcp.Credentials{"base_url": s.URL + "/forge/api/v1/", "token": "test-token"}))
	return f, s
}

func TestNewAndUnconfigured(t *testing.T) {
	f := New()
	require.Equal(t, "forgejo", f.Name())
	require.False(t, f.Healthy(context.Background()))
	var nilForgejo *forgejo
	require.False(t, nilForgejo.Healthy(context.Background()))
	result, err := f.Execute(context.Background(), "forgejo_get_current_user", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	result, err = f.Execute(context.Background(), "forgejo_unknown", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "unknown tool")
}

func TestConfigureValidationAndAtomicity(t *testing.T) {
	var calls atomic.Int32
	f, s := configured(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/forge/api/v1/user", r.URL.Path)
		require.Equal(t, "token test-token", r.Header.Get("Authorization"))
		fmt.Fprint(w, `{"id":1,"login":"alice"}`)
	})
	require.Zero(t, calls.Load())
	for _, creds := range []mcp.Credentials{
		{}, {"base_url": s.URL}, {"token": "secret"},
		{"base_url": "example.com", "token": "secret"},
		{"base_url": "ftp://example.com", "token": "secret"},
		{"base_url": "http://", "token": "secret"},
		{"base_url": "http://user:password@example.com", "token": "secret"},
		{"base_url": s.URL + "?token=secret", "token": "secret"},
		{"base_url": s.URL + "?", "token": "secret"},
		{"base_url": s.URL + "#fragment", "token": "secret"},
		{"base_url": s.URL + "#", "token": "secret"},
		{"base_url": s.URL + "/%zz", "token": "secret"},
		{"base_url": s.URL + "/../admin", "token": "secret"},
		{"base_url": s.URL + "/%2e%2e/admin", "token": "secret"},
		{"base_url": s.URL, "token": " \t"},
		{"base_url": s.URL, "token": "a\nb"},
	} {
		t.Run(fmt.Sprint(creds["base_url"], "/", len(creds["token"])), func(t *testing.T) {
			err := f.Configure(context.Background(), creds)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
			require.NotContains(t, err.Error(), "password")
		})
	}
	require.Zero(t, calls.Load())
	require.True(t, f.Healthy(context.Background()))
	require.EqualValues(t, 1, calls.Load())
}

func TestURLNormalization(t *testing.T) {
	for _, suffix := range []string{"", "/", "/api/v1", "/api/v1/", "/forge", "/forge/", "/forge/api/v1/"} {
		t.Run(suffix, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				prefix := ""
				if strings.Contains(suffix, "forge") {
					prefix = "/forge"
				}
				require.Equal(t, prefix+"/api/v1/user", r.URL.Path)
				fmt.Fprint(w, `{"id":1}`)
			}))
			defer s.Close()
			f := New()
			require.NoError(t, f.Configure(context.Background(), mcp.Credentials{"base_url": s.URL + suffix, "token": "token"}))
			require.True(t, f.Healthy(context.Background()))
		})
	}
}

func TestHTTPStatuses(t *testing.T) {
	for _, tool := range []mcp.ToolName{"forgejo_get_repo", "forgejo_merge_pull"} {
		for _, status := range []int{200, 204, 401, 403, 404, 429, 500} {
			t.Run(fmt.Sprintf("%s/%d", tool, status), func(t *testing.T) {
				f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "7")
					w.WriteHeader(status)
					if status != 204 {
						fmt.Fprint(w, `{"message":"upstream failure"}`)
					}
				})
				args := map[string]any{"owner": "alice", "repo": "demo"}
				if tool == "forgejo_merge_pull" {
					args["number"] = 12
				}
				result, err := f.Execute(context.Background(), tool, args)
				if tool == "forgejo_get_repo" && (status == 429 || status >= 500) {
					var retry *mcp.RetryableError
					require.ErrorAs(t, err, &retry)
					require.Equal(t, status, retry.StatusCode)
					require.Equal(t, 7*time.Second, retry.RetryAfter)
				} else {
					require.NoError(t, err)
					require.Equal(t, status >= 400, result.IsError, result.Data)
				}
			})
		}
	}
}

func TestRedirectsDenied(t *testing.T) {
	for _, sameHost := range []bool{false, true} {
		t.Run(fmt.Sprint(sameHost), func(t *testing.T) {
			var leaked atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1); fmt.Fprint(w, `{}`) }))
			defer target.Close()
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/forge/api/v1/user" {
					leaked.Add(1)
					fmt.Fprint(w, `{}`)
					return
				}
				location := target.URL + "/stolen"
				if sameHost {
					location = "/stolen"
				}
				http.Redirect(w, r, location, http.StatusFound)
			})
			require.False(t, f.Healthy(context.Background()))
			result, err := f.Execute(context.Background(), "forgejo_get_current_user", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.Zero(t, leaked.Load())
		})
	}
}

func TestContexts(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/forge/api/v1/repos/alice/slow" {
			close(started)
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `{"id":1}`)
	})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := f.Execute(canceled, "forgejo_get_current_user", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, context.Canceled.Error())
	require.Zero(t, calls.Load())
	require.False(t, f.Healthy(canceled))
	slow, cancelSlow := context.WithCancel(context.Background())
	defer cancelSlow()
	done := make(chan *mcp.ToolResult, 1)
	go func() {
		result, _ := f.Execute(slow, "forgejo_get_repo", map[string]any{"owner": "alice", "repo": "slow"})
		done <- result
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request not started")
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			result, err := f.Execute(context.Background(), "forgejo_get_current_user", nil)
			if err != nil || result == nil || result.IsError {
				t.Errorf("independent request failed: %v %v", result, err)
			}
		})
	}
	wg.Wait()
	cancelSlow()
	select {
	case result := <-done:
		require.True(t, result.IsError)
		require.Contains(t, result.Data, context.Canceled.Error())
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation not propagated")
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	require.Len(t, New().Tools(), 30)
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		require.False(t, seen[tool.Name], tool.Name)
		seen[tool.Name] = true
		require.NotNil(t, dispatch[tool.Name], tool.Name)
		require.True(t, strings.HasPrefix(string(tool.Name), "forgejo_"))
		require.NotEmpty(t, tool.Description)
		for _, key := range tool.Required {
			require.Contains(t, tool.Parameters, key)
		}
	}
	require.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		require.True(t, names[name], name)
	}
}

func TestResponseSizeCaps(t *testing.T) {
	f, ok := New().(mcp.PerToolMaxResponseBytesIntegration)
	require.True(t, ok)
	for _, tool := range []mcp.ToolName{"forgejo_get_pull_diff", "forgejo_get_file_contents"} {
		size, ok := f.MaxResponseBytesForTool(tool)
		require.True(t, ok)
		require.Equal(t, 1024*1024, size)
	}
	_, ok = f.MaxResponseBytesForTool("forgejo_get_repo")
	require.False(t, ok)
}

func TestMalformedInstallationURLs(t *testing.T) {
	for _, raw := range []string{"http://example.com:", "http://example.com:0", "http://example.com:65536", "http://example.com:abc", "http://[not-an-ip]", "http://example.com/%2fadmin", "http://example.com/forge//api/v1", "http://example.com/forge/./api/v1", "http://example.com/%252e%252e", "http://example.com/%23docs", "http://example.com/%3fdocs", "http://example.com/100%25", "http://example.com/%00docs", "http://example.com/\\admin"} {
		t.Run(raw, func(t *testing.T) {
			require.Error(t, New().Configure(context.Background(), mcp.Credentials{"base_url": raw, "token": "token"}))
		})
	}
}

func TestHealthResponses(t *testing.T) {
	for _, tt := range []struct {
		status  int
		body    string
		healthy bool
	}{
		{200, `{"id":1,"login":"alice"}`, true}, {200, `null`, false}, {200, `{}`, false}, {200, `not json`, false}, {204, "", false}, {401, `{"message":"unauthorized"}`, false}, {403, `{}`, false}, {404, `{}`, false}, {429, `{}`, false}, {500, `{}`, false},
	} {
		t.Run(fmt.Sprint(tt.status, tt.body), func(t *testing.T) {
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); fmt.Fprint(w, tt.body) })
			require.Equal(t, tt.healthy, f.Healthy(context.Background()))
		})
	}
}

func TestConfigureConcurrentWithCalls(t *testing.T) {
	f := New()
	newServer := func(prefix, token string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != prefix+"/api/v1/user" || r.Header.Get("Authorization") != "token "+token {
				t.Errorf("mixed configuration: %s %s", r.URL.Path, r.Header.Get("Authorization"))
			}
			fmt.Fprint(w, `{"id":1}`)
		}))
	}
	a, b := newServer("/a", "one"), newServer("/b", "two")
	defer a.Close()
	defer b.Close()
	creds := []mcp.Credentials{{"base_url": a.URL + "/a", "token": "one"}, {"base_url": b.URL + "/b/api/v1", "token": "two"}}
	require.NoError(t, f.Configure(context.Background(), creds[0]))
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			if err := f.Configure(context.Background(), creds[i%2]); err != nil {
				t.Error(err)
			}
			if !f.Healthy(context.Background()) {
				t.Error("health failed")
			}
			result, err := f.Execute(context.Background(), "forgejo_get_current_user", nil)
			if err != nil || result == nil || result.IsError {
				t.Errorf("execute failed: %v %v", result, err)
			}
		})
	}
	wg.Wait()
}

func TestClientTimeoutAndFreshContexts(t *testing.T) {
	f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	core := f.(*forgejo)
	require.Equal(t, 30*time.Second, core.httpClient.Timeout)
	core.httpClient.Timeout = 20 * time.Millisecond
	first, err := core.client(context.Background())
	require.NoError(t, err)
	second, err := core.client(context.Background())
	require.NoError(t, err)
	require.NotSame(t, first, second)
	result, err := f.Execute(context.Background(), "forgejo_get_current_user", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "Client.Timeout")
}

func TestUpstreamResponseCeiling(t *testing.T) {
	const ceiling = 8 * 1024 * 1024
	for _, tt := range []struct {
		tool    mcp.ToolName
		chunked bool
		status  int
		size    int
	}{
		{"forgejo_get_pull_diff", false, 200, ceiling + 1},
		{"forgejo_get_pull_diff", true, 200, ceiling + 1},
		{"forgejo_get_pull_diff", true, 200, ceiling},
		{"forgejo_get_file_contents", true, 200, ceiling + 1},
		{"forgejo_get_repo", true, 500, ceiling + 1},
	} {
		t.Run(fmt.Sprintf("%s/%t/%d/%d", tt.tool, tt.chunked, tt.status, tt.size), func(t *testing.T) {
			body := strings.Repeat("x", tt.size)
			if tt.tool == "forgejo_get_file_contents" {
				body = `{"content":"` + body + `","encoding":"base64"}`
			}
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				if !tt.chunked {
					w.Header().Set("Content-Length", fmt.Sprint(len(body)))
				}
				w.WriteHeader(tt.status)
				if tt.chunked {
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, body)
			})
			args := map[string]any{"owner": "alice", "repo": "demo"}
			if tt.tool == "forgejo_get_pull_diff" {
				args["number"] = 12
			}
			if tt.tool == "forgejo_get_file_contents" {
				args["path"] = "large.txt"
			}
			result, err := f.Execute(context.Background(), tt.tool, args)
			if tt.status == 500 {
				var retry *mcp.RetryableError
				require.ErrorAs(t, err, &retry)
				require.Contains(t, retry.Error(), "response exceeds")
				return
			}
			require.NoError(t, err)
			if tt.size > ceiling {
				require.True(t, result.IsError)
				require.Contains(t, result.Data, "response exceeds")
			} else {
				require.False(t, result.IsError)
			}
		})
	}
}

func TestMutationAmbiguousResponse(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprint(malformed), func(t *testing.T) {
			var calls atomic.Int32
			f, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if malformed {
					fmt.Fprint(w, `{"id":`)
					return
				}
				conn, _, err := w.(http.Hijacker).Hijack()
				require.NoError(t, err)
				require.NoError(t, conn.Close())
			})
			result, err := f.Execute(t.Context(), "forgejo_create_issue_comment", map[string]any{"owner": "alice", "repo": "demo", "number": 12, "body": "hello"})
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.Contains(t, result.Data, "uncertain")
			require.Contains(t, result.Data, "verify")
			require.Contains(t, result.Data, "before retry")
			if malformed {
				require.Contains(t, result.Data, "unexpected end of JSON input")
			} else {
				require.Contains(t, result.Data, "EOF")
			}
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestFinishTransportError(t *testing.T) {
	result, err := finish(nil, nil, errors.New("transport failed"))
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "transport failed")
}
