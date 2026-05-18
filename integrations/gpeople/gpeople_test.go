package gpeople

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Constructor / config ────────────────────────────────────────────

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gpeople", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "ya29.test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := &gpeople{client: &http.Client{}, baseURL: "https://people.googleapis.com/v1"}
	err := g.Configure(context.Background(), mcp.Credentials{
		"access_token": "ya29.test",
		"base_url":     "https://custom.example.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", g.baseURL)
}

// ── Tools metadata ──────────────────────────────────────────────────

func TestTools(t *testing.T) {
	i := New()
	defs := i.Tools()
	assert.NotEmpty(t, defs)
	for _, tool := range defs {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGpeoplePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gpeople_", "tool %s missing gpeople_ prefix", tool.Name)
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

func TestTools_EntryPointHasStartHere(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		if tool.Name == mcp.ToolName("gpeople_list_contacts") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gpeople_list_contacts must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gpeople_list_contacts tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gpeople{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gpeople_nonexistent", nil)
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

// ── Compaction parity ──────────────────────────────────────────────

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", name)
	}
}

// ── HTTP helpers ────────────────────────────────────────────────────

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"connections":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/people/me/connections?personFields=names")
	require.NoError(t, err)
	assert.Contains(t, string(data), "connections")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/people/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gpeople API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.delete(context.Background(), "/people/c123:deleteContact")
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_5xxIsRetryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"message":"Service Unavailable"}}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/people/me/connections")
	require.Error(t, err)
	re, ok := err.(*mcp.RetryableError)
	require.True(t, ok, "expected mcp.RetryableError")
	assert.Equal(t, 503, re.StatusCode)
}

// ── Handler: listContacts ───────────────────────────────────────────

func TestListContacts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/people/me/connections", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "names,emailAddresses", q.Get("personFields"))
		assert.Equal(t, "50", q.Get("pageSize"))
		assert.Equal(t, "tok-2", q.Get("pageToken"))
		assert.Equal(t, "LAST_NAME_ASCENDING", q.Get("sortOrder"))
		_, _ = w.Write([]byte(`{"connections":[{"resourceName":"people/c1","names":[{"displayName":"Jane"}]}]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_list_contacts", map[string]any{
		"person_fields": "names,emailAddresses",
		"page_size":     50,
		"page_token":    "tok-2",
		"sort_order":    "LAST_NAME_ASCENDING",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Jane")
}

func TestListContacts_DefaultPersonFields(t *testing.T) {
	var seenFields string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenFields = r.URL.Query().Get("personFields")
		_, _ = w.Write([]byte(`{"connections":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gpeople_list_contacts", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, defaultPersonFields, seenFields, "list_contacts should default to defaultPersonFields")
}

// ── Handler: searchContacts ─────────────────────────────────────────

func TestSearchContacts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/people:searchContacts", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "jane", q.Get("query"))
		assert.Equal(t, "names,emailAddresses", q.Get("readMask"))
		assert.Equal(t, "10", q.Get("pageSize"))
		_, _ = w.Write([]byte(`{"results":[{"person":{"resourceName":"people/c1","names":[{"displayName":"Jane Doe"}]}}]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_search_contacts", map[string]any{
		"query":     "jane",
		"read_mask": "names,emailAddresses",
		"page_size": 10,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Jane Doe")
}

func TestSearchContacts_MissingQuery(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gpeople_search_contacts", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "query is required")
}

// ── Handler: getPerson ──────────────────────────────────────────────

func TestGetPerson(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/people/c12345", r.URL.Path)
		assert.Equal(t, "names,emailAddresses", r.URL.Query().Get("personFields"))
		_, _ = w.Write([]byte(`{"resourceName":"people/c12345","names":[{"displayName":"Jane"}]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_get_person", map[string]any{
		"resource_name": "c12345",
		"person_fields": "names,emailAddresses",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Jane")
}

func TestGetPerson_AcceptsResourceName(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gpeople_get_person", map[string]any{
		"resource_name": "people/c12345",
	})
	require.NoError(t, err)
	assert.Equal(t, "/people/c12345", seenPath)
}

func TestGetPerson_AcceptsMe(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gpeople_get_person", map[string]any{
		"resource_name": "me",
	})
	require.NoError(t, err)
	assert.Equal(t, "/people/me", seenPath)
}

func TestGetPerson_MissingResourceName(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gpeople_get_person", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "resource_name is required")
}

// ── Handler: createContact ──────────────────────────────────────────

