package web

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/ntfy"
	"github.com/stretchr/testify/require"
)

const liveNtfyImage = "binwiederhier/ntfy:v2.30.0@sha256:8563a39da2f5a47360618b2ab9200239c76b37f4145f6d627d9ab4368abd23f0"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "ntfy", Tier: liveContainer, Adapter: ntfy.New(), Provision: liveNtfyProvision,
		Cases: []liveCase{
			{Tool: "ntfy_list_messages", Expect: []string{"${deployment_id}", "Deployment succeeded", "Production rollout healthy", "ops_notifications", "latest", "truncat"}},
			{Tool: "ntfy_list_messages", Args: map[string]any{"since": "${since}"}, Expect: []string{"${backup_id}", "${backup_pending_id}", "${maintenance_id}", "${deployment_id}", "Backup completed", "Backup pending", "Maintenance completed", "Deployment succeeded", "truncat"}},
			{Tool: "ntfy_list_messages", Args: map[string]any{"since": "${since}", "priority": 4, "tags": "backup", "limit": 1}, Expect: []string{"${backup_id}", "Backup completed", "Snapshot saved", "backup", "truncat"}},
			{Tool: "ntfy_list_messages", Args: map[string]any{"since": "${since}", "limit": 1}, Expect: []string{"truncat", "true", "ops_notifications"}},
			{Tool: "ntfy_get_message", Args: map[string]any{"id": "${backup_id}", "since": "${since}"}, Expect: []string{"${backup_id}", "Backup completed", "Snapshot saved", "backup", "ops_notifications", "truncat"}},
		},
	})
}

func liveNtfyTLS(t *testing.T) (string, []byte, *http.Client) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	certificate := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "ntfy live acceptance"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(2 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	require.NoError(t, err)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), cert, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(cert))
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return dir, cert, client
}

