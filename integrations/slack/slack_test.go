package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "slack", i.Name())
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveSlackPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "slack_", "tool %s missing slack_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &slackIntegration{clients: make(map[string]*slack.Client)}
	result, err := s.Execute(t.Context(), "slack_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

// --- cookie transport tests ---

func TestCookieTransport_InjectsCookie(t *testing.T) {
	var capturedCookie string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedCookie = req.Header.Get("Cookie")
		return &http.Response{StatusCode: 200}, nil
	})

	transport := &cookieTransport{cookie: "test-cookie-value", inner: inner}
	req, _ := http.NewRequest("GET", "https://slack.com/api/test", nil)
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "d=test-cookie-value", capturedCookie)
}

func TestCookieTransport_AppendsToExisting(t *testing.T) {
	var capturedCookie string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedCookie = req.Header.Get("Cookie")
		return &http.Response{StatusCode: 200}, nil
	})

	transport := &cookieTransport{cookie: "test-cookie", inner: inner}
	req, _ := http.NewRequest("GET", "https://slack.com/api/test", nil)
	req.Header.Set("Cookie", "existing=value")
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "existing=value; d=test-cookie", capturedCookie)
}

func TestCookieTransport_NoCookie(t *testing.T) {
	var capturedCookie string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedCookie = req.Header.Get("Cookie")
		return &http.Response{StatusCode: 200}, nil
	})

	transport := &cookieTransport{cookie: "", inner: inner}
	req, _ := http.NewRequest("GET", "https://slack.com/api/test", nil)
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Empty(t, capturedCookie)
}

// roundTripFunc adapts a function to the http.RoundTripper interface.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// --- helper function tests ---

func TestArgStr(t *testing.T) {
	v, err := mcp.ArgStr(map[string]any{"k": "val"}, "k")
	assert.NoError(t, err)
	assert.Equal(t, "val", v)
	v, err = mcp.ArgStr(map[string]any{}, "k")
	assert.NoError(t, err)
	assert.Empty(t, v)
}

func TestArgInt(t *testing.T) {
	v, err := mcp.ArgInt(map[string]any{"n": float64(42)}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{"n": 42}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{"n": "42"}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 0, v)
}

