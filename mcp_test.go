package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONResult(t *testing.T) {
	t.Run("marshals a struct to JSON string", func(t *testing.T) {
		v := map[string]any{"count": 5, "name": "test"}
		result, err := JSONResult(v)
		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Data, `"count":5`)
		assert.Contains(t, result.Data, `"name":"test"`)
	})

	t.Run("returns error result when marshal fails", func(t *testing.T) {
		v := make(chan int) // channels are not JSON-serializable
		result, err := JSONResult(v)
		require.NoError(t, err) // no Go error — wrapped in ToolResult
		assert.True(t, result.IsError)
		assert.Contains(t, result.Data, "unsupported type")
	})

	t.Run("handles nil value as JSON null", func(t *testing.T) {
		result, err := JSONResult(nil)
		require.NoError(t, err)
		assert.Equal(t, "null", result.Data)
		assert.False(t, result.IsError)
	})
}

func TestRawResult(t *testing.T) {
	t.Run("wraps raw bytes as tool result data", func(t *testing.T) {
		data := []byte(`{"items":[1,2,3]}`)
		result, err := RawResult(data)
		require.NoError(t, err)
		assert.Equal(t, `{"items":[1,2,3]}`, result.Data)
		assert.False(t, result.IsError)
	})

	t.Run("handles empty bytes", func(t *testing.T) {
		result, err := RawResult([]byte{})
		require.NoError(t, err)
		assert.Equal(t, "", result.Data)
		assert.False(t, result.IsError)
	})
}

func TestErrResult(t *testing.T) {
	t.Run("propagates retryable errors as Go errors", func(t *testing.T) {
		retryable := &RetryableError{StatusCode: 429, Err: errors.New("rate limited")}
		result, err := ErrResult(retryable)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.True(t, IsRetryable(err))
	})

	t.Run("wraps non-retryable errors in ToolResult", func(t *testing.T) {
		plain := errors.New("bad request")
		result, err := ErrResult(plain)
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Equal(t, "bad request", result.Data)
	})

	t.Run("propagates wrapped retryable errors", func(t *testing.T) {
		inner := &RetryableError{StatusCode: 503, Err: errors.New("timeout")}
		wrapped := fmt.Errorf("outer: %w", inner)
		result, err := ErrResult(wrapped)
		assert.Nil(t, result)
		assert.Error(t, err)
	})

	t.Run("returns nil result and nil error for nil input", func(t *testing.T) {
		result, err := ErrResult(nil)
		assert.Nil(t, result)
		assert.NoError(t, err)
	})
}

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
		Name:        ToolName("github_list_issues"),
		Description: "List issues",
		Parameters:  map[string]string{"owner": "Repo owner", "repo": "Repo name"},
		Required:    []string{"owner", "repo"},
	}

	assert.Equal(t, ToolName("github_list_issues"), td.Name)
	assert.Equal(t, "List issues", td.Description)
	assert.Len(t, td.Parameters, 2)
	assert.Equal(t, []string{"owner", "repo"}, td.Required)
}

func TestToolDefinitionOptionalRequired(t *testing.T) {
	td := ToolDefinition{
		Name:        ToolName("github_search_repos"),
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

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"valid seconds", "30", 30 * time.Second},
		{"empty header", "", 0},
		{"non-numeric", "abc", 0},
		{"negative value", "-5", 0},
		{"zero", "0", 0},
		{"capped at 60s", "999", 60 * time.Second},
		{"exactly 60s", "60", 60 * time.Second},
		{"just over cap", "61", 60 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseRetryAfter(tt.header))
		})
	}
}

func TestToolAllowed(t *testing.T) {
	tests := []struct {
		name      string
		globs     []string
		toolName  string
		wantAllow bool
	}{
		{"empty globs allows all", nil, "github_list_issues", true},
		{"empty slice allows all", []string{}, "github_list_issues", true},
		{"exact match", []string{"github_list_issues"}, "github_list_issues", true},
		{"no match", []string{"github_list_issues"}, "github_get_pull", false},
		{"wildcard suffix", []string{"github_*"}, "github_list_issues", true},
		{"wildcard suffix no match", []string{"github_*"}, "datadog_search_logs", false},
		{"wildcard prefix", []string{"*_issues"}, "github_list_issues", true},
		{"star matches everything", []string{"*"}, "anything_at_all", true},
		{"partial prefix match", []string{"github_get_*"}, "github_get_issue", true},
		{"partial prefix no match", []string{"github_get_*"}, "github_list_issues", false},
		{"multiple globs OR'd first match", []string{"github_*", "datadog_*"}, "github_list_issues", true},
		{"multiple globs OR'd second match", []string{"github_*", "datadog_*"}, "datadog_search_logs", true},
		{"multiple globs OR'd no match", []string{"github_*", "datadog_*"}, "slack_post_message", false},
		{"question mark wildcard", []string{"github_get_?"}, "github_get_x", true},
		{"question mark no match longer", []string{"github_get_?"}, "github_get_xy", false},
		{"empty tool name with star", []string{"*"}, "", true},
		{"empty tool name with pattern", []string{"github_*"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := &IntegrationConfig{ToolGlobs: tt.globs}
			assert.Equal(t, tt.wantAllow, ic.ToolAllowed(ToolName(tt.toolName)))
		})
	}
}

