package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/argoworkflows"
	"github.com/stretchr/testify/require"
)

const liveArgoImage = "quay.io/argoproj/argocli:v4.1.6@sha256:acb4c1de9a9c5fa109fb3094eebbc3eac42d2dd328181b0a395488cc97de75e5"
const liveArgoKubernetesImage = "rancher/k3s:v1.35.1-k3s1@sha256:634920385dc89133d80060b3a3b2b547e734d711ef8c050e6b5c6341800d53fd"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "argoworkflows", Tier: liveContainer, Adapter: argoworkflows.New(), Provision: liveArgoProvision,
		Cases: []liveCase{
			{Tool: "argoworkflows_list_workflows", Expect: []string{"failed-run", "Failed", "2026-11-01T10:00:00Z"}},
			{Tool: "argoworkflows_get_workflow", Args: map[string]any{"name": "failed-run"}, Expect: []string{"failed-run", "Failed", "0/1", "messages_omitted"}},
		},
	})
}

func liveArgoProvision(t *testing.T) liveProvision {
	kubernetesID := strings.TrimSpace(liveDocker(t, "run", "--detach", "--rm", "--privileged", "--publish", "127.0.0.1::2746/tcp", liveArgoKubernetesImage, "server", "--disable-agent", "--disable", "traefik", "--disable", "servicelb", "--disable", "metrics-server", "--disable", "local-storage", "--disable", "coredns"))
	kubernetes := &liveDockerContainer{id: kubernetesID}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "--force", "--volumes", kubernetesID).Run() })
	liveEventually(t, 90*time.Second, func() error {
		return exec.CommandContext(t.Context(), "docker", "exec", kubernetesID, "kubectl", "get", "--raw", "/readyz").Run()
	})
	apply := func(value any) {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		kubernetes.Exec(t, string(data), "kubectl", "apply", "-f", "-")
	}
	for _, resource := range []struct{ plural, kind, scope string }{
		{"workflows", "Workflow", "Namespaced"},
		{"workflowtemplates", "WorkflowTemplate", "Namespaced"},
		{"clusterworkflowtemplates", "ClusterWorkflowTemplate", "Cluster"},
	} {
		apply(map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
			"metadata": map[string]any{"name": resource.plural + ".argoproj.io"},
			"spec": map[string]any{
				"group": "argoproj.io", "scope": resource.scope,
				"names":    map[string]any{"plural": resource.plural, "singular": strings.ToLower(resource.kind), "kind": resource.kind},
				"versions": []any{map[string]any{"name": "v1alpha1", "served": true, "storage": true, "schema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object", "properties": map[string]any{"spec": map[string]any{"type": "object", "x-kubernetes-preserve-unknown-fields": true}, "status": map[string]any{"type": "object", "x-kubernetes-preserve-unknown-fields": true}}}}}},
			},
		})
	}
	kubernetes.Exec(t, "", "kubectl", "wait", "--for=condition=Established", "--timeout=30s", "crd/workflows.argoproj.io", "crd/workflowtemplates.argoproj.io", "crd/clusterworkflowtemplates.argoproj.io")
	kubernetes.Exec(t, "", "kubectl", "create", "namespace", "approved")
	kubernetes.Exec(t, "", "kubectl", "create", "namespace", "other")
	kubernetes.Exec(t, "", "kubectl", "-n", "approved", "create", "configmap", "workflow-controller-configmap")
	kubernetes.Exec(t, "", "kubectl", "-n", "approved", "create", "serviceaccount", "status-reader")
	kubernetes.Exec(t, "", "kubectl", "-n", "approved", "create", "role", "status-reader", "--verb=get,list", "--resource=workflows.argoproj.io")
	kubernetes.Exec(t, "", "kubectl", "-n", "approved", "create", "rolebinding", "status-reader", "--role=status-reader", "--serviceaccount=approved:status-reader")
	token := strings.TrimSpace(kubernetes.Exec(t, "", "kubectl", "-n", "approved", "create", "token", "status-reader", "--duration=1h"))
	for _, namespace := range []string{"approved", "other"} {
		apply(map[string]any{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Workflow", "metadata": map[string]any{"name": "failed-run", "namespace": namespace},
			"spec":   map[string]any{"entrypoint": "main", "arguments": map[string]any{"parameters": []any{map[string]any{"name": "secret", "value": "secret-sentinel"}}}, "templates": []any{map[string]any{"name": "main", "container": map[string]any{"image": "not-executed", "command": []string{"false"}}}}},
			"status": map[string]any{"phase": "Failed", "startedAt": "2026-11-01T10:00:00Z", "finishedAt": "2026-11-01T10:01:00Z", "progress": "0/1", "message": "secret-sentinel", "nodes": map[string]any{"failed-run": map[string]any{"id": "failed-run", "name": "failed-run", "type": "Pod", "phase": "Failed", "message": "secret-sentinel", "startedAt": "2026-11-01T10:00:00Z", "finishedAt": "2026-11-01T10:01:00Z"}}},
		})
	}
	certificate, key := liveArgoCertificate(t)
	apply(map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "argo-server-tls", "namespace": "approved"}, "type": "kubernetes.io/tls", "data": map[string]any{"tls.crt": base64.StdEncoding.EncodeToString(certificate), "tls.key": base64.StdEncoding.EncodeToString(key)}})
	home := t.TempDir()
	kubeconfig := filepath.Join(home, "kubeconfig")
	contents := kubernetes.Exec(t, "", "cat", "/etc/rancher/k3s/k3s.yaml")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(contents), 0644))
	serverID := strings.TrimSpace(liveDocker(t, "create", "--network", "container:"+kubernetesID, "--env", "KUBECONFIG=/tmp/kubeconfig", liveArgoImage, "server", "--kubeconfig", "/tmp/kubeconfig", "--namespace", "approved", "--namespaced", "--auth-mode", "client", "--tls-certificate-secret-name", "argo-server-tls"))
	t.Cleanup(func() {
		if t.Failed() {
			logs, _ := exec.Command("docker", "logs", "--tail", "40", serverID).CombinedOutput()
			t.Logf("disposable Argo Server logs: %s", logs)
		}
		_ = exec.Command("docker", "rm", "--force", serverID).Run()
	})
	liveDocker(t, "cp", kubeconfig, serverID+":/tmp/kubeconfig")
	liveDocker(t, "start", serverID)
	host, port := kubernetes.Address(t, "2746/tcp")
	baseURL := "https://" + net.JoinHostPort(host, port)
	creds := map[string]string{"base_url": baseURL, "namespace": "approved", "token": token, "ca_cert": strings.ReplaceAll(string(certificate), "\n", `\n`)}
	adapter := argoworkflows.New()
	require.NoError(t, adapter.Configure(t.Context(), creds))
	liveEventually(t, 60*time.Second, func() error {
		result, err := adapter.Execute(t.Context(), "argoworkflows_list_workflows", map[string]any{"limit": 1})
		if err != nil {
			return err
		}
		if result.IsError {
			return fmt.Errorf("Argo Server is not healthy: %s", result.Data)
		}
		return nil
	})
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(certificate))
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}}, Timeout: 10 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/workflows/other/failed-run"},
		{http.MethodPost, "/api/v1/workflows/approved"},
		{http.MethodDelete, "/api/v1/workflows/approved/failed-run"},
	} {
		request, err := http.NewRequestWithContext(t.Context(), test.method, baseURL+test.path, strings.NewReader(`{"workflow":{"metadata":{"name":"forbidden"},"spec":{"entrypoint":"main","templates":[{"name":"main","container":{"image":"not-executed"}}]}}}`))
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		require.NoError(t, err)
		_ = response.Body.Close()
		require.Equal(t, http.StatusForbidden, response.StatusCode, "%s %s must be denied upstream", test.method, test.path)
	}
	return liveProvision{Credentials: creds}
}

func liveArgoCertificate(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Argo disposable live fixture"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"}}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	encodedKey, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})
}
