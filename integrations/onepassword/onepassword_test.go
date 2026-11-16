package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	op "github.com/1password/onepassword-sdk-go"
	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	vaultID    = "x3k2kwy3ykguz7dctqds5mya6y"
	otherVault = "aaaaaaaaaaaaaaaaaaaaaaaaaa"
	itemID     = "ictxhvk6do4kmwmljwdgj7gkai"
	secret     = "hunter2-SECRET"
)

// fakeStore stands in for the 1Password SDK client.
type fakeStore struct {
	vaults    []op.VaultOverview
	items     map[string][]op.ItemOverview // by vault id
	full      map[string]op.Item           // by item id
	secrets   map[string]string            // by reference
	err       error
	created   *op.ItemCreateParams
	put       *op.Item
	generated int
}

func (f *fakeStore) ListVaults(context.Context) ([]op.VaultOverview, error) {
	return f.vaults, f.err
}

func (f *fakeStore) ListItems(_ context.Context, vault string) ([]op.ItemOverview, error) {
	return f.items[vault], f.err
}

func (f *fakeStore) GetItem(_ context.Context, vault, id string) (op.Item, error) {
	if f.err != nil {
		return op.Item{}, f.err
	}
	it, ok := f.full[id]
	if !ok || it.VaultID != vault {
		return op.Item{}, errors.New("item not found")
	}
	return it, nil
}

func (f *fakeStore) CreateItem(_ context.Context, p op.ItemCreateParams) (op.Item, error) {
	if f.err != nil {
		return op.Item{}, f.err
	}
	f.created = &p
	return op.Item{ID: "newitemnewitemnewitemnewit", Title: p.Title, Category: p.Category, VaultID: p.VaultID, Fields: p.Fields, Sections: p.Sections, Tags: p.Tags, Websites: p.Websites, Version: 1}, nil
}

func (f *fakeStore) PutItem(_ context.Context, it op.Item) (op.Item, error) {
	if f.err != nil {
		return op.Item{}, f.err
	}
	sent := it
	f.put = &sent
	it.Version++
	return it, nil
}

func (f *fakeStore) ResolveSecret(_ context.Context, ref string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.secrets[ref]
	if !ok {
		return "", errors.New("no item matched the secret reference query")
	}
	return v, nil
}

func (f *fakeStore) GeneratePassword(context.Context) (string, error) {
	f.generated++
	return "generated-" + secret, nil
}

func newFake() *fakeStore {
	sect := "sec1"
	return &fakeStore{
		vaults: []op.VaultOverview{
			{ID: vaultID, Title: "Claude", Description: "agent vault", VaultType: op.VaultTypeUserCreated, ActiveItemCount: 2, UpdatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
			{ID: otherVault, Title: "Shared"},
		},
		items: map[string][]op.ItemOverview{
			vaultID: {
				{ID: itemID, Title: "MacBook Pro recovery FileVault code", Category: op.ItemCategoryPassword, VaultID: vaultID, Tags: []string{"mac"}, State: op.ItemStateActive},
				{ID: "loginloginloginloginloginl", Title: "GitHub", Category: op.ItemCategoryLogin, VaultID: vaultID, Websites: []op.Website{{URL: "https://github.com", Label: "website"}}, State: op.ItemStateActive},
				{ID: "dupdupdupdupdupdupdupdupd1", Title: "Twin", Category: op.ItemCategorySecureNote, VaultID: vaultID},
				{ID: "dupdupdupdupdupdupdupdupd2", Title: "twin", Category: op.ItemCategorySecureNote, VaultID: vaultID},
			},
		},
		full: map[string]op.Item{
			itemID: {ID: itemID, Title: "MacBook Pro recovery FileVault code", Category: op.ItemCategoryPassword, VaultID: vaultID, Version: 3,
				Fields: []op.ItemField{{ID: "password", Title: "password", FieldType: op.ItemFieldTypeConcealed, Value: secret}}},
			"loginloginloginloginloginl": {ID: "loginloginloginloginloginl", Title: "GitHub", Category: op.ItemCategoryLogin, VaultID: vaultID, Version: 7, Notes: "work account",
				Sections: []op.ItemSection{{ID: sect, Title: "Security"}},
				Fields: []op.ItemField{
					{ID: "username", Title: "username", FieldType: op.ItemFieldTypeText, Value: "octocat"},
					{ID: "password", Title: "password", FieldType: op.ItemFieldTypeConcealed, Value: secret},
					{ID: "otp", Title: "one-time password", SectionID: &sect, FieldType: op.ItemFieldTypeTOTP, Value: "otpauth://totp/x?secret=" + secret,
						Details: otpDetails("123456")},
					{ID: "pin", Title: "Recovery PIN", SectionID: &sect, FieldType: op.ItemFieldTypeText, Value: "0000"},
				},
				Websites: []op.Website{{URL: "https://github.com", Label: "website", AutofillBehavior: op.AutofillBehaviorAnywhereOnWebsite}}},
		},
		secrets: map[string]string{"op://Claude/GitHub/password": secret},
	}
}

func otpDetails(code string) *op.ItemFieldDetails {
	var d op.ItemFieldDetails
	raw := `{"type":"Otp","content":{"code":"` + code + `"}}`
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		panic(err)
	}
	return &d
}

