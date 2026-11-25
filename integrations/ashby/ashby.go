// Package ashby proxies Ashby's official hosted MCP server
// (https://mcp.ashbyhq.com/mcp/v1) for recruiting and hiring: candidates,
// applications, jobs, interviews, feedback, and pipeline counts.
//
// Ashby signs each user in with OAuth and returns only data that user can
// already see in Ashby. The adapter exposes a reviewed allowlist with
// search-tuned descriptions and access classes, and refuses every other
// upstream tool before it reaches the network. Ashby does not publish tool
// input schemas and warns they may change, so parameters come from the hosted
// server's own tool list once connected.
package ashby

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName = "ashby"
	defaultBaseURL  = "https://mcp.ashbyhq.com"
	endpointPath    = "/mcp/v1"

	schemaFetchTimeout = 30 * time.Second
	schemaRetryAfter   = time.Minute
	// candidateMaxBytes lets a full candidate profile (resume, feedback,
	// emails, interview transcripts) through; there is no compaction spec
	// because Ashby does not document its response shapes.
	candidateMaxBytes = 256 * 1024
)

var (
	_ mcp.Integration                        = (*ashby)(nil)
	_ mcp.PlainTextCredentials               = (*ashby)(nil)
	_ mcp.OptionalCredentials                = (*ashby)(nil)
	_ mcp.PerToolMaxResponseBytesIntegration = (*ashby)(nil)
	_ io.Closer                              = (*ashby)(nil)
)

type ashby struct {
	mu      sync.RWMutex
	baseURL string
	remote  mcp.Integration
	creds   mcp.Credentials
	renewal remotemcp.TokenRenewal
	// configGen changes on every Configure and Close, so a renewal that
	// started from older credentials does not overwrite newer ones or
	// reopen a closed adapter.
	configGen uint64
	// newRemote builds the remote client; tests replace it.
	newRemote func(baseURL string) mcp.Integration

	// schemas holds upstream parameters by exposed tool name for the
	// current remote; nil until fetched.
	schemas       map[mcp.ToolName]mcp.ToolDefinition
	fetching      bool
	fetchFailedAt time.Time
	lifetime      context.Context
	stop          context.CancelFunc
	fetches       sync.WaitGroup
}

// SetConfigService lets the integration renew its OAuth login before the
// access token expires and save the rotated tokens.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if a, ok := i.(*ashby); ok {
		a.renewal.SetConfigService(svc)
	}
}

// New returns the Ashby hosted MCP integration.
func New() mcp.Integration {
	return &ashby{baseURL: defaultBaseURL}
}

func (a *ashby) Name() string { return integrationName }

func (a *ashby) PlainTextKeys() []string { return []string{"base_url"} }

func (a *ashby) OptionalKeys() []string { return []string{"base_url"} }

// MCPServerURL returns the hosted MCP origin the web UI uses for OAuth
// discovery, or "" when integration is not an Ashby adapter.
func MCPServerURL(integration mcp.Integration) string {
	a, ok := integration.(*ashby)
	if !ok || a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.baseURL
}

// MCPResource returns the MCP endpoint that OAuth tokens are bound to, or ""
// when integration is not an Ashby adapter.
func MCPResource(integration mcp.Integration) string {
	origin := MCPServerURL(integration)
	if origin == "" {
		return ""
	}
	return origin + endpointPath
}

func (a *ashby) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds[remotemcp.CredKeyAccessToken])
	if token == "" {
		return fmt.Errorf("ashby: mcp_access_token is required")
	}
	baseURL, err := normalizeBaseURL(creds["base_url"])
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.configureLocked(ctx, creds, token, baseURL)
}

