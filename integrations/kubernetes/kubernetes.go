package kubernetes

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
	clients         map[string]*clusterClient
}

type clusterClient struct {
	client          k8sclient.Interface
	context         string
	cluster         string
	authInfo        string
	namespace       string
	current         bool
	source          string
	kubeconfigBytes []byte
}

type configuredCluster struct {
	Name                  string `json:"name"`
	Context               string `json:"context"`
	Namespace             string `json:"namespace"`
	Kubeconfig            string `json:"kubeconfig"`
	KubeconfigPath        string `json:"kubeconfig_path"`
	APIServer             string `json:"api_server"`
	Token                 string `json:"token"`
	CACert                string `json:"ca_cert"`
	InsecureSkipTLSVerify string `json:"insecure_skip_tls_verify"`
	InCluster             string `json:"in_cluster"`
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
		"context":         "current kubeconfig context",
		"namespace":       "default",
	}
}

func (k *kubernetes) OptionalKeys() []string {
	return []string{"kubeconfig", "kubeconfig_path", "context", "namespace", "api_server", "token", "ca_cert", "insecure_skip_tls_verify", "in_cluster", "clusters"}
}

func (k *kubernetes) HasCredentials(creds mcp.Credentials) bool {
	return explicitCredentials(creds)
}

func (k *kubernetes) Configure(_ context.Context, creds mcp.Credentials) error {
	clients, defaultName, err := clusterClients(creds)
	if err != nil {
		return err
	}
	selected, ok := clients[defaultName]
	if !ok {
		return fmt.Errorf("kubernetes: configured context %q was not found", defaultName)
	}
	k.client = selected.client
	k.namespace = selected.namespace
	k.context = selected.context
	k.kubeconfigBytes = selected.kubeconfigBytes
	k.clients = clients
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
		return nil, nil, fmt.Errorf("kubernetes: explicit kubeconfig, kubeconfig_path, clusters, in_cluster=true, or api_server+token is required")
	}
	if creds["clusters"] != "" {
		return nil, nil, fmt.Errorf("kubernetes: clusters must be configured through clusterClients")
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
	return nil, nil, fmt.Errorf("kubernetes: kubeconfig, kubeconfig_path, clusters, in_cluster=true, or api_server+token is required")
}

func explicitCredentials(creds mcp.Credentials) bool {
	return creds["kubeconfig"] != "" || creds["kubeconfig_path"] != "" || creds["clusters"] != "" || creds["in_cluster"] == "true" || creds["api_server"] != "" || creds["token"] != ""
}

func clusterClients(creds mcp.Credentials) (map[string]*clusterClient, string, error) {
	if raw := creds["clusters"]; raw != "" {
		return clusterClientsFromJSON(creds, raw)
	}
	cfg, data, err := restConfig(creds)
	if err != nil {
		return nil, "", err
	}
	client, err := k8sclient.NewForConfig(cfg)
	if err != nil {
		return nil, "", fmt.Errorf("kubernetes: create client: %w", err)
	}
	if len(data) > 0 {
		apiCfg, err := clientcmd.Load(data)
		if err != nil {
			return nil, "", fmt.Errorf("kubernetes: load kubeconfig contexts: %w", err)
		}
		return clusterClientsFromKubeconfig(creds, apiCfg, data, client)
	}
	name := singleClusterName(creds)
	clients := map[string]*clusterClient{
		name: {
			client:    client,
			context:   name,
			cluster:   creds["api_server"],
			namespace: defaultNamespace(creds),
			current:   true,
			source:    singleClusterSource(creds),
		},
	}
	return clients, name, nil
}

func clusterClientsFromJSON(base mcp.Credentials, raw string) (map[string]*clusterClient, string, error) {
	var configured []configuredCluster
	if err := json.Unmarshal([]byte(raw), &configured); err != nil {
		return nil, "", fmt.Errorf("kubernetes: parse clusters: %w", err)
	}
	if len(configured) == 0 {
		return nil, "", fmt.Errorf("kubernetes: clusters must include at least one cluster")
	}
	clients := make(map[string]*clusterClient, len(configured))
	defaultName := base["context"]
	for i, item := range configured {
		creds := mcp.Credentials{
			"kubeconfig":               item.Kubeconfig,
			"kubeconfig_path":          item.KubeconfigPath,
			"context":                  item.Context,
			"namespace":                item.Namespace,
			"api_server":               item.APIServer,
			"token":                    item.Token,
			"ca_cert":                  item.CACert,
			"insecure_skip_tls_verify": item.InsecureSkipTLSVerify,
			"in_cluster":               item.InCluster,
		}
		if creds["namespace"] == "" {
			creds["namespace"] = base["namespace"]
		}
		cfg, data, err := restConfig(creds)
		if err != nil {
			return nil, "", fmt.Errorf("kubernetes: configure cluster %d: %w", i+1, err)
		}
		client, err := k8sclient.NewForConfig(cfg)
		if err != nil {
			return nil, "", fmt.Errorf("kubernetes: create client for cluster %d: %w", i+1, err)
		}
		name := item.Name
		if name == "" {
			name = item.Context
		}
		if name == "" {
			name = item.APIServer
		}
		if name == "" {
			name = fmt.Sprintf("cluster-%d", i+1)
		}
		clients[name] = &clusterClient{client: client, context: name, cluster: item.APIServer, namespace: defaultNamespace(creds), current: name == defaultName, source: singleClusterSource(creds), kubeconfigBytes: data}
		if defaultName == "" && i == 0 {
			defaultName = name
			clients[name].current = true
		}
	}
	return clients, defaultName, nil
}

