package wasm

import (
	"context"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/pluginoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func oauthCredentials() mcp.Credentials {
	return mcp.Credentials{
		"oauth_issuer":    "https://clerk.primerlms.com",
		"oauth_client_id": "public-client",
		"oauth_token_key": "api_key",
		"oauth_subject":   "expected-user",
		"oauth_email":     "expected@example.com",
		"base_url":        "https://api.example.com",
	}
}

func TestOAuthModuleAvailabilityAndUnauthorized(t *testing.T) {
	mod := loadTestModule(t)
	var integration mcp.Integration = mod
	_, ok := integration.(mcp.OAuthIntegration)
	require.True(t, ok)
	mod.SetConfigService(newLoaderConfig(map[string]*mcp.IntegrationConfig{"example": {Credentials: oauthCredentials()}}))
	require.NoError(t, mod.Configure(t.Context(), oauthCredentials()))
	require.NotNil(t, mod.oauth)
	assert.False(t, mod.Healthy(t.Context()))
	result, err := mod.Execute(t.Context(), "example_echo", map[string]any{"message": "must-not-run"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Equal(t, pluginoauth.ErrReauthorization.Error(), result.Data)
	require.NoError(t, mod.Configure(t.Context(), mcp.Credentials{"base_url": "https://api.example.com", "api_key": "generic-key"}))
	assert.Nil(t, mod.oauth)
	result, err = mod.Execute(t.Context(), "example_echo", map[string]any{"message": "generic-works"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "generic-works")
}

func TestOAuthNativeSecretsNeverEnterGuestMemory(t *testing.T) {
	mod := loadTestModule(t)
	creds := oauthCredentials()
	creds["oauth_refresh_token"] = "unique-native-refresh-secret"
	creds["oauth_access_token"] = "unique-native-access-not-yet-verified"
	mod.SetConfigService(newLoaderConfig(map[string]*mcp.IntegrationConfig{"example": {Credentials: creds}}))
	require.NoError(t, mod.Configure(t.Context(), creds))
	memory, ok := mod.mod.Memory().Read(0, mod.mod.Memory().Size())
	require.True(t, ok)
	assert.False(t, strings.Contains(string(memory), creds["oauth_refresh_token"]))
	assert.False(t, strings.Contains(string(memory), creds["oauth_access_token"]))
}

func TestOAuthLoaderInjectsConfigurationBeforeConfigure(t *testing.T) {
	creds := oauthCredentials()
	creds["api_key"] = ""
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{"custom": {Enabled: true, Credentials: creds, ToolGlobs: []string{"example_echo"}}})
	loader, path := newLoaderForTest(t, cfg)
	require.NoError(t, loader.LoadPlugin(context.Background(), path, "custom"))
	mod := loader.modules["custom"]
	require.NotNil(t, mod)
	require.NotNil(t, mod.oauth)
	assert.Same(t, cfg, mod.cfgMgr)
	assert.Zero(t, cfg.setCalls)
	assert.False(t, mod.Healthy(t.Context()))
}

func TestOAuthClosedModule(t *testing.T) {
	mod := loadTestModule(t)
	require.NoError(t, mod.Close(t.Context()))
	_, err := mod.StartOAuth(t.Context(), oauthCredentials(), "redirect", "cookie")
	assert.ErrorIs(t, err, ErrModuleClosed)
	assert.ErrorIs(t, mod.CompleteOAuth(t.Context(), "code", "state", "cookie", ""), ErrModuleClosed)
}
