package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	assert.Equal(t, "hubspot", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "pat-na1-test"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	h := &hubspot{client: &http.Client{}, baseURL: "https://api.hubapi.com"}
	err := h.Configure(context.Background(), mcp.Credentials{
		"access_token": "tok",
		"base_url":     "https://api.hubapi.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://api.hubapi.com", h.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tls := i.Tools()
	assert.NotEmpty(t, tls)
	for _, tool := range tls {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
	}
}

func TestTools_AllHaveHubSpotPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "hubspot_")
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
	found := false
	for _, tool := range i.Tools() {
		if tool.Name == "hubspot_search_contacts" {
			assert.Contains(t, tool.Description, "Start here")
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_UnknownTool(t *testing.T) {
	h := &hubspot{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := h.Execute(context.Background(), "hubspot_nonexistent", nil)
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

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "/crm/v3/objects/contacts", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"1"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := h.get(context.Background(), "/crm/v3/objects/contacts")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"status":"error","message":"unauthorized"}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	_, err := h.get(context.Background(), "/crm/v3/objects/contacts")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hubspot API error (401)")
}

func TestDoRequest_Retryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := h.get(context.Background(), "/crm/v3/objects/contacts")
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := h.doRequest(context.Background(), "DELETE", "/crm/v3/objects/contacts/1", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestSearchContacts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/crm/v3/objects/contacts/search", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		groups, ok := payload["filterGroups"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, groups)
		_, _ = w.Write([]byte(`{"total":1,"results":[{"id":"1","properties":{"email":"ada@example.com"}}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_search_contacts", map[string]any{
		"query": "ada@example.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "ada@example.com")
}

func TestSearchContacts_FiltersJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		groups := payload["filterGroups"].([]any)
		require.Len(t, groups, 1)
		_, _ = w.Write([]byte(`{"total":0,"results":[]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_search_contacts", map[string]any{
		"filters": `[{"propertyName":"email","operator":"EQ","value":"a@b.com"}]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListContacts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/objects/contacts", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Contains(t, r.URL.Query().Get("properties"), "email")
		_, _ = w.Write([]byte(`{"results":[{"id":"1"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_list_contacts", map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":"1"`)
}

func TestGetContact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/objects/contacts/1", r.URL.Path)
		assert.Equal(t, "email", r.URL.Query().Get("idProperty"))
		_, _ = w.Write([]byte(`{"id":"1","properties":{"email":"a@b.com"}}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_get_contact", map[string]any{
		"contact_id":  "1",
		"id_property": "email",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "a@b.com")
}

func TestCreateContact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/crm/v3/objects/contacts", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"email":"a@b.com"`)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"99","properties":{"email":"a@b.com"}}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_create_contact", map[string]any{
		"properties": `{"email":"a@b.com","firstname":"Ada"}`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"id":"99"`)
}

func TestCreateContact_InvalidJSON(t *testing.T) {
	h := &hubspot{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := h.Execute(context.Background(), "hubspot_create_contact", map[string]any{
		"properties": "{not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON")
}

func TestUpdateDeal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/crm/v3/objects/deals/d1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"d1","properties":{"dealstage":"closedwon"}}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_update_deal", map[string]any{
		"deal_id":    "d1",
		"properties": `{"dealstage":"closedwon"}`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "closedwon")
}

func TestDeleteObject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/crm/v3/objects/contacts/1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_delete_object", map[string]any{
		"object_type": "contacts",
		"id":          "1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestInvalidObjectType(t *testing.T) {
	h := &hubspot{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := h.Execute(context.Background(), "hubspot_list_objects", map[string]any{
		"object_type": "../contacts",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid object_type")
}

func TestListAssociations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/objects/contacts/1/associations/companies", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"9","type":"contact_to_company"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_list_associations", map[string]any{
		"from_object_type": "contacts",
		"from_object_id":   "1",
		"to_object_type":   "companies",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "contact_to_company")
}

func TestCreateAssociation_Default(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/crm/v4/objects/contacts/1/associations/default/companies/9", r.URL.Path)
		_, _ = w.Write([]byte(`{"fromObjectTypeId":"0-1","fromObjectId":1}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_create_association", map[string]any{
		"from_object_type": "contacts",
		"from_object_id":   "1",
		"to_object_type":   "companies",
		"to_object_id":     "9",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestCreateAssociation_Labeled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v4/objects/contacts/1/associations/companies/9", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"associationTypeId":279`)
		_, _ = w.Write([]byte(`{"status":"COMPLETE"}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_create_association", map[string]any{
		"from_object_type":    "contacts",
		"from_object_id":      "1",
		"to_object_type":      "companies",
		"to_object_id":        "9",
		"association_type_id": "279",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestListOwners(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/owners", r.URL.Path)
		assert.Equal(t, "a@b.com", r.URL.Query().Get("email"))
		_, _ = w.Write([]byte(`{"results":[{"id":"1","email":"a@b.com"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_list_owners", map[string]any{
		"email": "a@b.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "a@b.com")
}

func TestListPipelines(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/pipelines/deals", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"default","label":"Sales Pipeline"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_list_pipelines", map[string]any{
		"object_type": "deals",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Sales Pipeline")
}

func TestListProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/properties/contacts", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"name":"email","label":"Email"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_list_properties", map[string]any{
		"object_type": "contacts",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "email")
}

func TestSearchObjects_QueryUnsupported(t *testing.T) {
	h := &hubspot{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := h.Execute(context.Background(), "hubspot_search_objects", map[string]any{
		"object_type": "custom_thing",
		"query":       "acme",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "query is not supported")
}

func TestGetCompany(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/objects/companies/c1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"c1","properties":{"name":"Acme"}}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_get_company", map[string]any{
		"company_id": "c1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "Acme")
}

func TestListTickets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/crm/v3/objects/tickets", r.URL.Path)
		_, _ = w.Write([]byte(`{"results":[{"id":"t1"}]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := h.Execute(context.Background(), "hubspot_list_tickets", map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "t1")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/crm/v3/objects/contacts")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	h := &hubspot{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, h.Healthy(context.Background()))
}

func TestHealthy_Unconfigured(t *testing.T) {
	h := &hubspot{client: &http.Client{}, baseURL: "http://127.0.0.1:1"}
	assert.False(t, h.Healthy(context.Background()))
}

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

func TestPlaceholdersAndOptionalKeys(t *testing.T) {
	h := New().(*hubspot)
	assert.Contains(t, h.OptionalKeys(), "base_url")
	assert.Contains(t, h.PlainTextKeys(), "base_url")
	assert.Contains(t, h.Placeholders()["access_token"], "private app")
}

func TestMaxBytesUnknown(t *testing.T) {
	h := &hubspot{}
	_, ok := h.MaxBytes("hubspot_nonexistent")
	assert.False(t, ok)
}
