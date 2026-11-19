package web

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/growthbook"
	"github.com/stretchr/testify/require"
)

const (
	liveGrowthBookImage          = "growthbook/growthbook:latest@sha256:f7a708cce78ea53e7b80d787de919fccab151188dbac85f89b916c87986b9e07"
	liveGrowthBookMongoImage     = "mongo:8@sha256:05b334b0a7a78fe913caac0cd2e1ecccf4da515d9a3a44be911b6ea3734c17cd"
	liveGrowthBookWarehouseImage = "postgres:16-alpine@sha256:1d5b02d3a9941fbd52437db4107f8c52d6297dac553ca1e5c299ebb3756b4857"
)

func init() {
	registerLiveSpec(liveSpec{
		Integration: "growthbook", Tier: liveContainer, Adapter: growthbook.New(), Provision: liveGrowthBookProvision,
		Cases: []liveCase{
			{Tool: "growthbook_list_features", Expect: []string{"live-checkout-express", "live-beta-banner", "Beta testers only", "${group_id}"}},
			{Tool: "growthbook_get_feature", Args: map[string]any{"feature_id": "live-beta-banner"}, Expect: []string{"live-beta-banner", "rollout", "live-checkout-express", "${group_id}"}},
			{Tool: "growthbook_get_feature_staleness", Args: map[string]any{"feature_ids": "live-checkout-express,live-beta-banner"}, Expect: []string{"live-checkout-express", "staleByEnv"}},
			{Tool: "growthbook_list_feature_revisions", Args: map[string]any{"feature_id": "live-beta-banner", "status": "published"}, Expect: []string{"live-beta-banner", "published", "Created via REST API"}},
			{Tool: "growthbook_list_experiments", Expect: []string{"Live Checkout Test", "Live Banner Test", "won"}},
			{Tool: "growthbook_list_experiments", Args: map[string]any{"status": "stopped"}, Expect: []string{"Live Banner Test", `"total":1`}},
			{Tool: "growthbook_get_experiment", Args: map[string]any{"experiment_id": "${stopped_id}"}, Expect: []string{"Live Banner Test", "live-banner-flow", "Banner won"}},
			{Tool: "growthbook_get_experiment_results", Args: map[string]any{"experiment_id": "${experiment_id}"}, Expect: []string{"Purchase Rate", "chanceToBeatControl", "Express", `"totalUsers":400`}},
			{Tool: "growthbook_refresh_experiment_results", Args: map[string]any{"experiment_id": "${stopped_id}"}, Expect: []string{"snp_", "${stopped_id}"}},
			{Tool: "growthbook_get_snapshot", Args: map[string]any{"snapshot_id": "${snapshot_id}"}, Expect: []string{"${snapshot_id}", "success"}},
			{Tool: "growthbook_list_fact_metrics", Expect: []string{"Purchase Rate", "${fact_table_id}"}},
			{Tool: "growthbook_get_fact_metric", Args: map[string]any{"metric_id": "${metric_id}"}, Expect: []string{"Purchase Rate", "proportion"}},
			{Tool: "growthbook_list_metrics", Expect: []string{"Legacy Purchases", "binomial"}},
			{Tool: "growthbook_list_fact_tables", Expect: []string{"Purchases", "amount"}},
			{Tool: "growthbook_list_projects", Expect: []string{"Live Growth", "${project_id}"}},
			{Tool: "growthbook_list_environments", Expect: []string{"production", "staging"}},
			{Tool: "growthbook_list_sdk_connections", Expect: []string{"Live Web", "sdk-"}},
			{Tool: "growthbook_list_saved_groups", Expect: []string{"Beta Testers", "${group_id}"}},
			{Tool: "growthbook_get_saved_group", Args: map[string]any{"saved_group_id": "${group_id}"}, Expect: []string{"Beta Testers", "live-user-1"}},
			{Tool: "growthbook_list_attributes", Expect: []string{"country", "Visitor country"}},
			{Tool: "growthbook_list_data_sources", Expect: []string{"Live Warehouse", "Experiment Viewed"}},
			{Tool: "growthbook_list_segments", Expect: []string{"Big Spenders"}},
			{Tool: "growthbook_list_audit_logs", Args: map[string]any{"entity_type": "feature", "entity_id": "live-beta-banner"}, Expect: []string{"feature.toggle", "live launch"}},
			{Tool: "growthbook_list_code_refs", Expect: []string{"src/checkout.ts", "acme/web"}},
			{Tool: "growthbook_get_feature_code_refs", Args: map[string]any{"feature_id": "live-checkout-express"}, Expect: []string{"src/checkout.ts", "42"}},
			{Tool: "growthbook_get_settings", Expect: []string{"bayesian", "confidenceLevel"}},
			{Tool: "growthbook_list_members", Expect: []string{"admin@example.test", "admin"}},
			{Tool: "growthbook_toggle_feature", Args: map[string]any{"feature_id": "live-checkout-express", "environment": "production", "enabled": true, "reason": "switchboard live"}, Expect: []string{`"status":"updated"`, `"enabled":true`, "production"}},
			{Tool: "growthbook_create_feature", Args: map[string]any{"feature_id": "live-agent-flag", "value_type": "boolean", "default_value": "false", "description": "Created by the live test", "owner": "admin@example.test", "tags": "agent"}, Expect: []string{"live-agent-flag", `"enabled":false`}},
		},
	})
}

