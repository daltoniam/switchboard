package kubernetes

import (
	"context"
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestName(t *testing.T) {
	i := New()
	assert.Equal(t, "kubernetes", i.Name())
}

func TestHasCredentials_RequiresExplicitConnectionMode(t *testing.T) {
	i := New().(*kubernetes)
	assert.False(t, i.HasCredentials(mcp.Credentials{"namespace": "default"}))
	assert.True(t, i.HasCredentials(mcp.Credentials{"kubeconfig_path": "~/.kube/config"}))
	assert.True(t, i.HasCredentials(mcp.Credentials{"in_cluster": "true"}))
}

func TestConfigure_RequiresExplicitCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "explicit kubeconfig")
}

func TestConfigure_InvalidKubeconfigPath(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"kubeconfig_path": "/does/not/exist"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load kubeconfig")
}

func TestConfigure_InlineKubeconfig(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"kubeconfig": testKubeconfig(t),
		"context":    "test",
		"namespace":  "apps",
	})
	require.NoError(t, err)
	k := i.(*kubernetes)
	assert.Equal(t, "apps", k.namespace)
}

func TestConfigure_ServiceAccountToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"api_server": "https://kubernetes.example.test",
		"token":      "token",
		"namespace":  "default",
	})
	require.NoError(t, err)
	k := i.(*kubernetes)
	assert.Equal(t, "default", k.namespace)
}

func TestTools(t *testing.T) {
	i := New()
	tls := i.Tools()
	assert.Len(t, tls, 12)
	for _, tool := range tls {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.Contains(t, tool.Name, "kubernetes_")
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	k := &kubernetes{client: fake.NewSimpleClientset(), namespace: "default"}
	result, err := k.Execute(context.Background(), "kubernetes_nope", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestHealthy(t *testing.T) {
	k := &kubernetes{client: fake.NewSimpleClientset()}
	assert.True(t, k.Healthy(context.Background()))
}

func TestListNamespaces(t *testing.T) {
	k := &kubernetes{client: fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "apps"}},
	)}
	result, err := k.Execute(context.Background(), "kubernetes_list_namespaces", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var namespaces []namespaceSummary
	require.NoError(t, json.Unmarshal([]byte(result.Data), &namespaces))
	require.Len(t, namespaces, 2)
	assert.ElementsMatch(t, []string{"default", "apps"}, []string{namespaces[0].Name, namespaces[1].Name})
}

func TestListPods_DefaultsToConfiguredNamespace(t *testing.T) {
	k := &kubernetes{client: fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "apps", Labels: map[string]string{"app": "api"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "data"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
	), namespace: "apps"}
	result, err := k.Execute(context.Background(), "kubernetes_list_pods", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var pods []podSummary
	require.NoError(t, json.Unmarshal([]byte(result.Data), &pods))
	require.Len(t, pods, 1)
	assert.Equal(t, "api", pods[0].Name)
	assert.Equal(t, "Running", pods[0].Phase)
}

func TestReadPodLogs(t *testing.T) {
	k := &kubernetes{client: fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "apps"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}},
	)}
	result, err := k.Execute(context.Background(), "kubernetes_read_pod_logs", map[string]any{
		"namespace": "apps",
		"pod":       "api",
		"tail":      10,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "fake logs")
}

func TestListContexts(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"kubeconfig": testKubeconfig(t)})
	require.NoError(t, err)
	result, err := i.Execute(context.Background(), "kubernetes_list_contexts", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var contexts []contextSummary
	require.NoError(t, json.Unmarshal([]byte(result.Data), &contexts))
	require.Len(t, contexts, 1)
	assert.Equal(t, "test", contexts[0].Name)
	assert.True(t, contexts[0].Current)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compact spec %s has no dispatch handler", name)
	}
}

func testKubeconfig(t *testing.T) string {
	t.Helper()
	cfg := api.Config{
		Clusters: map[string]*api.Cluster{
			"test": {Server: "https://kubernetes.example.test", InsecureSkipTLSVerify: true},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"test": {Token: "token"},
		},
		Contexts: map[string]*api.Context{
			"test": {Cluster: "test", AuthInfo: "test", Namespace: "default"},
		},
		CurrentContext: "test",
	}
	data, err := clientcmd.Write(cfg)
	require.NoError(t, err)
	return string(data)
}
