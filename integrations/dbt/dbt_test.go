package dbt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "dbt", i.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "token and account", creds: mcp.Credentials{"token": "t", "account_id": "36788"}, wantURL: defaultBaseURL},
		{name: "custom access url", creds: mcp.Credentials{"token": "t", "account_id": "1", "base_url": "https://ab123.us1.dbt.com/"}, wantURL: "https://ab123.us1.dbt.com"},
		{name: "missing token", creds: mcp.Credentials{"account_id": "1"}, wantErr: "token is required"},
		{name: "missing account", creds: mcp.Credentials{"token": "t"}, wantErr: "account_id is required"},
		{name: "non numeric account", creds: mcp.Credentials{"token": "t", "account_id": "abc"}, wantErr: "account_id must be numeric"},
		{name: "non numeric prod job", creds: mcp.Credentials{"token": "t", "account_id": "1", "prod_job_id": "x"}, wantErr: "prod_job_id must be numeric"},
		{name: "token prefix is stripped", creds: mcp.Credentials{"token": "Token abc", "account_id": "1"}, wantURL: defaultBaseURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New().(*dbt)
			err := d.Configure(context.Background(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, d.baseURL)
			assert.False(t, strings.HasPrefix(d.token, "Token "))
		})
	}
}

func TestHealthy_Unconfigured(t *testing.T) {
	assert.False(t, (&dbt{}).Healthy(context.Background()))
}

func TestTools(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Description)
		assert.True(t, strings.HasPrefix(string(tool.Name), "dbt_"), "tool %s must be prefixed", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
	}
	assert.Contains(t, tools[0].Description, "Start here")
}

