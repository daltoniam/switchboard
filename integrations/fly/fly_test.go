package fly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	i := New()
	assert.Equal(t, "fly", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_token": "test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	f := &fly{client: &http.Client{}}
	err := f.Configure(context.Background(), mcp.Credentials{
		"api_token": "tok",
		"base_url":  "https://custom.fly.dev/v1/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://custom.fly.dev/v1", f.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tl := i.Tools()
	assert.Len(t, tl, 24)

	for _, tool := range tl {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveFlyPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "fly_", "tool %s missing fly_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	f := &fly{token: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := f.Execute(context.Background(), "fly_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.String(), "/apps")
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		w.WriteHeader(200)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, f.Healthy(context.Background()))
}

func TestHealthy_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	f := &fly{token: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, f.Healthy(context.Background()))
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	f := &fly{token: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := f.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer ts.Close()

	f := &fly{token: "bad", client: ts.Client(), baseURL: ts.URL}
	_, err := f.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fly API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := f.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := f.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestPut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Write([]byte(`{"updated":true}`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := f.put(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

func TestPatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		w.Write([]byte(`{"patched":true}`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := f.patch(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "patched")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := f.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "429 should produce RetryableError")

	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestDoRequest_RetryableOn5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte(`service unavailable`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := f.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestDoRequest_NonRetryableOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	f := &fly{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := f.get(context.Background(), "/test")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}

// --- result helper tests ---

func TestErrResult_PropagatesRetryableError(t *testing.T) {
	retryErr := &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
	result, err := mcp.ErrResult(retryErr)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestErrResult_WrapsNonRetryableError(t *testing.T) {
	plainErr := fmt.Errorf("bad request")
	result, err := mcp.ErrResult(plainErr)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "bad request", result.Data)
}

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

// --- queryEncode tests ---

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

// --- tool roundtrip tests ---

func newTestFly(t *testing.T, handler http.HandlerFunc) (*fly, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	f := &fly{token: "test-token", client: ts.Client(), baseURL: ts.URL}
	return f, ts
}

func TestListApps(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "personal", r.URL.Query().Get("org_slug"))
		w.Write([]byte(`[{"name":"myapp"}]`))
	})
	result, err := f.Execute(context.Background(), "fly_list_apps", map[string]any{"org_slug": "personal"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "myapp")
}

func TestListApps_OrgSlugEscaped(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "personal", r.URL.Query().Get("org_slug"))
		assert.Empty(t, r.URL.Query().Get("force"))
		w.Write([]byte(`[]`))
	})
	result, err := f.Execute(context.Background(), "fly_list_apps", map[string]any{"org_slug": "personal"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetApp(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/apps/myapp", r.URL.Path)
		w.Write([]byte(`{"name":"myapp","status":"deployed"}`))
	})
	result, err := f.Execute(context.Background(), "fly_get_app", map[string]any{"app_name": "myapp"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deployed")
}

func TestCreateApp(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps", r.URL.Path)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "newapp", body["app_name"])
		assert.Equal(t, "personal", body["org_slug"])
		w.WriteHeader(201)
		w.Write([]byte(`{"name":"newapp"}`))
	})
	result, err := f.Execute(context.Background(), "fly_create_app", map[string]any{
		"app_name": "newapp",
		"org_slug": "personal",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "newapp")
}

func TestDeleteApp(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/apps/myapp", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("force"))
		w.WriteHeader(202)
	})
	result, err := f.Execute(context.Background(), "fly_delete_app", map[string]any{
		"app_name": "myapp",
		"force":    true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListMachines(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/apps/myapp/machines")
		w.Write([]byte(`[{"id":"m1","state":"started"}]`))
	})
	result, err := f.Execute(context.Background(), "fly_list_machines", map[string]any{"app_name": "myapp"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "m1")
}

func TestGetMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/apps/myapp/machines/m1", r.URL.Path)
		w.Write([]byte(`{"id":"m1","state":"started","config":{}}`))
	})
	result, err := f.Execute(context.Background(), "fly_get_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "started")
}

func TestCreateMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps/myapp/machines", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "ord", body["region"])
		w.Write([]byte(`{"id":"m2","state":"created"}`))
	})
	result, err := f.Execute(context.Background(), "fly_create_machine", map[string]any{
		"app_name": "myapp",
		"region":   "ord",
		"config":   map[string]any{"image": "nginx:latest"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "m2")
}

func TestUpdateMachine_UsesPatch(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/apps/myapp/machines/m1", r.URL.Path)
		w.Write([]byte(`{"id":"m1","state":"started"}`))
	})
	result, err := f.Execute(context.Background(), "fly_update_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
		"config":     map[string]any{"image": "nginx:latest"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/apps/myapp/machines/m1", r.URL.Path)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_delete_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestStartMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps/myapp/machines/m1/start", r.URL.Path)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_start_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestStopMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps/myapp/machines/m1/stop", r.URL.Path)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_stop_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestRestartMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/restart")
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_restart_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSignalMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/signal")
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "SIGTERM", body["signal"])
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_signal_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
		"signal":     "SIGTERM",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestWaitMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/wait")
		assert.Equal(t, "started", r.URL.Query().Get("state"))
		w.Write([]byte(`{"ok":true}`))
	})
	result, err := f.Execute(context.Background(), "fly_wait_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
		"state":      "started",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestExecMachine(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/exec")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		cmd := body["command"].([]any)
		assert.Equal(t, "ls", cmd[0])
		w.Write([]byte(`{"stdout":"file1\nfile2"}`))
	})
	result, err := f.Execute(context.Background(), "fly_exec_machine", map[string]any{
		"app_name":   "myapp",
		"machine_id": "m1",
		"command":    []any{"ls", "-la"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "file1")
}

func TestListVolumes(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/apps/myapp/volumes", r.URL.Path)
		w.Write([]byte(`[{"id":"vol_1"}]`))
	})
	result, err := f.Execute(context.Background(), "fly_list_volumes", map[string]any{"app_name": "myapp"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "vol_1")
}

