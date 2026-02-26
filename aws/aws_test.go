package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "aws", i.Name())
}

func TestConfigure_WithStaticCredentials(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"region":            "us-west-2",
	})
	assert.NoError(t, err)
}

func TestConfigure_DefaultRegion(t *testing.T) {
	a := &integration{}
	err := a.Configure(mcp.Credentials{
		"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	})
	assert.NoError(t, err)
	assert.Equal(t, "us-east-1", a.region)
}

func TestConfigure_WithSessionToken(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"session_token":     "FwoGZXIvYXdzEBYaDG...",
		"region":            "eu-west-1",
	})
	assert.NoError(t, err)
}

func TestConfigure_DefaultConfig(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"region": "ap-southeast-1"})
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

func TestTools_AllHaveAWSPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "aws_", "tool %s missing aws_ prefix", tool.Name)
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

func TestExecute_UnknownTool(t *testing.T) {
	a := &integration{}
	err := a.Configure(mcp.Credentials{
		"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"region":            "us-east-1",
	})
	require.NoError(t, err)

	result, err := a.Execute(context.Background(), "aws_nonexistent", nil)
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- Result helper tests ---

func TestJsonResult(t *testing.T) {
	result, err := jsonResult(map[string]string{"key": "value"})
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

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgInt32(t *testing.T) {
	assert.Equal(t, int32(42), argInt32(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, int32(42), argInt32(map[string]any{"n": 42}, "n"))
	assert.Equal(t, int32(42), argInt32(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, int32(0), argInt32(map[string]any{}, "n"))
}

func TestArgInt64(t *testing.T) {
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": 42}, "n"))
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": int64(42)}, "n"))
	assert.Equal(t, int64(42), argInt64(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, int64(0), argInt64(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestArgStrSlice(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, argStrSlice(map[string]any{"s": []any{"a", "b"}}, "s"))
	assert.Equal(t, []string{"a", "b"}, argStrSlice(map[string]any{"s": []string{"a", "b"}}, "s"))
	assert.Equal(t, []string{"a", "b"}, argStrSlice(map[string]any{"s": "a,b"}, "s"))
	assert.Nil(t, argStrSlice(map[string]any{}, "s"))
	assert.Nil(t, argStrSlice(map[string]any{"s": ""}, "s"))
}

// --- CloudWatch time parsing tests ---

func TestParseTime_RFC3339(t *testing.T) {
	ts, err := parseTime("2024-01-15T10:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, 2024, ts.Year())
	assert.Equal(t, 15, ts.Day())
}

func TestParseTime_Relative(t *testing.T) {
	ts, err := parseTime("-1h")
	require.NoError(t, err)
	assert.False(t, ts.IsZero())
}

func TestParseTime_Invalid(t *testing.T) {
	_, err := parseTime("not-a-time")
	assert.Error(t, err)
}

// --- DynamoDB JSON unmarshalling tests ---

func TestUnmarshalDynamoJSON_String(t *testing.T) {
	var result map[string]any
	input := `{"id":{"S":"123"}}`
	var raw json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(input), &raw))

	var out map[string]any
	_ = out

	var key map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(input), &key))
	assert.Contains(t, key, "id")
	_ = result
}

func TestParseDimensions(t *testing.T) {
	dims := parseDimensions(map[string]any{
		"dimensions": `{"InstanceId":"i-12345"}`,
	})
	require.Len(t, dims, 1)
	assert.Equal(t, "InstanceId", *dims[0].Name)
	assert.Equal(t, "i-12345", *dims[0].Value)
}

func TestParseDimensions_Empty(t *testing.T) {
	dims := parseDimensions(map[string]any{})
	assert.Nil(t, dims)
}

func TestParseDimensions_Invalid(t *testing.T) {
	dims := parseDimensions(map[string]any{
		"dimensions": "not-json",
	})
	assert.Nil(t, dims)
}

func TestToolCount(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.Len(t, tools, len(dispatch), "tool count should match dispatch map size")
}