func newTestAdapter(t *testing.T, s *fakeStore) *onepassword {
	t.Helper()
	a := New().(*onepassword)
	a.connect = func(context.Context, string) (store, error) { return s, nil }
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"service_account_token": "ops_test-token"}))
	return a
}

func run(t *testing.T, a *onepassword, tool mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	res, err := a.Execute(context.Background(), tool, args)
	require.NoError(t, err)
	return res
}

func decode(t *testing.T, res *mcp.ToolResult) map[string]any {
	t.Helper()
	require.False(t, res.IsError, res.Data)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out))
	return out
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "onepassword", i.Name())
}

func TestConfigure(t *testing.T) {
	a := New().(*onepassword)
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"service_account_token": "  ops_abc \n"}))
	assert.Equal(t, "ops_abc", a.token)
}

func TestConfigure_Rejects(t *testing.T) {
	tests := []struct {
		name, token, want string
	}{
		{"missing", "", "service_account_token is required"},
		{"not a service account token", "dop_v1_abc", "service account token (ops_...)"},
		{"line break", "ops_a\nb", "service account token (ops_...)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := New().Configure(context.Background(), mcp.Credentials{"service_account_token": tc.token})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), "dop_v1_abc", "errors must not echo the token")
		})
	}
}

func TestConfigure_DropsConnectedClient(t *testing.T) {
	calls := 0
	a := New().(*onepassword)
	a.connect = func(context.Context, string) (store, error) { calls++; return newFake(), nil }
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"service_account_token": "ops_one"}))
	assert.True(t, a.Healthy(context.Background()))
	assert.True(t, a.Healthy(context.Background()))
	assert.Equal(t, 1, calls, "the signed-in client is reused")
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"service_account_token": "ops_two"}))
	assert.True(t, a.Healthy(context.Background()))
	assert.Equal(t, 2, calls, "a new token signs in again")
}

func TestHealthy(t *testing.T) {
	assert.False(t, New().Healthy(context.Background()), "unconfigured")

	a := New().(*onepassword)
	a.connect = func(context.Context, string) (store, error) { return nil, errors.New("invalid token") }
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"service_account_token": "ops_x"}))
	assert.False(t, a.Healthy(context.Background()), "sign-in fails")

	f := newFake()
	f.err = errors.New("forbidden")
	assert.False(t, newTestAdapter(t, f).Healthy(context.Background()), "listing vaults fails")

	assert.True(t, newTestAdapter(t, newFake()).Healthy(context.Background()))
}