func TestCreateContact_PersonAsString(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/people:createContact", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("personFields"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		names, ok := body["names"].([]any)
		require.True(t, ok)
		assert.Len(t, names, 1)
		_, _ = w.Write([]byte(`{"resourceName":"people/c777","names":[{"displayName":"Jane Doe"}]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_create_contact", map[string]any{
		"person": `{"names":[{"givenName":"Jane","familyName":"Doe"}],"emailAddresses":[{"value":"jane@example.com"}]}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "c777")
}

func TestCreateContact_PersonAsObject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body, "names")
		_, _ = w.Write([]byte(`{"resourceName":"people/c1"}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_create_contact", map[string]any{
		"person": map[string]any{
			"names": []any{map[string]any{"givenName": "Jane"}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateContact_InvalidPerson(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gpeople_create_contact", map[string]any{
		"person": "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON")
}

func TestCreateContact_MissingPerson(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gpeople_create_contact", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "person is required")
}

// ── Handler: updateContact ──────────────────────────────────────────

func TestUpdateContact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/people/c12345:updateContact", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "names,emailAddresses", q.Get("updatePersonFields"))
		assert.Equal(t, "names,emailAddresses,metadata", q.Get("personFields"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "etag-abc", body["etag"])
		_, _ = w.Write([]byte(`{"resourceName":"people/c12345","etag":"etag-new"}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_update_contact", map[string]any{
		"resource_name":        "c12345",
		"update_person_fields": "names,emailAddresses",
		"person_fields":        "names,emailAddresses,metadata",
		"person": map[string]any{
			"etag":  "etag-abc",
			"names": []any{map[string]any{"givenName": "Jane", "familyName": "Smith"}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "etag-new")
}

func TestUpdateContact_MissingArgs(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}

	r1, _ := g.Execute(context.Background(), "gpeople_update_contact", map[string]any{
		"update_person_fields": "names",
		"person":               map[string]any{"etag": "x"},
	})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "resource_name is required")

	r2, _ := g.Execute(context.Background(), "gpeople_update_contact", map[string]any{
		"resource_name": "c1",
		"person":        map[string]any{"etag": "x"},
	})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "update_person_fields is required")

	r3, _ := g.Execute(context.Background(), "gpeople_update_contact", map[string]any{
		"resource_name":        "c1",
		"update_person_fields": "names",
	})
	assert.True(t, r3.IsError)
	assert.Contains(t, r3.Data, "person is required")
}

// ── Handler: deleteContact ──────────────────────────────────────────

func TestDeleteContact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/people/c12345:deleteContact", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_delete_contact", map[string]any{
		"resource_name": "c12345",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteContact_MissingResourceName(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gpeople_delete_contact", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "resource_name is required")
}

// ── Handler: listDirectoryPeople ────────────────────────────────────

func TestListDirectoryPeople(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/people:listDirectoryPeople", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "names,emailAddresses", q.Get("readMask"))
		sources := q["sources"]
		assert.Contains(t, sources, "DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE")
		assert.Equal(t, "30", q.Get("pageSize"))
		assert.Equal(t, "pt-1", q.Get("pageToken"))
		_, _ = w.Write([]byte(`{"people":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_list_directory_people", map[string]any{
		"read_mask":  "names,emailAddresses",
		"sources":    "DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE",
		"page_size":  30,
		"page_token": "pt-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListDirectoryPeople_DefaultSources(t *testing.T) {
	var seenSources []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenSources = r.URL.Query()["sources"]
		_, _ = w.Write([]byte(`{"people":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gpeople_list_directory_people", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, seenSources, "DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT")
	assert.Contains(t, seenSources, "DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE")
}

// ── Handler: searchDirectoryPeople ──────────────────────────────────

func TestSearchDirectoryPeople(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/people:searchDirectoryPeople", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "alice", q.Get("query"))
		assert.NotEmpty(t, q.Get("readMask"))
		assert.NotEmpty(t, q["sources"])
		_, _ = w.Write([]byte(`{"people":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_search_directory_people", map[string]any{
		"query": "alice",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSearchDirectoryPeople_MissingQuery(t *testing.T) {
	g := &gpeople{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gpeople_search_directory_people", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "query is required")
}

// ── Handler: listOtherContacts ──────────────────────────────────────

func TestListOtherContacts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/otherContacts", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "names,emailAddresses", q.Get("readMask"))
		assert.Equal(t, "50", q.Get("pageSize"))
		_, _ = w.Write([]byte(`{"otherContacts":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gpeople_list_other_contacts", map[string]any{
		"read_mask": "names,emailAddresses",
		"page_size": 50,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"resourceName":"people/me","names":[]}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, g.Healthy(context.Background()))
}

// ── Path escaping ───────────────────────────────────────────────────

func TestResourceNameIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gpeople{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gpeople_get_person", map[string]any{
		"resource_name": "id with space",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "id%20with%20space") || strings.Contains(seenPath, "id+with+space"),
		"resource name with space should be URL-escaped; got %s", seenPath)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestNormalizeResourceName(t *testing.T) {
	assert.Equal(t, "people/c123", normalizeResourceName("c123"))
	assert.Equal(t, "people/c123", normalizeResourceName("people/c123"))
	assert.Equal(t, "people/me", normalizeResourceName("me"))
	assert.Equal(t, "people/me", normalizeResourceName("people/me"))
	assert.Equal(t, "", normalizeResourceName(""))
	assert.Equal(t, "people/c123", normalizeResourceName("  c123  "))
}

func TestParsePerson_String(t *testing.T) {
	got, err := parsePerson(`{"names":[{"givenName":"Jane"}]}`)
	require.NoError(t, err)
	require.NotNil(t, got)
	_, ok := got["names"]
	assert.True(t, ok)
}

func TestParsePerson_Invalid(t *testing.T) {
	_, err := parsePerson("not-json")
	assert.Error(t, err)
}

func TestParsePerson_EmptyString(t *testing.T) {
	got, err := parsePerson("")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParsePerson_Nil(t *testing.T) {
	got, err := parsePerson(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParsePerson_AlreadyParsed(t *testing.T) {
	in := map[string]any{"names": []any{map[string]any{"givenName": "Jane"}}}
	got, err := parsePerson(in)
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestParsePerson_EmptyMap(t *testing.T) {
	got, err := parsePerson(map[string]any{})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParsePerson_WrongType(t *testing.T) {
	_, err := parsePerson(123)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected JSON object or string")
}

// ── PlainTextKeys ──────────────────────────────────────────────────

func TestPlainTextKeys(t *testing.T) {
	g := &gpeople{}
	keys := g.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}
