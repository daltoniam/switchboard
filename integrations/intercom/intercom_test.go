package intercom

import (
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultServer(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "intercom", i.Name())
}

func TestNew_OverrideServerURL(t *testing.T) {
	i := New("https://custom.example.com")
	require.NotNil(t, i)
	assert.Equal(t, "intercom", i.Name())
}

func TestNew_EmptyOverrideUsesDefault(t *testing.T) {
	i := New("")
	require.NotNil(t, i)
	assert.Equal(t, "intercom", i.Name())
}

func TestConfigure_DelegatesAccessToken(t *testing.T) {
	i := New("https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "dG9rOmFiYw=="})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New("https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestConfigure_EmptyAccessToken(t *testing.T) {
	i := New("https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestHealthy_NoConfigure(t *testing.T) {
	i := New("https://example.com")
	assert.False(t, i.Healthy(context.Background()))
}

func TestExecute_NoConnection(t *testing.T) {
	i := New("https://invalid.example.com")
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"access_token": "tok"}))

	result, err := i.Execute(context.Background(), mcp.ToolName("intercom_search_conversations"), nil)
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}