func TestClose_IsRepeatable(t *testing.T) {
	a := newTestAdapter(t, newFake())
	require.True(t, a.Healthy(context.Background()))
	require.NoError(t, a.Close())
	require.NoError(t, a.Close())
	assert.Nil(t, a.client)
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
		assert.True(t, strings.HasPrefix(string(tool.Name), "onepassword_"), "tool %s missing prefix", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		assert.True(t, tool.Access.Declared(), "tool %s has no access class", tool.Name)
		assert.NotContains(t, string(tool.Name), "delete", "no delete tool")
		seen[tool.Name] = true
	}
	assert.Contains(t, tools[0].Description, "Start here")
}

func TestTools_OnlyCreateAndUpdateWrite(t *testing.T) {
	for _, tool := range tools {
		want := mcp.AccessRead
		if tool.Name == "onepassword_create_item" || tool.Name == "onepassword_update_item" {
			want = mcp.AccessWrite
		}
		assert.Equal(t, want, tool.Access, tool.Name)
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range tools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	res := run(t, newTestAdapter(t, newFake()), "onepassword_nope", nil)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tool")
}

func TestExecute_NotConfigured(t *testing.T) {
	res, err := New().Execute(context.Background(), "onepassword_list_vaults", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "not configured")
}

func TestExecute_SignInFailure(t *testing.T) {
	a := New().(*onepassword)
	a.connect = func(context.Context, string) (store, error) {
		return nil, errors.New("service account token had invalid format")
	}
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"service_account_token": "ops_x"}))
	res := run(t, a, "onepassword_list_vaults", nil)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "cannot sign in")
	assert.Contains(t, res.Data, "invalid format")
}

func TestListVaults(t *testing.T) {
	res := run(t, newTestAdapter(t, newFake()), "onepassword_list_vaults", nil)
	require.False(t, res.IsError, res.Data)
	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out))
	require.Len(t, out, 2)
	assert.Equal(t, vaultID, out[0]["id"])
	assert.Equal(t, "Claude", out[0]["title"])
	assert.Equal(t, "agent vault", out[0]["description"])
	assert.EqualValues(t, 2, out[0]["items"])
}

func TestListItems(t *testing.T) {
	tests := []struct {
		name  string
		args  map[string]any
		want  []string
		total int
	}{
		{"vault by title, any case", map[string]any{"vault": "claude"}, []string{"MacBook Pro recovery FileVault code", "GitHub", "Twin", "twin"}, 4},
		{"vault by id", map[string]any{"vault": vaultID}, []string{"MacBook Pro recovery FileVault code", "GitHub", "Twin", "twin"}, 4},
		{"query matches title", map[string]any{"vault": "Claude", "query": "filevault"}, []string{"MacBook Pro recovery FileVault code"}, 1},
		{"query matches url", map[string]any{"vault": "Claude", "query": "github.com"}, []string{"GitHub"}, 1},
		{"query matches tag", map[string]any{"vault": "Claude", "query": "MAC"}, []string{"MacBook Pro recovery FileVault code"}, 1},
		{"category", map[string]any{"vault": "Claude", "category": "secure_note"}, []string{"Twin", "twin"}, 2},
		{"limit", map[string]any{"vault": "Claude", "limit": 1}, []string{"MacBook Pro recovery FileVault code"}, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := decode(t, run(t, newTestAdapter(t, newFake()), "onepassword_list_items", tc.args))
			var titles []string
			for _, it := range out["items"].([]any) {
				titles = append(titles, it.(map[string]any)["title"].(string))
			}
			assert.Equal(t, tc.want, titles)
			assert.EqualValues(t, tc.total, out["total"])
			assert.EqualValues(t, len(tc.want), out["count"])
		})
	}
}

func TestListItems_Rejects(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing vault", map[string]any{}, "vault is required"},
		{"unknown vault", map[string]any{"vault": "Nope"}, `vault "Nope" not found; vaults: Claude, Shared`},
		{"bad category", map[string]any{"vault": "Claude", "category": "Spaceship"}, "unknown category"},
		{"bad limit", map[string]any{"vault": "Claude", "limit": 0}, "limit must be between 1 and 1000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, newTestAdapter(t, newFake()), "onepassword_list_items", tc.args)
			assert.True(t, res.IsError)
			assert.Contains(t, res.Data, tc.want)
		})
	}
}

