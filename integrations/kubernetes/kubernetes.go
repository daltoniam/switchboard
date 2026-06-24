package kubernetes

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("kubernetes", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.Integration                = (*kubernetesAdapter)(nil)
	_ mcp.FieldCompactionIntegration = (*kubernetesAdapter)(nil)
	_ mcp.PlainTextCredentials       = (*kubernetesAdapter)(nil)
	_ mcp.PlaceholderHints           = (*kubernetesAdapter)(nil)
	_ mcp.OptionalCredentials        = (*kubernetesAdapter)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*kubernetesAdapter)(nil)
)

type kubernetesAdapter = kubernetes

type kubernetes struct {
	client          k8sclient.Interface
	namespace       string
	context         string
	kubeconfigBytes []byte
}

func New() mcp.Integration {
	return &kubernetes{namespace: "default"}
}

func (k *kubernetes) Name() string { return "kubernetes" }

func (k *kubernetes) PlainTextKeys() []string {
	return []string{"kubeconfig", "kubeconfig_path", "context", "namespace", "api_server", "ca_cert", "insecure_skip_tls_verify", "in_cluster"}
}

func (k *kubernetes) Placeholders() map[string]string {
	return map[string]string{
		"kubeconfig_path": "~/.kube/config",
		"namespace":       "default",
	}
}

func (k *kubernetes) OptionalKeys() []string {
	return []string{"kubeconfig", "kubeconfig_path", "context", "namespace", "api_server", "token", "ca_cert", "insecure_skip_tls_verify", "in_cluster"}
}

func (k *kubernetes) HasCredentials(creds mcp.Credentials) bool {
	return explicitCredentials(creds)
}

func (k *kubernetes) Configure(_ context.Context, creds mcp.Credentials) error {
	cfg, data, err := restConfig(creds)
	if err != nil {
		return err
	}
	client, err := k8sclient.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("kubernetes: create client: %w", err)
	}
	k.client = client
	k.namespace = defaultNamespace(creds)
	k.context = creds["context"]
	k.kubeconfigBytes = data
	return nil
}

func (k *kubernetes) contextConfig() ([]byte, error) {
	if len(k.kubeconfigBytes) > 0 {
		return k.kubeconfigBytes, nil
	}
	path := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kubernetes: read kubeconfig: %w", err)
	}
	return data, nil
}

func restConfig(creds mcp.Credentials) (*rest.Config, []byte, error) {
	if !explicitCredentials(creds) {
		return nil, nil, fmt.Errorf("kubernetes: explicit kubeconfig, kubeconfig_path, in_cluster=true, or api_server+token is required")
	}
	if creds["in_cluster"] == "true" {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("kubernetes: in-cluster config: %w", err)
		}
		return cfg, nil, nil
	}
	if creds["api_server"] != "" || creds["token"] != "" {
		cfg, err := serviceAccountConfig(creds)
		return cfg, nil, err
	}
	if creds["kubeconfig"] != "" || creds["kubeconfig_path"] != "" {
		return kubeconfigConfig(creds)
	}
	return nil, nil, fmt.Errorf("kubernetes: kubeconfig, kubeconfig_path, in_cluster=true, or api_server+token is required")
}

func explicitCredentials(creds mcp.Credentials) bool {
	return creds["kubeconfig"] != "" || creds["kubeconfig_path"] != "" || creds["in_cluster"] == "true" || creds["api_server"] != "" || creds["token"] != ""
}

func serviceAccountConfig(creds mcp.Credentials) (*rest.Config, error) {
	apiServer := creds["api_server"]
	token := creds["token"]
	if apiServer == "" {
		return nil, fmt.Errorf("kubernetes: api_server is required when token is set")
	}
	if token == "" {
		return nil, fmt.Errorf("kubernetes: token is required when api_server is set")
	}
	cfg := &rest.Config{Host: apiServer, BearerToken: token, Timeout: 30 * time.Second}
	if creds["insecure_skip_tls_verify"] == "true" {
		cfg.Insecure = true
	}
	if ca := creds["ca_cert"]; ca != "" {
		data, err := decodeMaybeBase64(ca)
		if err != nil {
			return nil, fmt.Errorf("kubernetes: ca_cert: %w", err)
		}
		cfg.CAData = data
	}
	return cfg, nil
}

func kubeconfigConfig(creds mcp.Credentials) (*rest.Config, []byte, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	var data []byte
	if raw := creds["kubeconfig"]; raw != "" {
		data = []byte(raw)
		cfg, err := clientcmd.RESTConfigFromKubeConfig(data)
		if err != nil {
			return nil, nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
		}
		return cfg, data, nil
	}
	if path := creds["kubeconfig_path"]; path != "" {
		rules.ExplicitPath = expandHome(path)
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: creds["context"]}
	if ns := creds["namespace"]; ns != "" {
		overrides.Context.Namespace = ns
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
	}
	path := rules.ExplicitPath
	if path == "" {
		path = rules.GetDefaultFilename()
	}
	if path != "" {
		data, _ = os.ReadFile(path)
	}
	return cfg, data, nil
}

func defaultNamespace(creds mcp.Credentials) string {
	if ns := creds["namespace"]; ns != "" {
		return ns
	}
	return "default"
}

func expandHome(path string) string {
	if path == "~" {
		return homedir.HomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homedir.HomeDir(), strings.TrimPrefix(path, "~/"))
	}
	return path
}

func decodeMaybeBase64(value string) ([]byte, error) {
	if strings.Contains(value, "-----BEGIN") {
		return []byte(value), nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (k *kubernetes) Healthy(ctx context.Context) bool {
	if k.client == nil {
		return false
	}
	_, err := k.client.Discovery().ServerVersion()
	if err == nil {
		return true
	}
	_, err = k.client.CoreV1().Namespaces().List(ctx, listOpts(1, "", ""))
	return err == nil
}

func (k *kubernetes) Tools() []mcp.ToolDefinition {
	return tools
}

func (k *kubernetes) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	if k.client == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	return fn(ctx, k, args)
}

func (k *kubernetes) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (k *kubernetes) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

type handlerFunc func(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("kubernetes_list_contexts"):    listContexts,
	mcp.ToolName("kubernetes_list_namespaces"):  listNamespaces,
	mcp.ToolName("kubernetes_list_pods"):        listPods,
	mcp.ToolName("kubernetes_get_pod"):          getPod,
	mcp.ToolName("kubernetes_read_pod_logs"):    readPodLogs,
	mcp.ToolName("kubernetes_list_events"):      listEvents,
	mcp.ToolName("kubernetes_list_deployments"): listDeployments,
	mcp.ToolName("kubernetes_get_deployment"):   getDeployment,
	mcp.ToolName("kubernetes_rollout_status"):   rolloutStatus,
	mcp.ToolName("kubernetes_list_services"):    listServices,
	mcp.ToolName("kubernetes_list_ingresses"):   listIngresses,
	mcp.ToolName("kubernetes_list_nodes"):       listNodes,
}

func namespaceFromArgs(k *kubernetes, args map[string]any) (string, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	allNamespaces := r.Bool("all_namespaces")
	if err := r.Err(); err != nil {
		return "", err
	}
	if allNamespaces {
		return "", nil
	}
	if ns != "" {
		return ns, nil
	}
	return k.namespace, nil
}

func boundedLimit(n, def, max int) int64 {
	if n <= 0 {
		return int64(def)
	}
	if n > max {
		return int64(max)
	}
	return int64(n)
}
