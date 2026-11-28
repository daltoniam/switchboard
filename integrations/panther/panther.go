// Package panther connects to Panther, the cloud SIEM, for security alerts,
// detections, data lake log search, log sources, schemas, and users. It has
// two modes, and exactly one is active; both use the instance URL a user
// configures.
//
// Sign in with Panther proxies Panther Remote MCP (beta, Panther 1.124+) at
// https://api.<instance>/mcp. Each user signs in with OAuth (dynamic client
// registration, PKCE, loopback redirect) and grants a subset of their own
// role's permissions on Panther's consent screen. Panther does not publish the
// tool list, which depends on that consent, so tools come from the server and
// are classified from their MCP annotations.
//
// API token mode calls Panther's REST API, and its GraphQL API where REST has
// no coverage (data lake, log sources, schemas, metrics), with an API token.
// It works on any Panther version and needs no api. subdomain for Remote MCP.
// An api_token credential selects this mode.
package panther

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/remotemcp"
)

const (
	integrationName = "panther"

	toolFetchTimeout = 30 * time.Second
	toolRetryAfter   = time.Minute
	// hostedEntryTool gets the "Start here" hint in sign-in mode when
	// Panther's server offers it.
	hostedEntryTool = mcp.ToolName("panther_list_alerts")
)

// firstListWait bounds how long Tools waits for the first Remote MCP tool
// list. Search indexes tools when the configuration changes, so returning
// before the list arrives would leave a signed-in Panther unsearchable until
// the next change; a slow server still cannot stall search for long.
var firstListWait = 10 * time.Second

var (
	_ mcp.Integration                = (*panther)(nil)
	_ mcp.PlainTextCredentials       = (*panther)(nil)
	_ mcp.OptionalCredentials        = (*panther)(nil)
	_ mcp.FieldCompactionIntegration = (*panther)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*panther)(nil)
	_ io.Closer                      = (*panther)(nil)
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay(integrationName, compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type panther struct {
	mu sync.RWMutex
	// native is set in API token mode; hosted is nil then.
	native *client
	hosted *hostedSession
	// creds are the sign-in credentials the hosted session was built from.
	creds   mcp.Credentials
	renewal remotemcp.TokenRenewal
	// configGen changes on every Configure and Close, so a renewal that
	// started from older credentials does not overwrite newer ones or
	// reopen a closed adapter.
	configGen uint64
	// newRemote builds the Remote MCP client; tests replace it.
	newRemote func(origin string) mcp.Integration
}

// SetConfigService lets the integration renew its OAuth login before the
// access token expires and save the rotated tokens.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if p, ok := i.(*panther); ok {
		p.renewal.SetConfigService(svc)
	}
}

// New returns the Panther integration.
func New() mcp.Integration {
	return &panther{}
}

func (p *panther) Name() string { return integrationName }

func (p *panther) PlainTextKeys() []string { return []string{"instance_url"} }

func (p *panther) OptionalKeys() []string { return []string{"api_token", "mcp_access_token"} }

// Configure selects API token mode when api_token is set and Sign in with
// Panther otherwise. The setup page clears the other mode's credential on
// save, so both are present only when PANTHER_API_TOKEN overrides a saved
// sign-in.
func (p *panther) Configure(ctx context.Context, creds mcp.Credentials) error {
	inst, err := resolveInstance(creds["instance_url"])
	if err != nil {
		return err
	}
	if token := strings.TrimSpace(creds["api_token"]); token != "" {
		native := newClient(token, inst)
		p.mu.Lock()
		retired := p.detachHostedLocked()
		p.native = native
		p.mu.Unlock()
		// Ending the old session is best effort; it is abandoned either way.
		_ = retired.end()
		return nil
	}
	token := strings.TrimSpace(creds[remotemcp.CredKeyAccessToken])
	if token == "" {
		return errNoCredential
	}
	p.mu.Lock()
	retired, err := p.configureHostedLocked(ctx, creds, token, inst.mcpOrigin)
	if err == nil {
		// A working API token stays active until the sign-in is configured.
		p.native = nil
	}
	p.mu.Unlock()
	return errors.Join(err, retired.end())
}

// configureHostedLocked points the hosted session at origin with token and
// returns any session it replaced, for the caller to end after unlocking.
func (p *panther) configureHostedLocked(ctx context.Context, creds mcp.Credentials, token, origin string) (*hostedSession, error) {
	p.configGen++
	credentials := mcp.Credentials{"access_token": token}
	if p.hosted != nil && p.hosted.origin == origin {
		if err := p.hosted.remote.Configure(ctx, credentials); err != nil {
			return nil, err
		}
		if p.creds[remotemcp.CredKeyAccessToken] != token {
			// A new login may carry different consent, so the tool list is
			// read again.
			p.hosted.resetTools()
		}
		p.creds = maps.Clone(creds)
		return nil, nil
	}
	remote := p.buildRemote(origin)
	if err := remote.Configure(ctx, credentials); err != nil {
		return nil, fmt.Errorf("panther: configure Remote MCP: %w", errors.Join(err, closeRemote(remote)))
	}
	previous := p.hosted
	p.hosted = newHostedSession(remote, origin)
	p.creds = maps.Clone(creds)
	return previous, nil
}