type liveGrowthBookAPI struct {
	t      *testing.T
	base   string
	token  string
	org    string
	client *http.Client
}

func (api *liveGrowthBookAPI) call(method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(api.t.Context(), method, api.base+path, body)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if api.token != "" {
		request.Header.Set("Authorization", "Bearer "+api.token)
	}
	if api.org != "" {
		request.Header.Set("X-Organization", api.org)
	}
	response, err := api.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	return response.StatusCode, data, err
}

func (api *liveGrowthBookAPI) must(method, path string, payload any) map[string]any {
	api.t.Helper()
	status, data, err := api.call(method, path, payload)
	require.NoError(api.t, err)
	require.Equal(api.t, http.StatusOK, status, "seed %s %s: %s", method, path, data)
	result := map[string]any{}
	require.NoError(api.t, json.Unmarshal(data, &result))
	return result
}

func liveObject(value any, key string) map[string]any {
	object, _ := value.(map[string]any)
	inner, _ := object[key].(map[string]any)
	return inner
}

func liveContainerIP(t *testing.T, container *liveDockerContainer) string {
	t.Helper()
	ip := strings.TrimSpace(liveDocker(t, "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", container.id))
	require.NotEmpty(t, ip)
	return ip
}

const liveGrowthBookWarehouseSQL = `
CREATE TABLE experiment_viewed (user_id text, timestamp timestamptz, experiment_id text, variation_id text);
CREATE TABLE purchases (user_id text, timestamp timestamptz, amount numeric);
INSERT INTO experiment_viewed SELECT 'u'||g, now() - interval '2 days' + (g||' seconds')::interval, 'live-checkout-test', (g%2)::text FROM generate_series(1,400) g;
INSERT INTO purchases SELECT 'u'||g, now() - interval '1 day', 10 + g%7 FROM generate_series(1,400) g WHERE (g%2=1 AND g%3<>0) OR (g%2=0 AND g%5=0);
`

