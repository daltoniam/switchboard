package rwx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRunResults_UsesDirectAPIForRunURL(t *testing.T) {
	var requested []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		requested = append(requested, r.URL.String())
		switch r.URL.Path {
		case "/mint/api/results/status":
			assert.Equal(t, "257e954f472d48d18b596bcc251a35ea", r.URL.Query().Get("id"))
			_, _ = w.Write([]byte(`{"run_id":"257e954f472d48d18b596bcc251a35ea","task_id":"task-1","execution_status":"finished","result_status":"failed"}`))
		case "/mint/api/results/prompt":
			_, _ = w.Write([]byte(`# Failed tasks:

- lint (task-id: 11111111111111111111111111111111)
`))
		case "/mint/api/runs/257e954f472d48d18b596bcc251a35ea":
			_, _ = w.Write([]byte(`{"id":"257e954f472d48d18b596bcc251a35ea","title":"CI","branch":"main","commit_sha":"abc","definition_path":".rwx/ci.yml","completed_runtime_seconds":12}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "curri", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}
	result, err := getRunResults(context.Background(), r, map[string]any{"run_id": "https://cloud.rwx.com/mint/curri/runs/257e954f472d48d18b596bcc251a35ea"})
	require.NoError(t, err)
	require.True(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "257e954f472d48d18b596bcc251a35ea", parsed["run_id"])
	assert.Equal(t, "failure", parsed["status"])
	assert.Equal(t, "main", parsed["branch"])
	require.Len(t, parsed["failed_tasks"].([]any), 1)
	assert.Contains(t, requested, "/mint/api/results/status?id=257e954f472d48d18b596bcc251a35ea")
}

func TestDownloadLogs_UsesDirectAPIZipDownload(t *testing.T) {
	var zipData bytes.Buffer
	zw := zip.NewWriter(&zipData)
	f, err := zw.Create("task/output.log")
	require.NoError(t, err)
	_, err = f.Write([]byte("line 1\nerror line\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "tok", r.PostFormValue("token"))
		assert.Equal(t, "logs.zip", r.PostFormValue("filename"))
		assert.Equal(t, "contents", r.PostFormValue("contents"))
		_, _ = w.Write(zipData.Bytes())
	}))
	defer downloadServer.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/mint/api/log_download":
			assert.Equal(t, "run-123", r.URL.Query().Get("run_id"))
			assert.Equal(t, "lint", r.URL.Query().Get("task_key"))
			_, _ = w.Write([]byte(`{"url":"` + downloadServer.URL + `","token":"tok","filename":"logs.zip","contents":"contents"}`))
		case "/mint/api/runs/run-123":
			_, _ = w.Write([]byte(`{"completed_at":"2024-01-01T00:00:00Z","run_status":{"execution":"finished","result":"failed"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}
	logs, err := downloadLogs(context.Background(), r, "run-123", "lint")
	require.NoError(t, err)
	assert.Equal(t, "line 1\nerror line\n", logs)
}

func TestGetArtifacts_UsesDirectAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/mint/api/artifact_downloads", r.URL.Path)
		assert.Equal(t, "run-123", r.URL.Query().Get("run_id"))
		assert.Equal(t, "lint", r.URL.Query().Get("task_key"))
		_, _ = w.Write([]byte(`[{"key":"coverage","filename":"coverage.txt","url":"https://example.test/download","token":"secret-token"}]`))
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}
	result, err := getArtifacts(context.Background(), r, map[string]any{"run_id": "run-123", "task_key": "lint"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "listed", parsed["action"])
	assert.Equal(t, float64(1), parsed["count"])
	artifact := parsed["artifacts"].([]any)[0].(map[string]any)
	assert.Equal(t, "coverage", artifact["key"])
	assert.Nil(t, artifact["token"])
}

func TestGetArtifacts_RunIDWithoutTaskRejected(t *testing.T) {
	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}
	result, err := getArtifacts(context.Background(), r, map[string]any{"run_id": "run-123"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "task-scoped")
}
