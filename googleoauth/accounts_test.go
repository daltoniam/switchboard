package googleoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testBaseTools = []mcp.ToolDefinition{
	{Name: "gmail_get_profile", Description: "Start here. Get profile.", Parameters: map[string]string{"user_id": "User"}},
	{Name: "gmail_list_labels", Description: "List labels.", Parameters: map[string]string{}},
}

func twoIdentities() map[string]mcp.IntegrationIdentity {
	return map[string]mcp.IntegrationIdentity{
		"work": {
			Credentials: mcp.Credentials{"access_token": "work-access", "refresh_token": "work-refresh"},
			Metadata:    map[string]string{"label": "Work", "email": "me@work.example"},
		},
		"personal": {
			Credentials: mcp.Credentials{"access_token": "personal-access"},
			Metadata:    map[string]string{"label": "Personal"},
		},
	}
}

func TestAccountSet_NoIdentitiesKeepsBaseTools(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(nil, "cid", "csec"))
	assert.Equal(t, testBaseTools, s.Tools())
	assert.Equal(t, 0, s.Len())
}

func TestAccountSet_NilIsEmpty(t *testing.T) {
	var s *AccountSet
	assert.Equal(t, 0, s.Len())
	assert.Nil(t, s.Tools())
	ctx, args, res, err := s.Route(context.Background(), "gmail_get_profile", map[string]any{"x": 1})
	require.NoError(t, err)
	assert.Nil(t, res)
	assert.Nil(t, AccountFromContext(ctx))
	assert.Equal(t, map[string]any{"x": 1}, args)
}

func TestAccountSet_ToolsGainAccountParamAndListTool(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))

	tools := s.Tools()
	require.Len(t, tools, len(testBaseTools)+1)
	for _, tool := range tools[:len(testBaseTools)] {
		assert.Contains(t, tool.Parameters, AccountParam, "tool %s missing account param", tool.Name)
		assert.NotContains(t, tool.Required, AccountParam, "account must stay optional")
	}
	list := tools[len(tools)-1]
	assert.Equal(t, mcp.ToolName("gmail_list_accounts"), list.Name)
	assert.Contains(t, list.Description, "Gmail")

	// Base definitions must not be mutated.
	assert.NotContains(t, testBaseTools[0].Parameters, AccountParam)
}

func TestAccountSet_ConfigureValidation(t *testing.T) {
	tests := []struct {
		name       string
		identities map[string]mcp.IntegrationIdentity
		wantErr    string
	}{
		{
			name:       "missing access token",
			identities: map[string]mcp.IntegrationIdentity{"work": {Credentials: mcp.Credentials{"refresh_token": "r"}}},
			wantErr:    `account "work": access_token is required`,
		},
		{
			name:       "reserved default id",
			identities: map[string]mcp.IntegrationIdentity{"default": {Credentials: mcp.Credentials{"access_token": "a"}}},
			wantErr:    `"default" is reserved`,
		},
		{
			name:       "blank id",
			identities: map[string]mcp.IntegrationIdentity{"  ": {Credentials: mcp.Credentials{"access_token": "a"}}},
			wantErr:    "account id cannot be empty",
		},
		{
			// A padded key would not match the key persist() writes back to.
			name:       "padded id",
			identities: map[string]mcp.IntegrationIdentity{" work ": {Credentials: mcp.Credentials{"access_token": "a"}}},
			wantErr:    `account id " work " must not have leading or trailing spaces`,
		},
		{
			// Without rejection both keys would collapse into one account.
			name: "padded id colliding with another",
			identities: map[string]mcp.IntegrationIdentity{
				"work":  {Credentials: mcp.Credentials{"access_token": "a"}},
				"work ": {Credentials: mcp.Credentials{"access_token": "b"}},
			},
			wantErr: `account id "work " must not have leading or trailing spaces`,
		},
		{
			name:       "padded reserved id",
			identities: map[string]mcp.IntegrationIdentity{" default": {Credentials: mcp.Credentials{"access_token": "a"}}},
			wantErr:    "must not have leading or trailing spaces",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewAccountSet("gmail", testBaseTools)
			err := s.Configure(tt.identities, "cid", "csec")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAccountSet_FailedConfigureKeepsPreviousAccounts(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))
	err := s.Configure(map[string]mcp.IntegrationIdentity{"bad": {}}, "cid", "csec")
	require.Error(t, err)
	assert.Equal(t, 2, s.Len())
}

func TestAccountSet_Route(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))

	tests := []struct {
		name        string
		args        map[string]any
		wantAccount string // "" means default (no account in context)
		wantErr     string
	}{
		{name: "omitted uses default", args: map[string]any{"q": "x"}},
		{name: "explicit default", args: map[string]any{"q": "x", "account": "default"}},
		{name: "named account", args: map[string]any{"q": "x", "account": "work"}, wantAccount: "work"},
		{name: "trims whitespace", args: map[string]any{"q": "x", "account": " personal "}, wantAccount: "personal"},
		{name: "unknown account", args: map[string]any{"account": "nope"}, wantErr: `unknown account "nope"; available: default, personal, work`},
		{name: "wrong type", args: map[string]any{"account": 42}, wantErr: "account"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, args, res, err := s.Route(context.Background(), "gmail_list_labels", tt.args)
			require.NoError(t, err)
			if tt.wantErr != "" {
				require.NotNil(t, res)
				assert.True(t, res.IsError)
				assert.Contains(t, res.Data, tt.wantErr)
				return
			}
			require.Nil(t, res)
			assert.NotContains(t, args, AccountParam, "account arg must be stripped before dispatch")
			assert.Equal(t, "x", args["q"])
			acct := AccountFromContext(ctx)
			if tt.wantAccount == "" {
				assert.Nil(t, acct)
				return
			}
			require.NotNil(t, acct)
			assert.Equal(t, tt.wantAccount, acct.ID())
		})
	}
}

