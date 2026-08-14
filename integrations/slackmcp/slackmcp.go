// Package slackmcp proxies Slack's official hosted MCP server with multi-identity routing.
package slackmcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName     = "slackmcp"
	defaultBaseURL      = "https://mcp.slack.com"
	listIdentitiesTool  = "slackmcp_list_available_identites" // requested spelling
	identityParam       = "identity_id"
	upstreamSlackPrefix = "slack_"
	exposedPrefix       = "slackmcp_"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration              = (*slackmcp)(nil)
	_ mcp.MultiIdentityIntegration = (*slackmcp)(nil)
	_ mcp.PlainTextCredentials     = (*slackmcp)(nil)
	_ mcp.OptionalCredentials      = (*slackmcp)(nil)
)

type identityState struct {
	id       string
	token    string
	baseURL  string
	metadata map[string]string
	remote   mcp.Integration
	tools    []mcp.ToolDefinition // upstream-translated (slackmcp_*) without identity_id yet
}

type slackmcp struct {
	// cfgMu serializes Configure / ConfigureIdentities so network Tools() work
	// never runs under the state mutex, and snapshot/swap stays race-safe.
	cfgMu sync.Mutex

	mu      sync.RWMutex
	baseURL string

	// ordered identity IDs for deterministic tool union / listing
	order      []string
	identities map[string]*identityState

	// newRemote creates a per-identity remote client. Tests inject a factory;
	// production defaults to remotemcp via newRemoteClient.
	newRemote func(baseURL string) mcp.Integration
}

// New creates the Slack official hosted MCP multi-identity integration.
func New() mcp.Integration {
	return &slackmcp{
		baseURL:    defaultBaseURL,
		identities: make(map[string]*identityState),
		newRemote:  newRemoteClient,
	}
}

func (s *slackmcp) Name() string { return integrationName }

func (s *slackmcp) PlainTextKeys() []string { return []string{"base_url"} }

func (s *slackmcp) OptionalKeys() []string { return []string{"base_url"} }

func (s *slackmcp) Configure(_ context.Context, creds mcp.Credentials) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if v := strings.TrimSpace(creds["base_url"]); v != "" {
		s.baseURL = strings.TrimRight(v, "/")
	} else {
		s.baseURL = defaultBaseURL
	}
	return nil
}

func (s *slackmcp) ConfigureIdentities(ctx context.Context, identities map[string]mcp.IntegrationIdentity) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	if identities == nil {
		identities = map[string]mcp.IntegrationIdentity{}
	}

	// Validate first.
	ids := make([]string, 0, len(identities))
	for id, ident := range identities {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("slackmcp: identity id cannot be empty")
		}
		tok := strings.TrimSpace(ident.Credentials["access_token"])
		if tok == "" {
			return fmt.Errorf("slackmcp: identity %q: access_token is required", id)
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Snapshot current state without holding the write lock during network I/O.
	s.mu.RLock()
	baseURL := s.baseURL
	prev := make(map[string]*identityState, len(s.identities))
	for k, v := range s.identities {
		prev[k] = v
	}
	factory := s.newRemote
	s.mu.RUnlock()
	if factory == nil {
		factory = newRemoteClient
	}

	next := make(map[string]*identityState, len(identities))
	keptRemotes := make(map[mcp.Integration]struct{})
	var created []mcp.Integration

	for _, id := range ids {
		ident := identities[id]
		tok := strings.TrimSpace(ident.Credentials["access_token"])
		meta := copyMetadata(ident.Metadata)

		// Reuse existing remote only when ID + token + baseURL are unchanged.
		if p, ok := prev[id]; ok && p.token == tok && p.baseURL == baseURL && p.remote != nil {
			// Refresh tools outside the state write lock (network I/O).
			tools := translateUpstreamTools(p.remote.Tools())
			st := &identityState{
				id:       id,
				token:    tok,
				baseURL:  baseURL,
				metadata: meta,
				remote:   p.remote,
				tools:    tools,
			}
			next[id] = st
			keptRemotes[p.remote] = struct{}{}
			continue
		}

		// New or token/baseURL-changed identity: build a fresh remote client.
		// remotemcp always appends /mcp to the base URL.
		// We use an internal prefix that we strip during translation so upstream
		// slack_* names become slackmcp_* exactly once (never slackmcp_slack_*).
		remote := factory(baseURL)
		if err := remote.Configure(ctx, mcp.Credentials{"access_token": tok}); err != nil {
			for _, r := range created {
				closeRemote(r)
			}
			return fmt.Errorf("slackmcp: identity %q: %w", id, err)
		}
		created = append(created, remote)
		st := &identityState{
			id:       id,
			token:    tok,
			baseURL:  baseURL,
			metadata: meta,
			remote:   remote,
			tools:    translateUpstreamTools(remote.Tools()),
		}
		next[id] = st
		keptRemotes[remote] = struct{}{}
	}

	// Remotes belonging to removed or replaced identities must be closed.
	var toClose []mcp.Integration
	for _, p := range prev {
		if p == nil || p.remote == nil {
			continue
		}
		if _, keep := keptRemotes[p.remote]; !keep {
			toClose = append(toClose, p.remote)
		}
	}

	// Atomic snapshot swap under the state mutex only.
	s.mu.Lock()
	s.identities = next
	s.order = ids
	s.mu.Unlock()

	// Close outside the state mutex so session teardown cannot deadlock readers.
	for _, r := range toClose {
		closeRemote(r)
	}
	return nil
}

