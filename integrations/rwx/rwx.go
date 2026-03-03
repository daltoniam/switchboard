package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

const (
	rwxOrg         = "curri"
	minRWXVersion  = "3.0.0"
	rwxAPIBase     = "https://cloud.rwx.com"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

type rwx struct {
	accessToken string
	client      *http.Client
	proxy       *proxyClient
	logCache    *logCache
}

func New() mcp.Integration {
	return &rwx{
		client:   &http.Client{Timeout: 30 * time.Second},
		logCache: newLogCache(),
	}
}

func (r *rwx) Name() string { return "rwx" }

func (r *rwx) Configure(creds mcp.Credentials) error {
	r.accessToken = creds["access_token"]
	if r.accessToken == "" {
		return fmt.Errorf("rwx: access_token is required")
	}
	return nil
}

func (r *rwx) Healthy(ctx context.Context) bool {
	if r.accessToken == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, "GET", rwxAPIBase+"/mint/api/runs?limit=1", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)
	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 400
}

func (r *rwx) Tools() []mcp.ToolDefinition {
	nativeTools := tools
	if r.proxy != nil {
		return append(nativeTools, r.proxy.toolDefinitions()...)
	}
	return nativeTools
}

func (r *rwx) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if ok {
		return fn(ctx, r, args)
	}
	if r.proxy != nil {
		return r.proxy.execute(ctx, toolName, args)
	}
	return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
}

// StartProxy initializes the MCP proxy client by spawning `rwx mcp serve`.
// Called from main after Configure. Non-fatal — tools still work via CLI/API.
func (r *rwx) StartProxy() {
	p := newProxyClient()
	if err := p.start(); err != nil {
		fmt.Printf("[rwx] proxy start failed (tools still available via CLI): %v\n", err)
		return
	}
	r.proxy = p
}

// StopProxy shuts down the MCP proxy subprocess.
func (r *rwx) StopProxy() {
	if r.proxy != nil {
		r.proxy.stop()
		r.proxy = nil
	}
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error)

func jsonResult(v any) (*mcp.ToolResult, error) {
	return rawResult(mustJSON(v))
}

func rawResult(data string) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: data}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Runs
	"rwx_launch_ci_run":    launchCIRun,
	"rwx_wait_for_ci_run":  waitForCIRun,
	"rwx_get_recent_runs":  getRecentRuns,
	"rwx_get_run_results":  getRunResults,

	// Logs
	"rwx_get_task_logs": getTaskLogs,
	"rwx_head_logs":     headLogs,
	"rwx_tail_logs":     tailLogs,
	"rwx_grep_logs":     grepLogs,

	// Artifacts
	"rwx_get_artifacts": getArtifacts,

	// Workflow
	"rwx_validate_workflow": validateWorkflow,

	// CLI
	"rwx_verify_cli": verifyCLI,
}
