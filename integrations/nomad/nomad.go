package nomad

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("nomad", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*nomad)(nil)
	_ mcp.FieldCompactionIntegration = (*nomad)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*nomad)(nil)
	_ mcp.PlaceholderHints           = (*nomad)(nil)
	_ mcp.OptionalCredentials        = (*nomad)(nil)
)

type nomad struct {
	addresses []string
	token     string
	client    *http.Client

	// preferred is the index of the server address to try first. It advances to
	// the last server that answered so a downed leader is not retried on every
	// call (sticky failover). Safe for concurrent use.
	preferred atomic.Uint32
}

func New() mcp.Integration {
	return &nomad{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (n *nomad) Name() string { return "nomad" }

func (n *nomad) Configure(_ context.Context, creds mcp.Credentials) error {
	n.addresses = parseAddresses(creds["address"], creds["addresses"])
	if len(n.addresses) == 0 {
		return fmt.Errorf("nomad: address is required")
	}
	n.token = creds["token"]
	n.preferred.Store(0)
	return nil
}

// Placeholders provides web-UI hints for the credential fields.
func (n *nomad) Placeholders() map[string]string {
	return map[string]string{
		"address":   "http://localhost:4646",
		"addresses": "HA: comma- or newline-separated, e.g. http://server1:4646,http://server2:4646",
		"token":     "Nomad ACL token (optional)",
	}
}

// OptionalKeys marks credentials that are not strictly required. Either address
// or addresses must be set; addresses (the HA server list) and the ACL token are
// optional on their own.
func (n *nomad) OptionalKeys() []string {
	return []string{"addresses", "token"}
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

func (n *nomad) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n2, ok := maxBytesByTool[toolName]
	return n2, ok
}

func (n *nomad) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, n, args)
}

// --- HTTP helpers ---

// doRequest issues an HTTP request to the configured Nomad server(s). In HA
// deployments multiple server addresses may be configured; doRequest performs
// sticky failover: it starts at the last server that answered and walks the
// remaining servers in order. Definitive (non-retryable) responses are returned
// immediately, while transport failures and retryable statuses (429/5xx) move on
// to the next server.
func (n *nomad) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyData []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyData = data
	}

	if len(n.addresses) == 0 {
		return nil, fmt.Errorf("nomad: no server address configured")
	}

	start := int(n.preferred.Load()) % len(n.addresses)
	var lastErr error
	for i := range n.addresses {
		idx := (start + i) % len(n.addresses)
		data, err := n.doRequestTo(ctx, n.addresses[idx], method, path, bodyData, body != nil)
		if err == nil {
			n.preferred.Store(uint32(idx))
			return data, nil
		}
		lastErr = err
		if ctx.Err() != nil || !shouldFailover(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// doRequestTo issues a single request against one server address.
func (n *nomad) doRequestTo(ctx context.Context, address, method, path string, bodyData []byte, hasBody bool) (json.RawMessage, error) {
	var bodyReader io.Reader
	if hasBody {
		bodyReader = bytes.NewReader(bodyData)
	}

	req, err := http.NewRequestWithContext(ctx, method, address+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if n.token != "" {
		req.Header.Set("X-Nomad-Token", n.token)
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nomad: request to %s failed: %w", address, err)
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
		return nil, &apiError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// apiError is a definitive (non-retryable) HTTP error from a reachable Nomad
// server. Failover treats these as authoritative and does not retry the request
// against another server.
type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("nomad API error (%d): %s", e.StatusCode, e.Body)
}

// shouldFailover reports whether a failed request should be retried against the
// next server. Definitive 4xx responses (apiError) are authoritative and are not
// retried; transport failures and retryable statuses (429/5xx) are.
func shouldFailover(err error) bool {
	if err == nil {
		return false
	}
	var ae *apiError
	return !errors.As(err, &ae)
}

// parseAddresses splits one or more raw credential values into a deduplicated,
// ordered list of Nomad server base URLs. Values may be separated by commas or
// any whitespace (spaces, tabs, newlines), so a single multi-line credential
// field or a comma-separated NOMAD_ADDR both work for HA deployments. Trailing
// slashes are trimmed and duplicates are dropped while preserving order.
func parseAddresses(values ...string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, v := range values {
		fields := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		})
		for _, field := range fields {
			addr := strings.TrimRight(field, "/")
			if addr == "" {
				continue
			}
			if _, dup := seen[addr]; dup {
				continue
			}
			seen[addr] = struct{}{}
			out = append(out, addr)
		}
	}
	return out
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
