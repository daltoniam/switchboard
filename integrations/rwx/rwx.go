package rwx

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	minRWXVersion   = "3.0.0"
	rwxAPIBase      = "https://cloud.rwx.com"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*rwx)(nil)
	_ mcp.FieldCompactionIntegration = (*rwx)(nil)
)

type rwx struct {
	accessToken string
	org         string
	cliPath     string
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

func (r *rwx) Configure(_ context.Context, creds mcp.Credentials) error {
	r.accessToken = creds["access_token"]
	if r.accessToken == "" {
		return fmt.Errorf("rwx: access_token is required")
	}
	r.org = creds["org"]
	if r.org == "" {
		return fmt.Errorf("rwx: org is required")
	}
	r.cliPath = resolveRWXBinary(creds["cli_path"])
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

func (r *rwx) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
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
	if err := p.start(r.cliPath); err != nil {
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

// resolveRWXBinary determines the absolute path to the rwx CLI binary.
// Priority: explicit config > PATH lookup > common install locations.
func resolveRWXBinary(configured string) string {
	if configured != "" {
		if isExecutable(configured) {
			return configured
		}
	}
	if p, err := exec.LookPath("rwx"); err == nil {
		return p
	}
	candidates := absoluteCandidates()
	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate
		}
	}
	return "rwx"
}

// absoluteCandidates returns common rwx install paths, including
// home-relative paths when the home directory is available.
func absoluteCandidates() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".local", "bin", "rwx"),
			filepath.Join(home, ".rwx", "bin", "rwx"),
		)
	}
	paths = append(paths, "/usr/local/bin/rwx", "/opt/homebrew/bin/rwx")
	return paths
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0o111 != 0
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error)

// --- Argument helpers (use shared mcp.Arg* / mcp.Args) ---

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Runs
	"rwx_launch_ci_run":   launchCIRun,
	"rwx_wait_for_ci_run": waitForCIRun,
	"rwx_get_recent_runs": getRecentRuns,
	"rwx_get_run_results": getRunResults,

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
