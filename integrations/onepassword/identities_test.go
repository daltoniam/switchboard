package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	op "github.com/1password/onepassword-sdk-go"
	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storesByAuth returns an adapter whose sign-ins hand out one fake store per
// credential set, recording each sign-in.
func storesByAuth(t *testing.T, stores map[auth]*fakeStore) (*onepassword, *[]auth) {
	t.Helper()
	var signIns []auth
	a := New().(*onepassword)
	a.desktopAvailable = true
	a.connect = func(_ context.Context, au auth) (store, error) {
		signIns = append(signIns, au)
		s, ok := stores[au]
		if !ok {
			return nil, errors.New("unexpected sign-in")
		}
		return s, nil
	}
	return a, &signIns
}

func vaultStore(title string) *fakeStore {
	f := newFake()
	f.vaults = []op.VaultOverview{{ID: "v-" + title, Title: title, VaultType: op.VaultTypeUserCreated}}
	return f
}

func tokenIdentity(token, label string) mcp.IntegrationIdentity {
	return mcp.IntegrationIdentity{Credentials: mcp.Credentials{"service_account_token": token}, Metadata: map[string]string{"label": label}}
}

func desktopIdentity(account, label string) mcp.IntegrationIdentity {
	return mcp.IntegrationIdentity{Credentials: mcp.Credentials{"account_name": account}, Metadata: map[string]string{"label": label}}
}

func vaultTitles(t *testing.T, res *mcp.ToolResult) []string {
	t.Helper()
	require.False(t, res.IsError, res.Data)
	var out []vaultView
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out))
	titles := make([]string, 0, len(out))
	for _, v := range out {
		titles = append(titles, v.Title)
	}
	return titles
}

func TestIdentities_RouteEachCallToItsAccount(t *testing.T) {
	a, _ := storesByAuth(t, map[auth]*fakeStore{
		{account: "My Team"}: vaultStore("Personal"),
		{token: "ops_work"}:  vaultStore("Work"),
		{token: "ops_ci"}:    vaultStore("CI"),
	})
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"account_name": "My Team"},
		Identities: map[string]mcp.IntegrationIdentity{
			"work": tokenIdentity("ops_work", "Work"),
			"ci":   tokenIdentity("ops_ci", ""),
		},
	}))
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"omitted uses the default identity", nil, "Personal"},
		{"empty uses the default identity", map[string]any{identityParam: " "}, "Personal"},
		{"default by name", map[string]any{identityParam: "default"}, "Personal"},
		{"named identity", map[string]any{identityParam: "work"}, "Work"},
		{"another named identity", map[string]any{identityParam: " ci "}, "CI"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, []string{tc.want}, vaultTitles(t, run(t, a, "onepassword_list_vaults", tc.args)))
		})
	}
}

func TestIdentities_LookupsStayWithinTheIdentity(t *testing.T) {
	work := vaultStore("Work")
	a, _ := storesByAuth(t, map[auth]*fakeStore{{token: "ops_default"}: newFake(), {token: "ops_work"}: work})
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"service_account_token": "ops_default"},
		Identities:  map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work", "")},
	}))
	res := run(t, a, "onepassword_list_items", map[string]any{"vault": "Claude", identityParam: "work"})
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, `vault "Claude" not found; vaults: Work`)
	out := decode(t, run(t, a, "onepassword_list_items", map[string]any{"vault": "Claude"}))
	assert.EqualValues(t, 4, out["total"])
}

func TestIdentities_UnknownIdentity(t *testing.T) {
	a, _ := storesByAuth(t, map[auth]*fakeStore{{token: "ops_default"}: newFake()})
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"service_account_token": "ops_default"},
		Identities:  map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work", "")},
	}))
	res := run(t, a, "onepassword_get_item", map[string]any{"vault": "Claude", "item": "GitHub", identityParam: "home"})
	require.True(t, res.IsError)
	for _, want := range []string{`unknown identity "home"`, "default, work", "onepassword_list_identities"} {
		assert.Contains(t, res.Data, want)
	}

	res = run(t, a, "onepassword_list_vaults", map[string]any{identityParam: []any{"work"}})
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, identityParam)
}

func TestIdentities_DefaultWithoutTopLevelCredentials(t *testing.T) {
	a, _ := storesByAuth(t, map[auth]*fakeStore{{token: "ops_b"}: vaultStore("B"), {token: "ops_a"}: vaultStore("A")})
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
		Identities: map[string]mcp.IntegrationIdentity{"beta": tokenIdentity("ops_b", ""), "alpha": tokenIdentity("ops_a", "")},
	}))
	assert.Equal(t, []string{"A"}, vaultTitles(t, run(t, a, "onepassword_list_vaults", nil)), "the first identity by id is the default")
	assert.Equal(t, []string{"A"}, vaultTitles(t, run(t, a, "onepassword_list_vaults", map[string]any{identityParam: "default"})))
	assert.Equal(t, []string{"B"}, vaultTitles(t, run(t, a, "onepassword_list_vaults", map[string]any{identityParam: "beta"})))
}

