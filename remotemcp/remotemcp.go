package remotemcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/version"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTimeout      = 30 * time.Second
	defaultEndpointPath = "/mcp"
)

// ToolLister lists a remote server's tools under a caller's context, so a
// fetch can be canceled. Results are cached like Tools.
type ToolLister interface {
	ListTools(ctx context.Context) ([]mcp.ToolDefinition, error)
}

// Options configures a remote MCP integration built with NewWithOptions.
type Options struct {
	// EndpointPath is the MCP endpoint path on the server; "" means /mcp.
	EndpointPath string
	// Structured returns structuredContent as JSON, like NewStructured.
	Structured bool
}

type remote struct {
	name         string
	serverURL    string
	endpointPath string

	mu            sync.RWMutex
	token         string
	session       *liveSession
	sessionToken  string
	generation    uint64
	client        *mcpsdk.Client
	cachedTools   []mcp.ToolDefinition
	toolsFetched  bool
	optionalToken bool
	structured    bool

	// connecting admits one connection attempt at a time without holding mu
	// over the network, so waiters can give up when their context ends.
	connectingOnce sync.Once
	connecting     chan struct{}
	// afterDial, when set by tests, runs after a connection is opened and
	// before it is published.
	afterDial func()
}

// New creates a remote MCP integration that proxies to the given server URL.
func New(name, serverURL string) mcp.Integration {
	return &remote{
		name:      name,
		serverURL: serverURL,
	}
}

func NewOptionalToken(name, serverURL string) mcp.Integration {
	return &remote{
		name:          name,
		serverURL:     serverURL,
		optionalToken: true,
	}
}

// NewStructured is New for servers that put the JSON result in
// structuredContent and only a human-readable summary in content. Execute
// returns the structured JSON so compaction and scripts see real data.
func NewStructured(name, serverURL string) mcp.Integration {
	return &remote{
		name:       name,
		serverURL:  serverURL,
		structured: true,
	}
}

// NewWithOptions creates a remote MCP integration for servers whose endpoint
// is not /mcp or that need structured results.
func NewWithOptions(name, serverURL string, opts Options) mcp.Integration {
	return &remote{
		name:         name,
		serverURL:    serverURL,
		endpointPath: opts.EndpointPath,
		structured:   opts.Structured,
	}
}

func (r *remote) Name() string { return r.name }

func (r *remote) endpoint() string {
	if r.endpointPath == "" {
		return r.serverURL + defaultEndpointPath
	}
	return r.serverURL + r.endpointPath
}

func (r *remote) Configure(_ context.Context, creds mcp.Credentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	token := creds["access_token"]
	if token == "" && !r.optionalToken {
		return fmt.Errorf("%s: access_token is required", r.name)
	}

	if token != r.token {
		r.generation++
		if r.session != nil {
			r.session.close()
			r.session = nil
		}
		r.token = token
		r.toolsFetched = false
		r.cachedTools = nil
	}
	return nil
}

// bearerTransport injects an Authorization header into every request.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

func (r *remote) connectGate() chan struct{} {
	r.connectingOnce.Do(func() { r.connecting = make(chan struct{}, 1) })
	return r.connecting
}

func (r *remote) connect(ctx context.Context) (*liveSession, error) {
	gate := r.connectGate()
	for {
		r.mu.RLock()
		if r.session != nil {
			sess := r.session
			r.mu.RUnlock()
			return sess, nil
		}
		r.mu.RUnlock()

		select {
		case gate <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		sess, retry, err := r.dial(ctx)
		<-gate
		if !retry {
			return sess, err
		}
	}
}

// dial opens a session without holding mu. If Configure replaced the token
// meanwhile, the new session is closed and retry is true; if Close ran, the
// session is closed and an error returned so a closed adapter stays closed.
func (r *remote) dial(ctx context.Context) (sess *liveSession, retry bool, err error) {
	r.mu.RLock()
	if r.session != nil {
		sess = r.session
		r.mu.RUnlock()
		return sess, false, nil
	}
	token, generation := r.token, r.generation
	r.mu.RUnlock()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "switchboard",
		Version: version.String(),
	}, nil)

	httpClient := &http.Client{Timeout: defaultTimeout}
	if token != "" {
		httpClient.Transport = &bearerTransport{token: token}
	}
	transport := &mcpsdk.StreamableClientTransport{
		Endpoint:             r.endpoint(),
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, false, fmt.Errorf("connect to %s: %w", r.serverURL, err)
	}
	if r.afterDial != nil {
		r.afterDial()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.generation != generation {
		_ = session.Close()
		if r.token == token {
			return nil, false, fmt.Errorf("%s: closed while connecting", r.name)
		}
		return nil, true, nil
	}
	r.client = client
	r.session = &liveSession{ClientSession: session}
	r.sessionToken = token
	return r.session, false, nil
}

