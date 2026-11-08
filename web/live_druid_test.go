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

	"github.com/daltoniam/switchboard/integrations/druid"
	"github.com/stretchr/testify/require"
)

const liveDruidImage = "apache/druid:38.0.0@sha256:4156a8ca87b855ff6c14f9d95529149b64fe56e25cb394d3043957efae0b9054"
const liveDruidPostgresImage = "postgres:17.6@sha256:00bc86618629af00d2937fdc5a5d63db3ff8450acf52f0636ec813c7f4902929"
const liveDruidZookeeperImage = "zookeeper:3.5.10@sha256:abf67ca6ebb02a8ba0a81683a13210d18eebc6282c1d8b1c686bf3397e01bb57"

func init() {
	registerLiveSpec(liveSpec{Integration: "druid", Tier: liveContainer, Adapter: druid.New(), Provision: liveDruidProvision, Cases: []liveCase{
		{Tool: "druid_list_datasources", Expect: []string{"switchboard_events"}},
		{Tool: "druid_get_datasource", Args: map[string]any{"datasource": "switchboard_events"}, Expect: []string{"switchboard_events", `"count":1`, `"minTime":`}},
		{Tool: "druid_get_load_status", Args: map[string]any{"datasource": "switchboard_events"}, Expect: []string{"switchboard_events", `"percent_loaded":100`}},
		{Tool: "druid_list_tasks", Args: map[string]any{"datasource": "switchboard_events"}, Expect: []string{"switchboard-live-success", "SUCCESS", "switchboard-live-failure", "FAILED"}},
		{Tool: "druid_get_task_status", Args: map[string]any{"task_id": "switchboard-live-failure"}, Expect: []string{"switchboard-live-failure", "FAILED"}},
	}})
}