func TestGetItem_RedactsConcealedValues(t *testing.T) {
	res := run(t, newTestAdapter(t, newFake()), "onepassword_get_item", map[string]any{"vault": "Claude", "item": "GitHub"})
	assert.NotContains(t, res.Data, secret)
	assert.NotContains(t, res.Data, "123456", "one-time codes are secrets too")
	out := decode(t, res)
	assert.Equal(t, "work account", out["notes"])
	assert.EqualValues(t, 7, out["version"])
	assert.Equal(t, []any{"https://github.com"}, out["urls"])
	fields := map[string]map[string]any{}
	for _, f := range out["fields"].([]any) {
		m := f.(map[string]any)
		fields[m["id"].(string)] = m
	}
	assert.Equal(t, "octocat", fields["username"]["value"])
	assert.Equal(t, "[REDACTED]", fields["password"]["value"])
	assert.Equal(t, true, fields["password"]["concealed"])
	assert.Equal(t, "[REDACTED]", fields["otp"]["value"])
	assert.Equal(t, "[REDACTED]", fields["otp"]["otp_code"], "TOTP fields always carry otp_code, redacted until revealed")
	assert.NotContains(t, fields["password"], "otp_code")
	assert.Equal(t, "Security", fields["otp"]["section"])
	assert.Equal(t, "0000", fields["pin"]["value"])
	assert.Equal(t, "op://"+vaultID+"/loginloginloginloginloginl/password", fields["password"]["reference"])
	assert.Equal(t, "op://"+vaultID+"/loginloginloginloginloginl/sec1/pin", fields["pin"]["reference"])
}

func TestGetItem_Reveal(t *testing.T) {
	out := decode(t, run(t, newTestAdapter(t, newFake()), "onepassword_get_item", map[string]any{"vault": "Claude", "item": "GitHub", "reveal": true}))
	fields := map[string]map[string]any{}
	for _, f := range out["fields"].([]any) {
		m := f.(map[string]any)
		fields[m["id"].(string)] = m
	}
	assert.Equal(t, secret, fields["password"]["value"])
	assert.Equal(t, "123456", fields["otp"]["otp_code"])
}

func TestGetItem_ByID(t *testing.T) {
	out := decode(t, run(t, newTestAdapter(t, newFake()), "onepassword_get_item", map[string]any{"vault": vaultID, "item": itemID}))
	assert.Equal(t, "MacBook Pro recovery FileVault code", out["title"])
	assert.Equal(t, "Claude", out["vault"])
}

func TestGetItem_Rejects(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing item", map[string]any{"vault": "Claude"}, "item is required"},
		{"unknown item", map[string]any{"vault": "Claude", "item": "Nope"}, `item "Nope" not found in vault Claude`},
		{"ambiguous item", map[string]any{"vault": "Claude", "item": "TWIN"}, "dupdupdupdupdupdupdupdupd1, dupdupdupdupdupdupdupdupd2"},
		{"bad reveal", map[string]any{"vault": "Claude", "item": "GitHub", "reveal": "maybe"}, "reveal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, newTestAdapter(t, newFake()), "onepassword_get_item", tc.args)
			assert.True(t, res.IsError)
			assert.Contains(t, res.Data, tc.want)
		})
	}
}

