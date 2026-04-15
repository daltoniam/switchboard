package botidentity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "botidentity", i.Name())
}

func TestConfigure_GithubTokenOnly(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"github_token": "ghp_test123"})
	assert.NoError(t, err)
}

func TestConfigure_SlackTokenOnly(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"slack_config_token": "xoxe-test123"})
	assert.NoError(t, err)
}

func TestConfigure_BothTokens(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"github_token":       "ghp_test123",
		"slack_config_token": "xoxe-test123",
	})
	assert.NoError(t, err)
}

func TestConfigure_NoTokens(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of")
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"github_token": "", "slack_config_token": ""})
	assert.Error(t, err)
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

func TestTools_AllHavePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "botidentity_", "tool %s missing botidentity_ prefix", tool.Name)
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
	b := &botidentity{githubToken: "test", githubBaseURL: "http://localhost", slackBaseURL: "http://localhost/", client: &http.Client{}}
	result, err := b.Execute(context.Background(), "botidentity_nonexistent", nil)
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

func TestPlainTextKeys(t *testing.T) {
	p := New()
	ptc, ok := p.(mcp.PlainTextCredentials)
	require.True(t, ok, "botidentity must implement PlainTextCredentials")
	keys := ptc.PlainTextKeys()
	assert.Contains(t, keys, "github_app_id")
}

func TestOptionalKeys(t *testing.T) {
	p := New()
	oc, ok := p.(mcp.OptionalCredentials)
	require.True(t, ok, "botidentity must implement OptionalCredentials")
	keys := oc.OptionalKeys()
	assert.Contains(t, keys, "github_token")
	assert.Contains(t, keys, "slack_config_token")
	assert.Contains(t, keys, "slack_refresh_token")
}

// --- GitHub API handler tests ---

func newGithubTestBot(t *testing.T, handler http.HandlerFunc) (*botidentity, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	b := &botidentity{
		githubToken:   "test-token",
		githubBaseURL: ts.URL,
		slackBaseURL:  "http://localhost/",
		client:        ts.Client(),
	}
	return b, ts.Close
}

func TestGhGetApp_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   1,
			"slug": "my-bot",
			"name": "My Bot",
		})
	})
	defer cleanup()

	result, err := ghGetApp(context.Background(), b, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-bot")
}

func TestGhListInstallations_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/installations")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "account": map[string]string{"login": "my-org"}},
		})
	})
	defer cleanup()

	result, err := ghListInstallations(context.Background(), b, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-org")
}

func TestGhCreateInstallToken_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/access_tokens")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      "ghs_abc123",
			"expires_at": "2024-01-01T00:00:00Z",
		})
	})
	defer cleanup()

	result, err := ghCreateInstallToken(context.Background(), b, map[string]any{
		"installation_id": "123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ghs_abc123")
}

func TestGhCreateInstallToken_WithPermissions(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["permissions"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"token": "ghs_scoped"})
	})
	defer cleanup()

	result, err := ghCreateInstallToken(context.Background(), b, map[string]any{
		"installation_id": "123",
		"permissions":     `{"contents":"read","issues":"write"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ghs_scoped")
}

func TestGhSuspendInstallation_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/suspended")
		w.WriteHeader(204)
	})
	defer cleanup()

	result, err := ghSuspendInstallation(context.Background(), b, map[string]any{
		"installation_id": "123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGhGetInstallation_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/installations/456")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":          456,
			"target_type": "Organization",
		})
	})
	defer cleanup()

	result, err := ghGetInstallation(context.Background(), b, map[string]any{
		"installation_id": "456",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Organization")
}

func TestGhUnsuspendInstallation_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/suspended")
		w.WriteHeader(204)
	})
	defer cleanup()

	result, err := ghUnsuspendInstallation(context.Background(), b, map[string]any{
		"installation_id": "123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGhDeleteInstallation_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/app/installations/123")
		w.WriteHeader(204)
	})
	defer cleanup()

	result, err := ghDeleteInstallation(context.Background(), b, map[string]any{
		"installation_id": "123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGhGetWebhookConfig_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/app/hook/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"url":          "https://example.com/webhook",
			"content_type": "json",
		})
	})
	defer cleanup()

	result, err := ghGetWebhookConfig(context.Background(), b, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "example.com")
}

func TestGhUpdateWebhookConfig_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "https://new.example.com/webhook", body["url"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	})
	defer cleanup()

	result, err := ghUpdateWebhookConfig(context.Background(), b, map[string]any{
		"url": "https://new.example.com/webhook",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGhListWebhookDeliveries_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/app/hook/deliveries")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "event": "push", "status": "OK"},
		})
	})
	defer cleanup()

	result, err := ghListWebhookDeliveries(context.Background(), b, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "push")
}

func TestGhRedeliverWebhook_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/attempts")
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]any{"status": "accepted"})
	})
	defer cleanup()

	result, err := ghRedeliverWebhook(context.Background(), b, map[string]any{
		"delivery_id": "42",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGithubDo_APIError(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	})
	defer cleanup()

	_, err := b.githubGet(context.Background(), "/app")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "github API error (401)")
}

func TestGithubDo_RetryableError(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"message":"rate limit"}`))
	})
	defer cleanup()

	_, err := b.githubGet(context.Background(), "/app")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