func (p *panther) buildRemote(origin string) mcp.Integration {
	if p.newRemote != nil {
		return p.newRemote(origin)
	}
	return remotemcp.NewWithOptions(integrationName, origin, remotemcp.Options{Structured: true})
}

// renewIfDue renews the login when its access token has less than
// remotemcp.RenewalLead left. If renewal fails while the token still works,
// the failure is logged and the token is used until it expires.
func (p *panther) renewIfDue(ctx context.Context) error {
	p.mu.RLock()
	creds, generation, hosted := p.creds, p.configGen, p.hosted
	p.mu.RUnlock()
	if hosted == nil {
		return nil
	}
	target := remotemcp.RenewalTarget{Integration: integrationName, ServerURL: hosted.origin, Resource: hosted.origin + mcpPath}
	fresh, renewed, err := p.renewal.Fresh(ctx, target, creds)
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
		return fmt.Errorf("panther: renewed login has no access token")
	}
	p.mu.Lock()
	if p.configGen != generation {
		p.mu.Unlock()
		return nil
	}
	retired, err := p.configureHostedLocked(ctx, fresh, token, hosted.origin)
	p.mu.Unlock()
	return errors.Join(err, retired.end())
}

func (p *panther) active() (*client, *hostedSession) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.native, p.hosted
}

func (p *panther) Healthy(ctx context.Context) bool {
	native, hosted := p.active()
	if native != nil {
		return native.healthy(ctx)
	}
	if hosted == nil || p.renewIfDue(ctx) != nil {
		return false
	}
	_, hosted = p.active()
	if hosted == nil || !hosted.remote.Healthy(ctx) {
		return false
	}
	_ = hosted.awaitTools(ctx)
	return true
}

// Tools returns the native catalog in API token mode and before any
// configuration, so search can find Panther tools. Signed in, it returns the
// tools Panther's server offers, waiting at most firstListWait for the first
// list; the fetch runs in the background and is shared by every caller.
func (p *panther) Tools() []mcp.ToolDefinition {
	native, hosted := p.active()
	if native != nil || hosted == nil {
		return cloneTools(nativeTools)
	}
	tools, known := hosted.cachedTools()
	if !known {
		ctx, cancel := context.WithTimeout(context.Background(), firstListWait)
		defer cancel()
		// An expired login would fail the listing although its refresh
		// token still works.
		if err := p.renewIfDue(ctx); err != nil {
			log.Printf("WARN: %v", err)
		}
		if _, hosted = p.active(); hosted == nil {
			return cloneTools(nativeTools)
		}
		_ = hosted.awaitTools(ctx)
		tools, _ = hosted.cachedTools()
	}
	return decorateHosted(tools)
}

// decorateHosted adds wayfinding to Panther's own descriptions.
func decorateHosted(tools []mcp.ToolDefinition) []mcp.ToolDefinition {
	out := cloneTools(tools)
	for i := range out {
		if out[i].Name == hostedEntryTool && !strings.Contains(out[i].Description, "Start here") {
			out[i].Description += " Start here for Panther SIEM security alert triage and incident investigation."
		}
		if out[i].Parameters == nil {
			out[i].Parameters = map[string]string{}
		}
	}
	return out
}

func (p *panther) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	native, hosted := p.active()
	if native != nil {
		return native.execute(ctx, toolName, args)
	}
	if hosted == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	if err := p.renewIfDue(ctx); err != nil {
		return mcp.ErrResult(err)
	}
	_, hosted = p.active()
	if hosted == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	return hosted.execute(ctx, toolName, args)
}

