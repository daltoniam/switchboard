package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "digitalocean", i.Name())
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_WithToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"api_token": "dop_v1_test123",
	})
	assert.NoError(t, err)
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

func TestTools_AllHaveDOPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "digitalocean_", "tool %s missing digitalocean_ prefix", tool.Name)
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

func TestExecute_UnknownTool(t *testing.T) {
	d := &integration{}
	err := d.Configure(context.Background(), mcp.Credentials{
		"api_token": "dop_v1_test123",
	})
	require.NoError(t, err)

	result, err := d.Execute(context.Background(), "digitalocean_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
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

func TestToolCount(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.Len(t, tools, len(dispatch), "tool count should match dispatch map size")
}

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
	assert.Contains(t, result.Data, `"value"`)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestCreateDroplet_UserData(t *testing.T) {
	var captured map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/droplets")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"droplet":{"id":123,"name":"runner"}}`)
	}))
	defer ts.Close()

	d := &integration{}
	require.NoError(t, d.Configure(context.Background(), mcp.Credentials{
		"api_token": "dop_v1_test123",
		"base_url":  ts.URL,
	}))

	result, err := createDroplet(context.Background(), d, map[string]any{
		"name":      "runner",
		"region":    "sfo3",
		"size":      "s-2vcpu-4gb",
		"image":     "ubuntu-24-04-x64",
		"user_data": "#cloud-config\npackage_update: true\n",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "unexpected error: %s", result.Data)
	assert.Equal(t, "#cloud-config\npackage_update: true\n", captured["user_data"])
}

func TestWrapRetryable_RateLimited(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests}
	godoErr := &godo.ErrorResponse{Response: resp}
	wrapped := wrapRetryable(godoErr)
	var re *mcp.RetryableError
	require.ErrorAs(t, wrapped, &re)
	assert.Equal(t, http.StatusTooManyRequests, re.StatusCode)
}

func TestWrapRetryable_ServerError(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusBadGateway}
	godoErr := &godo.ErrorResponse{Response: resp}
	wrapped := wrapRetryable(godoErr)
	var re *mcp.RetryableError
	require.ErrorAs(t, wrapped, &re)
	assert.Equal(t, http.StatusBadGateway, re.StatusCode)
}

func TestWrapRetryable_NonRetryable(t *testing.T) {
	err := fmt.Errorf("not found")
	assert.Equal(t, err, wrapRetryable(err))
}

func TestWrapRetryable_Nil(t *testing.T) {
	assert.Nil(t, wrapRetryable(nil))
}
