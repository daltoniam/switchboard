package nomad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*nomad)(nil)
	_ mcp.FieldCompactionIntegration = (*nomad)(nil)
)

type nomad struct {
	address string
	token   string
	client  *http.Client
}

func New() mcp.Integration {
	return &nomad{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (n *nomad) Name() string { return "nomad" }

func (n *nomad) Configure(_ context.Context, creds mcp.Credentials) error {
	n.address = creds["address"]
	if n.address == "" {
		return fmt.Errorf("nomad: address is required")
	}
	n.address = strings.TrimRight(n.address, "/")
	n.token = creds["token"]
	return nil
}

func (n *nomad) Healthy(ctx context.Context) bool {
	if n.client == nil {
		return false
	}
	_, err := n.get(ctx, "/v1/agent/self")
	return err == nil
}

func (n *nomad) Tools() []mcp.ToolDefinition {
	return tools
}

func (n *nomad) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (n *nomad) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, n, args)
}

// --- HTTP helpers ---

func (n *nomad) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, n.address+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if n.token != "" {
		req.Header.Set("X-Nomad-Token", n.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	const maxResponseSize = 10 * 1024 * 1024 // 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("nomad API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("nomad API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (n *nomad) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return n.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (n *nomad) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, "POST", path, body)
}

func (n *nomad) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, "PUT", path, body)
}

func (n *nomad) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return n.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error)

// queryEncode builds a query string from non-empty key/value pairs.
func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Jobs
	mcp.ToolName("nomad_list_jobs"):        listJobs,
	mcp.ToolName("nomad_get_job"):          getJob,
	mcp.ToolName("nomad_get_job_versions"): getJobVersions,
	mcp.ToolName("nomad_register_job"):     registerJob,
	mcp.ToolName("nomad_stop_job"):         stopJob,
	mcp.ToolName("nomad_force_evaluate"):   forceEvaluate,

	// Allocations
	mcp.ToolName("nomad_list_allocations"):     listAllocations,
	mcp.ToolName("nomad_get_allocation"):       getAllocation,
	mcp.ToolName("nomad_get_job_allocations"):  getJobAllocations,
	mcp.ToolName("nomad_stop_allocation"):      stopAllocation,
	mcp.ToolName("nomad_restart_allocation"):   restartAllocation,
	mcp.ToolName("nomad_read_allocation_logs"): readAllocationLogs,

	// Nodes
	mcp.ToolName("nomad_list_nodes"):           listNodes,
	mcp.ToolName("nomad_get_node"):             getNode,
	mcp.ToolName("nomad_get_node_allocations"): getNodeAllocations,
	mcp.ToolName("nomad_drain_node"):           drainNode,
	mcp.ToolName("nomad_node_eligibility"):     nodeEligibility,

	// Deployments
	mcp.ToolName("nomad_list_deployments"):   listDeployments,
	mcp.ToolName("nomad_get_deployment"):     getDeployment,
	mcp.ToolName("nomad_promote_deployment"): promoteDeployment,
	mcp.ToolName("nomad_fail_deployment"):    failDeployment,

	// Evaluations
	mcp.ToolName("nomad_list_evaluations"): listEvaluations,

	// Services
	mcp.ToolName("nomad_list_services"): listServices,

	// Cluster
	mcp.ToolName("nomad_get_agent_self"):     getAgentSelf,
	mcp.ToolName("nomad_get_cluster_status"): getClusterStatus,
	mcp.ToolName("nomad_gc"):                 gc,
}
