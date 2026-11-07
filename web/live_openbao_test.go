package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/openbao"
	"github.com/stretchr/testify/require"
)

const liveOpenBaoImage = "ghcr.io/openbao/openbao:2.7.2@sha256:5046911075e137e154ce49ab051502f4ec7cf367d484ae4a4dce665d9b90561f"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "openbao", Tier: liveContainer, Adapter: openbao.New(), Provision: liveOpenBaoProvision,
		Cases: []liveCase{
			{Tool: "openbao_list_secret_keys", Args: map[string]any{"limit": 1}, Expect: []string{"alpha", "next_after", "has_more"}},
			{Tool: "openbao_list_secret_keys", Args: map[string]any{"limit": 1, "after": "alpha"}, Expect: []string{"database", "next_after", "has_more"}},
			{Tool: "openbao_list_secret_keys", Args: map[string]any{"limit": 1, "after": "database"}, Expect: []string{"nested/", "has_more"}},
			{Tool: "openbao_list_secret_keys", Args: map[string]any{"path": "nested", "limit": 10}, Expect: []string{"service", "has_more"}},
			{Tool: "openbao_get_secret_metadata", Args: map[string]any{"path": "database"}, Expect: []string{"current_version", "oldest_version", "created_time", "updated_time", "versions", "version", "deletion_time", "destroyed"}},
			{Tool: "openbao_get_health", Expect: []string{"initialized", "sealed", "standby", "2.7.2"}},
		},
	})
}

func liveOpenBaoProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image:      liveOpenBaoImage,
		Env:        map[string]string{"BAO_ADDR": "https://127.0.0.1:8200", "BAO_CACERT": "/tmp/bao/vault-ca.pem", "BAO_TOKEN": "switchboard-live-root"},
		Ports:      []string{"8200/tcp"},
		Entrypoint: "/bin/sh",
		Cmd:        []string{"-c", "mkdir -p /tmp/bao; exec bao server -dev -dev-tls -dev-tls-cert-dir=/tmp/bao -dev-listen-address=0.0.0.0:8200 -dev-root-token-id=switchboard-live-root -dev-no-store-token"},
	})
	host, port := container.Address(t, "8200/tcp")
	base := fmt.Sprintf("https://%s:%s", host, port)
	liveEventually(t, 30*time.Second, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+host+":"+port, nil)
		if err != nil {
			return err
		}
		response, err := (&http.Client{Timeout: time.Second}).Do(request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		return nil
	})
	certificate := container.Exec(t, "", "cat", "/tmp/bao/vault-ca.pem")
	for _, key := range []string{"alpha", "database", "nested/service", "outside/key"} {
		path := "secret/approved/app/" + key
		if key == "outside/key" {
			path = "secret/outside/key"
		}
		container.Exec(t, "", "bao", "kv", "put", path, "password=SECRET_SENTINEL")
	}
	container.Exec(t, "", "bao", "kv", "put", "secret/approved/app/database", "password=SECRET_SENTINEL_SECOND")
	container.Exec(t, "", "bao", "kv", "metadata", "put", "-custom-metadata=note=SECRET_SENTINEL", "secret/approved/app/database")
	policy := `path "secret/metadata/approved/app/*" { capabilities = ["read", "list"] }
path "secret/data/*" { capabilities = ["deny"] }
path "secret/detailed-metadata/*" { capabilities = ["deny"] }
`
	container.Exec(t, policy, "bao", "policy", "write", "switchboard-metadata", "-")
	tokenJSON := container.Exec(t, "", "bao", "token", "create", "-format=json", "-policy=switchboard-metadata", "-no-default-policy", "-ttl=1h", "-renewable=false")
	var tokenResponse struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	require.NoError(t, json.Unmarshal([]byte(tokenJSON), &tokenResponse))
	token := tokenResponse.Auth.ClientToken
	require.NotEmpty(t, token)
	credentials := map[string]string{"base_url": base, "token": token, "mount": "secret", "path_prefix": "approved/app", "ca_cert": strings.ReplaceAll(certificate, "\n", `\n`)}
	adapter := openbao.New()
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	require.True(t, adapter.Healthy(t.Context()), "least-privilege token must verify KV v2 and list access")
	for _, tool := range adapter.Tools() {
		args := map[string]any{}
		if tool.Name == "openbao_get_secret_metadata" {
			args["path"] = "database"
		}
		result, err := adapter.Execute(t.Context(), tool.Name, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		require.NotContains(t, result.Data, "SECRET_SENTINEL")
	}
	checks := []struct{ method, path, body string }{
		{"read", "secret/data/approved/app/database", ""},
		{"read", "secret/metadata/outside/key", ""},
		{"list", "secret/detailed-metadata/approved/app/", ""},
		{"write", "secret/data/approved/app/database", `{"data":{"password":"must-not-write"}}`},
		{"write", "secret/metadata/approved/app/database", `{"custom_metadata":{"note":"must-not-write"}}`},
		{"delete", "secret/metadata/approved/app/database", ""},
	}
	for _, check := range checks {
		command := []string{"sh", "-c", `export BAO_TOKEN="$1"; shift; if bao "$@" >/tmp/denied-output 2>&1; then echo unexpected-success; else cat /tmp/denied-output; fi`, "sh", token, check.method, check.path}
		if check.body != "" {
			command = append(command, "-")
		}
		output := container.Exec(t, check.body, command...)
		require.Contains(t, output, "Code: 403", check.method+" "+check.path)
	}
	parsed, err := url.Parse(base)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", parsed.Hostname())
	return liveProvision{Credentials: credentials}
}
