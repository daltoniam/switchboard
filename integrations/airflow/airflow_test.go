package airflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigure(t *testing.T) {
	a := New()
	assert.Equal(t, "airflow", a.Name())
	assert.False(t, a.Healthy(context.Background()))
	for _, tt := range []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{"missing base URL", mcp.Credentials{"username": "user", "password": "pass"}, "base_url"},
		{"missing credentials", mcp.Credentials{"base_url": "https://airflow.example.com"}, "username and password"},
		{"partial credentials", mcp.Credentials{"base_url": "https://airflow.example.com", "username": "user"}, "username and password"},
		{"invalid URL", mcp.Credentials{"base_url": "https://user:pass@airflow.example.com", "access_token": "token"}, "invalid base_url"},
		{"insecure URL", mcp.Credentials{"base_url": "http://airflow.example.com", "access_token": "token"}, "https"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := New().Configure(context.Background(), tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
	assert.NoError(t, a.Configure(context.Background(), mcp.Credentials{"base_url": "https://airflow.example.com/", "username": "user", "password": "pass"}))
	assert.NoError(t, New().Configure(context.Background(), mcp.Credentials{"base_url": "http://127.0.0.1:8080", "access_token": "token"}))
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.True(t, strings.HasPrefix(string(tool.Name), "airflow_"))
		assert.NotEmpty(t, tool.Description)
		assert.False(t, seen[tool.Name])
		seen[tool.Name] = true
		assert.NotNil(t, dispatch[tool.Name])
	}
	assert.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			found = found || tool.Name == name
		}
		assert.True(t, found, "%s", name)
	}
}

func TestExecuteRoutes(t *testing.T) {
	var tokens atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/token" {
			tokens.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			var login map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&login))
			assert.Equal(t, "user", login["username"])
			assert.Equal(t, "pass", login["password"])
			_, _ = w.Write([]byte(`{"access_token":"jwt"}`))
			return
		}
		assert.Equal(t, "Bearer jwt", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/v2/dags":
			assert.Contains(t, []string{"1", "25"}, r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"dags":[{"dag_id":"nightly"}],"total_entries":1}`))
		case "/api/v2/dags/nightly":
			_, _ = w.Write([]byte(`{"dag_id":"nightly"}`))
		case "/api/v2/dags/nightly/dagRuns":
			if r.Method == http.MethodPost {
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Contains(t, body, "logical_date")
				assert.Equal(t, map[string]any{"dry_run": true}, body["conf"])
				_, _ = w.Write([]byte(`{"dag_id":"nightly","dag_run_id":"manual__1"}`))
			} else {
				_, _ = w.Write([]byte(`{"dag_runs":[{"dag_run_id":"manual__1"}],"total_entries":1}`))
			}
		case "/api/v2/dags/nightly/dagRuns/manual__1":
			_, _ = w.Write([]byte(`{"dag_run_id":"manual__1"}`))
		case "/api/v2/dags/nightly/dagRuns/manual__1/taskInstances":
			_, _ = w.Write([]byte(`{"task_instances":[{"task_id":"extract"}],"total_entries":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	a := New().(*airflow)
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"base_url": server.URL, "username": "user", "password": "pass"}))
	assert.True(t, a.Healthy(context.Background()))
	for _, tt := range []struct {
		tool mcp.ToolName
		args map[string]any
		want string
	}{
		{"airflow_list_dags", nil, "nightly"},
		{"airflow_get_dag", map[string]any{"dag_id": "nightly"}, "nightly"},
		{"airflow_list_dag_runs", map[string]any{"dag_id": "nightly"}, "manual__1"},
		{"airflow_get_dag_run", map[string]any{"dag_id": "nightly", "dag_run_id": "manual__1"}, "manual__1"},
		{"airflow_list_task_instances", map[string]any{"dag_id": "nightly", "dag_run_id": "manual__1"}, "extract"},
		{"airflow_trigger_dag", map[string]any{"dag_id": "nightly", "conf": map[string]any{"dry_run": true}}, "manual__1"},
	} {
		t.Run(string(tt.tool), func(t *testing.T) {
			result, err := a.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.False(t, result.IsError, result.Data)
			assert.Contains(t, result.Data, tt.want)
		})
	}
	assert.EqualValues(t, 1, tokens.Load())
}

func TestExecuteValidation(t *testing.T) {
	a := New()
	for _, tt := range []struct {
		tool mcp.ToolName
		args map[string]any
		want string
	}{
		{"airflow_unknown", nil, "unknown tool"},
		{"airflow_get_dag", nil, "dag_id"},
		{"airflow_get_dag_run", map[string]any{"dag_id": "nightly"}, "dag_run_id"},
		{"airflow_list_dags", map[string]any{"limit": 101}, "limit"},
		{"airflow_list_dags", map[string]any{"limit": -1}, "limit"},
		{"airflow_list_dags", map[string]any{"limit": "bad"}, "limit"},
		{"airflow_list_dags", map[string]any{"offset": -1}, "offset"},
		{"airflow_trigger_dag", map[string]any{"dag_id": "nightly", "conf": "oops"}, "conf"},
		{"airflow_get_dag", map[string]any{"dag_id": []int{1}}, "dag_id"},
	} {
		t.Run(string(tt.tool)+tt.want, func(t *testing.T) {
			result, err := a.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestErrorBodyDoesNotExposeRunConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"invalid conf: private-secret"}`))
	}))
	t.Cleanup(server.Close)
	a := New().(*airflow)
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"base_url": server.URL, "access_token": "token"}))
	result, err := a.Execute(context.Background(), "airflow_trigger_dag", map[string]any{"dag_id": "nightly", "conf": map[string]any{"password": "private-secret"}})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.NotContains(t, result.Data, "private-secret")
}

func TestTokenRefreshOnUnauthorized(t *testing.T) {
	var exchanges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/token" {
			count := exchanges.Add(1)
			_, _ = w.Write([]byte(`{"access_token":"jwt-` + string(rune('0'+count)) + `"}`))
			return
		}
		if r.Header.Get("Authorization") == "Bearer jwt-1" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"expired"}`))
			return
		}
		assert.Equal(t, "Bearer jwt-2", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"dags":[],"total_entries":0}`))
	}))
	t.Cleanup(server.Close)
	a := New().(*airflow)
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"base_url": server.URL, "username": "user", "password": "pass"}))
	result, err := a.Execute(context.Background(), "airflow_list_dags", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.EqualValues(t, 2, exchanges.Load())
}

func TestAPIErrorsAndRedirects(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"detail":"denied"}`))
			}))
			t.Cleanup(server.Close)
			a := New().(*airflow)
			require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"base_url": server.URL, "access_token": "token"}))
			result, err := a.Execute(context.Background(), "airflow_list_dags", nil)
			if status == 429 || status >= 500 {
				require.Error(t, err)
				assert.Contains(t, err.Error(), http.StatusText(status))
			} else {
				require.NoError(t, err)
				assert.True(t, result.IsError)
				assert.Contains(t, result.Data, http.StatusText(status))
			}
		})
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
	}))
	t.Cleanup(other.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	a := New().(*airflow)
	require.NoError(t, a.Configure(context.Background(), mcp.Credentials{"base_url": server.URL, "access_token": "token"}))
	result, err := a.Execute(context.Background(), "airflow_list_dags", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "redirect")
}