// CompactSpec serves the REST and GraphQL specs in API token mode only.
// Remote MCP responses are shaped by Panther's server, so a native spec must
// never project them even where a tool name is shared.
func (p *panther) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	if native, _ := p.active(); native == nil {
		return nil, false
	}
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *panther) MaxBytes(toolName mcp.ToolName) (int, bool) {
	if native, _ := p.active(); native == nil {
		return 0, false
	}
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// Close ends the Remote MCP session and any tool fetch. It is safe to call
// more than once; a later Configure starts fresh.
func (p *panther) Close() error {
	p.mu.Lock()
	retired := p.detachHostedLocked()
	p.native = nil
	p.mu.Unlock()
	return retired.end()
}

// detachHostedLocked unhooks the hosted session so it can be ended after
// the lock is released; callers hold p.mu.
func (p *panther) detachHostedLocked() *hostedSession {
	p.configGen++
	retired := p.hosted
	p.hosted = nil
	p.creds = nil
	return retired
}

// hostedSession is one Remote MCP connection and the tool list it offers.
type hostedSession struct {
	remote mcp.Integration
	origin string

	lifetime context.Context
	stop     context.CancelFunc
	fetches  sync.WaitGroup

	mu       sync.Mutex
	tools    []mcp.ToolDefinition
	known    bool
	fetching bool
	// fetchDone is closed when the running background fetch ends.
	fetchDone     chan struct{}
	fetchFailedAt time.Time
	fetchErr      error
	// toolsGen changes when the list is reset, so a fetch that started for
	// an older login does not store its result.
	toolsGen uint64
}

func newHostedSession(remote mcp.Integration, origin string) *hostedSession {
	// #nosec G118 -- stop is called by end when the session is replaced or
	// the adapter closes.
	lifetime, stop := context.WithCancel(context.Background())
	return &hostedSession{remote: remote, origin: origin, lifetime: lifetime, stop: stop}
}

func (h *hostedSession) cachedTools() ([]mcp.ToolDefinition, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tools, h.known
}

func (h *hostedSession) resetTools() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tools, h.known, h.fetching, h.fetchFailedAt, h.fetchErr = nil, false, false, time.Time{}, nil
	h.toolsGen++
}

// startFetch starts a background tool list fetch unless one is running, the
// list is known, the session ended, or a fetch failed within toolRetryAfter.
// It returns a channel closed when the running fetch ends, or nil when
// there is nothing to wait for.
func (h *hostedSession) startFetch() <-chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fetching {
		return h.fetchDone
	}
	if h.known || h.lifetime.Err() != nil || (!h.fetchFailedAt.IsZero() && time.Since(h.fetchFailedAt) < toolRetryAfter) {
		return nil
	}
	h.fetching = true
	done := make(chan struct{})
	h.fetchDone = done
	h.fetches.Add(1)
	go func() {
		defer h.fetches.Done()
		defer close(done)
		ctx, cancel := context.WithTimeout(h.lifetime, toolFetchTimeout)
		defer cancel()
		_ = h.fetchTools(ctx)
	}()
	return done
}

// fetchTools reads the server's tool list and stores it unless the list
// was reset meanwhile.
func (h *hostedSession) fetchTools(ctx context.Context) error {
	lister, ok := h.remote.(remotemcp.ToolLister)
	if !ok {
		return fmt.Errorf("panther: Remote MCP client cannot list tools")
	}
	h.mu.Lock()
	generation := h.toolsGen
	h.mu.Unlock()
	listed, err := lister.ListTools(ctx)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.toolsGen != generation {
		return err
	}
	h.fetching = false
	if err != nil {
		h.fetchFailedAt, h.fetchErr = time.Now(), err
		return err
	}
	h.tools, h.known, h.fetchErr = slices.Clone(listed), true, nil
	return nil
}

// awaitTools waits, at most until ctx ends, for the shared background fetch
// when the list is not known yet, so concurrent callers make one ListTools
// call and a recent failure is not retried before toolRetryAfter. It
// returns why the list is still unknown.
func (h *hostedSession) awaitTools(ctx context.Context) error {
	if _, known := h.cachedTools(); known {
		return nil
	}
	if done := h.startFetch(); done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case h.known:
		return nil
	case h.fetchErr != nil:
		return h.fetchErr
	default:
		return fmt.Errorf("the tool list is not available yet")
	}
}

// execute forwards a call only for a tool Panther's server listed, so a
// name the consent screen did not grant fails here with a clear error.
func (h *hostedSession) execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if err := h.awaitTools(ctx); err != nil {
		return mcp.ErrResult(fmt.Errorf("panther: list Remote MCP tools: %w", err))
	}
	tools, _ := h.cachedTools()
	if !slices.ContainsFunc(tools, func(t mcp.ToolDefinition) bool { return t.Name == toolName }) {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s (signed in to Panther Remote MCP, which offers only the tools your consent granted; search again or sign in again to grant more)", toolName))
	}
	// remotemcp strips its own "panther_" prefix to recover the upstream name.
	return h.remote.Execute(ctx, toolName, args)
}

// end cancels the session's tool fetches, waits for them, and closes it.
// It is nil-safe so callers can end whatever session they detached.
func (h *hostedSession) end() error {
	if h == nil {
		return nil
	}
	// Stopping under h.mu orders it with startFetch's check, so no fetch
	// is added after Wait starts.
	h.mu.Lock()
	h.stop()
	h.mu.Unlock()
	h.fetches.Wait()
	return closeRemote(h.remote)
}

func closeRemote(remote mcp.Integration) error {
	if closer, ok := remote.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