func TestArgBool(t *testing.T) {
	v, err := mcp.ArgBool(map[string]any{"b": true}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": false}, "b")
	assert.NoError(t, err)
	assert.False(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": "true"}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": "1"}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{}, "b")
	assert.NoError(t, err)
	assert.False(t, v)
}

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestWrapRetryable_SlackRateLimitedError(t *testing.T) {
	rle := &slack.RateLimitedError{RetryAfter: 30 * time.Second}
	wrapped := wrapRetryable(rle)
	require.Error(t, wrapped)

	var re *mcp.RetryableError
	require.ErrorAs(t, wrapped, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestWrapRetryable_NonRetryableError(t *testing.T) {
	err := fmt.Errorf("channel not found")
	wrapped := wrapRetryable(err)
	assert.Equal(t, err, wrapped, "non-retryable errors should pass through unchanged")

	assert.False(t, mcp.IsRetryable(wrapped))
}

func TestWrapRetryable_NilError(t *testing.T) {
	assert.Nil(t, wrapRetryable(nil))
}

func TestErrResult_PropagatesSlackRateLimitedError(t *testing.T) {
	rle := &slack.RateLimitedError{RetryAfter: 10 * time.Second}
	result, err := errResult(rle)
	assert.Nil(t, result, "retryable error should not produce a ToolResult")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

// --- multi-workspace token store tests ---

func TestTokenStore_SetAndGetWorkspace(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T1", TeamName: "Acme", Token: "xoxc-1", Cookie: "xoxd-1"})

	ws := ts.getWorkspace("T1")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxc-1", ws.Token)
	assert.Equal(t, "Acme", ws.TeamName)
}

func TestTokenStore_DefaultWorkspace(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-1"})
	ts.setWorkspace(&workspace{TeamID: "T2", Token: "xoxc-2"})

	assert.Equal(t, "T1", ts.defaultID(), "first workspace set becomes default")

	ws := ts.getDefault()
	require.NotNil(t, ws)
	assert.Equal(t, "T1", ws.TeamID)

	ts.setDefault("T2")
	ws = ts.getDefault()
	require.NotNil(t, ws)
	assert.Equal(t, "T2", ws.TeamID)
}

func TestTokenStore_BackwardCompatGet(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-tok", Cookie: "xoxd-cook"})

	tok, cook := ts.get()
	assert.Equal(t, "xoxc-tok", tok)
	assert.Equal(t, "xoxd-cook", cook)
}

func TestTokenStore_SaveAndLoadV2(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tokens.json")

	ts := &tokenStore{workspaces: make(map[string]*workspace), filePath: fp}
	ts.setWorkspace(&workspace{TeamID: "T1", TeamName: "Acme", Token: "xoxc-1", Cookie: "xoxd-1", Source: "chrome"})
	ts.setWorkspace(&workspace{TeamID: "T2", TeamName: "Beta", Token: "xoxc-2", Cookie: "xoxd-2", Source: "web"})
	ts.setDefault("T2")

	require.NoError(t, ts.saveToFile())

	ts2 := &tokenStore{workspaces: make(map[string]*workspace), filePath: fp}
	ts2.loadFromFile()

	assert.Equal(t, "T2", ts2.defaultID())
	assert.Len(t, ts2.allWorkspaces(), 2)

	ws1 := ts2.getWorkspace("T1")
	require.NotNil(t, ws1)
	assert.Equal(t, "Acme", ws1.TeamName)
	assert.Equal(t, "xoxc-1", ws1.Token)
}

func TestTokenStore_LoadLegacyFormat(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tokens.json")

	legacy := map[string]string{
		"token":      "xoxc-legacy",
		"cookie":     "xoxd-legacy",
		"team_id":    "TLEGACY",
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	require.NoError(t, os.WriteFile(fp, data, 0600))

	ts := &tokenStore{workspaces: make(map[string]*workspace), filePath: fp}
	ts.loadFromFile()

	assert.Equal(t, "TLEGACY", ts.defaultID())
	ws := ts.getWorkspace("TLEGACY")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxc-legacy", ws.Token)
	assert.Equal(t, "xoxd-legacy", ws.Cookie)
}

func TestTokenStore_AllWorkspaces(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T2", TeamName: "Beta", Token: "xoxc-2"})
	ts.setWorkspace(&workspace{TeamID: "T1", TeamName: "Alpha", Token: "xoxc-1"})

	all := ts.allWorkspaces()
	require.Len(t, all, 2)
	assert.Equal(t, "Alpha", all[0].TeamName, "should be sorted by name")
	assert.Equal(t, "Beta", all[1].TeamName)
}

func TestGetClientForArgs_DefaultWorkspace(t *testing.T) {
	s := &slackIntegration{
		clients: map[string]*slack.Client{"T1": slack.New("xoxc-1")},
		store:   &tokenStore{workspaces: make(map[string]*workspace), defaultTeamID: "T1"},
	}

	client, err := s.getClientForArgs(map[string]any{})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGetClientForArgs_SpecificWorkspace(t *testing.T) {
	s := &slackIntegration{
		clients: map[string]*slack.Client{
			"T1": slack.New("xoxc-1"),
			"T2": slack.New("xoxc-2"),
		},
		store: &tokenStore{workspaces: make(map[string]*workspace), defaultTeamID: "T1"},
	}

	client, err := s.getClientForArgs(map[string]any{"team_id": "T2"})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGetClientForArgs_UnknownWorkspace(t *testing.T) {
	s := &slackIntegration{
		clients: map[string]*slack.Client{"T1": slack.New("xoxc-1")},
		store:   &tokenStore{workspaces: make(map[string]*workspace), defaultTeamID: "T1"},
	}

	_, err := s.getClientForArgs(map[string]any{"team_id": "T_UNKNOWN"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown workspace")
}

func TestErrResult_NonRetryableProducesToolResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("no workspace"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "no workspace", result.Data)
}