func TestAccountSet_ListAccountsNeverLeaksTokens(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))

	_, _, res, err := s.Route(context.Background(), "gmail_list_accounts", nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.IsError, res.Data)
	for _, secret := range []string{"work-access", "work-refresh", "personal-access", "csec"} {
		assert.NotContains(t, res.Data, secret)
	}

	var out struct {
		DefaultAccount string `json:"default_account"`
		Accounts       []struct {
			ID       string            `json:"id"`
			Default  bool              `json:"default"`
			Metadata map[string]string `json:"metadata"`
		} `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out))
	assert.Equal(t, "default", out.DefaultAccount)
	require.Len(t, out.Accounts, 3)
	assert.Equal(t, "default", out.Accounts[0].ID)
	assert.True(t, out.Accounts[0].Default)
	assert.Equal(t, "personal", out.Accounts[1].ID)
	assert.Equal(t, "work", out.Accounts[2].ID)
	assert.Equal(t, "me@work.example", out.Accounts[2].Metadata["email"])
}

func TestAccountSet_ListToolNotRoutedWithoutIdentities(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(nil, "cid", "csec"))
	_, _, res, err := s.Route(context.Background(), "gmail_list_accounts", nil)
	require.NoError(t, err)
	assert.Nil(t, res, "without named accounts the list tool does not exist; the adapter reports unknown tool")
}

func TestAccount_InheritsAndOverridesClient(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	ids := twoIdentities()
	ids["other"] = mcp.IntegrationIdentity{Credentials: mcp.Credentials{
		"access_token": "o", "client_id": "other-cid", "client_secret": "other-csec",
	}}
	require.NoError(t, s.Configure(ids, "cid", "csec"))

	work, err := s.lookup("work")
	require.NoError(t, err)
	assert.Equal(t, "cid", work.clientID)
	other, err := s.lookup("other")
	require.NoError(t, err)
	assert.Equal(t, "other-cid", other.clientID)
	assert.Equal(t, "other-csec", other.clientSecret)
}

type fakeConfig struct {
	mcp.ConfigService
	mu  sync.Mutex
	ics map[string]*mcp.IntegrationConfig
}

func (f *fakeConfig) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ic, ok := f.ics[name]
	return ic, ok
}

func (f *fakeConfig) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ics[name] = ic
	return nil
}

func TestAccount_RefreshPersistsToIdentity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "work-refresh", r.FormValue("refresh_token"))
		assert.Equal(t, "cid", r.FormValue("client_id"))
		_, _ = w.Write([]byte(`{"access_token":"work-new","expires_in":3600}`))
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	cfg := &fakeConfig{ics: map[string]*mcp.IntegrationConfig{
		"gmail": {
			Enabled:     true,
			Credentials: mcp.Credentials{"access_token": "default-access"},
			Identities:  twoIdentities(),
		},
	}}
	s := NewAccountSet("gmail", testBaseTools)
	s.SetConfigService(cfg)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))
	work, err := s.lookup("work")
	require.NoError(t, err)

	tok, err := work.Refresh(context.Background(), "work-access")
	require.NoError(t, err)
	assert.Equal(t, "work-new", tok)
	assert.Equal(t, "work-new", work.AccessToken())

	ic, _ := cfg.GetIntegration("gmail")
	assert.Equal(t, "work-new", ic.Identities["work"].Credentials["access_token"])
	assert.Equal(t, "default-access", ic.Credentials["access_token"], "default account must not change")
	assert.Equal(t, "personal-access", ic.Identities["personal"].Credentials["access_token"])
}

func TestAccount_RefreshSkipsWhenTokenAlreadyRotated(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"access_token":"never"}`))
	}))
	defer ts.Close()
	withTokenURL(t, ts.URL)

	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))
	work, err := s.lookup("work")
	require.NoError(t, err)

	tok, err := work.Refresh(context.Background(), "some-older-token")
	require.NoError(t, err)
	assert.Equal(t, "work-access", tok)
	assert.Equal(t, 0, calls)
}

func TestAccount_RefreshWithoutRefreshToken(t *testing.T) {
	s := NewAccountSet("gmail", testBaseTools)
	require.NoError(t, s.Configure(twoIdentities(), "cid", "csec"))
	personal, err := s.lookup("personal")
	require.NoError(t, err)
	_, err = personal.Refresh(context.Background(), "personal-access")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "refresh_token"))
}