func TestToolAllowed_PathMatchSemantics(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"foo", "foo", true},
		{"foo", "bar", false},
		{"foo*", "foobar", true},
		{"foo*", "foo", true},
		{"*bar", "foobar", true},
		{"f*r", "foobar", true},
		{"f*r", "foo", false},
		{"f?o", "foo", true},
		{"f?o", "fooo", false},
		{"*_*", "github_list", true},
		{"github_get_*", "github_get_issue", true},
		{"github_get_*", "github_list_issues", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.name, func(t *testing.T) {
			ic := &IntegrationConfig{ToolGlobs: []string{tt.pattern}}
			assert.Equal(t, tt.want, ic.ToolAllowed(ToolName(tt.name)))
		})
	}
}

func TestValidateToolGlobs(t *testing.T) {
	assert.NoError(t, ValidateToolGlobs(nil))
	assert.NoError(t, ValidateToolGlobs([]string{}))
	assert.NoError(t, ValidateToolGlobs([]string{"github_*", "datadog_get_?"}))
	assert.Error(t, ValidateToolGlobs([]string{"github_[unclosed"}))
	assert.Error(t, ValidateToolGlobs([]string{"valid_*", "[bad"}))
}

func TestToolAllowed_InvalidPatternSkipped(t *testing.T) {
	ic := &IntegrationConfig{ToolGlobs: []string{"[bad", "github_*"}}
	assert.True(t, ic.ToolAllowed(ToolName("github_list_issues")))
	assert.False(t, ic.ToolAllowed(ToolName("datadog_search_logs")))
}

