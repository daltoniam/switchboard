package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/grafana"
	"github.com/stretchr/testify/require"
)

const liveGrafanaImage = "grafana/grafana:13.3.1@sha256:0615ec4fbd4072004b7d86b46ded63013a74b7d6eb1119273dd89e8fa25c13cf"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "grafana",
		Tier:        liveContainer,
		Adapter:     grafana.New(),
		Provision:   liveGrafanaProvision,
		Cases: []liveCase{
			{Tool: "grafana_search_dashboards", Args: map[string]any{"query": "Live Service"}, Expect: []string{"live-svc", "Live Service", "live-ops", "Live Ops"}},
			{Tool: "grafana_get_dashboard", Args: map[string]any{"uid": "live-svc"}, Expect: []string{"Live Service", "live-ops", "live_up"}},
			{Tool: "grafana_save_dashboard", Args: map[string]any{"dashboard": map[string]any{"uid": "live-new", "title": "Live New", "panels": []any{}}, "folder_uid": "live-ops", "message": "created by the live test"}, Expect: []string{"success", "live-new"}},
			{Tool: "grafana_save_dashboard", Args: map[string]any{
				"dashboard": map[string]any{
					"__inputs": []any{map[string]any{"name": "DS_TESTDATA", "type": "datasource", "pluginId": "grafana-testdata-datasource"}},
					"uid":      "live-imported", "title": "Live Imported",
					"panels": []any{map[string]any{"type": "timeseries", "title": "Walk", "datasource": map[string]any{"uid": "${DS_TESTDATA}"}, "targets": []any{map[string]any{"refId": "A"}}}},
				},
				"inputs": map[string]any{"DS_TESTDATA": "live-testdata"},
			}, Expect: []string{"live-imported", `"imported":true`}},
			// Only a substituted import holds the data source uid; an unresolved
			// ${DS_TESTDATA} would not.
			{Tool: "grafana_get_dashboard", Args: map[string]any{"uid": "live-imported"}, Expect: []string{"Live Imported", "live-testdata"}},
			{Tool: "grafana_list_folders", Args: map[string]any{"parent_uid": "live-ops"}, Expect: []string{"live-k8s", "Live K8s"}},
			{Tool: "grafana_list_datasources", Expect: []string{"live-testdata", "grafana-testdata-datasource"}},
			{Tool: "grafana_query_datasource", Args: map[string]any{"datasource_uid": "live-testdata", "query": map[string]any{"scenarioId": "random_walk", "seriesCount": 1}, "from": "now-10m", "max_data_points": 10}, Expect: []string{`"ref_id":"A"`, "A-series", "values"}},
			{Tool: "grafana_get_health", Expect: []string{"13.3.1", "Main Org."}},
		},
	})
}

// liveGrafanaProvision starts Grafana, seeds a data source, nested folders
// and a dashboard as the bootstrap admin, then hands Switchboard an Editor
// service account token: Editor is the least role that can save dashboards.
func liveGrafanaProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveGrafanaImage,
		Env: map[string]string{
			"GF_SECURITY_ADMIN_PASSWORD":            "live-admin-password",
			"GF_ANALYTICS_REPORTING_ENABLED":        "false",
			"GF_ANALYTICS_CHECK_FOR_UPDATES":        "false",
			"GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES": "false",
			"GF_NEWS_NEWS_FEED_ENABLED":             "false",
			"GF_PLUGINS_PREINSTALL_DISABLED":        "true",
		},
		Ports: []string{"3000/tcp"},
	})
	host, port := container.Address(t, "3000/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 10 * time.Second}
	admin := func(method, path string, body any) map[string]any {
		t.Helper()
		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			require.NoError(t, err)
			reader = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(t.Context(), method, baseURL+path, reader)
		require.NoError(t, err)
		req.SetBasicAuth("admin", "live-admin-password")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Less(t, resp.StatusCode, 300, "%s %s: %s", method, path, data)
		var out map[string]any
		require.NoError(t, json.Unmarshal(data, &out))
		return out
	}
	liveEventually(t, 2*time.Minute, func() error {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/api/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Grafana not ready: %d", resp.StatusCode)
		}
		return nil
	})

	admin(http.MethodPost, "/api/datasources", map[string]any{"name": "Live TestData", "uid": "live-testdata", "type": "grafana-testdata-datasource", "access": "proxy"})
	admin(http.MethodPost, "/api/folders", map[string]any{"uid": "live-ops", "title": "Live Ops"})
	admin(http.MethodPost, "/api/folders", map[string]any{"uid": "live-k8s", "title": "Live K8s", "parentUid": "live-ops"})
	admin(http.MethodPost, "/api/dashboards/db", map[string]any{
		"folderUid": "live-ops",
		"dashboard": map[string]any{
			"uid": "live-svc", "title": "Live Service", "tags": []any{"live"},
			"panels": []any{map[string]any{"type": "timeseries", "title": "Up", "datasource": map[string]any{"uid": "live-testdata"}, "targets": []any{map[string]any{"refId": "A", "alias": "live_up"}}}},
		},
	})
	account := admin(http.MethodPost, "/api/serviceaccounts", map[string]any{"name": "switchboard", "role": "Editor"})
	token := admin(http.MethodPost, fmt.Sprintf("/api/serviceaccounts/%v/tokens", account["id"]), map[string]any{"name": "live"})
	key, _ := token["key"].(string)
	require.NotEmpty(t, key)
	return liveProvision{Credentials: map[string]string{"base_url": baseURL, "service_account_token": key}}
}
