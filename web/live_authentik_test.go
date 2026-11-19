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
	"github.com/daltoniam/switchboard/integrations/authentik"
	"github.com/stretchr/testify/require"
)

const liveAuthentikImage = "ghcr.io/goauthentik/server:2026.8.3@sha256:ab9b4e8cc4ab3f8d1198d2db6aeea66bafea1963b3f2843589e0d163f97d9849"
const liveAuthentikPostgresImage = "postgres:16-alpine@sha256:1d5b02d3a9941fbd52437db4107f8c52d6297dac553ca1e5c299ebb3756b4857"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "authentik", Tier: liveContainer, Adapter: authentik.New(), Provision: liveAuthentikProvision,
		Cases: []liveCase{
			{Tool: "authentik_list_users", Expect: []string{"live.ada", "live.grace", "${ada_id}", "${group_id}", "false"}},
			{Tool: "authentik_list_users", Args: map[string]any{"username": "live.ada", "is_active": true}, Expect: []string{"live.ada", "${ada_id}"}},
			{Tool: "authentik_list_users", Args: map[string]any{"search": "live.grace", "is_active": false}, Expect: []string{"live.grace", "false"}},
			{Tool: "authentik_get_user", Args: map[string]any{"user_id": "${ada_id}"}, Expect: []string{"live.ada", "${group_id}"}},
			{Tool: "authentik_get_user", Args: map[string]any{"user_id": "${grace_id}"}, Expect: []string{"live.grace", "false"}},
			{Tool: "authentik_list_user_applications", Args: map[string]any{"user_id": "${ada_id}"}, Expect: []string{"B Allowed", "live-allowed", "candidate-count-before-policy-filter", "${allowed_id}"}},
			{Tool: "authentik_list_user_applications", Args: map[string]any{"user_id": "${ada_id}", "page_size": 1}, Expect: []string{`"results":[]`, `"next":2`, `"count":2`}},
			{Tool: "authentik_list_user_applications", Args: map[string]any{"user_id": "${ada_id}", "page_size": 1, "page": 2}, Expect: []string{"B Allowed", `"current":2`}},
			{Tool: "authentik_list_user_applications", Args: map[string]any{"user_id": "${grace_id}", "search": "Denied"}, Expect: []string{"A Denied", "live-denied"}},
		},
	})
}

type liveAuthentikAPI struct {
	t           *testing.T
	base, token string
	client      *http.Client
}

func (api *liveAuthentikAPI) call(method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(api.t.Context(), method, api.base+"/api/v3/"+path, body)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Authorization", "Bearer "+api.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := api.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	return response.StatusCode, data, err
}

func (api *liveAuthentikAPI) create(path string, payload any) map[string]any {
	api.t.Helper()
	status, data, err := api.call(http.MethodPost, path, payload)
	require.NoError(api.t, err)
	require.Contains(api.t, []int{200, 201, 204}, status, "seed %s: %s", path, data)
	result := map[string]any{}
	if len(data) > 0 {
		require.NoError(api.t, json.Unmarshal(data, &result))
	}
	return result
}

