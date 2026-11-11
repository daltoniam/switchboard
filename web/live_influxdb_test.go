package web

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/influxdb"
	"github.com/stretchr/testify/require"
)

const liveInfluxImage = "influxdb:2.8@sha256:e6d293bedc5800bc5f7f61af6335d90a355c78be88fa41e591431ff44df553a5"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "influxdb", Tier: liveContainer, Adapter: influxdb.New(), Provision: liveInfluxProvision,
		Cases: []liveCase{
			{Tool: "influxdb_list_buckets", Expect: []string{"retention-fixture", "86400"}},
			{Tool: "influxdb_list_tasks", Expect: []string{"diagnostic-task", "${task_id}", "active"}},
			{Tool: "influxdb_get_task", Args: map[string]any{"task_id": "${task_id}"}, Expect: []string{"diagnostic-task", "${org_id}", "1h"}},
			{Tool: "influxdb_list_task_runs", Args: map[string]any{"task_id": "${task_id}"}, Expect: []string{"${run_id}", "failed"}},
			{Tool: "influxdb_get_task_run", Args: map[string]any{"task_id": "${task_id}", "run_id": "${run_id}"}, Expect: []string{"${run_id}", "failed"}},
		},
	})
}

func liveInfluxProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{Image: liveInfluxImage, Ports: []string{"8086/tcp"}, Entrypoint: "/bin/sh", Cmd: []string{"-c", `set -eu
openssl req -x509 -newkey rsa:2048 -nodes -keyout /tmp/server.key -out /tmp/server.crt -days 1 -subj /CN=localhost -addext subjectAltName=IP:127.0.0.1,DNS:localhost
exec influxd --tls-cert /tmp/server.crt --tls-key /tmp/server.key`}})
	var certificate string
	liveEventually(t, time.Minute, func() error {
		output := container.Exec(t, "", "sh", "-c", "test ! -f /tmp/server.crt || cat /tmp/server.crt")
		if output == "" {
			return fmt.Errorf("waiting for TLS certificate")
		}
		certificate = output
		return nil
	})
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(certificate)))
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}}}
	t.Cleanup(client.CloseIdleConnections)
	host, port := container.Address(t, "8086/tcp")
	baseURL := "https://" + net.JoinHostPort(host, port)
	liveEventually(t, time.Minute, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/health", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("InfluxDB is not ready")
		}
		return nil
	})
	call := func(method, path, token string, body any, status int) map[string]any {
		t.Helper()
		var input io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			require.NoError(t, err)
			input = bytes.NewReader(data)
		}
		request, err := http.NewRequestWithContext(t.Context(), method, baseURL+path, input)
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Token "+token)
		}
		response, err := client.Do(request)
		require.NoError(t, err)
		defer func() { _ = response.Body.Close() }()
		data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
		require.NoError(t, err)
		require.Equal(t, status, response.StatusCode, "%s %s: %s", method, path, string(data))
		var value map[string]any
		if len(data) > 0 {
			require.NoError(t, json.Unmarshal(data, &value))
		}
		return value
	}
	bootstrap := "live-bootstrap-token"
	setup := call(http.MethodPost, "/api/v2/setup", "", map[string]any{"username": "bootstrap", "password": "live-password-123", "org": "switchboard", "bucket": "retention-fixture", "retentionPeriodSeconds": 86400, "token": bootstrap}, http.StatusCreated)
	orgID := setup["org"].(map[string]any)["id"].(string)
	bucketID := setup["bucket"].(map[string]any)["id"].(string)
	flux := `option task = {name: "diagnostic-task", every: 1h}
die(msg: "SECRET_SENTINEL")`
	task := call(http.MethodPost, "/api/v2/tasks", bootstrap, map[string]any{"orgID": orgID, "flux": flux, "description": "SECRET_SENTINEL"}, http.StatusCreated)
	taskID := task["id"].(string)
	forced := call(http.MethodPost, "/api/v2/tasks/"+taskID+"/runs", bootstrap, map[string]any{}, http.StatusCreated)
	runID := forced["id"].(string)
	liveEventually(t, time.Minute, func() error {
		run := call(http.MethodGet, "/api/v2/tasks/"+taskID+"/runs/"+runID, bootstrap, nil, http.StatusOK)
		if run["status"] != "failed" {
			return fmt.Errorf("waiting for seeded failed run")
		}
		return nil
	})
	otherOrg := call(http.MethodPost, "/api/v2/orgs", bootstrap, map[string]any{"name": "outside-scope"}, http.StatusCreated)["id"].(string)
	otherTask := call(http.MethodPost, "/api/v2/tasks", bootstrap, map[string]any{"orgID": otherOrg, "flux": strings.ReplaceAll(flux, "diagnostic-task", "outside-task")}, http.StatusCreated)["id"].(string)
	authorization := call(http.MethodPost, "/api/v2/authorizations", bootstrap, map[string]any{"orgID": orgID, "description": "least-privilege Switchboard metadata reader", "permissions": []any{
		map[string]any{"action": "read", "resource": map[string]any{"type": "tasks", "orgID": orgID}},
		map[string]any{"action": "read", "resource": map[string]any{"type": "buckets", "orgID": orgID, "id": bucketID}},
	}}, http.StatusCreated)
	token := authorization["token"].(string)
	for _, test := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v2/tasks", map[string]any{"orgID": orgID, "flux": flux}},
		{http.MethodPatch, "/api/v2/tasks/" + taskID, map[string]any{"status": "inactive"}},
		{http.MethodDelete, "/api/v2/tasks/" + taskID, nil},
		{http.MethodPost, "/api/v2/tasks/" + taskID + "/runs", map[string]any{}},
		{http.MethodPost, "/api/v2/tasks/" + taskID + "/runs/" + runID + "/retry", map[string]any{}},
	} {
		denied := call(test.method, test.path, token, test.body, http.StatusInternalServerError)
		require.Contains(t, denied["message"], "unauthorized")
	}
	call(http.MethodPost, "/api/v2/authorizations", token, map[string]any{"orgID": orgID, "permissions": []any{map[string]any{"action": "write", "resource": map[string]any{"type": "tasks", "orgID": orgID}}}}, http.StatusUnauthorized)
	unchanged := call(http.MethodGet, "/api/v2/tasks/"+taskID, bootstrap, nil, http.StatusOK)
	require.Equal(t, "active", unchanged["status"])
	creds := mcp.Credentials{"base_url": baseURL, "org_id": orgID, "api_token": token, "ca_cert": strings.ReplaceAll(certificate, "\n", `\n`)}
	adapter := influxdb.New()
	require.NoError(t, adapter.Configure(t.Context(), creds))
	require.True(t, adapter.Healthy(t.Context()))
	for _, tool := range []mcp.ToolName{"influxdb_get_task", "influxdb_list_task_runs"} {
		result, err := adapter.Execute(t.Context(), tool, map[string]any{"task_id": otherTask})
		require.NoError(t, err)
		require.True(t, result.IsError, "out-of-scope task must fail closed")
	}
	for _, test := range []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"influxdb_list_tasks", nil}, {"influxdb_get_task", map[string]any{"task_id": taskID}}, {"influxdb_list_task_runs", map[string]any{"task_id": taskID}}, {"influxdb_get_task_run", map[string]any{"task_id": taskID, "run_id": runID}},
	} {
		result, err := adapter.Execute(t.Context(), test.tool, test.args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		require.NotContains(t, result.Data, "SECRET_SENTINEL")
	}
	return liveProvision{Credentials: map[string]string(creds), Values: map[string]string{"org_id": orgID, "task_id": taskID, "run_id": runID}}
}