func TestCreateItem_Login(t *testing.T) {
	f := newFake()
	res := run(t, newTestAdapter(t, f), "onepassword_create_item", map[string]any{
		"vault": "Claude", "title": "Test login", "username": "me", "password": secret,
		"url": "https://example.com", "notes": "hello", "tags": []any{"test"},
		"fields": []any{
			map[string]any{"title": "API key", "value": secret, "type": "concealed"},
			map[string]any{"title": "Region", "value": "us-east", "section": "Server"},
		},
	})
	assert.NotContains(t, res.Data, secret, "created secrets are not echoed")
	out := decode(t, res)
	assert.Equal(t, "Test login", out["title"])
	require.NotNil(t, f.created)
	p := f.created
	assert.Equal(t, op.ItemCategoryLogin, p.Category)
	assert.Equal(t, vaultID, p.VaultID)
	assert.Equal(t, "hello", *p.Notes)
	assert.Equal(t, []string{"test"}, p.Tags)
	assert.Equal(t, []op.Website{{URL: "https://example.com", Label: "website", AutofillBehavior: op.AutofillBehaviorAnywhereOnWebsite}}, p.Websites)
	require.Len(t, p.Fields, 4)
	assert.Equal(t, op.ItemField{ID: "username", Title: "username", FieldType: op.ItemFieldTypeText, Value: "me"}, p.Fields[0])
	assert.Equal(t, op.ItemField{ID: "password", Title: "password", FieldType: op.ItemFieldTypeConcealed, Value: secret}, p.Fields[1])
	assert.Equal(t, "API key", p.Fields[2].Title)
	assert.Equal(t, op.ItemFieldTypeConcealed, p.Fields[2].FieldType)
	assert.NotEmpty(t, p.Fields[2].ID)
	require.NotNil(t, p.Fields[2].SectionID)
	assert.Equal(t, "Region", p.Fields[3].Title)
	assert.Equal(t, op.ItemFieldTypeText, p.Fields[3].FieldType)
	require.NotNil(t, p.Fields[3].SectionID)
	assert.NotEqual(t, *p.Fields[2].SectionID, *p.Fields[3].SectionID)
	sections := map[string]string{}
	for _, s := range p.Sections {
		sections[s.ID] = s.Title
	}
	assert.Equal(t, "", sections[*p.Fields[2].SectionID], "unsectioned extra fields share an untitled section")
	assert.Equal(t, "Server", sections[*p.Fields[3].SectionID])
}

func TestCreateItem_GeneratePassword(t *testing.T) {
	f := newFake()
	res := run(t, newTestAdapter(t, f), "onepassword_create_item", map[string]any{"vault": "Claude", "title": "Gen", "category": "password", "generate_password": true})
	require.False(t, res.IsError, res.Data)
	assert.NotContains(t, res.Data, "generated-")
	assert.Equal(t, 1, f.generated)
	assert.Equal(t, op.ItemCategoryPassword, f.created.Category)
	require.Len(t, f.created.Fields, 1)
	assert.Equal(t, "generated-"+secret, f.created.Fields[0].Value)
	assert.Equal(t, op.ItemFieldTypeConcealed, f.created.Fields[0].FieldType)
}

func TestCreateItem_SecureNote(t *testing.T) {
	f := newFake()
	res := run(t, newTestAdapter(t, f), "onepassword_create_item", map[string]any{"vault": "Claude", "title": "Note", "category": "Secure Note", "notes": "text"})
	require.False(t, res.IsError, res.Data)
	assert.Equal(t, op.ItemCategorySecureNote, f.created.Category)
	assert.Empty(t, f.created.Fields)
}

func TestCreateItem_Rejects(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing title", map[string]any{"vault": "Claude"}, "title is required"},
		{"unknown category", map[string]any{"vault": "Claude", "title": "x", "category": "Spaceship"}, "unknown category"},
		{"document needs a file", map[string]any{"vault": "Claude", "title": "x", "category": "Document"}, "unknown category"},
		{"password on a note", map[string]any{"vault": "Claude", "title": "x", "category": "SecureNote", "password": "p"}, "password applies to Login and Password items"},
		{"username on a password", map[string]any{"vault": "Claude", "title": "x", "category": "Password", "username": "u"}, "username applies to Login items"},
		{"url on a note", map[string]any{"vault": "Claude", "title": "x", "category": "SecureNote", "url": "https://x"}, "url applies to Login and Password items"},
		{"password and generate", map[string]any{"vault": "Claude", "title": "x", "password": "p", "generate_password": true}, "either password or generate_password"},
		{"bad field type", map[string]any{"vault": "Claude", "title": "x", "fields": []any{map[string]any{"title": "a", "value": "b", "type": "hologram"}}}, `unknown field type "hologram"`},
		{"field without title", map[string]any{"vault": "Claude", "title": "x", "fields": []any{map[string]any{"value": "b"}}}, "fields[0] needs a title"},
		{"fields not a list", map[string]any{"vault": "Claude", "title": "x", "fields": "nope"}, "fields must be an array"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			res := run(t, newTestAdapter(t, f), "onepassword_create_item", tc.args)
			assert.True(t, res.IsError)
			assert.Contains(t, res.Data, tc.want)
			assert.Nil(t, f.created)
		})
	}
}

