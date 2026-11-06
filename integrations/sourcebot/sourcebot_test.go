package sourcebot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "sourcebot-0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab"

func newTestServer(t *testing.T, handler http.HandlerFunc) *sourcebot {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	s := New().(*sourcebot)
	require.NoError(t, s.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL, "api_key": testKey}))
	return s
}

func execute(t *testing.T, s *sourcebot, tool string, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := s.Execute(context.Background(), mcp.ToolName(tool), args)
	require.NoError(t, err)
	return result
}

func decodeResult(t *testing.T, result *mcp.ToolResult) map[string]any {
	t.Helper()
	require.False(t, result.IsError, result.Data)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	return out
}

func readBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	return body
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "sourcebot", i.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantURL string
		wantKey string
	}{
		{"url and key", mcp.Credentials{"base_url": "https://sourcebot.example.com", "api_key": testKey}, "https://sourcebot.example.com", testKey},
		{"trailing slash trimmed", mcp.Credentials{"base_url": " https://sourcebot.example.com/ "}, "https://sourcebot.example.com", ""},
		{"api suffix trimmed", mcp.Credentials{"base_url": "https://sourcebot.example.com/api/"}, "https://sourcebot.example.com", ""},
		{"path prefix kept", mcp.Credentials{"base_url": "http://internal.example.com/sourcebot"}, "http://internal.example.com/sourcebot", ""},
		{"key whitespace trimmed", mcp.Credentials{"base_url": "https://sourcebot.example.com", "api_key": " sbk_abc "}, "https://sourcebot.example.com", "sbk_abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New().(*sourcebot)
			require.NoError(t, s.Configure(context.Background(), tt.creds))
			assert.Equal(t, tt.wantURL, s.baseURL)
			assert.Equal(t, tt.wantKey, s.apiKey)
		})
	}
}

func TestConfigure_Errors(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{"missing base_url", mcp.Credentials{"api_key": testKey}, "base_url is required"},
		{"blank base_url", mcp.Credentials{"base_url": "   "}, "base_url is required"},
		{"no scheme", mcp.Credentials{"base_url": "sourcebot.example.com"}, "http(s) URL"},
		{"bad scheme", mcp.Credentials{"base_url": "ftp://sourcebot.example.com"}, "http(s) URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New().Configure(context.Background(), tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestExecute_NotConfigured(t *testing.T) {
	s := New().(*sourcebot)
	require.Error(t, s.Configure(context.Background(), mcp.Credentials{}))
	result := execute(t, s, "sourcebot_list_repos", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestExecute_UnknownTool(t *testing.T) {
	result := execute(t, New().(*sourcebot), "sourcebot_nope", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestCredentialHints(t *testing.T) {
	s := New().(*sourcebot)
	assert.Equal(t, []string{"base_url"}, s.PlainTextKeys())
	assert.Equal(t, []string{"api_key"}, s.OptionalKeys())
	hints := s.Placeholders()
	assert.Contains(t, hints["base_url"], "https://sourcebot.example.com")
	assert.Contains(t, hints["api_key"], "sbk_")
}

func TestTools(t *testing.T) {
	tools := New().Tools()
	require.Len(t, tools, 9)
	seen := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "sourcebot_"), tool.Name)
		assert.NotEmpty(t, tool.Description, tool.Name)
		assert.Equal(t, mcp.AccessRead, tool.Access, tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		for _, req := range tool.Required {
			assert.Contains(t, tool.Parameters, req, "%s requires undocumented parameter %s", tool.Name, req)
		}
	}
}

func TestTools_EntryPointHasStartHere(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "sourcebot_search_code" {
			assert.Contains(t, tool.Description, "Start here")
			for _, word := range []string{"code search", "source code", "codebase", "grep", "symbols", "functions", "repositories"} {
				assert.Contains(t, tool.Description, word)
			}
			return
		}
	}
	t.Fatal("sourcebot_search_code not found")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestRequestHeaders(t *testing.T) {
	var got http.Header
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, `[]`)
	})
	execute(t, s, "sourcebot_list_language_models", nil)
	assert.Equal(t, testKey, got.Get("X-Sourcebot-Api-Key"))
	assert.Equal(t, "~", got.Get("X-Org-Domain"))
	assert.Equal(t, "switchboard", got.Get("X-Sourcebot-Client-Source"))
}