func liveNtfyProvision(t *testing.T) liveProvision {
	dir, cert, client := liveNtfyTLS(t)
	configuration := `listen-http: "-"
listen-https: ":8443"
key-file: /tmp/tls.key
cert-file: /tmp/tls.crt
auth-file: /tmp/auth.db
auth-default-access: deny-all
cache-file: /tmp/cache.db
cache-duration: 2h
enable-login: true
visitor-request-limit-burst: 200
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.yml"), []byte(configuration), 0600))
	id := strings.TrimSpace(liveDocker(t, "create", "--publish", "127.0.0.1::8443/tcp", liveNtfyImage, "serve", "--config", "/tmp/server.yml"))
	require.NotEmpty(t, id)
	container := &liveDockerContainer{id: id}
	t.Cleanup(func() {
		if t.Failed() {
			logs, _ := exec.Command("docker", "logs", "--tail", "40", id).CombinedOutput()
			t.Logf("ntfy provider logs: %s", logs)
		}
		require.NoError(t, exec.Command("docker", "rm", "--force", id).Run())
	})
	for _, name := range []string{"server.yml", "tls.crt", "tls.key"} {
		liveDocker(t, "cp", filepath.Join(dir, name), id+":/tmp/"+name)
	}
	liveDocker(t, "start", id)
	host, port := container.Address(t, "8443/tcp")
	base := fmt.Sprintf("https://%s:%s", host, port)
	call := func(method, path, username, password, token string, body any) (int, http.Header, []byte, error) {
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			if err != nil {
				return 0, nil, nil, err
			}
		}
		request, err := http.NewRequestWithContext(t.Context(), method, base+path, bytes.NewReader(data))
		if err != nil {
			return 0, nil, nil, err
		}
		if username != "" {
			request.SetBasicAuth(username, password)
		} else if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, nil, err
		}
		defer func() { _ = response.Body.Close() }()
		data, err = io.ReadAll(response.Body)
		return response.StatusCode, response.Header, data, err
	}
	liveEventually(t, time.Minute, func() error {
		status, _, _, err := call(http.MethodGet, "/v1/health", "", "", "", nil)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("ntfy health: HTTP %d", status)
		}
		return nil
	})
	for _, account := range []struct{ username, password, role string }{
		{"live-seed", "live-seed-password", "admin"},
		{"switchboard-reader", "live-reader-password", "user"},
	} {
		liveDocker(t, "exec", "--env", "NTFY_PASSWORD="+account.password, id, "ntfy", "user", "--config", "/tmp/server.yml", "add", "--role="+account.role, account.username)
	}
	container.Exec(t, "", "ntfy", "access", "--config", "/tmp/server.yml", "switchboard-reader", "*", "deny")
	container.Exec(t, "", "ntfy", "access", "--config", "/tmp/server.yml", "switchboard-reader", "ops_notifications", "read-only")
	status, _, tokenData, err := call(http.MethodPost, "/v1/account/token", "switchboard-reader", "live-reader-password", "", map[string]any{"label": "switchboard-live", "expires": time.Now().Add(time.Hour).Unix()})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "%s", tokenData)
	var tokenResponse struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(tokenData, &tokenResponse))
	require.NotEmpty(t, tokenResponse.Token)
	expiry := time.Now().Add(time.Second)
	status, _, expiryData, err := call(http.MethodPost, "/v1/account/token", "switchboard-reader", "live-reader-password", "", map[string]any{"label": "expiry-proof", "expires": expiry.Unix()})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "%s", expiryData)
	var expiredToken struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(expiryData, &expiredToken))
	require.NotEmpty(t, expiredToken.Token)
	time.Sleep(time.Until(expiry.Add(time.Second)))
	status, _, accountData, err := call(http.MethodGet, "/v1/account", "", "", tokenResponse.Token, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	var account struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	require.NoError(t, json.Unmarshal(accountData, &account))
	require.Equal(t, "switchboard-reader", account.Username)
	require.Equal(t, "user", account.Role)
	since := strconv.FormatInt(time.Now().Add(-30*time.Minute).Unix(), 10)
	values := map[string]string{"since": since}
	for _, seed := range []struct {
		key      string
		title    string
		message  string
		priority int
		tags     []string
	}{
		{"routine_id", "Routine check", "Cache can retain multiple messages", 3, []string{"routine"}},
		{"backup_id", "Backup completed " + strings.Repeat("T", 128) + "TITLE_OVERFLOW", "Snapshot saved " + strings.Repeat("m", 1024) + "MESSAGE_OVERFLOW", 4, []string{"backup", strings.Repeat("t", 64) + "TAG_OVERFLOW", "three", "four", "five", "six", "seven", "eight", "TAG_OVERFLOW"}},
		{"backup_pending_id", "Backup pending", "Snapshot queued", 3, []string{"backup"}},
		{"maintenance_id", "Maintenance completed", "Certificate renewed", 4, []string{"maintenance"}},
		{"deployment_id", "Deployment succeeded", "Production rollout healthy", 5, []string{"deploy", "production"}},
	} {
		if seed.key != "routine_id" {
			time.Sleep(1100 * time.Millisecond)
		}
		body := map[string]any{
			"topic": "ops_notifications", "title": seed.title, "message": seed.message, "priority": seed.priority, "tags": seed.tags,
			"click": "https://SECRET_SENTINEL.invalid/click", "icon": "https://SECRET_SENTINEL.invalid/icon",
			"attach": "https://SECRET_SENTINEL.invalid/attachment", "filename": "SECRET_SENTINEL.txt",
			"actions": []map[string]any{{"action": "http", "label": "SECRET_SENTINEL", "url": "https://SECRET_SENTINEL.invalid/action", "headers": map[string]string{"Authorization": "SECRET_SENTINEL"}}},
		}
		status, _, data, err := call(http.MethodPost, "/", "live-seed", "live-seed-password", "", body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status, "%s", data)
		var message map[string]any
		require.NoError(t, json.Unmarshal(data, &message))
		require.Equal(t, "message", message["event"])
		require.NotEmpty(t, message["id"])
		require.Contains(t, message, "expires")
		for _, key := range []string{"click", "icon", "actions", "attachment"} {
			require.Contains(t, message, key)
		}
		values[seed.key] = message["id"].(string)
	}
	status, _, unrelatedData, err := call(http.MethodPost, "/", "live-seed", "live-seed-password", "", map[string]any{"topic": "unrelated_notifications", "message": "Unrelated private alert"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "%s", unrelatedData)
	status, headers, data, err := call(http.MethodGet, "/ops_notifications/json?poll=1&since="+since, "", "", tokenResponse.Token, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, headers.Get("Content-Type"), "application/x-ndjson")
	scanner := bufio.NewScanner(bytes.NewReader(data))
	count := 0
	for scanner.Scan() {
		var message map[string]any
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &message))
		require.Equal(t, "message", message["event"])
		require.Equal(t, "ops_notifications", message["topic"])
		count++
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, 5, count, "real finite polls return only cached NDJSON message records")
	require.Contains(t, string(data), "SECRET_SENTINEL")
	for _, denial := range []struct {
		name, method, path, token string
		body                      any
		status                    int
	}{
		{"publish", http.MethodPost, "/", tokenResponse.Token, map[string]any{"topic": "ops_notifications", "message": "Denied write"}, http.StatusForbidden},
		{"GET publish", http.MethodGet, "/ops_notifications/publish?message=Denied", tokenResponse.Token, nil, http.StatusForbidden},
		{"unrelated read", http.MethodGet, "/unrelated_notifications/json?poll=1&since=latest", tokenResponse.Token, nil, http.StatusForbidden},
		{"invalid token", http.MethodGet, "/ops_notifications/json?poll=1&since=latest", "tk_00000000000000000000000000000", nil, http.StatusUnauthorized},
		{"expired token", http.MethodGet, "/ops_notifications/json?poll=1&since=latest", expiredToken.Token, nil, http.StatusUnauthorized},
		{"anonymous", http.MethodGet, "/ops_notifications/json?poll=1&since=latest", "", nil, http.StatusForbidden},
	} {
		t.Run(denial.name, func(t *testing.T) {
			status, _, _, err := call(denial.method, denial.path, "", "", denial.token, denial.body)
			require.NoError(t, err)
			require.Equal(t, denial.status, status)
		})
	}
	credentials := map[string]string{"base_url": base, "topic": "ops_notifications", "api_token": tokenResponse.Token, "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}
	adapter := ntfy.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials(credentials)))
	require.True(t, adapter.Healthy(t.Context()))
	for _, c := range []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"ntfy_list_messages", nil},
		{"ntfy_list_messages", map[string]any{"since": since}},
		{"ntfy_list_messages", map[string]any{"since": since, "limit": 1}},
		{"ntfy_list_messages", map[string]any{"since": since, "priority": 4, "tags": "backup", "limit": 1}},
		{"ntfy_get_message", map[string]any{"id": values["backup_id"], "since": since}},
	} {
		result, err := adapter.Execute(t.Context(), c.tool, c.args)
		require.NoError(t, err)
		require.False(t, result.IsError, "%s", result.Data)
		uiNtfyAssertSanitized(t, result.Data)
		require.NotContains(t, result.Data, tokenResponse.Token)
		var value map[string]any
		require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
		messages := value["messages"].([]any)
		if c.args == nil {
			require.Len(t, messages, 1)
			require.Equal(t, values["deployment_id"], messages[0].(map[string]any)["id"])
			require.Equal(t, "ok", value["status"])
			require.Equal(t, false, value["adapter_truncated"])
		} else if c.args["limit"] == 1 || c.tool == "ntfy_get_message" {
			require.Len(t, messages, 1)
			if c.args["priority"] == 4 || c.tool == "ntfy_get_message" {
				require.Equal(t, values["backup_id"], messages[0].(map[string]any)["id"])
			} else {
				require.Equal(t, values["deployment_id"], messages[0].(map[string]any)["id"])
				truncations := value["truncations"].(map[string]any)
				require.Equal(t, true, truncations["message_limit"])
			}
			require.Equal(t, true, value["adapter_truncated"])
		} else {
			require.Len(t, messages, 5)
			require.Equal(t, values["deployment_id"], messages[0].(map[string]any)["id"])
			require.Equal(t, values["maintenance_id"], messages[1].(map[string]any)["id"])
			require.Equal(t, values["backup_pending_id"], messages[2].(map[string]any)["id"])
			require.Equal(t, values["backup_id"], messages[3].(map[string]any)["id"])
			require.Equal(t, values["routine_id"], messages[4].(map[string]any)["id"])
		}
	}
	return liveProvision{Credentials: credentials, Values: values}
}
