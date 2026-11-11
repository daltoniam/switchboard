package argoworkflows

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("argoworkflows", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var literalName = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)
var progressPattern = regexp.MustCompile(`^[0-9]{1,10}/[0-9]{1,10}$`)

const maxResponseBytes = 1024 * 1024

var (
	_ mcp.Integration                = (*argoworkflows)(nil)
	_ mcp.FieldCompactionIntegration = (*argoworkflows)(nil)
	_ mcp.PlainTextCredentials       = (*argoworkflows)(nil)
	_ mcp.PlaceholderHints           = (*argoworkflows)(nil)
	_ mcp.OptionalCredentials        = (*argoworkflows)(nil)
)

type argoworkflows struct {
	mu        sync.RWMutex
	baseURL   string
	namespace string
	token     string
	client    *http.Client
}

func New() mcp.Integration                       { return &argoworkflows{} }
func (a *argoworkflows) Name() string            { return "argoworkflows" }
func (a *argoworkflows) PlainTextKeys() []string { return []string{"base_url", "namespace", "ca_cert"} }
func (a *argoworkflows) OptionalKeys() []string  { return []string{"ca_cert"} }
func (a *argoworkflows) Placeholders() map[string]string {
	return map[string]string{
		"base_url":  "Trusted HTTPS Argo Server URL, including any installation prefix",
		"namespace": "One Kubernetes namespace (get/list workflows only)",
		"token":     "Service-account bearer token without Bearer prefix; Argo client auth mode required",
		"ca_cert":   `Optional PEM CA certificate with literal \n between lines; certificate verification stays enabled`,
	}
}

func validName(name string, max int) bool {
	return len(name) > 0 && len(name) <= max && literalName.MatchString(name) && !strings.Contains(name, "..")
}

func (a *argoworkflows) Configure(_ context.Context, creds mcp.Credentials) error {
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, unicode.IsSpace) {
		return fmt.Errorf("argoworkflows: base_url must be a trusted HTTPS installation URL without credentials, query, fragment, or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("argoworkflows: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("argoworkflows: invalid base_url port")
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("argoworkflows: invalid installation prefix")
		}
	}
	namespace := creds["namespace"]
	if !validName(namespace, 63) || strings.Contains(namespace, ".") {
		return fmt.Errorf("argoworkflows: namespace must be one literal Kubernetes namespace")
	}
	token := creds["token"]
	if token == "" || strings.ContainsFunc(token, func(value rune) bool { return unicode.IsSpace(value) || unicode.IsControl(value) }) {
		return fmt.Errorf("argoworkflows: token is required without whitespace or Bearer prefix")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("argoworkflows: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	a.mu.Lock()
	if a.client != nil {
		a.client.CloseIdleConnections()
	}
	a.baseURL, a.namespace, a.token, a.client = strings.TrimRight(endpoint.String(), "/"), namespace, token, client
	a.mu.Unlock()
	return nil
}

func (a *argoworkflows) Healthy(ctx context.Context) bool {
	result, err := a.Execute(ctx, "argoworkflows_list_workflows", map[string]any{"limit": 1})
	return err == nil && result != nil && !result.IsError
}

var tools = []mcp.ToolDefinition{
	{
		Name: "argoworkflows_list_workflows", Access: mcp.AccessRead,
		Description: "List Argo Workflows Kubernetes workflow runs and status in the configured namespace. Start here for failed workflow, job and DAG diagnosis. Returns phase, timestamps, safe failure summary and next_cursor; use the same limit and label_selector with each cursor. No specs, parameters, artifacts or logs. Failure messages are omitted because they may contain secrets.",
		Parameters:  map[string]string{"limit": "Page size (default 20, 1-100); oversized pages fail explicitly", "cursor": "Opaque next_cursor from the previous page; keep other filters unchanged; restart without cursor after HTTP 410", "label_selector": "Optional Kubernetes label selector; keep unchanged during pagination"},
	},
	{
		Name: "argoworkflows_get_workflow", Access: mcp.AccessRead,
		Description: "Get one exact Argo Workflows run in the configured Kubernetes namespace. Use after argoworkflows_list_workflows to inspect phase, timestamps, progress and up to 100 failed node identifiers and phases. Arbitrary failure messages and display names are omitted to avoid exposing secrets. No specs, inputs, outputs, logs, artifacts or mutations.",
		Parameters:  map[string]string{"name": "Exact workflow metadata.name from argoworkflows_list_workflows; not an alias, path or URL"}, Required: []string{"name"},
	},
}

func (a *argoworkflows) Tools() []mcp.ToolDefinition { return tools }
func (a *argoworkflows) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handler func(context.Context, *argoworkflows, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{"argoworkflows_list_workflows": listWorkflows, "argoworkflows_get_workflow": getWorkflow}

func (a *argoworkflows) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: unknown tool: %s", name))
	}
	for key := range args {
		allowed := key == "name" && name == "argoworkflows_get_workflow" || name == "argoworkflows_list_workflows" && (key == "limit" || key == "cursor" || key == "label_selector")
		if !allowed {
			return mcp.ErrResult(fmt.Errorf("argoworkflows: unsupported argument"))
		}
	}
	return fn(ctx, a, args)
}