func TestRequestHeaders_NoKeyForAnonymousAccess(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, `[]`)
	}))
	t.Cleanup(srv.Close)
	s := New().(*sourcebot)
	require.NoError(t, s.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL}))
	execute(t, s, "sourcebot_list_language_models", nil)
	_, present := got["X-Sourcebot-Api-Key"]
	assert.False(t, present)
}

func TestHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{"authorized", http.StatusOK, true},
		{"unauthorized", http.StatusUnauthorized, false},
		{"server error", http.StatusInternalServerError, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.RequestURI()
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, `[]`)
			})
			assert.Equal(t, tt.want, s.Healthy(context.Background()))
			assert.Equal(t, "/api/repos?perPage=1", path)
		})
	}
}

func TestHealthy_Unconfigured(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()))
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		apiKey    string
		want      []string
		retryable bool
	}{
		{"unauthorized without key", 401, `{"statusCode":401,"errorCode":"NOT_AUTHENTICATED","message":"Not authenticated"}`, "", []string{"401", "NOT_AUTHENTICATED: Not authenticated", "set api_key"}, false},
		{"unauthorized with key", 401, `{"statusCode":401,"errorCode":"NOT_AUTHENTICATED","message":"Not authenticated"}`, testKey, []string{"api_key was rejected"}, false},
		{"not found", 404, `{"statusCode":404,"errorCode":"FILE_NOT_FOUND","message":"File \"x.go\" not found"}`, testKey, []string{"404", "FILE_NOT_FOUND", `File "x.go" not found`}, false},
		{"server error", 500, `{"statusCode":500,"errorCode":"UNEXPECTED_ERROR","message":"boom"}`, testKey, []string{"500", "UNEXPECTED_ERROR: boom"}, true},
		{"rate limited", 429, `slow down`, testKey, []string{"429", "slow down"}, true},
		{"plain text error", 502, `Bad Gateway`, testKey, []string{"502", "Bad Gateway"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(srv.Close)
			s := New().(*sourcebot)
			require.NoError(t, s.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL, "api_key": tt.apiKey}))
			_, _, err := s.do(context.Background(), s.client, http.MethodGet, "/api/models", nil)
			require.Error(t, err)
			for _, want := range tt.want {
				assert.Contains(t, err.Error(), want)
			}
			var re *mcp.RetryableError
			assert.Equal(t, tt.retryable, errors.As(err, &re))

			result, err := s.Execute(context.Background(), "sourcebot_list_language_models", nil)
			if tt.retryable {
				require.Error(t, err, "retryable errors go back to the server so it can retry")
				assert.Contains(t, err.Error(), tt.want[0])
				return
			}
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want[0])
		})
	}
}

func TestOversizedResponseFails(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"source":"`)
		chunk := strings.Repeat("a", 64*1024)
		for written := 0; written <= maxResponseSize; written += len(chunk) {
			_, _ = io.WriteString(w, chunk)
		}
		_, _ = io.WriteString(w, `"}`)
	})
	result := execute(t, s, "sourcebot_read_file", map[string]any{"repo": "github.com/acme/api", "path": "big.txt"})
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "exceeds")
}

func TestReadLimited(t *testing.T) {
	data, err := readLimited(strings.NewReader("12345"), 5)
	require.NoError(t, err)
	assert.Equal(t, "12345", string(data))
	_, err = readLimited(strings.NewReader("123456"), 5)
	require.Error(t, err)
}

func TestMalformedResponse(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html>login</html>`)
	})
	result := execute(t, s, "sourcebot_list_repos", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unexpected response")
}