func TestTools_NoWriteOperations(t *testing.T) {
	for _, tool := range New().Tools() {
		for _, verb := range []string{"trigger", "retry", "cancel", "rerun", "delete", "create", "update"} {
			assert.NotContains(t, string(tool.Name), verb, "dbt integration must stay read-only")
		}
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
	res, err := New().Execute(context.Background(), "dbt_nope", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tool")
}

// fakeCloud serves a minimal dbt Cloud Admin API v2 for account 1.
type fakeCloud struct {
	t              *testing.T
	manifestHits   atomic.Int32
	artifactsSaved bool
	noRunResults   bool
	lastQuery      map[string]string
}

const (
	runErrorJSON = `{"id":100,"job_definition_id":7,"status":20,"status_humanized":"Error","git_sha":"bad100","git_branch":"master","created_at":"2026-10-15T10:00:00Z","finished_at":"2026-10-15T10:05:00Z","href":"https://cloud.getdbt.com/deploy/1/projects/2/runs/100","artifacts_saved":%ARTIFACTS%,"trigger":{"cause":"Scheduled"},"run_steps":[
 {"id":1,"index":1,"name":"Clone git repository","status":10,"logs":"cloned","truncated_debug_logs":"debug noise"},
 {"id":2,"index":2,"name":"Invoke dbt with dbt deps","status":10,"logs":"deps ok","truncated_debug_logs":"debug noise"},
 {"id":3,"index":3,"name":"Invoke dbt with dbt build","status":20,"logs":"line1\nline2\nDatabase Error in model mart_orders (models/marts/mart_orders.sql)\n  invalid identifier 'AMOUNT'\nDone. PASS=10 ERROR=1","truncated_debug_logs":"debug noise"},
 {"id":4,"index":4,"name":"Invoke dbt with dbt docs generate","status":30,"logs":"skipped","truncated_debug_logs":"debug noise"}]}`
	runResultsJSON = `{"results":[
 {"unique_id":"model.data_dbt.stg_orders","status":"success","message":"OK","execution_time":1.2},
 {"unique_id":"model.data_dbt.mart_orders","status":"error","message":"Database Error in model mart_orders\n invalid identifier 'AMOUNT'","relation_name":"analytics.marts.mart_orders","compiled_code":"select amount from x","execution_time":0.4},
 {"unique_id":"test.data_dbt.not_null_stg_orders_id.abc","status":"fail","message":"Got 3 results, configured to fail if != 0","failures":3,"execution_time":0.1}]}`
)

func (f *fakeCloud) handler() http.Handler {
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
	mux.HandleFunc("GET /api/v2/accounts/1/{$}", func(w http.ResponseWriter, r *http.Request) {
		write(w, `{"status":{"code":200},"data":{"id":1,"name":"Curri"}}`)
	})
	mux.HandleFunc("GET /api/v2/accounts/1/runs/{$}", func(w http.ResponseWriter, r *http.Request) {
		f.capture(r)
		if r.URL.Query().Get("status") == "10" {
			write(w, `{"status":{"code":200},"data":[{"id":98,"job_definition_id":7,"status":10,"git_sha":"good98","finished_at":"2026-10-14T10:05:00Z"}],"extra":{"pagination":{"count":1,"total_count":1}}}`)
			return
		}
		write(w, `{"status":{"code":200},"data":[
 {"id":101,"job_definition_id":7,"status":3,"git_sha":"bad100"},
 {"id":100,"job_definition_id":7,"status":20,"git_sha":"bad100","finished_at":"2026-10-15T10:05:00Z"},
 {"id":99,"job_definition_id":7,"status":20,"git_sha":"mid99","finished_at":"2026-10-14T22:05:00Z"},
 {"id":98,"job_definition_id":7,"status":10,"git_sha":"good98","finished_at":"2026-10-14T10:05:00Z"},
 {"id":97,"job_definition_id":7,"status":10,"git_sha":"older97"}],"extra":{"pagination":{"count":5,"total_count":5}}}`)
	})
	mux.HandleFunc("GET /api/v2/accounts/1/runs/100/{$}", func(w http.ResponseWriter, r *http.Request) {
		f.capture(r)
		saved := "false"
		if f.artifactsSaved {
			saved = "true"
		}
		write(w, `{"status":{"code":200},"data":`+strings.Replace(runErrorJSON, "%ARTIFACTS%", saved, 1)+`}`)
	})
	mux.HandleFunc("GET /api/v2/accounts/1/runs/{run}/artifacts/{path...}", func(w http.ResponseWriter, r *http.Request) {
		f.capture(r)
		if !f.artifactsSaved && r.PathValue("run") == "100" {
			http.Error(w, `{"status":{"code":404,"user_message":"not found"}}`, http.StatusNotFound)
			return
		}
		switch r.PathValue("path") {
		case "run_results.json":
			if f.noRunResults {
				http.Error(w, `{"status":{"code":404}}`, http.StatusNotFound)
				return
			}
			assert.Equal(f.t, "3", r.URL.Query().Get("step"), "run_results must come from the failed step")
			write(w, runResultsJSON)
		case "manifest.json":
			f.manifestHits.Add(1)
			write(w, testManifestJSON)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /api/v2/accounts/1/runs/403/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":{"code":403,"user_message":"Access denied"}}`, http.StatusForbidden)
	})
	mux.HandleFunc("GET /api/v2/accounts/1/runs/429/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, `slow down`, http.StatusTooManyRequests)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(f.t, "Token secret-token", r.Header.Get("Authorization"))
		assert.Equal(f.t, "application/json", r.Header.Get("Accept"))
		mux.ServeHTTP(w, r)
	})
}

func (f *fakeCloud) capture(r *http.Request) {
	f.lastQuery = map[string]string{"path": r.URL.Path}
	for k, v := range r.URL.Query() {
		f.lastQuery[k] = v[0]
	}
}

func newTestDBT(t *testing.T, f *fakeCloud, prodJobID string) *dbt {
	t.Helper()
	f.t = t
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	d := New().(*dbt)
	creds := mcp.Credentials{"token": "secret-token", "account_id": "1", "base_url": srv.URL}
	if prodJobID != "" {
		creds["prod_job_id"] = prodJobID
	}
	require.NoError(t, d.Configure(context.Background(), creds))
	return d
}

func execJSON(t *testing.T, d *dbt, tool string, args map[string]any) map[string]any {
	t.Helper()
	res, err := d.Execute(context.Background(), mcp.ToolName(tool), args)
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error result: %s", res.Data)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &out), res.Data)
	return out
}

func execErr(t *testing.T, d *dbt, tool string, args map[string]any) string {
	t.Helper()
	res, err := d.Execute(context.Background(), mcp.ToolName(tool), args)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected error result, got: %s", res.Data)
	return res.Data
}

func TestHealthy(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{}, "")
	assert.True(t, d.Healthy(context.Background()))
}

func TestListJobsRuns_QueryParams(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		wantStatus string
	}{
		{name: "named status", args: map[string]any{"job_id": "7", "status": "error"}, wantStatus: "20"},
		{name: "numeric status", args: map[string]any{"job_id": "7", "status": "10"}, wantStatus: "10"},
		{name: "no status", args: map[string]any{"job_id": "7"}, wantStatus: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeCloud{}
			d := newTestDBT(t, f, "")
			out := execJSON(t, d, "dbt_list_jobs_runs", tt.args)
			assert.NotEmpty(t, out["runs"])
			assert.Equal(t, "7", f.lastQuery["job_definition_id"])
			assert.Equal(t, "-id", f.lastQuery["order_by"])
			assert.Equal(t, "10", f.lastQuery["limit"])
			assert.Equal(t, tt.wantStatus, f.lastQuery["status"])
		})
	}
}

