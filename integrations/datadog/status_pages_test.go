package datadog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPageID        = "1b94088e-a8cf-4350-ba3d-b24413f2d840"
	testComponentID   = "6c4b1f2e-0d8e-4b55-9a51-1f0c8f2c9a10"
	testDegradationID = "1345c83d-bbbe-4368-8be2-8a1e4df1f09c"
)

type capturedRequest struct {
	Method string
	Path   string
	Query  map[string]string
	Body   map[string]any
}

// captureServer records the last request and answers with body.
func captureServer(t *testing.T, status int, body string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	got := &capturedRequest{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.Path
		got.Query = map[string]string{}
		for k, v := range r.URL.Query() {
			got.Query[k] = v[0]
		}
		raw, _ := io.ReadAll(r.Body)
		got.Body = nil
		if len(raw) > 0 {
			require.NoError(t, json.Unmarshal(raw, &got.Body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, got
}

func TestListStatusPages(t *testing.T) {
	ts, got := captureServer(t, 200, `{"data":[{"id":"`+testPageID+`","type":"status_pages","attributes":{"name":"Curri"}}]}`)
	d := setupDD(t, ts)
	result, err := listStatusPages(d.ctx(context.Background()), d, map[string]any{"page_limit": float64(5)})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, http.MethodGet, got.Method)
	assert.Equal(t, "/api/v2/statuspages", got.Path)
	assert.Equal(t, "5", got.Query["page[limit]"])
	assert.Contains(t, result.Data, testPageID)
}

func TestStatusPageReadTools_Paths(t *testing.T) {
	cases := []struct {
		name    string
		handler handlerFunc
		args    map[string]any
		path    string
	}{
		{"get page", getStatusPage, map[string]any{"page_id": testPageID}, "/api/v2/statuspages/" + testPageID},
		{"list components", listStatusPageComponents, map[string]any{"page_id": testPageID}, "/api/v2/statuspages/" + testPageID + "/components"},
		{"get component", getStatusPageComponent, map[string]any{"page_id": testPageID, "component_id": testComponentID}, "/api/v2/statuspages/" + testPageID + "/components/" + testComponentID},
		{"get degradation", getStatusPageDegradation, map[string]any{"page_id": testPageID, "degradation_id": testDegradationID}, "/api/v2/statuspages/" + testPageID + "/degradations/" + testDegradationID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, got := captureServer(t, 200, `{"data":{"id":"x","type":"status_pages"}}`)
			d := setupDD(t, ts)
			result, err := tc.handler(d.ctx(context.Background()), d, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, http.MethodGet, got.Method)
			assert.Equal(t, tc.path, got.Path)
		})
	}
}

func TestStatusPageTools_RejectBadUUIDs(t *testing.T) {
	cases := []struct {
		name    string
		handler handlerFunc
		args    map[string]any
		want    string
	}{
		{"missing page", getStatusPage, map[string]any{}, `"page_id" is required`},
		{"bad page", getStatusPage, map[string]any{"page_id": "status.curri.com"}, "not a valid UUID"},
		{"bad component", getStatusPageComponent, map[string]any{"page_id": testPageID, "component_id": "api"}, "not a valid UUID"},
		{"bad degradation", updateStatusPageDegradation, map[string]any{"page_id": testPageID, "degradation_id": "nope", "status": "resolved"}, "not a valid UUID"},
		{"bad list filter", listStatusPageDegradations, map[string]any{"page_id": "nope"}, "not a valid UUID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, _ := captureServer(t, 500, `{}`)
			d := setupDD(t, ts)
			result, err := tc.handler(d.ctx(context.Background()), d, tc.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.want)
		})
	}
}

func TestListStatusPageDegradations_Filters(t *testing.T) {
	ts, got := captureServer(t, 200, `{"data":[]}`)
	d := setupDD(t, ts)
	result, err := listStatusPageDegradations(d.ctx(context.Background()), d, map[string]any{
		"page_id": testPageID,
		"status":  "investigating,identified",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "/api/v2/statuspages/degradations", got.Path)
	assert.Equal(t, testPageID, got.Query["filter[page_id]"])
	assert.Equal(t, "investigating,identified", got.Query["filter[status]"])
}

func TestListStatusPageDegradations_RejectsBadStatus(t *testing.T) {
	ts, _ := captureServer(t, 200, `{"data":[]}`)
	d := setupDD(t, ts)
	result, err := listStatusPageDegradations(d.ctx(context.Background()), d, map[string]any{"status": "down"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "investigating, identified, monitoring, resolved")
}

func TestCreateStatusPageDegradation_Body(t *testing.T) {
	for _, tc := range []struct {
		name       string
		components any
	}{
		{"decoded array", []any{map[string]any{"id": testComponentID, "status": "partial_outage", "name": "API"}}},
		{"json string", `[{"id":"` + testComponentID + `","status":"partial_outage","name":"API"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, got := captureServer(t, 201, `{"data":{"id":"`+testDegradationID+`","type":"degradations"}}`)
			d := setupDD(t, ts)
			result, err := createStatusPageDegradation(d.ctx(context.Background()), d, map[string]any{
				"page_id":             testPageID,
				"title":               "Slow API responses",
				"status":              "investigating",
				"description":         "Some pages are loading slowly. We are investigating.",
				"components_affected": tc.components,
				"notify_subscribers":  true,
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, http.MethodPost, got.Method)
			assert.Equal(t, "/api/v2/statuspages/"+testPageID+"/degradations", got.Path)
			assert.Equal(t, "true", got.Query["notify_subscribers"])
			data := got.Body["data"].(map[string]any)
			assert.Equal(t, "degradations", data["type"])
			attrs := data["attributes"].(map[string]any)
			assert.Equal(t, "Slow API responses", attrs["title"])
			assert.Equal(t, "investigating", attrs["status"])
			assert.Equal(t, "Some pages are loading slowly. We are investigating.", attrs["description"])
			comps := attrs["components_affected"].([]any)
			require.Len(t, comps, 1)
			comp := comps[0].(map[string]any)
			assert.Equal(t, testComponentID, comp["id"])
			assert.Equal(t, "partial_outage", comp["status"])
			assert.Equal(t, "API", comp["name"])
			assert.Contains(t, result.Data, testDegradationID)
		})
	}
}

func TestCreateStatusPageDegradation_Validation(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"page_id":             testPageID,
			"title":               "Outage",
			"status":              "investigating",
			"components_affected": []any{map[string]any{"id": testComponentID, "status": "major_outage"}},
			"notify_subscribers":  false,
		}
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"missing notify", func(a map[string]any) { delete(a, "notify_subscribers") }, `"notify_subscribers" is required`},
		{"missing title", func(a map[string]any) { a["title"] = " " }, `"title" is required`},
		{"bad status", func(a map[string]any) { a["status"] = "down" }, "must be one of investigating"},
		{"no components", func(a map[string]any) { a["components_affected"] = []any{} }, "at least one component"},
		{"missing components", func(a map[string]any) { delete(a, "components_affected") }, "at least one component"},
		{"bad component status", func(a map[string]any) {
			a["components_affected"] = []any{map[string]any{"id": testComponentID, "status": "broken"}}
		}, "must be one of operational"},
		{"bad component id", func(a map[string]any) {
			a["components_affected"] = []any{map[string]any{"id": "api", "status": "degraded"}}
		}, "not a valid UUID"},
		{"component not object", func(a map[string]any) { a["components_affected"] = []any{"api"} }, "not an object"},
		{"bad json", func(a map[string]any) { a["components_affected"] = "[{" }, "must be a JSON array"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, got := captureServer(t, 201, `{}`)
			d := setupDD(t, ts)
			args := valid()
			tc.mutate(args)
			result, err := createStatusPageDegradation(d.ctx(context.Background()), d, args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.want)
			assert.Empty(t, got.Method, "no request should be sent for invalid input")
		})
	}
}

func TestUpdateStatusPageDegradation_Body(t *testing.T) {
	ts, got := captureServer(t, 200, `{"data":{"id":"`+testDegradationID+`","type":"degradations"}}`)
	d := setupDD(t, ts)
	result, err := updateStatusPageDegradation(d.ctx(context.Background()), d, map[string]any{
		"page_id":             testPageID,
		"degradation_id":      testDegradationID,
		"status":              "resolved",
		"description":         "Everything is back to normal.",
		"components_affected": []any{map[string]any{"id": testComponentID, "status": "operational"}},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, http.MethodPatch, got.Method)
	assert.Equal(t, "/api/v2/statuspages/"+testPageID+"/degradations/"+testDegradationID, got.Path)
	_, notify := got.Query["notify_subscribers"]
	assert.False(t, notify, "notify_subscribers is only sent when set")
	data := got.Body["data"].(map[string]any)
	assert.Equal(t, testDegradationID, data["id"])
	attrs := data["attributes"].(map[string]any)
	assert.Equal(t, "resolved", attrs["status"])
	assert.Equal(t, "Everything is back to normal.", attrs["description"])
	_, hasTitle := attrs["title"]
	assert.False(t, hasTitle)
}

func TestUpdateStatusPageDegradation_RequiresAChange(t *testing.T) {
	ts, got := captureServer(t, 200, `{}`)
	d := setupDD(t, ts)
	result, err := updateStatusPageDegradation(d.ctx(context.Background()), d, map[string]any{
		"page_id":        testPageID,
		"degradation_id": testDegradationID,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "nothing to update")
	assert.Empty(t, got.Method)
}

func TestDeleteStatusPageDegradation(t *testing.T) {
	ts, got := captureServer(t, 204, ``)
	d := setupDD(t, ts)
	result, err := deleteStatusPageDegradation(d.ctx(context.Background()), d, map[string]any{
		"page_id":        testPageID,
		"degradation_id": testDegradationID,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, http.MethodDelete, got.Method)
	assert.Contains(t, result.Data, "deleted")
}

func TestStatusPageCompactSpecs_ReadToolsCovered(t *testing.T) {
	for _, name := range []string{
		"datadog_list_status_pages",
		"datadog_list_status_page_components",
		"datadog_list_status_page_degradations",
		"datadog_list_incident_integrations",
		"datadog_list_incident_impacts",
		"datadog_list_incident_types",
	} {
		fields, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.True(t, ok, "missing compact spec for %s", name)
		assert.NotEmpty(t, fields, "empty compact spec for %s", name)
	}
}