// --- Slack API handler tests ---

func newSlackTestBot(t *testing.T, handler http.HandlerFunc) (*botidentity, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	b := &botidentity{
		slackConfigToken: "xoxe-test",
		githubBaseURL:    "http://localhost",
		slackBaseURL:     ts.URL + "/",
		client:           ts.Client(),
	}
	return b, ts.Close
}

func TestSlackCreateApp_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer xoxe-test")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"app_id": "A12345678",
			"credentials": map[string]string{
				"client_id":     "123.456",
				"client_secret": "secret",
			},
		})
	})
	defer cleanup()

	result, err := slackCreateApp(context.Background(), b, map[string]any{
		"manifest": `{"display_information":{"name":"Test Bot"}}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "A12345678")
}

func TestSlackUpdateApp_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "A12345678", body["app_id"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "app_id": "A12345678"})
	})
	defer cleanup()

	result, err := slackUpdateApp(context.Background(), b, map[string]any{
		"app_id":   "A12345678",
		"manifest": `{"display_information":{"name":"Updated Bot"}}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSlackValidateApp_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"errors": []any{},
		})
	})
	defer cleanup()

	result, err := slackValidateApp(context.Background(), b, map[string]any{
		"manifest": `{"display_information":{"name":"Test Bot"}}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSlackExportApp_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"manifest": map[string]any{"display_information": map[string]string{"name": "My Bot"}},
		})
	})
	defer cleanup()

	result, err := slackExportApp(context.Background(), b, map[string]any{
		"app_id": "A12345678",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Bot")
}

func TestSlackDeleteApp_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	defer cleanup()

	result, err := slackDeleteApp(context.Background(), b, map[string]any{
		"app_id": "A12345678",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSlackGetBotInfo_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"bot": map[string]any{
				"id":     "B12345",
				"name":   "test-bot",
				"app_id": "A12345",
			},
		})
	})
	defer cleanup()

	result, err := slackGetBotInfo(context.Background(), b, map[string]any{
		"bot": "B12345",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "test-bot")
}

func TestSlackPost_APIError(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "invalid_auth",
		})
	})
	defer cleanup()

	_, err := b.slackPost(context.Background(), "auth.test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_auth")
}

func TestSlackPost_RetryableError(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(429)
		w.Write([]byte(`{"ok":false,"error":"ratelimited"}`))
	})
	defer cleanup()

	_, err := b.slackPost(context.Background(), "auth.test", nil)
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestSlackRotateToken_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "xoxe-refresh-old", body["refresh_token"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":            true,
			"token":         "xoxe.xoxp-new-token",
			"refresh_token": "xoxe-refresh-new",
			"team_id":       "T12345",
			"user_id":       "U12345",
			"iat":           1633095660,
			"exp":           1633138860,
		})
	})
	defer cleanup()

	b.slackRefreshToken = "xoxe-refresh-old"

	result, err := slackRotateToken(context.Background(), b, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "xoxe.xoxp-new-token")
	assert.Equal(t, "xoxe.xoxp-new-token", b.slackConfigToken)
	assert.Equal(t, "xoxe-refresh-new", b.slackRefreshToken)
}

func TestSlackRotateToken_WithExplicitRefreshToken(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "xoxe-explicit-refresh", body["refresh_token"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":            true,
			"token":         "xoxe.xoxp-rotated",
			"refresh_token": "xoxe-refresh-rotated",
		})
	})
	defer cleanup()

	result, err := slackRotateToken(context.Background(), b, map[string]any{
		"refresh_token": "xoxe-explicit-refresh",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "xoxe.xoxp-rotated", b.slackConfigToken)
	assert.Equal(t, "xoxe-refresh-rotated", b.slackRefreshToken)
}

func TestSlackRotateToken_NoRefreshToken(t *testing.T) {
	b := &botidentity{slackConfigToken: "xoxe-test", slackBaseURL: "http://localhost/", githubBaseURL: "http://localhost", client: &http.Client{}}
	result, err := slackRotateToken(context.Background(), b, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "refresh_token is required")
}

func TestSlackRotateToken_UpdatesTokenUsedBySubsequentCalls(t *testing.T) {
	callCount := 0
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"ok":            true,
				"token":         "xoxe.xoxp-rotated-token",
				"refresh_token": "xoxe-refresh-rotated",
			})
			return
		}
		assert.Equal(t, "Bearer xoxe.xoxp-rotated-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	defer cleanup()

	b.slackRefreshToken = "xoxe-refresh-old"

	_, err := slackRotateToken(context.Background(), b, map[string]any{})
	require.NoError(t, err)

	_, err = b.slackPost(context.Background(), "auth.test", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestSlackCreateApp_InvalidManifest(t *testing.T) {
	b := &botidentity{slackConfigToken: "xoxe-test", slackBaseURL: "http://localhost/", githubBaseURL: "http://localhost", client: &http.Client{}}
	result, err := slackCreateApp(context.Background(), b, map[string]any{
		"manifest": "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "valid JSON")
}

func TestGhCreateInstallToken_InvalidPermissions(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	defer cleanup()

	result, err := ghCreateInstallToken(context.Background(), b, map[string]any{
		"installation_id": "123",
		"permissions":     "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON object")
}

func TestGhListInstallRepos_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/user/installations/789/repositories")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"total_count":  1,
			"repositories": []map[string]any{{"id": 1, "name": "my-repo"}},
		})
	})
	defer cleanup()

	result, err := ghListInstallRepos(context.Background(), b, map[string]any{
		"installation_id": "789",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-repo")
}

func TestGhAddInstallRepo_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(204)
	})
	defer cleanup()

	result, err := ghAddInstallRepo(context.Background(), b, map[string]any{
		"installation_id": "123",
		"repository_id":   "456",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGhRemoveInstallRepo_Success(t *testing.T) {
	b, cleanup := newGithubTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(204)
	})
	defer cleanup()

	result, err := ghRemoveInstallRepo(context.Background(), b, map[string]any{
		"installation_id": "123",
		"repository_id":   "456",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Logo generation tests ---

func TestGenerateLogo_NilBedrockClient(t *testing.T) {
	b := &botidentity{githubToken: "test", githubBaseURL: "http://localhost", slackBaseURL: "http://localhost/", client: &http.Client{}}
	result, err := generateLogo(context.Background(), b, map[string]any{
		"prompt": "a blue robot",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "AWS credentials not configured")
}

func TestGenerateLogo_MissingPrompt(t *testing.T) {
	b := &botidentity{githubToken: "test", githubBaseURL: "http://localhost", slackBaseURL: "http://localhost/", client: &http.Client{}}
	result, err := generateLogo(context.Background(), b, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// --- Slack set bot icon tests ---

func TestSlackSetBotIcon_Success(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		assert.Equal(t, "Bearer xoxb-bot-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	defer cleanup()

	b.slackBotToken = "xoxb-bot-token"
	imageB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	result, err := slackSetBotIcon(context.Background(), b, map[string]any{
		"image_base64": imageB64,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSlackSetBotIcon_NoToken(t *testing.T) {
	b := &botidentity{slackConfigToken: "xoxe-test", slackBaseURL: "http://localhost/", githubBaseURL: "http://localhost", client: &http.Client{}}
	result, err := slackSetBotIcon(context.Background(), b, map[string]any{
		"image_base64": "dGVzdA==",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "token is required")
}

func TestSlackSetBotIcon_InvalidBase64(t *testing.T) {
	b := &botidentity{slackBotToken: "xoxb-test", slackBaseURL: "http://localhost/", githubBaseURL: "http://localhost", client: &http.Client{}}
	result, err := slackSetBotIcon(context.Background(), b, map[string]any{
		"image_base64": "not-valid-base64!!!",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid base64")
}

func TestSlackSetBotIcon_WithExplicitToken(t *testing.T) {
	b, cleanup := newSlackTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer xoxb-explicit", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	defer cleanup()

	imageB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	result, err := slackSetBotIcon(context.Background(), b, map[string]any{
		"image_base64": imageB64,
		"token":        "xoxb-explicit",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestJSONResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key":"value"`)
}
