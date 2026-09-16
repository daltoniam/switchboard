// Package gitlab proxies GitLab's official MCP server at /api/v4/mcp (GitLab.com and self-managed).
//
// Phase 2 (native PAT + REST for MCP-disabled instances) is intentionally out of scope here.
package gitlab

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName    = "gitlab"
	defaultInstanceURL = "https://gitlab.com"
	apiV4Suffix        = "/api/v4"
	exposedPrefix      = "gitlab_"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay(integrationName, compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// toolDescriptionOverrides enrich upstream tool docs for Switchboard search/discovery.
// Unlisted tools keep the official GitLab MCP description from the upstream server.
var toolDescriptionOverrides = map[mcp.ToolName]string{
	"gitlab_list_merge_requests": "Start here. List or search merge requests in a GitLab project (compact metadata). Use get_merge_request for full detail, save_note to comment on an MR, and get_job with include log for failed CI job traces.",
	"gitlab_get_merge_request":   "Get full merge request details for a project. Pair with get_merge_request_diffs, get_merge_request_commits, or get_merge_request_notes for review workflows.",
	"gitlab_save_note":           "Add a comment on a merge request or work item, or reply to an existing discussion thread.",
	"gitlab_get_job":             "Get CI/CD job metadata and optionally the job trace/log (include: [\"log\"], with byte_offset/byte_limit for large logs).",
	"gitlab_list_pipelines":      "List pipelines in a project with optional ref, status, and pagination filters.",
	"gitlab_get_pipeline_jobs":   "List jobs in a pipeline — use get_job on a failed job ID to fetch logs.",
}

var (
	_ mcp.Integration                = (*gitlab)(nil)
	_ mcp.FieldCompactionIntegration = (*gitlab)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gitlab)(nil)
	_ mcp.PlainTextCredentials       = (*gitlab)(nil)
	_ mcp.OptionalCredentials        = (*gitlab)(nil)
)

type gitlab struct {
	mu          sync.RWMutex
	instanceURL string
	mcpAPIBase  string
	remote      mcp.Integration
	newRemote   func(mcpAPIBase string) mcp.Integration
}

// New creates the GitLab official MCP proxy integration.
func New() mcp.Integration {
	return &gitlab{
		instanceURL: defaultInstanceURL,
		mcpAPIBase:  mcpAPIBaseURL(defaultInstanceURL),
		newRemote: func(base string) mcp.Integration {
			return remotemcp.New(integrationName, base)
		},
	}
}

func (g *gitlab) Name() string { return integrationName }

func (g *gitlab) PlainTextKeys() []string { return []string{"base_url"} }

func (g *gitlab) OptionalKeys() []string { return []string{"base_url", "token"} }

// MCPServerURL returns the configured GitLab instance URL (scheme + host [+ path prefix])
// for OAuth metadata discovery. It does not include /api/v4/mcp.
func MCPServerURL(i mcp.Integration) string {
	if g, ok := i.(*gitlab); ok {
		g.mu.RLock()
		defer g.mu.RUnlock()
		return g.instanceURL
	}
	return ""
}

// MCPAPIBaseURL returns the remotemcp base URL (…/api/v4; remotemcp appends /mcp).
func MCPAPIBaseURL(i mcp.Integration) string {
	if g, ok := i.(*gitlab); ok {
		g.mu.RLock()
		defer g.mu.RUnlock()
		return g.mcpAPIBase
	}
	return ""
}

// ApplyBaseURL updates the instance and MCP API base from config without requiring a token.
// Used by the web UI before OAuth discovery on self-managed instances.
func ApplyBaseURL(i mcp.Integration, baseURL string) {
	g, ok := i.(*gitlab)
	if !ok {
		return
	}
	instance := normalizeInstanceURL(baseURL)
	g.mu.Lock()
	g.instanceURL = instance
	g.mcpAPIBase = mcpAPIBaseURL(instance)
	g.mu.Unlock()
}

func (g *gitlab) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["mcp_access_token"])
	if token == "" {
		token = strings.TrimSpace(creds["token"])
	}
	if token == "" {
		return fmt.Errorf("gitlab: mcp_access_token or token is required")
	}

	instance := normalizeInstanceURL(creds["base_url"])
	apiBase := mcpAPIBaseURL(instance)

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.remote == nil || g.mcpAPIBase != apiBase {
		factory := g.newRemote
		if factory == nil {
			factory = func(base string) mcp.Integration {
				return remotemcp.New(integrationName, base)
			}
		}
		if g.remote != nil {
			closeRemote(g.remote)
		}
		g.remote = factory(apiBase)
	}
	g.instanceURL = instance
	g.mcpAPIBase = apiBase

	return g.remote.Configure(ctx, mcp.Credentials{"access_token": token})
}

func (g *gitlab) Healthy(ctx context.Context) bool {
	g.mu.RLock()
	remote := g.remote
	g.mu.RUnlock()
	return remote != nil && remote.Healthy(ctx)
}

func (g *gitlab) Tools() []mcp.ToolDefinition {
	g.mu.RLock()
	remote := g.remote
	g.mu.RUnlock()
	if remote == nil {
		return nil
	}
	return enrichTools(remote.Tools())
}

func (g *gitlab) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if !strings.HasPrefix(string(toolName), exposedPrefix) {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	g.mu.RLock()
	remote := g.remote
	g.mu.RUnlock()
	if remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	return remote.Execute(ctx, toolName, args)
}

func (g *gitlab) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gitlab) MaxBytes(toolName mcp.ToolName) (int, bool) {
	maxBytes, ok := maxBytesByTool[toolName]
	return maxBytes, ok
}

func (g *gitlab) Close() error {
	g.mu.Lock()
	remote := g.remote
	g.remote = nil
	g.mu.Unlock()
	closeRemote(remote)
	return nil
}

func enrichTools(upstream []mcp.ToolDefinition) []mcp.ToolDefinition {
	out := make([]mcp.ToolDefinition, 0, len(upstream))
	for _, t := range upstream {
		if desc, ok := toolDescriptionOverrides[t.Name]; ok {
			t.Description = desc
		}
		out = append(out, t)
	}
	return out
}

func closeRemote(r mcp.Integration) {
	if c, ok := r.(interface{ Close() error }); ok {
		_ = c.Close()
	}
}

// normalizeInstanceURL parses user base_url into a GitLab instance root (no /api/v4/mcp).
func normalizeInstanceURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return defaultInstanceURL
	}
	v = strings.TrimRight(v, "/")
	if strings.HasSuffix(v, "/mcp") {
		v = strings.TrimSuffix(v, "/mcp")
		v = strings.TrimRight(v, "/")
	}
	if strings.HasSuffix(v, apiV4Suffix) {
		v = strings.TrimSuffix(v, apiV4Suffix)
		v = strings.TrimRight(v, "/")
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	return v
}

func mcpAPIBaseURL(instance string) string {
	instance = strings.TrimRight(normalizeInstanceURL(instance), "/")
	parsed, err := url.Parse(instance)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(defaultInstanceURL, "/") + apiV4Suffix
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if strings.HasSuffix(path, apiV4Suffix) {
		parsed.Path = path
	} else if path == "" || path == "/" {
		parsed.Path = apiV4Suffix
	} else {
		parsed.Path = path + apiV4Suffix
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}