// newRemoteClient creates a remotemcp client. Tools come back prefixed with
// internalRemotePrefix; translateUpstreamTools rewrites them to slackmcp_*.
func newRemoteClient(baseURL string) mcp.Integration {
	return remotemcp.New(internalRemotePrefix, baseURL)
}

func closeRemote(r mcp.Integration) {
	type closer interface {
		Close() error
	}
	if c, ok := r.(closer); ok {
		_ = c.Close()
	}
}

// internalRemotePrefix is used only inside remotemcp name mangling. Upstream
// Slack tools are already named slack_*; remotemcp would emit
// <prefix>_slack_send_message. We choose a prefix we can detect and strip.
const internalRemotePrefix = "sbslackup"

func translateUpstreamTools(upstream []mcp.ToolDefinition) []mcp.ToolDefinition {
	out := make([]mcp.ToolDefinition, 0, len(upstream))
	for _, t := range upstream {
		name := string(t.Name)
		// remotemcp emits internalRemotePrefix + "_" + upstreamName
		name = strings.TrimPrefix(name, internalRemotePrefix+"_")
		exposed := exposeToolName(name)
		params := copyParams(t.Parameters)
		req := append([]string(nil), t.Required...)
		out = append(out, mcp.ToolDefinition{
			Name:        mcp.ToolName(exposed),
			Description: t.Description,
			Parameters:  params,
			Required:    req,
		})
	}
	return out
}

func exposeToolName(upstream string) string {
	if strings.HasPrefix(upstream, upstreamSlackPrefix) {
		return exposedPrefix + strings.TrimPrefix(upstream, upstreamSlackPrefix)
	}
	// Fallback: still namespace under slackmcp_
	return exposedPrefix + upstream
}

func upstreamToolName(exposed string) string {
	if !strings.HasPrefix(exposed, exposedPrefix) {
		return exposed
	}
	rest := strings.TrimPrefix(exposed, exposedPrefix)
	return upstreamSlackPrefix + rest
}

func copyParams(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyMetadata(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *slackmcp) Tools() []mcp.ToolDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deterministic union keyed by exposed tool name.
	union := map[string]mcp.ToolDefinition{}
	for _, id := range s.order {
		st := s.identities[id]
		if st == nil {
			continue
		}
		for _, t := range st.tools {
			name := string(t.Name)
			if _, exists := union[name]; exists {
				continue
			}
			params := copyParams(t.Parameters)
			params[identityParam] = "Configured identity ID (from slackmcp_list_available_identites)"
			req := append([]string(nil), t.Required...)
			if !containsString(req, identityParam) {
				req = append([]string{identityParam}, req...)
			}
			union[name] = mcp.ToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
				Required:    req,
			}
		}
	}

	names := make([]string, 0, len(union))
	for n := range union {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]mcp.ToolDefinition, 0, len(names)+1)
	out = append(out, mcp.ToolDefinition{
		Name:        mcp.ToolName(listIdentitiesTool),
		Description: "Start here. List configured Slack MCP identities (IDs, metadata, and available tools). Does not return secrets.",
		Parameters:  map[string]string{},
	})
	for _, n := range names {
		out = append(out, union[n])
	}
	return out
}

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func (s *slackmcp) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	name := string(toolName)
	if name == listIdentitiesTool {
		return s.listIdentities()
	}

	if !strings.HasPrefix(name, exposedPrefix) {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}

	r := mcp.NewArgs(args)
	identityID := strings.TrimSpace(r.Str(identityParam))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if identityID == "" {
		return mcp.ErrResult(fmt.Errorf("identity_id is required"))
	}

	s.mu.RLock()
	st, ok := s.identities[identityID]
	s.mu.RUnlock()
	if !ok || st == nil || st.remote == nil {
		return mcp.ErrResult(fmt.Errorf("unknown identity: %s", identityID))
	}

	// Strip synthetic identity_id before forwarding.
	fwd := make(map[string]any, len(args))
	for k, v := range args {
		if k == identityParam {
			continue
		}
		fwd[k] = v
	}

	upstream := upstreamToolName(name)
	// remotemcp.Execute expects the prefixed tool name it produced.
	remoteName := mcp.ToolName(internalRemotePrefix + "_" + upstream)
	return st.remote.Execute(ctx, remoteName, fwd)
}

func (s *slackmcp) listIdentities() (*mcp.ToolResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type identityInfo struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata,omitempty"`
		Tools    []string          `json:"tools"`
	}
	list := make([]identityInfo, 0, len(s.order))
	for _, id := range s.order {
		st := s.identities[id]
		if st == nil {
			continue
		}
		toolNames := make([]string, 0, len(st.tools))
		for _, t := range st.tools {
			toolNames = append(toolNames, string(t.Name))
		}
		sort.Strings(toolNames)
		list = append(list, identityInfo{
			ID:       id,
			Metadata: copyMetadata(st.metadata),
			Tools:    toolNames,
		})
	}
	return mcp.JSONResult(map[string]any{
		"identities": list,
	})
}

func (s *slackmcp) Healthy(ctx context.Context) bool {
	s.mu.RLock()
	ids := append([]string(nil), s.order...)
	idents := make(map[string]*identityState, len(s.identities))
	for k, v := range s.identities {
		idents[k] = v
	}
	s.mu.RUnlock()

	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		st := idents[id]
		if st == nil || st.remote == nil {
			continue
		}
		if st.remote.Healthy(ctx) {
			return true
		}
	}
	return false
}
