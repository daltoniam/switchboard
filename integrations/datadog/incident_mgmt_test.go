package datadog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIncidentID = "7f2a3b4c-5d6e-4f70-8a9b-0c1d2e3f4a5b"
	testUserID     = "00000000-1111-2222-3333-444444444444"
)

// setupIncidentDD is setupDD with the unstable incident operations enabled,
// matching what Configure does in production.
func setupIncidentDD(t *testing.T, ts *httptest.Server) *dd {
	t.Helper()
	cfg := datadog.NewConfiguration()
	cfg.Servers = datadog.ServerConfigurations{{URL: ts.URL}}
	cfg.HTTPClient = ts.Client()
	enableUnstableOperations(cfg)
	return &dd{apiKey: "test-api-key", appKey: "test-app-key", client: datadog.NewAPIClient(cfg)}
}

func TestUnstableOperations_IncludeNewIncidentOps(t *testing.T) {
	for _, op := range []string{
		"v2.GetIncidentTodo", "v2.CreateIncidentTodo", "v2.UpdateIncidentTodo", "v2.DeleteIncidentTodo",
		"v2.ListIncidentIntegrations", "v2.GetIncidentIntegration", "v2.CreateIncidentAttachment", "v2.ListIncidentTypes",
	} {
		assert.Contains(t, unstableOperations, op)
	}
}

func TestResolveIncidentID(t *testing.T) {
	list := `{"data":[` +
		`{"id":"aaaaaaaa-0000-0000-0000-000000000001","type":"incidents","attributes":{"public_id":95,"title":"other"}},` +
		`{"id":"` + testIncidentID + `","type":"incidents","attributes":{"public_id":94,"title":"slow API times"}}]}`
	cases := []struct {
		name     string
		input    string
		want     string
		wantErr  string
		requests int
	}{
		{"uuid passthrough", testIncidentID, testIncidentID, "", 0},
		{"plain number", "94", testIncidentID, "", 1},
		{"IR prefix", "IR-94", testIncidentID, "", 1},
		{"lowercase ir", "ir-94", testIncidentID, "", 1},
		{"hash", "#94", testIncidentID, "", 1},
		{"channel style", "incident-94", testIncidentID, "", 1},
		{"not found", "IR-12", "", "no incident with public id 12", 1},
		{"other id passthrough", "abc", "abc", "", 0},
		{"empty", " ", "", "incident id is required", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				assert.Equal(t, "/api/v2/incidents", r.URL.Path)
				assert.Equal(t, "100", r.URL.Query().Get("page[size]"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(list))
			}))
			defer ts.Close()
			d := setupIncidentDD(t, ts)
			got, err := resolveIncidentID(d.ctx(context.Background()), d, tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
			assert.Equal(t, tc.requests, requests)
		})
	}
}

func TestGetIncident_ByPublicID(t *testing.T) {
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v2/incidents" {
			_, _ = w.Write([]byte(`{"data":[{"id":"` + testIncidentID + `","type":"incidents","attributes":{"public_id":94,"title":"t"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"` + testIncidentID + `","type":"incidents","attributes":{"public_id":94,"title":"t"}}}`))
	}))
	defer ts.Close()
	d := setupIncidentDD(t, ts)
	result, err := getIncident(d.ctx(context.Background()), d, map[string]any{"id": "IR-94"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, []string{"/api/v2/incidents", "/api/v2/incidents/" + testIncidentID}, paths)
}