func liveGrowthBookProvision(t *testing.T) liveProvision {
	warehouse := liveStartContainer(t, liveDockerOptions{Image: liveGrowthBookWarehouseImage, Env: map[string]string{"POSTGRES_USER": "warehouse_reader", "POSTGRES_PASSWORD": "live-warehouse-password", "POSTGRES_DB": "warehouse"}})
	mongo := liveStartContainer(t, liveDockerOptions{Image: liveGrowthBookMongoImage, Env: map[string]string{"MONGO_INITDB_ROOT_USERNAME": "root", "MONGO_INITDB_ROOT_PASSWORD": "live-mongo-password"}})
	liveEventually(t, time.Minute, func() error {
		return exec.CommandContext(t.Context(), "docker", "exec", warehouse.id, "pg_isready", "-U", "warehouse_reader", "-d", "warehouse").Run()
	})
	liveEventually(t, time.Minute, func() error {
		_, err := exec.CommandContext(t.Context(), "docker", "exec", warehouse.id, "psql", "-U", "warehouse_reader", "-d", "warehouse", "-c", "SELECT 1").CombinedOutput()
		return err
	})
	liveEventually(t, time.Minute, func() error {
		return exec.CommandContext(t.Context(), "docker", "exec", mongo.id, "mongosh", "--quiet", "-u", "root", "-p", "live-mongo-password", "--eval", "db.adminCommand('ping')").Run()
	})
	warehouse.Exec(t, liveGrowthBookWarehouseSQL, "psql", "-v", "ON_ERROR_STOP=1", "-U", "warehouse_reader", "-d", "warehouse")

	server := liveStartContainer(t, liveDockerOptions{Image: liveGrowthBookImage, Ports: []string{"3100/tcp"}, Env: map[string]string{
		"MONGODB_URI":        fmt.Sprintf("mongodb://root:live-mongo-password@%s:27017/growthbook?authSource=admin", liveContainerIP(t, mongo)),
		"JWT_SECRET":         "live-isolated-growthbook-jwt-secret",
		"ENCRYPTION_KEY":     "live-isolated-growthbook-encryption-key",
		"NODE_ENV":           "production",
		"APP_ORIGIN":         "http://localhost:3000",
		"API_HOST":           "http://localhost:3100",
		"API_RATE_LIMIT_MAX": "10000",
	}})
	host, port := server.Address(t, "3100/tcp")
	api := &liveGrowthBookAPI{t: t, base: fmt.Sprintf("http://%s:%s", host, port), client: &http.Client{Timeout: 60 * time.Second}}
	liveEventually(t, 4*time.Minute, func() error {
		status, _, err := api.call(http.MethodGet, "/healthcheck", nil)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("waiting for GrowthBook: HTTP %d", status)
		}
		return nil
	})

	// Bootstrap uses the internal app API, the same calls the setup screens make.
	api.token, _ = api.must(http.MethodPost, "/auth/firsttime", map[string]any{"email": "admin@example.test", "name": "Live Admin", "password": "live-admin-password-1", "companyname": "Switchboard Live"})["token"].(string)
	user := api.must(http.MethodGet, "/user", nil)
	organizations, _ := user["organizations"].([]any)
	require.NotEmpty(t, organizations)
	api.org, _ = organizations[0].(map[string]any)["id"].(string)
	status, data, err := api.call(http.MethodPost, "/keys", map[string]any{"type": "readonly", "description": "least privilege probe"})
	require.NoError(t, err)
	require.Equal(t, http.StatusPaymentRequired, status, "the open-source image only issues admin-role keys: %s", data)
	key, _ := liveObject(api.must(http.MethodPost, "/keys", map[string]any{"type": "admin", "description": "switchboard live"}), "key")["key"].(string)
	require.True(t, strings.HasPrefix(key, "secret_"))
	api.token, api.org = key, ""

	rest := func(method, path string, payload any) map[string]any {
		t.Helper()
		return api.must(method, "/api"+path, payload)
	}
	projects, _ := rest(http.MethodGet, "/v1/projects", nil)["projects"].([]any)
	require.NotEmpty(t, projects)
	projectID, _ := projects[0].(map[string]any)["id"].(string)
	rest(http.MethodPut, "/v1/projects/"+projectID, map[string]any{"name": "Live Growth", "description": "Checkout growth experiments"})
	rest(http.MethodPost, "/v1/environments", map[string]any{"id": "staging", "description": "Staging", "projects": []string{projectID}})
	dataSource := liveObject(rest(http.MethodPost, "/v1/data-sources", map[string]any{
		"name": "Live Warehouse", "projectIds": []string{projectID},
		"connection":      map[string]any{"type": "postgres", "host": liveContainerIP(t, warehouse), "port": 5432, "database": "warehouse", "user": "warehouse_reader", "password": "live-warehouse-password", "defaultSchema": "public"},
		"identifierTypes": []any{map[string]any{"id": "user_id"}},
		"assignmentQueries": []any{map[string]any{"name": "Experiment Viewed", "identifierTypes": []string{"user_id"},
			"sql": "SELECT user_id, timestamp, experiment_id, variation_id FROM experiment_viewed"}},
	}), "dataSource")
	dataSourceID, _ := dataSource["id"].(string)
	queries, _ := dataSource["assignmentQueries"].([]any)
	require.NotEmpty(t, queries)
	assignmentID, _ := queries[0].(map[string]any)["id"].(string)
	factTableID, _ := liveObject(rest(http.MethodPost, "/v1/fact-tables", map[string]any{"name": "Purchases", "datasource": dataSourceID, "userIdTypes": []string{"user_id"}, "sql": "SELECT user_id, timestamp, amount FROM purchases", "owner": "admin@example.test", "projects": []string{projectID}, "tags": []string{"checkout"}}), "factTable")["id"].(string)
	metricID, _ := liveObject(rest(http.MethodPost, "/v1/fact-metrics", map[string]any{"name": "Purchase Rate", "metricType": "proportion", "numerator": map[string]any{"factTableId": factTableID}, "owner": "admin@example.test", "tags": []string{"checkout"}, "projects": []string{projectID}}), "factMetric")["id"].(string)
	legacyID, _ := liveObject(rest(http.MethodPost, "/v1/metrics", map[string]any{"datasourceId": dataSourceID, "name": "Legacy Purchases", "type": "binomial", "owner": "admin@example.test", "tags": []string{"checkout"}, "projects": []string{projectID}, "sql": map[string]any{"identifierTypes": []string{"user_id"}, "conversionSQL": "SELECT user_id, timestamp FROM purchases"}}), "metric")["id"].(string)
	rest(http.MethodPost, "/v1/segments", map[string]any{"name": "Big Spenders", "datasourceId": dataSourceID, "identifierType": "user_id", "type": "SQL", "query": "SELECT user_id, timestamp as date FROM purchases WHERE amount > 14", "owner": "admin@example.test", "projects": []string{projectID}})
	groupID, _ := liveObject(rest(http.MethodPost, "/v1/saved-groups", map[string]any{"name": "Beta Testers", "type": "list", "attributeKey": "id", "values": []string{"live-user-1", "live-user-2"}, "projects": []string{projectID}}), "savedGroup")["id"].(string)
	rest(http.MethodPost, "/v1/attributes", map[string]any{"property": "country", "datatype": "string", "description": "Visitor country", "projects": []string{projectID}})
	rest(http.MethodPost, "/v1/sdk-connections", map[string]any{"name": "Live Web", "language": "javascript", "environment": "production", "projects": []string{projectID}})

	disabled := map[string]any{"production": map[string]any{"enabled": false}, "staging": map[string]any{"enabled": false}}
	rest(http.MethodPost, "/v2/features", map[string]any{"id": "live-checkout-express", "description": "Express checkout", "owner": "admin@example.test", "project": projectID, "valueType": "boolean", "defaultValue": "false", "tags": []string{"checkout"},
		"environments": map[string]any{"production": map[string]any{"enabled": false}, "staging": map[string]any{"enabled": true}}})
	rest(http.MethodPost, "/v2/features", map[string]any{"id": "live-beta-banner", "description": "Beta banner", "owner": "admin@example.test", "project": projectID, "valueType": "string", "defaultValue": "off", "tags": []string{"growth"},
		"prerequisites": []string{"live-checkout-express"}, "environments": disabled,
		"rules": []any{
			map[string]any{"type": "force", "description": "Beta testers only", "value": "on", "savedGroups": []any{map[string]any{"match": "any", "ids": []string{groupID}}}, "environments": []string{"production"}},
			map[string]any{"type": "rollout", "description": "10% rollout", "value": "on", "coverage": 0.1, "hashAttribute": "id", "environments": []string{"production"}},
		}})

	experiment := func(trackingKey, name string) map[string]any {
		return liveObject(rest(http.MethodPost, "/v1/experiments", map[string]any{
			"datasourceId": dataSourceID, "assignmentQueryId": assignmentID, "trackingKey": trackingKey, "name": name, "hypothesis": name + " raises conversion",
			"owner": "admin@example.test", "project": projectID, "metrics": []string{metricID}, "secondaryMetrics": []string{legacyID}, "guardrailMetrics": []string{metricID},
			"tags": []string{"checkout"}, "status": "running",
			"variations": []any{map[string]any{"key": "0", "name": "Control"}, map[string]any{"key": "1", "name": "Express"}},
			"phases":     []any{map[string]any{"name": "Main", "dateStarted": time.Now().UTC().Add(-5 * 24 * time.Hour).Format(time.RFC3339), "coverage": 1, "variationWeights": []float64{0.5, 0.5}}},
		}), "experiment")
	}
	linkFeature := func(featureID string, target map[string]any, values [2]string) {
		variations, _ := target["variations"].([]any)
		require.Len(t, variations, 2)
		refs := make([]any, 0, 2)
		for index, variation := range variations {
			refs = append(refs, map[string]any{"variationId": variation.(map[string]any)["variationId"], "value": values[index]})
		}
		rest(http.MethodPost, "/v2/features", map[string]any{"id": featureID, "description": "Experiment flag", "owner": "admin@example.test", "project": projectID, "valueType": "string", "defaultValue": values[0], "environments": disabled,
			"rules": []any{map[string]any{"type": "experiment-ref", "experimentId": target["id"], "variations": refs, "environments": []string{"production"}}}})
	}
	running := experiment("live-checkout-test", "Live Checkout Test")
	stopped := experiment("live-banner-test", "Live Banner Test")
	rest(http.MethodPost, fmt.Sprintf("/v1/experiments/%v/stop", stopped["id"]), map[string]any{"results": "won", "analysis": "Banner won", "reason": "Reached significance"})
	linkFeature("live-checkout-flow", running, [2]string{"classic", "express"})
	linkFeature("live-banner-flow", stopped, [2]string{"none", "banner"})

	snapshotID, _ := liveObject(rest(http.MethodPost, fmt.Sprintf("/v1/experiments/%v/snapshot", running["id"]), map[string]any{}), "snapshot")["id"].(string)
	liveEventually(t, 2*time.Minute, func() error {
		state, _ := liveObject(rest(http.MethodGet, "/v1/snapshots/"+snapshotID, nil), "snapshot")["status"].(string)
		if state != "success" {
			return fmt.Errorf("experiment snapshot status %q", state)
		}
		return nil
	})
	// Only the running experiment has a snapshot, so the refresh case on the
	// stopped one can only pass by creating a new analysis.
	status, data, err = api.call(http.MethodGet, fmt.Sprintf("/api/v1/experiments/%v/results", stopped["id"]), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status, "stopped experiment must start without results: %s", data)
	rest(http.MethodPost, "/v2/features/live-beta-banner/toggle", map[string]any{"environments": map[string]any{"production": true}, "reason": "live launch"})
	rest(http.MethodPost, "/v1/code-refs", map[string]any{"branch": "main", "repoName": "acme/web", "refs": []any{map[string]any{"filePath": "src/checkout.ts", "startingLineNumber": 42, "lines": "if (gb.isOn('live-checkout-express')) {", "flagKey": "live-checkout-express", "contentHash": "live"}}})

	target, err := url.Parse(api.base)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	creds := map[string]string{"api_key": key, "base_url": proxy.URL, "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}

	adapter := growthbook.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials(creds)))
	for _, tool := range []mcp.ToolName{"growthbook_list_sdk_connections", "growthbook_get_settings", "growthbook_list_data_sources"} {
		result, err := adapter.Execute(t.Context(), tool, nil)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		for _, forbidden := range []string{"encryptionKey", "proxySigningKey", "secureAttributeSalt", "live-warehouse-password", key} {
			require.NotContains(t, result.Data, forbidden)
		}
	}

	return liveProvision{Credentials: creds, Values: map[string]string{
		"project_id": projectID, "group_id": groupID, "metric_id": metricID, "fact_table_id": factTableID,
		"experiment_id": fmt.Sprint(running["id"]), "stopped_id": fmt.Sprint(stopped["id"]), "snapshot_id": snapshotID,
	}}
}
