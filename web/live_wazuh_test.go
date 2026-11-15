package web

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/wazuh"
	"github.com/stretchr/testify/require"
)

const liveWazuhImage = "wazuh/wazuh-manager:4.14.8@sha256:412f665c77af5497780d29d0d9f069d7a169c5eca45e9a3a47a587d9cfc99b55"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "wazuh", Tier: liveContainer, Adapter: wazuh.New(), Provision: liveWazuhProvision,
		Cases: []liveCase{
			{Tool: "wazuh_list_agents", Expect: []string{"endpoint-one", "never_connected", "not synced", "next_offset"}},
			{Tool: "wazuh_list_agents", Args: map[string]any{"limit": 1}, Expect: []string{"000", "active", "Amazon Linux", "Wazuh v4.14.8", "lastKeepAlive", "next_offset"}},
			{Tool: "wazuh_list_agents", Args: map[string]any{"limit": 1, "offset": 1}, Expect: []string{"endpoint-one", "never_connected", "next_offset"}},
			{Tool: "wazuh_list_agents", Args: map[string]any{"search": "endpoint-one", "status": "never_connected"}, Expect: []string{"endpoint-one", "never_connected", "not synced"}},
			{Tool: "wazuh_get_agent", Args: map[string]any{"agent_id": "${agent_id}"}, Expect: []string{"endpoint-one", "never_connected", "not synced"}},
			{Tool: "wazuh_get_agent", Args: map[string]any{"agent_id": "000"}, Expect: []string{"Amazon Linux", "Wazuh v4.14.8", "lastKeepAlive", "synced"}},
			{Tool: "wazuh_get_agent_status_summary", Expect: []string{"connection", "never_connected", "configuration", "not_synced"}},
		},
	})
}

func liveWazuhProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{Image: liveWazuhImage, Ports: []string{"55000/tcp"}})
	_, port := container.Address(t, "55000/tcp")
	base := "https://localhost:" + port
	certificate := container.Exec(t, "", "bash", "-c", `for attempt in {1..90}; do if test -s /var/ossec/api/configuration/ssl/server.crt; then cat /var/ossec/api/configuration/ssl/server.crt; exit 0; fi; sleep 1; done; exit 1`)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(certificate)))
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(client.CloseIdleConnections)
	login := func(user, password string) (string, error) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/security/user/authenticate", nil)
		if err != nil {
			return "", err
		}
		request.SetBasicAuth(user, password)
		response, err := client.Do(request)
		if err != nil {
			return "", fmt.Errorf("manager not ready")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return "", fmt.Errorf("manager login HTTP %d", response.StatusCode)
		}
		var payload struct {
			Error int `json:"error"`
			Data  struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if json.NewDecoder(response.Body).Decode(&payload) != nil || payload.Error != 0 || payload.Data.Token == "" {
			return "", fmt.Errorf("manager login did not issue a token")
		}
		return payload.Data.Token, nil
	}
	var admin string
	liveEventually(t, 2*time.Minute, func() error { var err error; admin, err = login("wazuh", "wazuh"); return err })
	liveEventually(t, 2*time.Minute, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/manager/status", nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+admin)
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("manager processes not ready")
		}
		defer response.Body.Close()
		var payload struct {
			Data struct {
				Items []map[string]string `json:"affected_items"`
			} `json:"data"`
		}
		if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&payload) != nil || len(payload.Data.Items) != 1 {
			return fmt.Errorf("manager processes not ready")
		}
		for _, daemon := range []string{"wazuh-authd", "wazuh-db", "wazuh-remoted", "wazuh-modulesd"} {
			if payload.Data.Items[0][daemon] != "running" {
				return fmt.Errorf("waiting for %s", daemon)
			}
		}
		return nil
	})
	call := func(token, method, path string, body any, want int) map[string]any {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			require.NoError(t, err)
		}
		request, err := http.NewRequestWithContext(t.Context(), method, base+path, bytes.NewReader(data))
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, want, response.StatusCode, "%s %s", method, path)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
		if want == 200 {
			require.Equal(t, float64(0), payload["error"], "%s %s", method, path)
		}
		return payload
	}
	config := call(admin, http.MethodGet, "/security/config", nil, 200)
	require.Equal(t, "white", config["data"].(map[string]any)["rbac_mode"])
	seedAgent := func(name string) string {
		t.Helper()
		result := call(admin, http.MethodPost, "/agents", map[string]any{"name": name}, 200)
		return result["data"].(map[string]any)["id"].(string)
	}
	agentID, hiddenID := seedAgent("endpoint-one"), seedAgent("endpoint-hidden")
	resources := []string{"agent:id:000", "agent:id:" + agentID}
	for index := range 24 {
		resources = append(resources, "agent:id:"+seedAgent(fmt.Sprintf("endpoint-extra-%02d", index)))
	}
	createID := func(path string, body any) string {
		t.Helper()
		result := call(admin, http.MethodPost, path, body, 200)
		items := result["data"].(map[string]any)["affected_items"].([]any)
		require.Len(t, items, 1)
		return fmt.Sprint(items[0].(map[string]any)["id"])
	}
	policyID := createID("/security/policies", map[string]any{"name": "switchboard-reader", "policy": map[string]any{"actions": []string{"agent:read"}, "resources": resources, "effect": "allow"}})
	roleID := createID("/security/roles", map[string]any{"name": "switchboard-reader"})
	const readerPassword = "Sw1tchboard-ReadOnly!"
	userID := createID("/security/users", map[string]any{"username": "switchboard-reader", "password": readerPassword})
	call(admin, http.MethodPost, "/security/roles/"+roleID+"/policies?policy_ids="+policyID, nil, 200)
	call(admin, http.MethodPost, "/security/users/"+userID+"/roles?role_ids="+roleID, nil, 200)
	user := call(admin, http.MethodGet, "/security/users?user_ids="+userID, nil, 200)["data"].(map[string]any)["affected_items"].([]any)[0].(map[string]any)
	require.Equal(t, false, user["allow_run_as"])
	roles := user["roles"].([]any)
	require.Len(t, roles, 1)
	require.Equal(t, roleID, fmt.Sprint(roles[0]))
	var reader string
	liveEventually(t, 15*time.Second, func() error {
		var err error
		reader, err = login("switchboard-reader", readerPassword)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/agents?limit=1", nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+reader)
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("reader role not ready")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return fmt.Errorf("reader role not ready: HTTP %d", response.StatusCode)
		}
		return nil
	})
	page := call(reader, http.MethodGet, "/agents", nil, 200)["data"].(map[string]any)
	require.Equal(t, float64(26), page["total_affected_items"])
	for _, item := range page["affected_items"].([]any) {
		require.NotEqual(t, hiddenID, item.(map[string]any)["id"])
	}
	summary := call(reader, http.MethodGet, "/agents/summary/status", nil, 200)["data"].(map[string]any)
	require.Equal(t, float64(25), summary["connection"].(map[string]any)["total"])
	require.Equal(t, float64(25), summary["connection"].(map[string]any)["never_connected"])
	require.Equal(t, float64(25), summary["configuration"].(map[string]any)["not_synced"])
	for _, denied := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/agents?agents_list=" + url.QueryEscape(hiddenID), nil},
		{http.MethodPost, "/agents", map[string]any{"name": "denied-agent"}},
		{http.MethodPut, "/agents/restart?agents_list=" + agentID, nil},
		{http.MethodDelete, "/agents?agents_list=" + agentID + "&status=all&older_than=0s", nil},
		{http.MethodPut, "/active-response?agents_list=" + agentID, map[string]any{"command": "restart-wazuh"}},
		{http.MethodPost, "/security/roles", map[string]any{"name": "denied-role"}},
		{http.MethodGet, "/security/config", nil},
		{http.MethodPut, "/security/config", map[string]any{"rbac_mode": "white"}},
	} {
		call(reader, denied.method, denied.path, denied.body, 403)
	}
	require.Empty(t, call(reader, http.MethodGet, "/security/users", nil, 200)["data"].(map[string]any)["affected_items"])
	credentials := map[string]string{"base_url": base, "username": "switchboard-reader", "password": readerPassword, "ca_cert": strings.ReplaceAll(certificate, "\n", `\n`)}
	adapter := wazuh.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	for _, tool := range adapter.Tools() {
		args := map[string]any{}
		if tool.Name == "wazuh_get_agent" {
			args["agent_id"] = agentID
		}
		result, err := adapter.Execute(t.Context(), tool.Name, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		for _, forbidden := range []string{readerPassword, reader, "endpoint-hidden", `"ip"`, "uname", "configSum", "mergedSum", "registerIP"} {
			require.NotContains(t, result.Data, forbidden)
		}
	}
	return liveProvision{Credentials: credentials, Values: map[string]string{"agent_id": agentID}}
}