func (r *remote) disconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != nil {
		r.session.close()
		r.session = nil
	}
}

// drop forgets session if it is still the current one, so a concurrent
// caller that already reconnected keeps its new session.
func (r *remote) drop(session *liveSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == session {
		r.session = nil
	}
	session.close()
}

// liveSession is a client session that remembers whether this adapter has
// closed it, so a caller holding it can tell that nothing was sent on it.
type liveSession struct {
	*mcpsdk.ClientSession
	closed atomic.Bool
}

func (s *liveSession) close() {
	s.closed.Store(true)
	_ = s.Close()
}

// upstreamContext keeps ctx's cancellation and deadline but drops its values.
// When Execute runs inside a go-sdk server handler, ctx carries the caller's
// MCP-Protocol-Version, and the go-sdk client would send that version to the
// remote server instead of the one negotiated with it. A server that does not
// support the caller's version answers 404, which reads as "session not found".
func upstreamContext(ctx context.Context) context.Context {
	return valuelessContext{ctx}
}

type valuelessContext struct{ context.Context }

func (valuelessContext) Value(any) any { return nil }

// sessionGone reports whether connecting failed because the session ended:
// the server answered 404 for it, or it was closed locally. Connecting sends
// no tool call, so trying again is safe.
func sessionGone(err error) bool {
	return errors.Is(err, mcpsdk.ErrSessionMissing) || errors.Is(err, mcpsdk.ErrConnectionClosed)
}

// retrySafe reports whether a failed call can be sent again: only when the
// server answered that it has no such session, so it never ran the call. A
// closed connection proves nothing, since the call may already have run.
func retrySafe(err error) bool {
	return errors.Is(err, mcpsdk.ErrSessionMissing)
}

// callTool calls the tool on session. If this adapter closed the session
// before the call (Configure replaced it), nothing was sent, so it uses a new
// session. If the server answers that the session is gone, it opens a new
// session and tries once more. Other failures are not retried because the
// call may already have taken effect.
func (r *remote) callTool(ctx context.Context, session *liveSession, params *mcpsdk.CallToolParams) (*mcpsdk.CallToolResult, error) {
	if session.closed.Load() {
		r.drop(session)
		var err error
		if session, err = r.connect(ctx); err != nil {
			return nil, err
		}
	}
	result, err := session.CallTool(ctx, params)
	if err == nil {
		return result, nil
	}
	r.drop(session)
	if !retrySafe(err) {
		return nil, err
	}
	session, err = r.connect(ctx)
	if err != nil {
		return nil, err
	}
	result, err = session.CallTool(ctx, params)
	if err != nil {
		r.drop(session)
		return nil, err
	}
	return result, nil
}

// Close closes any active session and clears session/client/tool cache.
// It is safe and idempotent; concurrent Configure/Tools/Execute serialize on r.mu.
func (r *remote) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generation++
	if r.session != nil {
		r.session.close()
		r.session = nil
	}
	r.client = nil
	r.toolsFetched = false
	r.cachedTools = nil
	return nil
}

func (r *remote) Healthy(ctx context.Context) bool {
	ctx = upstreamContext(ctx)
	session, err := r.connect(ctx)
	if err != nil {
		return false
	}
	_, err = session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		r.disconnect()
		return false
	}
	return true
}