func liveAuthentikProvision(t *testing.T) liveProvision {
	database := liveStartContainer(t, liveDockerOptions{Image: liveAuthentikPostgresImage, Env: map[string]string{"POSTGRES_USER": "authentik", "POSTGRES_DB": "authentik", "POSTGRES_PASSWORD": "live-db-password"}})
	liveEventually(t, time.Minute, func() error {
		return exec.CommandContext(t.Context(), "docker", "exec", database.id, "pg_isready", "-U", "authentik").Run()
	})
	databaseIP := strings.TrimSpace(liveDocker(t, "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", database.id))
	require.NotEmpty(t, databaseIP)
	environment := map[string]string{
		"AUTHENTIK_POSTGRESQL__HOST": databaseIP, "AUTHENTIK_POSTGRESQL__NAME": "authentik", "AUTHENTIK_POSTGRESQL__USER": "authentik", "AUTHENTIK_POSTGRESQL__PASSWORD": "live-db-password",
		"AUTHENTIK_SECRET_KEY": "live-isolated-authentik-secret-key-not-production", "AUTHENTIK_BOOTSTRAP_TOKEN": "live-bootstrap-admin-token", "AUTHENTIK_BOOTSTRAP_PASSWORD": "live-bootstrap-password",
		"AUTHENTIK_ERROR_REPORTING__ENABLED": "false", "AUTHENTIK_DISABLE_UPDATE_CHECK": "true",
	}
	server := liveStartContainer(t, liveDockerOptions{Image: liveAuthentikImage, Env: environment, Ports: []string{"9000/tcp"}, Cmd: []string{"server"}})
	liveStartContainer(t, liveDockerOptions{Image: liveAuthentikImage, Env: environment, Cmd: []string{"worker"}})
	host, port := server.Address(t, "9000/tcp")
	api := &liveAuthentikAPI{t: t, base: fmt.Sprintf("http://%s:%s", host, port), token: "live-bootstrap-admin-token", client: &http.Client{Timeout: 30 * time.Second}}
	liveEventually(t, 4*time.Minute, func() error {
		status, _, err := api.call(http.MethodGet, "core/users/?page_size=1", nil)
		if err != nil {
			return err
		}
		if status != 200 {
			return fmt.Errorf("waiting for authentik bootstrap: HTTP %d", status)
		}
		return nil
	})
	status, schema, err := api.call(http.MethodGet, "schema/?format=json", nil)
	require.NoError(t, err)
	require.Equal(t, 200, status)
	for _, operation := range []string{"/core/users/", "/core/users/{id}/", "/core/applications/", "for_user", "include_groups", "include_roles"} {
		require.Contains(t, string(schema), operation)
	}
	group := api.create("core/groups/", map[string]any{"name": "live-investigated", "is_superuser": false})
	ada := api.create("core/users/", map[string]any{"username": "live.ada", "name": "SECRET_SENTINEL", "email": "SECRET_SENTINEL@example.test", "is_active": true, "groups": []any{group["pk"]}, "attributes": map[string]any{"secret": "SECRET_SENTINEL"}})
	grace := api.create("core/users/", map[string]any{"username": "live.grace", "name": "SECRET_SENTINEL", "is_active": false})
	hidden := api.create("core/users/", map[string]any{"username": "private.user", "name": "SECRET_SENTINEL", "is_active": true})
	reader := api.create("core/users/", map[string]any{"username": "switchboard-reader", "name": "Switchboard reader", "type": "service_account", "is_active": true})
	role := api.create("rbac/roles/", map[string]any{"name": "Switchboard bounded investigator"})
	api.create(fmt.Sprintf("rbac/roles/%v/add_user/", role["pk"]), map[string]any{"pk": reader["pk"]})
	for _, target := range []map[string]any{ada, grace} {
		path := fmt.Sprintf("rbac/permissions/assigned_by_roles/%v/assign/", role["pk"])
		status, data, err := api.call(http.MethodPost, path, map[string]any{"permissions": []string{"authentik_core.view_user", "authentik_core.view_user_applications"}, "model": "authentik_core.user", "object_pk": fmt.Sprint(target["pk"])})
		require.NoError(t, err)
		require.Equal(t, 200, status, "%s", data)
	}
	for _, target := range []struct {
		name, slug string
		user       map[string]any
		hide       bool
	}{
		{"A Denied", "live-denied", grace, false}, {"B Allowed", "live-allowed", ada, false}, {"C Hidden", "live-hidden", ada, true},
	} {
		app := api.create("core/applications/", map[string]any{"name": target.name, "slug": target.slug, "meta_hide": target.hide, "meta_description": "SECRET_SENTINEL"})
		api.create("policies/bindings/", map[string]any{"target": app["pk"], "user": target.user["pk"], "order": 0, "enabled": true, "negate": false, "timeout": 5, "failure_result": false})
		if target.slug == "live-allowed" {
			ada["allowed_id"] = app["pk"]
		}
	}
	api.create("core/tokens/", map[string]any{"identifier": "switchboard-live-reader", "intent": "api", "user": reader["pk"], "expiring": true})
	status, data, err := api.call(http.MethodGet, "core/tokens/switchboard-live-reader/view_key/", nil)
	require.NoError(t, err)
	require.Equal(t, 200, status)
	var key struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(data, &key))
	require.NotEmpty(t, key.Key)
	api.token = key.Key
	for _, target := range []struct {
		method, path string
		payload      any
		statuses     []int
	}{
		{http.MethodGet, fmt.Sprintf("core/users/%v/", hidden["pk"]), nil, []int{403, 404}},
		{http.MethodGet, fmt.Sprintf("core/applications/?for_user=%v", hidden["pk"]), nil, []int{400, 403, 404}},
		{http.MethodPost, "core/users/", map[string]any{"username": "unauthorized", "name": "unauthorized"}, []int{403}},
		{http.MethodPatch, fmt.Sprintf("core/users/%v/", ada["pk"]), map[string]any{"name": "unauthorized"}, []int{403}},
		{http.MethodDelete, fmt.Sprintf("core/users/%v/", ada["pk"]), nil, []int{403}},
		{http.MethodPost, fmt.Sprintf("core/users/%v/set_password/", ada["pk"]), map[string]any{"password": "unauthorized"}, []int{403}},
		{http.MethodPost, fmt.Sprintf("core/users/%v/impersonate/", ada["pk"]), nil, []int{403}},
		{http.MethodGet, "core/tokens/authentik-bootstrap-token/view_key/", nil, []int{403, 404}},
		{http.MethodPost, "core/applications/", map[string]any{"name": "unauthorized", "slug": "unauthorized"}, []int{403}},
	} {
		status, _, err := api.call(target.method, target.path, target.payload)
		require.NoError(t, err)
		require.Contains(t, target.statuses, status, "%s %s must remain denied", target.method, target.path)
	}
	status, data, err = api.call(http.MethodGet, "core/users/?include_groups=false&include_roles=false", nil)
	require.NoError(t, err)
	require.Equal(t, 200, status)
	require.NotContains(t, string(data), "private.user")
	status, data, err = api.call(http.MethodGet, fmt.Sprintf("core/applications/?for_user=%v&superuser_full_list=false", ada["pk"]), nil)
	require.NoError(t, err)
	require.Equal(t, 200, status)
	require.Contains(t, string(data), "B Allowed")
	require.NotContains(t, string(data), "A Denied")
	require.NotContains(t, string(data), "C Hidden")
	target, err := url.Parse(api.base)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	creds := map[string]string{"base_url": proxy.URL, "token": key.Key, "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}
	adapter := authentik.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials(creds)))
	for _, tool := range []mcp.ToolName{"authentik_list_users", "authentik_get_user", "authentik_list_user_applications"} {
		result, err := adapter.Execute(t.Context(), tool, map[string]any{"user_id": fmt.Sprint(ada["pk"])})
		if tool == "authentik_list_users" {
			result, err = adapter.Execute(t.Context(), tool, nil)
		}
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		for _, forbidden := range []string{"SECRET_SENTINEL", "private.user", "provider_obj", "groups_obj", "roles_obj", "launch_url", "attributes", key.Key} {
			require.NotContains(t, result.Data, forbidden)
		}
	}
	return liveProvision{Credentials: creds, Values: map[string]string{"ada_id": fmt.Sprint(ada["pk"]), "grace_id": fmt.Sprint(grace["pk"]), "group_id": fmt.Sprint(group["pk"]), "allowed_id": fmt.Sprint(ada["allowed_id"])}}
}
