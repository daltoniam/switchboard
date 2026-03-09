package mcp

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentials(t *testing.T) {
	creds := Credentials{"token": "abc123", "secret": "xyz"}

	assert.Equal(t, "abc123", creds["token"])
	assert.Equal(t, "xyz", creds["secret"])
	assert.Empty(t, creds["missing"])
}

func TestIntegrationConfig(t *testing.T) {
	ic := &IntegrationConfig{
		Enabled:     true,
		Credentials: Credentials{"api_key": "key1"},
	}

	assert.True(t, ic.Enabled)
	assert.Equal(t, "key1", ic.Credentials["api_key"])
}

func TestConfig(t *testing.T) {
	cfg := &Config{
		Integrations: map[string]*IntegrationConfig{
			"github": {Enabled: true, Credentials: Credentials{"token": "t1"}},
			"slack":  {Enabled: false, Credentials: Credentials{}},
		},
	}

	require.Len(t, cfg.Integrations, 2)
	assert.True(t, cfg.Integrations["github"].Enabled)
	assert.False(t, cfg.Integrations["slack"].Enabled)
}

func TestToolDefinition(t *testing.T) {
	td := ToolDefinition{
		Name:        "github_list_issues",
		Description: "List issues",
		Parameters:  map[string]string{"owner": "Repo owner", "repo": "Repo name"},
		Required:    []string{"owner", "repo"},
	}

	assert.Equal(t, "github_list_issues", td.Name)
	assert.Equal(t, "List issues", td.Description)
	assert.Len(t, td.Parameters, 2)
	assert.Equal(t, []string{"owner", "repo"}, td.Required)
}

func TestToolDefinitionOptionalRequired(t *testing.T) {
	td := ToolDefinition{
		Name:        "github_search_repos",
		Description: "Search repos",
		Parameters:  map[string]string{"query": "Search query"},
	}

	assert.Nil(t, td.Required)
}

func TestToolResult(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		r := &ToolResult{Data: `{"count":5}`}
		assert.Equal(t, `{"count":5}`, r.Data)
		assert.False(t, r.IsError)
	})

	t.Run("error result", func(t *testing.T) {
		r := &ToolResult{Data: "something went wrong", IsError: true}
		assert.True(t, r.IsError)
		assert.Equal(t, "something went wrong", r.Data)
	})
}

func TestHealthStatus(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		hs := HealthStatus{Name: "github", Healthy: true}
		assert.True(t, hs.Healthy)
		assert.Empty(t, hs.Error)
	})

	t.Run("unhealthy", func(t *testing.T) {
		hs := HealthStatus{Name: "github", Healthy: false, Error: "connection refused"}
		assert.False(t, hs.Healthy)
		assert.Equal(t, "connection refused", hs.Error)
	})
}

func TestErrNotConfigured(t *testing.T) {
	assert.EqualError(t, ErrNotConfigured, "integration not configured")
}

func TestErrUnhealthy(t *testing.T) {
	assert.EqualError(t, ErrUnhealthy, "integration unhealthy")
}

func TestServicesStruct(t *testing.T) {
	s := &Services{}
	assert.Nil(t, s.Config)
	assert.Nil(t, s.Registry)
}

func TestRetryableError_IncludesStatusCodeInMessage(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		inner   string
		wantMsg string
	}{
		{"503", 503, "service unavailable", "retryable (503): service unavailable"},
		{"429", 429, "rate limited", "retryable (429): rate limited"},
		{"502", 502, "bad gateway", "retryable (502): bad gateway"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RetryableError{StatusCode: tt.code, Err: errors.New(tt.inner)}
			assert.Equal(t, tt.wantMsg, err.Error())
		})
	}
}

func TestRetryableError_UnwrapsInnerError(t *testing.T) {
	inner := errors.New("connection reset")
	err := &RetryableError{StatusCode: 502, Err: inner}
	assert.ErrorIs(t, err, inner)
}

func TestRetryableError_PreservesRetryAfterThroughErrorsAs(t *testing.T) {
	inner := errors.New("rate limited")
	err := &RetryableError{StatusCode: 429, Err: inner, RetryAfter: 5 * time.Second}

	var re *RetryableError
	require.True(t, errors.As(err, &re))
	assert.Equal(t, 5*time.Second, re.RetryAfter)
	assert.Equal(t, 429, re.StatusCode)
}

func TestIsRetryable_DistinguishesRetryableFromPermanentErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"RetryableError is retryable", &RetryableError{StatusCode: 503, Err: errors.New("timeout")}, true},
		{"plain error is not retryable", errors.New("bad request"), false},
		{"wrapped RetryableError is retryable", fmt.Errorf("outer: %w", &RetryableError{StatusCode: 429, Err: errors.New("rate limit")}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, IsRetryable(tt.err))
		})
	}
}