func TestUpdateIncident_FullBody(t *testing.T) {
	ts, got := captureServer(t, 200, `{"data":{"id":"`+testIncidentID+`","type":"incidents"}}`)
	d := setupIncidentDD(t, ts)
	result, err := updateIncident(d.ctx(context.Background()), d, map[string]any{
		"id":                    testIncidentID,
		"severity":              "SEV-2",
		"status":                "stable",
		"customer_impacted":     true,
		"customer_impact_scope": "Admin and app pages loaded slowly",
		"customer_impact_start": "2026-10-23T18:52:00Z",
		"customer_impact_end":   "2026-10-23T20:05:00Z",
		"commander_user_id":     testUserID,
		"fields": map[string]any{
			"root_cause": "Cloudflare closed origin connections",
			"services":   []any{"curri-api", "curri-driver-service"},
			"summary":    map[string]any{"type": "textbox", "value": "Cloudflare incident"},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, http.MethodPatch, got.Method)
	assert.Equal(t, "/api/v2/incidents/"+testIncidentID, got.Path)

	data := got.Body["data"].(map[string]any)
	attrs := data["attributes"].(map[string]any)
	assert.Equal(t, true, attrs["customer_impacted"])
	assert.Equal(t, "Admin and app pages loaded slowly", attrs["customer_impact_scope"])
	assert.Equal(t, "2026-10-23T18:52:00Z", attrs["customer_impact_start"])
	assert.Equal(t, "2026-10-23T20:05:00Z", attrs["customer_impact_end"])

	fields := attrs["fields"].(map[string]any)
	assert.Equal(t, map[string]any{"type": "dropdown", "value": "SEV-2"}, fields["severity"])
	assert.Equal(t, map[string]any{"type": "dropdown", "value": "stable"}, fields["state"])
	assert.Equal(t, map[string]any{"value": "Cloudflare closed origin connections"}, fields["root_cause"])
	assert.Equal(t, map[string]any{"value": []any{"curri-api", "curri-driver-service"}}, fields["services"])
	assert.Equal(t, map[string]any{"type": "textbox", "value": "Cloudflare incident"}, fields["summary"])

	rel := data["relationships"].(map[string]any)
	commander := rel["commander_user"].(map[string]any)["data"].(map[string]any)
	assert.Equal(t, testUserID, commander["id"])
	assert.Equal(t, "users", commander["type"])
}

func TestUpdateIncident_Validation(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"nothing", map[string]any{"id": testIncidentID}, "nothing to update"},
		{"missing id", map[string]any{"title": "x"}, `"id" is required`},
		{"bad commander", map[string]any{"id": testIncidentID, "commander_user_id": "alice@curri.com"}, "not a Datadog user UUID"},
		{"bad time", map[string]any{"id": testIncidentID, "detected": "yesterday"}, "RFC3339"},
		{"bad field", map[string]any{"id": testIncidentID, "fields": map[string]any{"n": float64(1)}}, "string or an array"},
		{"bad field type", map[string]any{"id": testIncidentID, "fields": map[string]any{"n": map[string]any{"type": "multiselect", "value": "x"}}}, "dropdown or textbox"},
		{"bad list type", map[string]any{"id": testIncidentID, "fields": map[string]any{"n": map[string]any{"type": "textbox", "value": []any{"x"}}}}, "multiselect, textarray"},
		{"bad field element", map[string]any{"id": testIncidentID, "fields": map[string]any{"n": []any{float64(1)}}}, "not string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, got := captureServer(t, 200, `{}`)
			d := setupIncidentDD(t, ts)
			result, err := updateIncident(d.ctx(context.Background()), d, tc.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.want)
			assert.Empty(t, got.Method)
		})
	}
}