type upstreamWorkflow struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Status struct {
		Phase      string `json:"phase"`
		StartedAt  string `json:"startedAt"`
		FinishedAt string `json:"finishedAt"`
		Progress   string `json:"progress"`
		Nodes      map[string]struct {
			Phase string `json:"phase"`
		} `json:"nodes"`
	} `json:"status"`
}

type workflowStatus struct {
	Name           string `json:"name"`
	Phase          string `json:"phase"`
	StartedAt      string `json:"started_at"`
	FinishedAt     string `json:"finished_at"`
	FailureSummary string `json:"failure_summary"`
}

type failedNode struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
}

func phase(value string) string {
	switch value {
	case "Pending", "Running", "Succeeded", "Failed", "Error", "Skipped", "Omitted":
		return value
	default:
		return "Unknown"
	}
}

func timestamp(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func project(wf upstreamWorkflow, namespace string) (workflowStatus, error) {
	if !validName(wf.Metadata.Name, 253) || wf.Metadata.Namespace != namespace {
		return workflowStatus{}, fmt.Errorf("argoworkflows: invalid workflow identity in response")
	}
	status := workflowStatus{Name: wf.Metadata.Name, Phase: phase(wf.Status.Phase), StartedAt: timestamp(wf.Status.StartedAt), FinishedAt: timestamp(wf.Status.FinishedAt)}
	if status.Phase == "Failed" || status.Phase == "Error" {
		status.FailureSummary = "Workflow failed; provider messages omitted because they may contain secrets"
	}
	return status, nil
}

func listWorkflows(ctx context.Context, a *argoworkflows, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	cursor, selector := reader.Str("cursor"), reader.Str("label_selector")
	limit := 20
	if _, present := args["limit"]; present {
		limit = reader.Int("limit")
	}
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit < 1 || limit > 100 || len(cursor) > 8192 || len(selector) > 2048 || strings.ContainsFunc(cursor+selector, unicode.IsControl) {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: invalid limit, cursor or label_selector"))
	}
	query := url.Values{"listOptions.limit": {strconv.Itoa(limit)}, "fields": {"metadata.continue,items.metadata.name,items.metadata.namespace,items.status.phase,items.status.startedAt,items.status.finishedAt"}}
	if cursor != "" {
		query.Set("listOptions.continue", cursor)
	}
	if selector != "" {
		query.Set("listOptions.labelSelector", selector)
	}
	var response struct {
		Metadata struct {
			Continue string `json:"continue"`
		} `json:"metadata"`
		Items json.RawMessage `json:"items"`
	}
	namespace, err := a.get(ctx, "", query, &response)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(response.Items) == 0 {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: response is missing items"))
	}
	var workflows []upstreamWorkflow
	if err := json.Unmarshal(response.Items, &workflows); err != nil {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: invalid workflow items JSON"))
	}
	if len(workflows) > limit {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: server ignored page limit; inventory incomplete, narrow label_selector"))
	}
	if len(response.Metadata.Continue) > 8192 || strings.ContainsFunc(response.Metadata.Continue, unicode.IsControl) {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: invalid continuation response"))
	}
	statuses := make([]workflowStatus, 0, len(workflows))
	for _, wf := range workflows {
		status, err := project(wf, namespace)
		if err != nil {
			return mcp.ErrResult(err)
		}
		statuses = append(statuses, status)
	}
	return mcp.JSONResult(struct {
		Workflows  []workflowStatus `json:"workflows"`
		NextCursor string           `json:"next_cursor"`
		Incomplete bool             `json:"incomplete"`
	}{statuses, response.Metadata.Continue, response.Metadata.Continue != ""})
}

func getWorkflow(ctx context.Context, a *argoworkflows, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	name := reader.Str("name")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if !validName(name, 253) {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: name must be one exact literal workflow name"))
	}
	var workflow upstreamWorkflow
	namespace, err := a.get(ctx, name, url.Values{"fields": {"metadata.name,metadata.namespace,status.phase,status.startedAt,status.finishedAt,status.progress,status.nodes"}}, &workflow)
	if err != nil {
		return mcp.ErrResult(err)
	}
	status, err := project(workflow, namespace)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if status.Name != name {
		return mcp.ErrResult(fmt.Errorf("argoworkflows: workflow identity does not match request"))
	}
	names := make([]string, 0, len(workflow.Status.Nodes))
	for node, state := range workflow.Status.Nodes {
		if state.Phase == "Failed" || state.Phase == "Error" {
			if !validName(node, 253) {
				return mcp.ErrResult(fmt.Errorf("argoworkflows: invalid failed node identifier"))
			}
			names = append(names, node)
		}
	}
	sort.Strings(names)
	truncated := len(names) > 100
	if truncated {
		names = names[:100]
	}
	nodes := make([]failedNode, 0, len(names))
	for _, name := range names {
		nodes = append(nodes, failedNode{Name: name, Phase: phase(workflow.Status.Nodes[name].Phase)})
	}
	progress := ""
	if progressPattern.MatchString(workflow.Status.Progress) {
		progress = workflow.Status.Progress
	}
	return mcp.JSONResult(struct {
		workflowStatus
		Progress        string       `json:"progress"`
		FailedNodes     []failedNode `json:"failed_nodes"`
		NodesTruncated  bool         `json:"nodes_truncated"`
		MessagesOmitted bool         `json:"messages_omitted"`
	}{status, progress, nodes, truncated, true})
}

func (a *argoworkflows) get(ctx context.Context, name string, query url.Values, target any) (string, error) {
	a.mu.RLock()
	baseURL, namespace, token, client := a.baseURL, a.namespace, a.token, a.client
	a.mu.RUnlock()
	if client == nil {
		return "", mcp.ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	endpoint := baseURL + "/api/v1/workflows/" + url.PathEscape(namespace)
	if name != "" {
		endpoint += "/" + url.PathEscape(name)
	}
	endpoint += "?" + query.Encode()
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", fmt.Errorf("argoworkflows: cannot create request")
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Accept", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return "", fmt.Errorf("argoworkflows: HTTPS request failed; check server certificate, connectivity and deadline")
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := mcp.ParseRetryAfter(response.Header.Get("Retry-After"))
			_ = response.Body.Close()
			if delay <= 0 {
				delay = time.Duration(attempt+1) * time.Second
			}
			if delay > 3*time.Second {
				return "", fmt.Errorf("argoworkflows: HTTP 429; Retry-After exceeds bounded retry budget, retry later")
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			guidance := ""
			switch response.StatusCode {
			case 401:
				guidance = "; token rejected or expired, use a service-account token and client auth mode"
			case 403:
				guidance = "; grant only get/list workflows in the configured namespace"
			case 410:
				guidance = "; cursor expired, restart listing without cursor"
			}
			return "", fmt.Errorf("argoworkflows: HTTP %d%s", response.StatusCode, guidance)
		}
		if readErr != nil {
			return "", fmt.Errorf("argoworkflows: cannot read response")
		}
		if len(data) > maxResponseBytes {
			return "", fmt.Errorf("argoworkflows: response exceeds 1 MiB limit; results incomplete, narrow the query")
		}
		if err := json.Unmarshal(data, target); err != nil {
			return "", fmt.Errorf("argoworkflows: invalid upstream JSON")
		}
		return namespace, nil
	}
	return "", fmt.Errorf("argoworkflows: retry budget exhausted")
}