func TestCreateItem_FieldsAsJSONString(t *testing.T) {
	f := newFake()
	res := run(t, newTestAdapter(t, f), "onepassword_create_item", map[string]any{"vault": "Claude", "title": "x", "category": "SecureNote", "fields": `[{"title":"a","value":"b"}]`})
	require.False(t, res.IsError, res.Data)
	require.Len(t, f.created.Fields, 1)
	assert.Equal(t, "b", f.created.Fields[0].Value)
}

func TestUpdateItem(t *testing.T) {
	f := newFake()
	res := run(t, newTestAdapter(t, f), "onepassword_update_item", map[string]any{
		"vault": "Claude", "item": "GitHub", "title": "GitHub (work)", "password": "new-" + secret, "notes": "changed", "tags": []any{"work"},
		"fields": []any{
			map[string]any{"title": "recovery pin", "value": "1234"},
			map[string]any{"id": "username", "value": "hubot"},
			map[string]any{"title": "Backup code", "value": "abc", "type": "concealed", "section": "Security"},
			map[string]any{"title": "Owner", "value": "ops"},
		},
	})
	assert.NotContains(t, res.Data, secret)
	out := decode(t, res)
	assert.EqualValues(t, 8, out["version"])
	require.NotNil(t, f.put)
	it := f.put
	assert.EqualValues(t, 7, it.Version, "the version read is sent back so concurrent edits are detected")
	assert.Equal(t, "GitHub (work)", it.Title)
	assert.Equal(t, "changed", it.Notes)
	assert.Equal(t, []string{"work"}, it.Tags)
	byTitle := map[string]op.ItemField{}
	for _, fl := range it.Fields {
		byTitle[fl.Title] = fl
	}
	assert.Equal(t, "hubot", byTitle["username"].Value)
	assert.Equal(t, "new-"+secret, byTitle["password"].Value)
	assert.Equal(t, "1234", byTitle["Recovery PIN"].Value, "matched by title, any case, keeping its title")
	assert.Equal(t, op.ItemFieldTypeTOTP, byTitle["one-time password"].FieldType, "untouched fields stay")
	require.Contains(t, byTitle, "Backup code")
	assert.Equal(t, "sec1", *byTitle["Backup code"].SectionID, "added to the existing section")
	assert.Equal(t, op.ItemFieldTypeConcealed, byTitle["Backup code"].FieldType)
	require.Contains(t, byTitle, "Owner")
	require.NotNil(t, byTitle["Owner"].SectionID)
	assert.Len(t, it.Fields, 6)
}

func TestUpdateItem_GeneratePasswordAddsMissingField(t *testing.T) {
	f := newFake()
	f.full[itemID] = op.Item{ID: itemID, Title: "Bare", Category: op.ItemCategoryPassword, VaultID: vaultID, Version: 1}
	res := run(t, newTestAdapter(t, f), "onepassword_update_item", map[string]any{"vault": "Claude", "item": itemID, "generate_password": true})
	require.False(t, res.IsError, res.Data)
	require.Len(t, f.put.Fields, 1)
	assert.Equal(t, "password", f.put.Fields[0].ID)
	assert.Equal(t, op.ItemFieldTypeConcealed, f.put.Fields[0].FieldType)
	assert.Equal(t, "generated-"+secret, f.put.Fields[0].Value)
}