func TestIdentities_BackwardCompatibleSingleCredential(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  auth
	}{
		{"service account token", mcp.Credentials{"service_account_token": "ops_old"}, auth{token: "ops_old"}},
		{"desktop app account", mcp.Credentials{"account_name": "My Team"}, auth{account: "My Team"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, signIns := storesByAuth(t, map[auth]*fakeStore{tc.want: newFake()})
			require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{Credentials: tc.creds}))
			assert.Len(t, vaultTitles(t, run(t, a, "onepassword_list_vaults", nil)), 2)
			assert.Equal(t, []auth{tc.want}, *signIns)
			out := decode(t, run(t, a, "onepassword_list_identities", nil))
			assert.Equal(t, "default", out["default_identity"])
			require.Len(t, out["identities"], 1)
		})
	}
}

func TestIdentities_NothingConfigured(t *testing.T) {
	a := New().(*onepassword)
	err := mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set account_name")
	res := run(t, a, "onepassword_list_identities", nil)
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, "not configured")
}

func TestIdentities_DesktopAppLimit(t *testing.T) {
	tests := []struct {
		name       string
		creds      mcp.Credentials
		identities map[string]mcp.IntegrationIdentity
		wantErr    string
	}{
		{"one desktop identity at the top level and service accounts",
			mcp.Credentials{"account_name": "My Team"},
			map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work", ""), "ci": tokenIdentity("ops_ci", "")}, ""},
		{"one named desktop identity and a top-level service account",
			mcp.Credentials{"service_account_token": "ops_default"},
			map[string]mcp.IntegrationIdentity{"me": desktopIdentity("My Team", "")}, ""},
		{"a second desktop identity next to the top-level one",
			mcp.Credentials{"account_name": "My Team"},
			map[string]mcp.IntegrationIdentity{"family": desktopIdentity("Family", "")}, `identities "default" and "family"`},
		{"two named desktop identities",
			nil,
			map[string]mcp.IntegrationIdentity{"family": desktopIdentity("Family", ""), "me": desktopIdentity("My Team", "")}, `identities "family" and "me"`},
		{"the same desktop account twice",
			mcp.Credentials{"account_name": "My Team"},
			map[string]mcp.IntegrationIdentity{"me": desktopIdentity("My Team", "")}, "only one identity can sign in through the 1Password desktop app"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := storesByAuth(t, nil)
			err := mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{Credentials: tc.creds, Identities: tc.identities})
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range []string{tc.wantErr, "only one identity can sign in through the 1Password desktop app", "first account", "service account token"} {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestIdentities_RejectedChangeKeepsWorkingIdentities(t *testing.T) {
	a, _ := storesByAuth(t, map[auth]*fakeStore{{account: "My Team"}: vaultStore("Personal"), {token: "ops_work"}: vaultStore("Work")})
	cfg := &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"account_name": "My Team"},
		Identities:  map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work", "")},
	}
	ctx := context.Background()
	require.NoError(t, mcp.ConfigureIntegration(ctx, a, cfg))
	cfg.Identities["family"] = desktopIdentity("Family", "")
	require.Error(t, mcp.ConfigureIntegration(ctx, a, cfg))
	assert.Equal(t, []string{"Work"}, vaultTitles(t, run(t, a, "onepassword_list_vaults", map[string]any{identityParam: "work"})))
	res := run(t, a, "onepassword_list_vaults", map[string]any{identityParam: "family"})
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, `unknown identity "family"`)
}

func TestIdentities_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		identity mcp.IntegrationIdentity
		id       string
		want     string
	}{
		{"reserved id", tokenIdentity("ops_x", ""), "default", `"default" is reserved`},
		{"no credentials", mcp.IntegrationIdentity{}, "work", `identity "work": set account_name`},
		{"both credentials", mcp.IntegrationIdentity{Credentials: mcp.Credentials{"account_name": "A", "service_account_token": "ops_x"}}, "work", `identity "work": set account_name or service_account_token, not both`},
		{"bad token", tokenIdentity("dop_v1_secretvalue", ""), "work", `identity "work": service_account_token must be`},
		{"untrimmed id", tokenIdentity("ops_x", ""), " work", "leading or trailing spaces"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := storesByAuth(t, nil)
			err := mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
				Credentials: mcp.Credentials{"service_account_token": "ops_default"},
				Identities:  map[string]mcp.IntegrationIdentity{tc.id: tc.identity},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), "secretvalue", "errors must not echo the token")
		})
	}
}