func TestListJobsRuns_BadStatus(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{}, "")
	msg := execErr(t, d, "dbt_list_jobs_runs", map[string]any{"status": "exploded"})
	assert.Contains(t, msg, "unknown status")
}

func TestGetJobRunError_SummarizesFailedStep(t *testing.T) {
	f := &fakeCloud{artifactsSaved: true}
	d := newTestDBT(t, f, "")
	out := execJSON(t, d, "dbt_get_job_run_error", map[string]any{"run_id": 100})

	assert.Equal(t, "bad100", out["git_sha"])
	assert.Equal(t, true, out["run_results_available"])
	assert.Nil(t, out["artifacts_pending"])

	steps := out["failed_steps"].([]any)
	require.Len(t, steps, 1)
	step := steps[0].(map[string]any)
	assert.EqualValues(t, 3, step["index"])
	assert.Contains(t, step["log_tail"], "invalid identifier 'AMOUNT'")

	nodes := out["failed_nodes"].([]any)
	require.Len(t, nodes, 2)
	byID := map[string]map[string]any{}
	for _, n := range nodes {
		m := n.(map[string]any)
		byID[m["unique_id"].(string)] = m
	}
	mart := byID["model.data_dbt.mart_orders"]
	require.NotNil(t, mart)
	assert.Equal(t, "error", mart["status"])
	assert.Equal(t, "models/marts/mart_orders.sql", mart["original_file_path"])
	assert.Equal(t, "select amount from x", mart["compiled_code"])
	test := byID["test.data_dbt.not_null_stg_orders_id.abc"]
	require.NotNil(t, test)
	assert.Equal(t, "models/staging/_staging.yml", test["original_file_path"])
	assert.EqualValues(t, 3, test["failures"])
}

func TestGetJobRunError_ArtifactsPending(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{artifactsSaved: false}, "")
	out := execJSON(t, d, "dbt_get_job_run_error", map[string]any{"run_id": "100"})
	assert.Equal(t, true, out["artifacts_pending"])
	assert.Equal(t, false, out["run_results_available"])
	assert.Contains(t, out["failed_steps"].([]any)[0].(map[string]any)["log_tail"], "invalid identifier")
}

func TestGetJobRunError_NoRunResults(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{artifactsSaved: true, noRunResults: true}, "")
	out := execJSON(t, d, "dbt_get_job_run_error", map[string]any{"run_id": "100"})
	assert.Nil(t, out["artifacts_pending"])
	assert.Equal(t, false, out["run_results_available"])
	assert.Empty(t, out["failed_nodes"])
	assert.Contains(t, out["failed_steps"].([]any)[0].(map[string]any)["log_tail"], "Database Error")
}

func TestGetJobRunRegressionRange(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{artifactsSaved: true}, "")
	out := execJSON(t, d, "dbt_get_job_run_regression_range", map[string]any{"run_id": "100"})
	assert.Equal(t, "good98", out["compare_base"])
	assert.Equal(t, "mid99", out["compare_head"])
	assert.EqualValues(t, 2, out["failing_runs_since_success"])
	assert.EqualValues(t, 98, out["last_success_run"].(map[string]any)["id"])
	assert.EqualValues(t, 99, out["first_failing_run"].(map[string]any)["id"])
}