func TestUpdateItem_PasswordAlwaysConcealed(t *testing.T) {
	sect := "sec1"
	tests := []struct {
		name   string
		fields []op.ItemField
	}{
		{"built-in stored as text", []op.ItemField{{ID: "password", Title: "password", FieldType: op.ItemFieldTypeText, Value: "old"}}},
		{"custom text field holds the id", []op.ItemField{{ID: "password", Title: "password", SectionID: &sect, FieldType: op.ItemFieldTypeText, Value: "note"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			f.full[itemID] = op.Item{ID: itemID, Title: "Bare", Category: op.ItemCategoryPassword, VaultID: vaultID, Version: 1,
				Sections: []op.ItemSection{{ID: sect, Title: "Notes"}}, Fields: tc.fields}
			res := run(t, newTestAdapter(t, f), "onepassword_update_item", map[string]any{"vault": "Claude", "item": itemID, "password": secret})
			require.False(t, res.IsError, res.Data)
			assert.NotContains(t, res.Data, secret)
			for _, fl := range f.put.Fields {
				if fl.Value == secret {
					assert.Nil(t, fl.SectionID, "the password goes in the built-in field, not a custom one")
					assert.Equal(t, op.ItemFieldTypeConcealed, fl.FieldType)
				}
			}
		})
	}
}

func TestUpdateItem_CustomFieldNeverTakesBuiltInID(t *testing.T) {
	f := newFake()
	f.full[itemID] = op.Item{ID: itemID, Title: "Bare", Category: op.ItemCategoryPassword, VaultID: vaultID, Version: 1}
	res := run(t, newTestAdapter(t, f), "onepassword_update_item", map[string]any{"vault": "Claude", "item": itemID,
		"fields": []any{map[string]any{"title": "Password", "value": "x"}, map[string]any{"title": "Username", "value": "y"}}})
	require.False(t, res.IsError, res.Data)
	for _, fl := range f.put.Fields {
		assert.NotContains(t, []string{"password", "username"}, fl.ID)
	}
}

func TestUpdateItem_Rejects(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"nothing to change", map[string]any{"vault": "Claude", "item": "GitHub"}, "nothing to update"},
		{"username on a password item", map[string]any{"vault": "Claude", "item": itemID, "username": "u"}, "username applies to Login items"},
		{"field without id or title", map[string]any{"vault": "Claude", "item": "GitHub", "fields": []any{map[string]any{"value": "x"}}}, "fields[0] needs an id or a title"},
		{"unknown field id", map[string]any{"vault": "Claude", "item": "GitHub", "fields": []any{map[string]any{"id": "nope", "value": "x"}}}, `no field with id "nope"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			res := run(t, newTestAdapter(t, f), "onepassword_update_item", tc.args)
			assert.True(t, res.IsError)
			assert.Contains(t, res.Data, tc.want)
			assert.Nil(t, f.put)
		})
	}
}

func TestResolveSecret(t *testing.T) {
	out := decode(t, run(t, newTestAdapter(t, newFake()), "onepassword_resolve_secret", map[string]any{"reference": "op://Claude/GitHub/password"}))
	assert.Equal(t, secret, out["value"])
	assert.Equal(t, "op://Claude/GitHub/password", out["reference"])
}

func TestResolveSecret_Rejects(t *testing.T) {
	tests := []struct {
		name, ref, want string
	}{
		{"missing", "", "reference is required"},
		{"not a reference", "Claude/GitHub/password", "must start with op://"},
		{"no match", "op://Claude/Nope/password", "no item matched"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, newTestAdapter(t, newFake()), "onepassword_resolve_secret", map[string]any{"reference": tc.ref})
			assert.True(t, res.IsError)
			assert.Contains(t, res.Data, tc.want)
		})
	}
}

func TestErrors_RateLimitIsRetryable(t *testing.T) {
	f := newFake()
	f.err = &op.RateLimitExceededError{}
	_, err := newTestAdapter(t, f).Execute(context.Background(), "onepassword_list_vaults", nil)
	var retry *mcp.RetryableError
	require.ErrorAs(t, err, &retry)
	assert.Equal(t, 429, retry.StatusCode)
}

func TestErrors_StoreFailureIsReported(t *testing.T) {
	f := newFake()
	f.err = errors.New("permission denied")
	res := run(t, newTestAdapter(t, f), "onepassword_list_vaults", nil)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "onepassword: permission denied")
}