func TestUpdateIncident_TitleOnlyKeepsLegacyBehavior(t *testing.T) {
	ts, got := captureServer(t, 200, `{"data":{"id":"`+testIncidentID+`","type":"incidents"}}`)
	d := setupIncidentDD(t, ts)
	result, err := updateIncident(d.ctx(context.Background()), d, map[string]any{"id": testIncidentID, "title": "New"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	data := got.Body["data"].(map[string]any)
	attrs := data["attributes"].(map[string]any)
	assert.Equal(t, "New", attrs["title"])
	_, hasFields := attrs["fields"]
	assert.False(t, hasFields)
	_, hasRel := data["relationships"]
	assert.False(t, hasRel)
}

func TestIncidentTodos(t *testing.T) {
	todoPath := "/api/v2/incidents/" + testIncidentID + "/relationships/todos"
	t.Run("create", func(t *testing.T) {
		ts, got := captureServer(t, 201, `{"data":{"id":"t1","type":"incident_todos","attributes":{"content":"x","assignees":[]}}}`)
		d := setupIncidentDD(t, ts)
		result, err := createIncidentTodo(d.ctx(context.Background()), d, map[string]any{
			"incident_id": testIncidentID,
			"content":     "Add a Cloudflare edge monitor",
			"assignees":   "alice@curri.com, bob@curri.com",
			"due_date":    "2026-10-30",
		})
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		assert.Equal(t, http.MethodPost, got.Method)
		assert.Equal(t, todoPath, got.Path)
		attrs := got.Body["data"].(map[string]any)["attributes"].(map[string]any)
		assert.Equal(t, "Add a Cloudflare edge monitor", attrs["content"])
		assert.Equal(t, []any{"alice@curri.com", "bob@curri.com"}, attrs["assignees"])
		assert.Equal(t, "2026-10-30", attrs["due_date"])
	})
	t.Run("create requires content", func(t *testing.T) {
		ts, got := captureServer(t, 201, `{}`)
		d := setupIncidentDD(t, ts)
		result, err := createIncidentTodo(d.ctx(context.Background()), d, map[string]any{"incident_id": testIncidentID})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Empty(t, got.Method)
	})
	t.Run("update", func(t *testing.T) {
		ts, got := captureServer(t, 200, `{"data":{"id":"t1","type":"incident_todos","attributes":{"content":"x","assignees":[]}}}`)
		d := setupIncidentDD(t, ts)
		result, err := updateIncidentTodo(d.ctx(context.Background()), d, map[string]any{
			"incident_id": testIncidentID, "todo_id": "t1", "content": "x", "completed": "2026-10-24T00:00:00Z",
		})
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		assert.Equal(t, http.MethodPatch, got.Method)
		assert.Equal(t, todoPath+"/t1", got.Path)
		attrs := got.Body["data"].(map[string]any)["attributes"].(map[string]any)
		assert.Equal(t, "2026-10-24T00:00:00Z", attrs["completed"])
	})
	t.Run("get", func(t *testing.T) {
		ts, got := captureServer(t, 200, `{"data":{"id":"t1","type":"incident_todos","attributes":{"content":"x","assignees":[]}}}`)
		d := setupIncidentDD(t, ts)
		result, err := getIncidentTodo(d.ctx(context.Background()), d, map[string]any{"incident_id": testIncidentID, "todo_id": "t1"})
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		assert.Equal(t, todoPath+"/t1", got.Path)
	})
	t.Run("delete", func(t *testing.T) {
		ts, got := captureServer(t, 204, ``)
		d := setupIncidentDD(t, ts)
		result, err := deleteIncidentTodo(d.ctx(context.Background()), d, map[string]any{"incident_id": testIncidentID, "todo_id": "t1"})
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		assert.Equal(t, http.MethodDelete, got.Method)
		assert.Equal(t, todoPath+"/t1", got.Path)
	})
}

func TestIncidentIntegrationsAndTypes(t *testing.T) {
	cases := []struct {
		name    string
		handler handlerFunc
		args    map[string]any
		path    string
	}{
		{"list integrations", listIncidentIntegrations, map[string]any{"incident_id": testIncidentID}, "/api/v2/incidents/" + testIncidentID + "/relationships/integrations"},
		{"get integration", getIncidentIntegration, map[string]any{"incident_id": testIncidentID, "integration_id": "i1"}, "/api/v2/incidents/" + testIncidentID + "/relationships/integrations/i1"},
		{"list impacts", listIncidentImpacts, map[string]any{"incident_id": testIncidentID}, "/api/v2/incidents/" + testIncidentID + "/impacts"},
		{"list types", listIncidentTypes, map[string]any{}, "/api/v2/incidents/config/types"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, got := captureServer(t, 200, `{"data":[]}`)
			d := setupIncidentDD(t, ts)
			result, err := tc.handler(d.ctx(context.Background()), d, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, http.MethodGet, got.Method)
			assert.Equal(t, tc.path, got.Path)
		})
	}
}

func TestCreateIncidentImpact(t *testing.T) {
	ts, got := captureServer(t, 201, `{"data":{"id":"imp1","type":"incident_impacts"}}`)
	d := setupIncidentDD(t, ts)
	result, err := createIncidentImpact(d.ctx(context.Background()), d, map[string]any{
		"incident_id": testIncidentID,
		"description": "Order integrations failed",
		"start_at":    "2026-10-23T18:52:00Z",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, http.MethodPost, got.Method)
	attrs := got.Body["data"].(map[string]any)["attributes"].(map[string]any)
	assert.Equal(t, "Order integrations failed", attrs["description"])
	assert.Equal(t, "2026-10-23T18:52:00Z", attrs["start_at"])

	ts2, got2 := captureServer(t, 201, `{}`)
	d2 := setupIncidentDD(t, ts2)
	result, err = createIncidentImpact(d2.ctx(context.Background()), d2, map[string]any{"incident_id": testIncidentID, "description": "x"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, `"start_at" is required`)
	assert.Empty(t, got2.Method)
}

func TestCreateIncidentAttachment(t *testing.T) {
	ts, got := captureServer(t, 201, `{"data":{"id":"a1","type":"incident_attachments"}}`)
	d := setupIncidentDD(t, ts)
	result, err := createIncidentAttachment(d.ctx(context.Background()), d, map[string]any{
		"incident_id": testIncidentID,
		"url":         "https://status.curri.com/" + testDegradationID,
		"title":       "Status page",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "/api/v2/incidents/"+testIncidentID+"/attachments", got.Path)
	attrs := got.Body["data"].(map[string]any)["attributes"].(map[string]any)
	assert.Equal(t, "link", attrs["attachment_type"])
	attachment := attrs["attachment"].(map[string]any)
	assert.Equal(t, "https://status.curri.com/"+testDegradationID, attachment["documentUrl"])
	assert.Equal(t, "Status page", attachment["title"])

	ts2, got2 := captureServer(t, 201, `{}`)
	d2 := setupIncidentDD(t, ts2)
	result, err = createIncidentAttachment(d2.ctx(context.Background()), d2, map[string]any{
		"incident_id": testIncidentID, "url": "https://x", "attachment_type": "file",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "link or postmortem")
	assert.Empty(t, got2.Method)
}