func liveDruidProvision(t *testing.T) liveProvision {
	network := strings.TrimSpace(liveDocker(t, "network", "create", fmt.Sprintf("switchboard-druid-%d", time.Now().UnixNano())))
	t.Cleanup(func() { _ = exec.Command("docker", "network", "rm", network).Run() })
	volume := strings.TrimSpace(liveDocker(t, "volume", "create"))
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", volume).Run() })
	start := func(image, alias string, environment map[string]string, ports []string, command ...string) *liveDockerContainer {
		args := []string{"run", "--detach", "--rm", "--network", network, "--network-alias", alias}
		if image == liveDruidImage {
			args = append(args, "--volume", volume+":/opt/shared")
		}
		for key, value := range environment {
			args = append(args, "--env", key+"="+value)
		}
		for _, port := range ports {
			args = append(args, "--publish", "127.0.0.1::"+port)
		}
		args = append(args, image)
		args = append(args, command...)
		id := strings.TrimSpace(liveDocker(t, args...))
		t.Cleanup(func() {
			if t.Failed() {
				output, _ := exec.Command("docker", "logs", "--tail", "35", id).CombinedOutput()
				t.Logf("Druid %s: %s", alias, output)
			}
			_ = exec.Command("docker", "rm", "--force", id).Run()
		})
		return &liveDockerContainer{id: id}
	}
	start(liveDruidPostgresImage, "metadata", map[string]string{"POSTGRES_USER": "druid", "POSTGRES_PASSWORD": "metadata-password", "POSTGRES_DB": "druid"}, nil)
	start(liveDruidZookeeperImage, "zookeeper", nil, nil)
	environment := map[string]string{
		"DRUID_SINGLE_NODE_CONF": "micro-quickstart", "DRUID_XMX": "384m", "DRUID_XMS": "128m", "DRUID_MAXDIRECTMEMORYSIZE": "512m", "DRUID_LOG_LEVEL": "warn",
		"druid_extensions_loadList": `["postgresql-metadata-storage","druid-basic-security"]`,
		"druid_zk_service_host":     "zookeeper:2181", "druid_metadata_storage_type": "postgresql", "druid_metadata_storage_connector_connectURI": "jdbc:postgresql://metadata:5432/druid", "druid_metadata_storage_connector_user": "druid", "druid_metadata_storage_connector_password": "metadata-password",
		"druid_storage_type": "local", "druid_storage_storageDirectory": "/opt/shared/segments", "druid_indexer_logs_type": "file", "druid_indexer_logs_directory": "/opt/shared/indexing-logs",
		"druid_processing_numThreads": "1", "druid_processing_numMergeBuffers": "2", "druid_processing_buffer_sizeBytes": "32MiB",
		"druid_coordinator_period": "PT1S", "druid_coordinator_period_indexingPeriod": "PT1S", "druid_coordinator_startDelay": "PT1S", "druid_manager_segments_pollDuration": "PT1S",
		"druid_router_managementProxy_enabled": "true",
		"druid_auth_authenticatorChain":        `["basic"]`, "druid_auth_authenticator_basic_type": "basic", "druid_auth_authenticator_basic_initialAdminPassword": "bootstrap-password", "druid_auth_authenticator_basic_initialInternalClientPassword": "internal-password", "druid_auth_authenticator_basic_credentialsValidator_type": "metadata", "druid_auth_authenticator_basic_authorizerName": "basic",
		"druid_auth_authorizers": `["basic"]`, "druid_auth_authorizer_basic_type": "basic", "druid_escalator_type": "basic", "druid_escalator_internalClientUsername": "druid_system", "druid_escalator_internalClientPassword": "internal-password", "druid_escalator_authorizerName": "basic",
		"druid_auth_basic_common_pollingPeriod": "1000", "druid_auth_basic_common_maxRandomDelay": "0",
		"druid_indexer_runner_javaOptsArray":                            `["-server","-Xmx256m","-Xms128m","-XX:MaxDirectMemorySize=256m","-Duser.timezone=UTC","-Dfile.encoding=UTF-8"]`,
		"druid_indexer_fork_property_druid_processing_buffer_sizeBytes": "32MiB", "druid_worker_capacity": "1",
	}
	coordinator := start(liveDruidImage, "coordinator", environment, []string{"8081/tcp"}, "coordinator")
	host, port := coordinator.Address(t, "8081/tcp")
	admin := &liveDruidAPI{t: t, base: fmt.Sprintf("http://%s:%s", host, port), username: "admin", password: "bootstrap-password", client: &http.Client{Timeout: 15 * time.Second}}
	liveEventually(t, 180*time.Second, func() error { _, _, err := admin.call("GET", "/druid/coordinator/v1/isLeader", nil); return err })
	for _, role := range []string{"historical", "middleManager", "broker"} {
		start(liveDruidImage, strings.ToLower(role), environment, nil, role)
	}
	router := start(liveDruidImage, "router", environment, []string{"8888/tcp"}, "router")
	host, port = router.Address(t, "8888/tcp")
	routerURL := fmt.Sprintf("http://%s:%s", host, port)
	admin.must("POST", "/druid-ext/basic-security/authentication/db/basic/users/diagnostic", nil)
	admin.must("POST", "/druid-ext/basic-security/authentication/db/basic/users/diagnostic/credentials", map[string]any{"password": "diagnostic-password"})
	admin.must("POST", "/druid-ext/basic-security/authorization/db/basic/users/diagnostic", nil)
	admin.must("POST", "/druid-ext/basic-security/authorization/db/basic/roles/diagnostic", nil)
	admin.must("POST", "/druid-ext/basic-security/authorization/db/basic/roles/diagnostic/permissions", []map[string]any{
		{"resource": map[string]string{"name": "switchboard_events", "type": "DATASOURCE"}, "action": "READ"},
		{"resource": map[string]string{"name": "STATE", "type": "STATE"}, "action": "READ"},
	})
	admin.must("POST", "/druid-ext/basic-security/authorization/db/basic/users/diagnostic/roles/diagnostic", nil)
	admin.base = routerURL
	liveEventually(t, 180*time.Second, func() error {
		_, _, err := admin.call("GET", "/druid/indexer/v1/tasks?state=complete", nil)
		return err
	})
	for _, seed := range []struct{ id, data string }{
		{"switchboard-live-success", `{"timestamp":"2026-01-01T00:00:00Z","value":42}`},
		{"switchboard-live-failure", `{"timestamp":"not-a-timestamp","value":42}`},
	} {
		task := map[string]any{"type": "index_parallel", "id": seed.id, "spec": map[string]any{
			"dataSchema":   map[string]any{"dataSource": "switchboard_events", "timestampSpec": map[string]any{"column": "timestamp", "format": "iso"}, "dimensionsSpec": map[string]any{"dimensions": []string{}}, "metricsSpec": []map[string]any{{"type": "longSum", "name": "value", "fieldName": "value"}}, "granularitySpec": map[string]any{"type": "uniform", "segmentGranularity": "DAY", "queryGranularity": "NONE", "rollup": false, "intervals": []string{"2026-01-01/2026-01-02"}}},
			"ioConfig":     map[string]any{"type": "index_parallel", "inputSource": map[string]string{"type": "inline", "data": seed.data}, "inputFormat": map[string]string{"type": "json"}},
			"tuningConfig": map[string]any{"type": "index_parallel", "maxNumConcurrentSubTasks": 1, "maxParseExceptions": 0},
		}}
		admin.must("POST", "/druid/indexer/v1/task", task)
		expected := "SUCCESS"
		if strings.Contains(seed.id, "failure") {
			expected = "FAILED"
		}
		liveEventually(t, 180*time.Second, func() error {
			body, _, err := admin.call("GET", "/druid/indexer/v1/task/"+seed.id+"/status", nil)
			if err != nil {
				return err
			}
			var status struct {
				Status struct {
					Status string `json:"status"`
				} `json:"status"`
			}
			if err = json.Unmarshal(body, &status); err != nil {
				return err
			}
			if status.Status.Status != expected {
				return fmt.Errorf("task %s status %s, expected %s", seed.id, status.Status.Status, expected)
			}
			return nil
		})
	}
	diagnostic := &liveDruidAPI{t: t, base: routerURL, username: "diagnostic", password: "diagnostic-password", client: admin.client}
	liveEventually(t, 120*time.Second, func() error {
		body, _, err := diagnostic.call("GET", "/druid/coordinator/v1/loadstatus", nil)
		if err != nil {
			return err
		}
		var loads map[string]float64
		if err = json.Unmarshal(body, &loads); err != nil {
			return err
		}
		if loads["switchboard_events"] != 100 {
			return fmt.Errorf("seeded segments not loaded")
		}
		return nil
	})
	for _, denied := range []struct {
		method, route string
		payload       any
	}{
		{"POST", "/druid/indexer/v1/task", map[string]any{"type": "noop", "dataSource": "switchboard_events"}},
		{"POST", "/druid/indexer/v1/task/switchboard-live-success/shutdown", nil},
		{"POST", "/druid/coordinator/v1/config", map[string]any{}},
		{"GET", "/druid-ext/basic-security/authentication/db/basic/users", nil},
	} {
		_, status, err := diagnostic.call(denied.method, denied.route, denied.payload)
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, status, "least privilege: %s %s", denied.method, denied.route)
	}
	target, err := url.Parse(routerURL)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	return liveProvision{Credentials: map[string]string{"base_url": proxy.URL, "username": "diagnostic", "password": "diagnostic-password", "ca_cert": strings.ReplaceAll(string(certificate), "\n", `\n`)}}
}

type liveDruidAPI struct {
	t                        *testing.T
	base, username, password string
	client                   *http.Client
}

func (api *liveDruidAPI) call(method, route string, payload any) ([]byte, int, error) {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
	}
	request, err := http.NewRequestWithContext(api.t.Context(), method, api.base+route, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	request.SetBasicAuth(api.username, api.password)
	request.Header.Set("Content-Type", "application/json")
	response, err := api.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return data, response.StatusCode, fmt.Errorf("Druid %s %s: HTTP %d: %s", method, route, response.StatusCode, data)
	}
	return data, response.StatusCode, nil
}
func (api *liveDruidAPI) must(method, route string, payload any) []byte {
	api.t.Helper()
	body, _, err := api.call(method, route, payload)
	require.NoError(api.t, err)
	return body
}