func (a *ashby) configureLocked(ctx context.Context, creds mcp.Credentials, token, baseURL string) error {
	a.configGen++
	credentials := mcp.Credentials{"access_token": token}
	if a.lifetime == nil {
		a.lifetime, a.stop = context.WithCancel(context.Background())
	}
	if a.remote != nil && a.baseURL == baseURL {
		if err := a.remote.Configure(ctx, credentials); err != nil {
			return err
		}
		a.creds = maps.Clone(creds)
		return nil
	}
	remote := a.buildRemote(baseURL)
	if err := remote.Configure(ctx, credentials); err != nil {
		return fmt.Errorf("ashby: configure remote: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := a.remote
	a.remote = remote
	a.baseURL = baseURL
	a.creds = maps.Clone(creds)
	a.schemas = nil
	a.fetching = false
	a.fetchFailedAt = time.Time{}
	return closeRemote(previous)
}

func (a *ashby) buildRemote(baseURL string) mcp.Integration {
	if a.newRemote != nil {
		return a.newRemote(baseURL)
	}
	return remotemcp.NewWithOptions(integrationName, baseURL, remotemcp.Options{EndpointPath: endpointPath, Structured: true})
}

// normalizeBaseURL accepts the origin or the full /mcp/v1 endpoint; the
// remote client appends the endpoint path itself.
func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultBaseURL, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("ashby: invalid base_url: must be an HTTP(S) URL with a host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), endpointPath)
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// renewIfDue renews the login when its access token has less than
// remotemcp.RenewalLead left. If renewal fails while the token still works,
// the failure is logged and the token is used until it expires.
func (a *ashby) renewIfDue(ctx context.Context) error {
	a.mu.RLock()
	creds, baseURL, generation := a.creds, a.baseURL, a.configGen
	a.mu.RUnlock()
	target := remotemcp.RenewalTarget{Integration: integrationName, ServerURL: baseURL, Resource: baseURL + endpointPath}
	fresh, renewed, err := a.renewal.Fresh(ctx, target, creds)
	if err != nil {
		if remotemcp.TokenExpired(creds, time.Now()) {
			return err
		}
		log.Printf("WARN: %v", err)
		return nil
	}
	if !renewed {
		return nil
	}
	token := strings.TrimSpace(fresh[remotemcp.CredKeyAccessToken])
	if token == "" {
		return fmt.Errorf("ashby: renewed login has no access token")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.configGen != generation {
		return nil
	}
	return a.configureLocked(ctx, fresh, token, baseURL)
}

func (a *ashby) Healthy(ctx context.Context) bool {
	if a.renewIfDue(ctx) != nil {
		return false
	}
	a.mu.RLock()
	remote := a.remote
	a.mu.RUnlock()
	if remote == nil || !remote.Healthy(ctx) {
		return false
	}
	a.fetchSchemas(ctx, remote)
	return true
}

// Tools returns the reviewed catalog. It never waits on the network: when
// connected but upstream schemas are not known yet, it starts one background
// fetch and returns the catalog without parameters. Callers get a deep copy.
func (a *ashby) Tools() []mcp.ToolDefinition {
	a.mu.RLock()
	schemas := a.schemas
	a.mu.RUnlock()
	if schemas == nil {
		a.startSchemaFetch()
	}
	out := slices.Clone(tools)
	for i := range out {
		if upstream, ok := schemas[out[i].Name]; ok {
			out[i].Parameters = maps.Clone(upstream.Parameters)
			out[i].Required = slices.Clone(upstream.Required)
		}
		if out[i].Parameters == nil {
			out[i].Parameters = map[string]string{}
		}
	}
	return out
}

func (a *ashby) startSchemaFetch() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.remote == nil || a.lifetime == nil || a.schemas != nil || a.fetching {
		return
	}
	if !a.fetchFailedAt.IsZero() && time.Since(a.fetchFailedAt) < schemaRetryAfter {
		return
	}
	a.fetching = true
	remote, lifetime := a.remote, a.lifetime
	a.fetches.Add(1)
	go func() {
		defer a.fetches.Done()
		ctx, cancel := context.WithTimeout(lifetime, schemaFetchTimeout)
		defer cancel()
		a.fetchSchemas(ctx, remote)
	}()
}

// fetchSchemas stores the upstream tool list for remote, unless remote has
// been replaced or closed meanwhile.
func (a *ashby) fetchSchemas(ctx context.Context, remote mcp.Integration) {
	lister, ok := remote.(remotemcp.ToolLister)
	if !ok {
		return
	}
	listed, err := lister.ListTools(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.remote != remote {
		return
	}
	a.fetching = false
	if err != nil {
		a.fetchFailedAt = time.Now()
		return
	}
	schemas := make(map[mcp.ToolName]mcp.ToolDefinition, len(listed))
	for _, tool := range listed {
		if _, reviewed := dispatch[tool.Name]; reviewed {
			schemas[tool.Name] = tool
		}
	}
	a.schemas = schemas
}

func (a *ashby) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	upstream, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	if err := a.renewIfDue(ctx); err != nil {
		return mcp.ErrResult(err)
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.remote == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	// remotemcp strips its own "<name>_" prefix to recover the upstream name.
	return a.remote.Execute(ctx, mcp.ToolName(integrationName+"_"+upstream), args)
}

func (a *ashby) MaxResponseBytesForTool(toolName mcp.ToolName) (int, bool) {
	if toolName == "ashby_get_candidate" {
		return candidateMaxBytes, true
	}
	return 0, false
}

// Close ends the remote session and any schema fetch. It is safe to call
// more than once; a later Configure starts fresh.
func (a *ashby) Close() error {
	a.mu.Lock()
	a.configGen++
	remote, stop := a.remote, a.stop
	a.remote = nil
	a.schemas = nil
	a.fetching = false
	a.lifetime, a.stop = nil, nil
	a.mu.Unlock()
	if stop != nil {
		stop()
	}
	a.fetches.Wait()
	return closeRemote(remote)
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