func TestGetVolume(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/apps/myapp/volumes/vol_1", r.URL.Path)
		w.Write([]byte(`{"id":"vol_1","size_gb":3}`))
	})
	result, err := f.Execute(context.Background(), "fly_get_volume", map[string]any{
		"app_name":  "myapp",
		"volume_id": "vol_1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateVolume(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps/myapp/volumes", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "data", body["name"])
		assert.Equal(t, "ord", body["region"])
		w.Write([]byte(`{"id":"vol_2"}`))
	})
	result, err := f.Execute(context.Background(), "fly_create_volume", map[string]any{
		"app_name": "myapp",
		"name":     "data",
		"region":   "ord",
		"size_gb":  float64(5),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateVolume_UsesPut(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/apps/myapp/volumes/vol_1", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, true, body["auto_backup_enabled"])
		w.Write([]byte(`{"id":"vol_1"}`))
	})
	result, err := f.Execute(context.Background(), "fly_update_volume", map[string]any{
		"app_name":            "myapp",
		"volume_id":           "vol_1",
		"auto_backup_enabled": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteVolume(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/apps/myapp/volumes/vol_1", r.URL.Path)
		w.WriteHeader(204)
	})
	result, err := f.Execute(context.Background(), "fly_delete_volume", map[string]any{
		"app_name":  "myapp",
		"volume_id": "vol_1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListVolumeSnapshots(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/apps/myapp/volumes/vol_1/snapshots", r.URL.Path)
		w.Write([]byte(`[{"id":"snap_1"}]`))
	})
	result, err := f.Execute(context.Background(), "fly_list_volume_snapshots", map[string]any{
		"app_name":  "myapp",
		"volume_id": "vol_1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "snap_1")
}

func TestListSecrets(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/apps/myapp/secrets", r.URL.Path)
		w.Write([]byte(`[{"label":"DATABASE_URL","digest":"abc123"}]`))
	})
	result, err := f.Execute(context.Background(), "fly_list_secrets", map[string]any{"app_name": "myapp"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "DATABASE_URL")
}

func TestSetSecrets(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps/myapp/secrets", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "postgres://...", body["DATABASE_URL"])
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_set_secrets", map[string]any{
		"app_name": "myapp",
		"secrets":  map[string]any{"DATABASE_URL": "postgres://..."},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSetSecrets_MissingSecrets(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	})
	result, err := f.Execute(context.Background(), "fly_set_secrets", map[string]any{
		"app_name": "myapp",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "secrets parameter is required")
}

func TestUnsetSecrets(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/apps/myapp/secrets/unset", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		keys := body["keys"].([]any)
		assert.Contains(t, keys, "DATABASE_URL")
		w.Write([]byte(`{"status":"success"}`))
	})
	result, err := f.Execute(context.Background(), "fly_unset_secrets", map[string]any{
		"app_name": "myapp",
		"keys":     []any{"DATABASE_URL"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUnsetSecrets_MissingKeys(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	})
	result, err := f.Execute(context.Background(), "fly_unset_secrets", map[string]any{
		"app_name": "myapp",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "keys parameter is required")
}

// --- path escape tests ---

func TestPathEscape_AppName(t *testing.T) {
	f, _ := newTestFly(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/apps/my%2Fapp", r.URL.RawPath)
		w.Write([]byte(`{"name":"my/app"}`))
	})
	result, err := f.Execute(context.Background(), "fly_get_app", map[string]any{"app_name": "my/app"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