func clusterClientsFromKubeconfig(creds mcp.Credentials, apiCfg *clientcmdapi.Config, data []byte, selected k8sclient.Interface) (map[string]*clusterClient, string, error) {
	defaultName := creds["context"]
	if defaultName == "" {
		defaultName = apiCfg.CurrentContext
	}
	if defaultName == "" && len(apiCfg.Contexts) == 1 {
		for name := range apiCfg.Contexts {
			defaultName = name
		}
	}
	if defaultName == "" {
		return nil, "", fmt.Errorf("kubernetes: kubeconfig has no current context; set context explicitly")
	}
	clients := make(map[string]*clusterClient, len(apiCfg.Contexts))
	names := make([]string, 0, len(apiCfg.Contexts))
	for name := range apiCfg.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ctx := apiCfg.Contexts[name]
		client := selected
		if name != defaultName {
			cfg, err := restConfigFromKubeconfig(apiCfg, name, creds["namespace"])
			if err != nil {
				continue
			}
			created, err := k8sclient.NewForConfig(cfg)
			if err != nil {
				continue
			}
			client = created
		}
		clients[name] = &clusterClient{client: client, context: name, cluster: ctx.Cluster, authInfo: ctx.AuthInfo, namespace: namespaceForContext(ctx, creds["namespace"]), current: name == defaultName, source: "kubeconfig", kubeconfigBytes: data}
	}
	if _, ok := clients[defaultName]; !ok {
		return nil, "", fmt.Errorf("kubernetes: kubeconfig context %q was not found", defaultName)
	}
	return clients, defaultName, nil
}

func restConfigFromKubeconfig(apiCfg *clientcmdapi.Config, contextName string, namespace string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{}
	if namespace != "" {
		overrides.Context.Namespace = namespace
	}
	return clientcmd.NewNonInteractiveClientConfig(*apiCfg, contextName, overrides, nil).ClientConfig()
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
	cfg, data, err := loadKubeconfig(creds)
	if err != nil {
		return nil, nil, err
	}
	contextName := creds["context"]
	if contextName == "" {
		contextName = cfg.CurrentContext
	}
	restCfg, err := restConfigFromKubeconfig(cfg, contextName, creds["namespace"])
	if err != nil {
		return nil, nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
	}
	return restCfg, data, nil
}

func loadKubeconfig(creds mcp.Credentials) (*clientcmdapi.Config, []byte, error) {
	if raw := creds["kubeconfig"]; raw != "" {
		data := []byte(raw)
		cfg, err := clientcmd.Load(data)
		if err != nil {
			return nil, nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
		}
		return cfg, data, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if path := creds["kubeconfig_path"]; path != "" {
		expanded := expandHome(path)
		if strings.ContainsRune(expanded, os.PathListSeparator) {
			rules.Precedence = strings.Split(expanded, string(os.PathListSeparator))
		} else {
			rules.ExplicitPath = expanded
		}
	}
	cfg, err := rules.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
	}
	data, err := clientcmd.Write(*cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("kubernetes: serialize kubeconfig: %w", err)
	}
	return cfg, data, nil
}

func defaultNamespace(creds mcp.Credentials) string {
	if ns := creds["namespace"]; ns != "" {
		return ns
	}
	return "default"
}

func namespaceForContext(ctx *clientcmdapi.Context, override string) string {
	if override != "" {
		return override
	}
	if ctx != nil && ctx.Namespace != "" {
		return ctx.Namespace
	}
	return "default"
}

func singleClusterName(creds mcp.Credentials) string {
	if name := creds["context"]; name != "" {
		return name
	}
	if creds["in_cluster"] == "true" {
		return "in-cluster"
	}
	if apiServer := creds["api_server"]; apiServer != "" {
		return apiServer
	}
	return "default"
}

func singleClusterSource(creds mcp.Credentials) string {
	switch {
	case creds["in_cluster"] == "true":
		return "in_cluster"
	case creds["api_server"] != "" || creds["token"] != "":
		return "api_server"
	case creds["kubeconfig"] != "" || creds["kubeconfig_path"] != "":
		return "kubeconfig"
	default:
		return "unknown"
	}
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
	selected, err := k.withSelectedContext(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return fn(ctx, selected, args)
}

func (k *kubernetes) withSelectedContext(args map[string]any) (*kubernetes, error) {
	if len(k.clients) == 0 {
		return k, nil
	}
	r := mcp.NewArgs(args)
	contextName := r.Str("context")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if contextName == "" || contextName == k.context {
		return k, nil
	}
	selected, ok := k.clients[contextName]
	if !ok {
		return nil, fmt.Errorf("kubernetes: unknown context %q", contextName)
	}
	clone := *k
	clone.client = selected.client
	clone.namespace = selected.namespace
	clone.context = selected.context
	clone.kubeconfigBytes = selected.kubeconfigBytes
	return &clone, nil
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
	mcp.ToolName("kubernetes_list_clusters"):    listClusters,
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