func TestIdentities_ListIdentities(t *testing.T) {
	a, signIns := storesByAuth(t, nil)
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"account_name": "My Team"},
		Identities:  map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work-secret", "Work vaults")},
	}))
	res := run(t, a, "onepassword_list_identities", nil)
	require.False(t, res.IsError, res.Data)
	assert.NotContains(t, res.Data, "ops_work-secret", "never returns tokens")
	assert.Empty(t, *signIns, "listing identities does not sign in")
	var out struct {
		Default    string `json:"default_identity"`
		Identities []struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			Method  string `json:"method"`
			Account string `json:"account"`
			Default bool   `json:"default"`
		} `json:"identities"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out))
	assert.Equal(t, "default", out.Default)
	require.Len(t, out.Identities, 2)
	assert.Equal(t, "default", out.Identities[0].ID)
	assert.True(t, out.Identities[0].Default)
	assert.Equal(t, "desktop_app", out.Identities[0].Method)
	assert.Equal(t, "My Team", out.Identities[0].Account)
	assert.Equal(t, "work", out.Identities[1].ID)
	assert.False(t, out.Identities[1].Default)
	assert.Equal(t, "Work vaults", out.Identities[1].Label)
	assert.Equal(t, "service_account", out.Identities[1].Method)
	assert.Empty(t, out.Identities[1].Account)
}

func TestIdentities_SignInOncePerCredentialSet(t *testing.T) {
	a, signIns := storesByAuth(t, map[auth]*fakeStore{{token: "ops_default"}: newFake(), {token: "ops_work"}: newFake()})
	cfg := &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"service_account_token": "ops_default"},
		Identities:  map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work", "")},
	}
	ctx := context.Background()
	require.NoError(t, mcp.ConfigureIntegration(ctx, a, cfg))
	run(t, a, "onepassword_list_vaults", map[string]any{identityParam: "work"})
	run(t, a, "onepassword_list_vaults", nil)
	require.NoError(t, mcp.ConfigureIntegration(ctx, a, cfg))
	run(t, a, "onepassword_list_vaults", map[string]any{identityParam: "work"})
	run(t, a, "onepassword_list_vaults", nil)
	assert.Equal(t, []auth{{token: "ops_work"}, {token: "ops_default"}}, *signIns, "reloading the same settings keeps every signed-in client")
}

func TestIdentities_HealthCoversEveryIdentity(t *testing.T) {
	broken := newFake()
	broken.err = errors.New("forbidden")
	a, _ := storesByAuth(t, map[auth]*fakeStore{{token: "ops_default"}: newFake(), {token: "ops_work"}: broken})
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), a, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"service_account_token": "ops_default"},
		Identities:  map[string]mcp.IntegrationIdentity{"work": tokenIdentity("ops_work", "")},
	}))
	assert.False(t, a.Healthy(context.Background()))
	assert.Contains(t, a.SignInError(), `identity "work"`)
	assert.Contains(t, a.SignInError(), "forbidden")
	assert.NotContains(t, a.SignInError(), `identity "default"`)
}

func TestIdentities_ToolsTakeIdentity(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "onepassword_list_identities" {
			assert.Empty(t, tool.Parameters)
			continue
		}
		assert.Contains(t, tool.Parameters[identityParam], "onepassword_list_identities", tool.Name)
		assert.NotContains(t, tool.Required, identityParam, tool.Name)
	}
}

func TestIdentityHints(t *testing.T) {
	a := New()
	_, ok := a.(mcp.MultiIdentityIntegration)
	require.True(t, ok)
	hints, ok := a.(mcp.IdentityConfigHints)
	require.True(t, ok)
	assert.Equal(t, []string{"account_name", "service_account_token"}, hints.IdentityCredentialKeys())
	assert.Equal(t, []string{"label"}, hints.IdentityMetadataKeys())
}

func TestDesktopBinding(t *testing.T) {
	tests := []struct {
		name    string
		binds   []string
		wantErr string
	}{
		{"first account binds", []string{"My Team"}, ""},
		{"same account again", []string{"My Team", "My Team"}, ""},
		{"another account after the first", []string{"My Team", "Family"}, `already signed in to the 1Password desktop app as "My Team"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b desktopBinding
			var err error
			for _, account := range tc.binds {
				if err = b.bind(account); err != nil {
					break
				}
			}
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Contains(t, err.Error(), "Restart Switchboard")
		})
	}
}

func TestListVaults_BuiltInVaultDescriptions(t *testing.T) {
	f := newFake()
	f.vaults = []op.VaultOverview{
		{ID: "p", Title: "Private", Description: "GYkuBJsjEZmMUuiP", VaultType: op.VaultTypePersonal},
		{ID: "s", Title: "Shared", Description: "GYkuBJsjEZmMUuiP", VaultType: op.VaultTypeEveryone},
		{ID: "u", Title: "Curri", Description: "Curri Passwords", VaultType: op.VaultTypeUserCreated},
		{ID: "e", Title: "Empty", VaultType: op.VaultTypeUserCreated},
	}
	res := run(t, newTestAdapter(t, f), "onepassword_list_vaults", nil)
	require.False(t, res.IsError, res.Data)
	assert.NotContains(t, res.Data, "GYkuBJsjEZmMUuiP")
	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out))
	require.Len(t, out, 4)
	tests := []struct {
		title       string
		description any
	}{
		{"Private", ""},
		{"Shared", ""},
		{"Curri", "Curri Passwords"},
		{"Empty", ""},
	}
	for i, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			assert.Equal(t, tc.title, out[i]["title"])
			assert.Equal(t, tc.description, out[i]["description"])
		})
	}
	assert.True(t, strings.Contains(res.Data, `"type":"personal"`))
}
