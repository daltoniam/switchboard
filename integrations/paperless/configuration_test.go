package paperless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigurationResourceOperations(t *testing.T) {
	tests := []struct {
		name       string
		tool       mcp.ToolName
		args       map[string]any
		wantMethod string
		wantPath   string
	}{
		{"list tags", "paperless_list_tags", map[string]any{"page": 2, "page_size": 10}, http.MethodGet, "/api/tags/"},
		{"get correspondent", "paperless_get_correspondent", map[string]any{"id": 3}, http.MethodGet, "/api/correspondents/3/"},
		{"create document type", "paperless_create_document_type", map[string]any{"data": map[string]any{"name": "Invoice"}}, http.MethodPost, "/api/document_types/"},
		{"update storage path", "paperless_update_storage_path", map[string]any{"id": 4, "data": map[string]any{"name": "Archive"}}, http.MethodPatch, "/api/storage_paths/4/"},
		{"delete saved view", "paperless_delete_saved_view", map[string]any{"id": 5}, http.MethodDelete, "/api/saved_views/5/"},
		{"create workflow", "paperless_create_workflow", map[string]any{"data": map[string]any{
			"name": "Route invoices", "enabled": true,
			"triggers": []any{map[string]any{"type": 1, "filter_filename": "*.pdf"}},
			"actions":  []any{map[string]any{"type": 1}},
		}}, http.MethodPost, "/api/workflows/"},
		{"create mail rule", "paperless_create_mail_rule", map[string]any{"data": map[string]any{"name": "Invoices"}}, http.MethodPost, "/api/mail_rules/"},
		{"list users", "paperless_list_users", nil, http.MethodGet, "/api/users/"},
		{"create group", "paperless_create_group", map[string]any{"data": map[string]any{"name": "Accounting"}}, http.MethodPost, "/api/groups/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantMethod, r.Method)
				assert.Equal(t, tt.wantPath, r.URL.Path)
				if r.Method == http.MethodPost || r.Method == http.MethodPatch {
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.NotEmpty(t, body)
					if tt.tool == "paperless_create_workflow" {
						triggers, ok := body["triggers"].([]any)
						require.True(t, ok)
						require.Len(t, triggers, 1)
						trigger := triggers[0].(map[string]any)
						assert.Equal(t, float64(1), trigger["type"])
						assert.Equal(t, "*.pdf", trigger["filter_filename"])
					}
				}
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				_, _ = w.Write([]byte(`{"id":1,"name":"Example"}`))
			}))
			defer ts.Close()

			p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
			result, err := p.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.False(t, result.IsError, result.Data)
		})
	}
}

func TestTaskManagementOperations(t *testing.T) {
	tests := []struct {
		tool       mcp.ToolName
		args       map[string]any
		wantMethod string
		wantPath   string
	}{
		{"paperless_list_tasks", map[string]any{"page": 2}, http.MethodGet, "/api/tasks/"},
		{"paperless_get_task", map[string]any{"id": 8}, http.MethodGet, "/api/tasks/8/"},
		{"paperless_get_task_summary", map[string]any{"days": 14}, http.MethodGet, "/api/tasks/summary/"},
		{"paperless_get_task_status_counts", nil, http.MethodGet, "/api/tasks/status_counts/"},
		{"paperless_list_active_tasks", nil, http.MethodGet, "/api/tasks/active/"},
		{"paperless_run_task", map[string]any{"task_type": "train_classifier"}, http.MethodPost, "/api/tasks/run/"},
		{"paperless_acknowledge_tasks", map[string]any{"data": map[string]any{"tasks": []any{8}}}, http.MethodPost, "/api/tasks/acknowledge/"},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantMethod, r.Method)
				assert.Equal(t, tt.wantPath, r.URL.Path)
				_, _ = w.Write([]byte(`{"result":"OK"}`))
			}))
			defer ts.Close()
			p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
			result, err := p.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.False(t, result.IsError, result.Data)
		})
	}
}
