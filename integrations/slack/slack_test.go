package slack

import (
	"fmt"
	"net/http"
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
	seen := make(map[string]bool)
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &slackIntegration{}
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
	// errResult composes wrapRetryable internally — no explicit wrapping needed at call sites.
	rle := &slack.RateLimitedError{RetryAfter: 10 * time.Second}
	result, err := errResult(rle)
	assert.Nil(t, result, "retryable error should not produce a ToolResult")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}