func TestToolName_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ToolName
		wantErr bool
	}{
		{"normal name", `"github_list_issues"`, ToolName("github_list_issues"), false},
		{"trims whitespace", `"  github_list_issues  "`, ToolName("github_list_issues"), false},
		{"empty string errors", `""`, "", true},
		{"whitespace-only errors", `"   "`, "", true},
		{"null is zero value", `null`, "", false},
		{"number errors", `123`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ToolName
			err := got.UnmarshalJSON([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
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

func TestIntegrationIdentity_JSONRoundTrip(t *testing.T) {
	ic := &IntegrationConfig{
		Enabled:     true,
		Credentials: Credentials{"base_url": "https://mcp.example.com"},
		Identities: map[string]IntegrationIdentity{
			"work": {
				Credentials: Credentials{"access_token": "xoxp-work"},
				Metadata:    map[string]string{"label": "Work", "team": "T1"},
			},
			"personal": {
				Credentials: Credentials{"access_token": "xoxp-personal"},
				Metadata:    map[string]string{"label": "Personal"},
			},
		},
	}

	data, err := json.Marshal(ic)
	require.NoError(t, err)

	var decoded IntegrationConfig
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.True(t, decoded.Enabled)
	assert.Equal(t, "https://mcp.example.com", decoded.Credentials["base_url"])
	require.Len(t, decoded.Identities, 2)
	assert.Equal(t, "xoxp-work", decoded.Identities["work"].Credentials["access_token"])
	assert.Equal(t, "Work", decoded.Identities["work"].Metadata["label"])
	assert.Equal(t, "T1", decoded.Identities["work"].Metadata["team"])
	assert.Equal(t, "xoxp-personal", decoded.Identities["personal"].Credentials["access_token"])
	assert.Equal(t, "Personal", decoded.Identities["personal"].Metadata["label"])
}

func TestIntegrationConfig_HasUsableCredentials_WithIdentities(t *testing.T) {
	ic := &IntegrationConfig{
		Enabled:     false,
		Credentials: Credentials{},
		Identities: map[string]IntegrationIdentity{
			"work": {Credentials: Credentials{"access_token": "tok"}},
		},
	}
	assert.True(t, ic.HasUsableCredentials())

	empty := &IntegrationConfig{Credentials: Credentials{}}
	assert.False(t, empty.HasUsableCredentials())

	oauthOnly := &IntegrationConfig{Credentials: Credentials{CredKeyClientID: "cid"}}
	assert.False(t, oauthOnly.HasUsableCredentials())

	topLevel := &IntegrationConfig{Credentials: Credentials{"token": "t"}}
	assert.True(t, topLevel.HasUsableCredentials())
}

type multiIdentityMock struct {
	name            string
	lastCreds       Credentials
	lastIdentities  map[string]IntegrationIdentity
	configureErr    error
	identitiesErr   error
	configureCalls  int
	identitiesCalls int
}

func (m *multiIdentityMock) Name() string { return m.name }
func (m *multiIdentityMock) Configure(_ context.Context, creds Credentials) error {
	m.configureCalls++
	m.lastCreds = creds
	return m.configureErr
}
func (m *multiIdentityMock) ConfigureIdentities(_ context.Context, identities map[string]IntegrationIdentity) error {
	m.identitiesCalls++
	m.lastIdentities = identities
	return m.identitiesErr
}
func (m *multiIdentityMock) Tools() []ToolDefinition { return nil }
func (m *multiIdentityMock) Execute(context.Context, ToolName, map[string]any) (*ToolResult, error) {
	return &ToolResult{Data: "ok"}, nil
}
func (m *multiIdentityMock) Healthy(context.Context) bool { return true }

func TestConfigureIntegration_CallsConfigureThenIdentities(t *testing.T) {
	m := &multiIdentityMock{name: "multi"}
	ic := &IntegrationConfig{
		Credentials: Credentials{"base_url": "https://example.com"},
		Identities: map[string]IntegrationIdentity{
			"a": {Credentials: Credentials{"access_token": "t1"}},
		},
	}
	err := ConfigureIntegration(context.Background(), m, ic)
	require.NoError(t, err)
	assert.Equal(t, 1, m.configureCalls)
	assert.Equal(t, 1, m.identitiesCalls)
	assert.Equal(t, "https://example.com", m.lastCreds["base_url"])
	assert.Equal(t, "t1", m.lastIdentities["a"].Credentials["access_token"])
}

type singleIdentityMock struct {
	lastCreds      Credentials
	configureCalls int
}

func (m *singleIdentityMock) Name() string { return "single" }
func (m *singleIdentityMock) Configure(_ context.Context, creds Credentials) error {
	m.configureCalls++
	m.lastCreds = creds
	return nil
}
func (m *singleIdentityMock) Tools() []ToolDefinition { return nil }
func (m *singleIdentityMock) Execute(context.Context, ToolName, map[string]any) (*ToolResult, error) {
	return &ToolResult{Data: "ok"}, nil
}
func (m *singleIdentityMock) Healthy(context.Context) bool { return true }

func TestConfigureIntegration_SkipsIdentitiesWhenNotImplemented(t *testing.T) {
	m := &singleIdentityMock{}
	ic := &IntegrationConfig{
		Credentials: Credentials{"k": "v"},
		Identities: map[string]IntegrationIdentity{
			"a": {Credentials: Credentials{"access_token": "t"}},
		},
	}
	require.NoError(t, ConfigureIntegration(context.Background(), m, ic))
	assert.Equal(t, 1, m.configureCalls)
	assert.Equal(t, "v", m.lastCreds["k"])
}

func TestConfigureIntegration_AlwaysCallsConfigureIdentitiesWhenImplemented(t *testing.T) {
	m := &multiIdentityMock{name: "multi"}
	ic := &IntegrationConfig{Credentials: Credentials{"k": "v"}}
	require.NoError(t, ConfigureIntegration(context.Background(), m, ic))
	assert.Equal(t, 1, m.configureCalls)
	assert.Equal(t, 1, m.identitiesCalls)
	assert.Nil(t, m.lastIdentities)
}

func TestConfigureIntegration_PropagatesConfigureError(t *testing.T) {
	m := &multiIdentityMock{name: "multi", configureErr: fmt.Errorf("bad creds")}
	err := ConfigureIntegration(context.Background(), m, &IntegrationConfig{Credentials: Credentials{"k": "v"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad creds")
	assert.Equal(t, 0, m.identitiesCalls)
}

func TestConfigureIntegration_PropagatesIdentitiesError(t *testing.T) {
	m := &multiIdentityMock{name: "multi", identitiesErr: fmt.Errorf("bad id")}
	err := ConfigureIntegration(context.Background(), m, &IntegrationConfig{
		Credentials: Credentials{},
		Identities:  map[string]IntegrationIdentity{"a": {Credentials: Credentials{"access_token": "t"}}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad id")
}