func TestGetJobRunArtifact(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		inlineMax int
		wantErr   string
		wantData  string
	}{
		{name: "jq filter", args: map[string]any{"run_id": "98", "path": "manifest.json", "jq": ".metadata.project_name"}, inlineMax: 64, wantData: `"data_dbt"`},
		{name: "jq multiple outputs become array", args: map[string]any{"run_id": "98", "path": "manifest.json", "jq": ".nodes[] | select(.config.materialized==\"table\") | .name"}, inlineMax: 64, wantData: `["mart_orders","mart_orders_daily"]`},
		{name: "oversize without jq", args: map[string]any{"run_id": "98", "path": "manifest.json"}, inlineMax: 64, wantErr: "pass jq"},
		{name: "small artifact inline", args: map[string]any{"run_id": "98", "path": "manifest.json"}, inlineMax: 1 << 20, wantData: `"project_name": "data_dbt"`},
		{name: "bad jq", args: map[string]any{"run_id": "98", "path": "manifest.json", "jq": ".["}, inlineMax: 64, wantErr: "jq"},
		{name: "path traversal", args: map[string]any{"run_id": "98", "path": "../secrets"}, inlineMax: 64, wantErr: "invalid artifact path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDBT(t, &fakeCloud{artifactsSaved: true}, "")
			d.inlineArtifactMax = tt.inlineMax
			res, err := d.Execute(context.Background(), "dbt_get_job_run_artifact", tt.args)
			require.NoError(t, err)
			if tt.wantErr != "" {
				require.True(t, res.IsError, res.Data)
				assert.Contains(t, res.Data, tt.wantErr)
				return
			}
			require.False(t, res.IsError, res.Data)
			assert.Contains(t, res.Data, tt.wantData)
		})
	}
}

func TestGetManifestNodes_UsesLatestProdRunAndCaches(t *testing.T) {
	f := &fakeCloud{artifactsSaved: true}
	d := newTestDBT(t, f, "7")
	args := map[string]any{"file_paths": []any{"models/staging/stg_orders.sql", "models/marts/new.sql"}}
	out := execJSON(t, d, "dbt_get_manifest_nodes", args)
	assert.EqualValues(t, 98, out["manifest_run_id"])
	assert.Equal(t, "good98", out["manifest_git_sha"])
	nodes := out["nodes"].([]any)
	require.Len(t, nodes, 1)
	assert.Equal(t, "model.data_dbt.stg_orders", nodes[0].(map[string]any)["unique_id"])
	assert.Equal(t, []any{"models/marts/new.sql"}, out["unmatched"])

	execJSON(t, d, "dbt_get_manifest_nodes", args)
	assert.EqualValues(t, 1, f.manifestHits.Load(), "manifest should be cached per run")
}

func TestGetManifestNodes_RequiresJob(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{}, "")
	msg := execErr(t, d, "dbt_get_manifest_nodes", map[string]any{"file_paths": []any{"models/a.sql"}})
	assert.Contains(t, msg, "prod_job_id")
}

func TestGetManifestNodes_RequiresInput(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{}, "7")
	msg := execErr(t, d, "dbt_get_manifest_nodes", map[string]any{})
	assert.Contains(t, msg, "file_paths or names")
}

func TestGetModelLineage(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{artifactsSaved: true}, "7")
	out := execJSON(t, d, "dbt_get_model_lineage", map[string]any{"names": []any{"stg_orders"}, "depth": 0})
	nodes := out["nodes"].([]any)
	assert.Len(t, nodes, 3)
	assert.Len(t, out["exposures"], 1)
	assert.Equal(t, "down", out["direction"])
	assert.EqualValues(t, 98, out["manifest_run_id"])
}

func TestGetModelLineage_DefaultsDepthTwo(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{artifactsSaved: true}, "7")
	out := execJSON(t, d, "dbt_get_model_lineage", map[string]any{"unique_ids": []any{"model.data_dbt.stg_orders"}})
	assert.EqualValues(t, 2, out["depth"])
	assert.Len(t, out["nodes"], 3)
	assert.Empty(t, out["exposures"])
}

func TestGetModelLineage_BadDirection(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{artifactsSaved: true}, "7")
	msg := execErr(t, d, "dbt_get_model_lineage", map[string]any{"names": []any{"stg_orders"}, "direction": "sideways"})
	assert.Contains(t, msg, "direction")
}

func TestAPIErrors(t *testing.T) {
	d := newTestDBT(t, &fakeCloud{}, "")

	msg := execErr(t, d, "dbt_get_job_run_error", map[string]any{"run_id": "403"})
	assert.Contains(t, msg, "403")
	assert.Contains(t, msg, "IP allowlist")

	_, err := d.getJSON(context.Background(), "/runs/429/", nil)
	var re *mcp.RetryableError
	require.True(t, errors.As(err, &re), "429 should be retryable: %v", err)
	assert.Equal(t, http.StatusTooManyRequests, re.StatusCode)
}
