package web

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/gatus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const liveGatusImage = "twinproduction/gatus:v5.37.0@sha256:094eb186e55235db367e90e9d56140e5897b7044c41de23cde4cc18e6da1242e"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "gatus", Tier: liveContainer, Adapter: gatus.New(), Provision: liveGatusProvision,
		Cases: []liveCase{
			{Tool: "gatus_list_endpoints", Expect: []string{"live_api", "live_worker", "live_http", "total_fetched", "duration_ms", "truncated"}},
			{Tool: "gatus_list_endpoints", Args: map[string]any{"group": "live", "name": "api", "only_unhealthy": true, "limit": 1}, Expect: []string{"live_api", "false", "matching_count"}},
			{Tool: "gatus_list_endpoints", Args: map[string]any{"limit": 1}, Expect: []string{"live_api", "\"truncated\":true"}},
			{Tool: "gatus_get_endpoint", Args: map[string]any{"key": "live_api", "page_size": 2}, Expect: []string{"live_api", "duration_ms", "false", "true"}},
			{Tool: "gatus_get_endpoint", Args: map[string]any{"key": "live_api", "page": 2, "page_size": 1}, Expect: []string{"live_api", "\"page\":2", "true"}},
			{Tool: "gatus_get_endpoint", Args: map[string]any{"key": "live_http"}, Expect: []string{"live_http", "200", "true"}},
			{Tool: "gatus_get_uptime", Args: map[string]any{"key": "live_worker"}, Expect: []string{"live_worker", "24h", "\"ratio\":1", "no observations"}},
			{Tool: "gatus_get_uptime", Args: map[string]any{"key": "live_api", "duration": "1h"}, Expect: []string{"live_api", "1h", "two hours"}},
			{Tool: "gatus_get_uptime", Args: map[string]any{"key": "live_api", "duration": "7d"}, Expect: []string{"live_api", "7d", "ratio"}},
			{Tool: "gatus_get_uptime", Args: map[string]any{"key": "live_api", "duration": "30d"}, Expect: []string{"live_api", "30d", "ratio"}},
		},
	})
}

func liveGatusProvision(t *testing.T) liveProvision {
	var credentials map[string]string
	for _, storage := range []string{"sqlite", "memory"} {
		credentials = liveGatusProvider(t, storage)
		adapter := gatus.New()
		t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
		require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials(credentials)))
		require.True(t, adapter.Healthy(t.Context()), storage)
		for _, c := range []struct {
			tool mcp.ToolName
			args map[string]any
		}{
			{"gatus_list_endpoints", nil},
			{"gatus_get_endpoint", map[string]any{"key": "live_api"}},
			{"gatus_get_uptime", map[string]any{"key": "live_worker"}},
		} {
			result, err := adapter.Execute(t.Context(), c.tool, c.args)
			require.NoError(t, err)
			require.False(t, result.IsError, "%s: %s", storage, result.Data)
			for _, forbidden := range []string{"SECRET_SENTINEL", "hostname", "conditionResults", "errors", "events", "seed-only-token"} {
				require.NotContains(t, result.Data, forbidden)
			}
		}
		for _, tool := range []mcp.ToolName{"gatus_get_endpoint", "gatus_get_uptime"} {
			result, err := adapter.Execute(t.Context(), tool, map[string]any{"key": "missing"})
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.Contains(t, result.Data, "404")
		}
	}
	return liveProvision{Credentials: credentials}
}

func liveGatusProvider(t *testing.T, storage string) map[string]string {
	password := "live-reader-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	configuration := fmt.Sprintf(`security:
  basic:
    username: reader
    password-bcrypt-base64: "%s"
storage:
  type: %s
  path: /data.db
  maximum-number-of-results: 100
endpoints:
  - name: http
    group: live
    url: http://127.0.0.1:8080/health
    interval: 1s
    conditions:
      - "[STATUS] == 200"
external-endpoints:
  - name: api
    group: live
    token: seed-only-token
  - name: worker
    group: live
    token: seed-only-token
`, base64.URLEncoding.EncodeToString(hash), storage)
	if storage == "memory" {
		configuration = strings.Replace(configuration, "  path: /data.db\n", "", 1)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configuration), 0600))
	id := strings.TrimSpace(liveDocker(t, "create", "--publish", "127.0.0.1::8080/tcp", liveGatusImage))
	require.NotEmpty(t, id)
	container := &liveDockerContainer{id: id}
	t.Cleanup(func() {
		if t.Failed() {
			output, _ := exec.Command("docker", "logs", "--tail", "30", id).CombinedOutput()
			t.Logf("Gatus provider startup: %s", output)
		}
		require.NoError(t, exec.Command("docker", "rm", "--force", id).Run())
	})
	liveDocker(t, "cp", configPath, id+":/config/config.yaml")
	liveDocker(t, "start", id)
	host, port := container.Address(t, "8080/tcp")
	base := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(method, path, auth string) (int, []byte, error) {
		request, err := http.NewRequestWithContext(t.Context(), method, base+path, nil)
		if err != nil {
			return 0, nil, err
		}
		if auth == "basic" {
			request.SetBasicAuth("reader", password)
		} else if auth != "" {
			request.Header.Set("Authorization", "Bearer "+auth)
		}
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer func() { _ = response.Body.Close() }()
		data, err := io.ReadAll(response.Body)
		return response.StatusCode, data, err
	}
	liveEventually(t, time.Minute, func() error {
		status, _, err := call(http.MethodGet, "/health", "")
		if err != nil {
			return err
		}
		if status != 200 {
			return fmt.Errorf("not ready: HTTP %d", status)
		}
		return nil
	})
	for _, seed := range []struct{ key, success string }{
		{"live_api", "false"}, {"live_api", "true"}, {"live_api", "false"}, {"live_worker", "true"},
	} {
		status, _, err := call(http.MethodPost, "/api/v1/endpoints/"+seed.key+"/external?success="+seed.success+"&duration=2ms&error=SECRET_SENTINEL", "seed-only-token")
		require.NoError(t, err)
		require.Equal(t, 200, status)
	}
	for _, path := range []string{"/api/v1/endpoints/statuses", "/api/v1/endpoints/live_api/statuses"} {
		status, _, err := call(http.MethodGet, path, "")
		require.NoError(t, err)
		require.Equal(t, 401, status)
	}
	status, _, err := call(http.MethodPost, "/api/v1/endpoints/live_api/external?success=true", "basic")
	require.NoError(t, err)
	require.Equal(t, 401, status)
	status, _, err = call(http.MethodGet, "/api/v1/endpoints/live_worker/uptimes/24h", "")
	require.NoError(t, err)
	require.Equal(t, 200, status)
	liveEventually(t, 30*time.Second, func() error {
		status, data, err := call(http.MethodGet, "/api/v1/endpoints/live_http/statuses?page=1&pageSize=1", "basic")
		if err != nil {
			return err
		}
		if status != 200 {
			return fmt.Errorf("waiting for monitored endpoint")
		}
		var endpoint struct {
			Results []struct {
				Success bool `json:"success"`
				Status  int  `json:"status"`
			} `json:"results"`
		}
		if err := json.Unmarshal(data, &endpoint); err != nil {
			return err
		}
		if len(endpoint.Results) == 0 || !endpoint.Results[0].Success || endpoint.Results[0].Status != 200 {
			return fmt.Errorf("waiting for real HTTP probe")
		}
		return nil
	})
	target, err := url.Parse(base)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	return map[string]string{"base_url": proxy.URL, "username": "reader", "password": password, "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}
}