func (r *remote) Tools() []mcp.ToolDefinition {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	tools, _ := r.ListTools(ctx)
	return tools
}

// ListTools returns the cached tool list, fetching it under ctx when no
// fetch has succeeded yet.
func (r *remote) ListTools(ctx context.Context) ([]mcp.ToolDefinition, error) {
	r.mu.RLock()
	if r.toolsFetched {
		tools := r.cachedTools
		r.mu.RUnlock()
		return tools, nil
	}
	r.mu.RUnlock()

	ctx = upstreamContext(ctx)
	session, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}

	result, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	tools := convertTools(r.name, result.Tools)

	r.mu.Lock()
	r.cachedTools = tools
	r.toolsFetched = true
	r.mu.Unlock()

	return tools, nil
}

func (r *remote) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	ctx = upstreamContext(ctx)
	session, err := r.connect(ctx)
	if sessionGone(err) {
		session, err = r.connect(ctx)
	}
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	remoteName := strings.TrimPrefix(string(toolName), r.name+"_")
	if args == nil {
		args = map[string]any{}
	}

	result, err := r.callTool(ctx, session, &mcpsdk.CallToolParams{
		Name:      remoteName,
		Arguments: args,
	})
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	if r.structured {
		return convertStructuredResult(result), nil
	}
	return convertResult(result), nil
}

func convertTools(prefix string, tools []*mcpsdk.Tool) []mcp.ToolDefinition {
	var defs []mcp.ToolDefinition
	for _, t := range tools {
		params := extractParams(t.InputSchema)
		required := extractRequired(t.InputSchema)

		defs = append(defs, mcp.ToolDefinition{
			Name:        mcp.ToolName(prefix + "_" + t.Name),
			Description: t.Description,
			Parameters:  params,
			Required:    required,
		})
	}
	return defs
}

func extractParams(schema any) map[string]string {
	params := make(map[string]string)
	schemaMap, ok := toMap(schema)
	if !ok {
		return params
	}
	props, ok := toMap(schemaMap["properties"])
	if !ok {
		return params
	}
	for k, v := range props {
		desc := ""
		if vMap, ok := toMap(v); ok {
			if d, ok := vMap["description"].(string); ok {
				desc = d
			}
		}
		params[k] = desc
	}
	return params
}

func extractRequired(schema any) []string {
	schemaMap, ok := toMap(schema)
	if !ok {
		return nil
	}
	req, ok := schemaMap["required"]
	if !ok {
		return nil
	}
	reqArr, ok := req.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, v := range reqArr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

// convertStructuredResult returns structuredContent as JSON when the call
// succeeded and the server sent it; errors keep their text explanation.
func convertStructuredResult(result *mcpsdk.CallToolResult) *mcp.ToolResult {
	converted := convertResult(result)
	if result == nil || result.IsError || result.StructuredContent == nil {
		return converted
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return converted
	}
	converted.Data = string(data)
	return converted
}

func convertResult(result *mcpsdk.CallToolResult) *mcp.ToolResult {
	if result == nil {
		return &mcp.ToolResult{Data: "no result", IsError: true}
	}

	var parts []string
	var media []mcp.MediaContent
	for _, content := range result.Content {
		switch item := content.(type) {
		case *mcpsdk.TextContent:
			parts = append(parts, item.Text)
		case *mcpsdk.ImageContent:
			media = append(media, mcp.MediaContent{Data: item.Data, MIMEType: item.MIMEType})
		case *mcpsdk.EmbeddedResource:
			if item.Resource == nil {
				continue
			}
			if item.Resource.Text != "" {
				parts = append(parts, item.Resource.Text)
			}
			if len(item.Resource.Blob) > 0 {
				media = append(media, mcp.MediaContent{Data: item.Resource.Blob, MIMEType: item.Resource.MIMEType, Name: item.Resource.URI})
			}
		default:
			data, err := json.Marshal(content)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}

	return &mcp.ToolResult{
		Data:    strings.Join(parts, "\n"),
		Media:   media,
		IsError: result.IsError,
	}
}
