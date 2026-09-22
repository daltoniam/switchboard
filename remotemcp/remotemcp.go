package remotemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/version"
	"github.com/modelcontextprotocol/go-sdk/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

const (
	defaultTimeout      = 30 * time.Second
	defaultEndpointPath = "/mcp"
)

// Options tunes how a remote MCP integration reaches and authenticates with its server.
type Options struct {
	// EndpointPath is appended to the server URL to form the MCP endpoint; defaults to "/mcp".
	EndpointPath string
	// OptionalToken allows Configure to succeed without an access_token.
	OptionalToken bool
	// OnTokenRefresh receives every rotated token set so the caller can persist it.
	OnTokenRefresh func(TokenSet)
}

type remote struct {
	name         string
	serverURL    string
	endpointPath string

	mu             sync.RWMutex
	token          string
	refreshToken   string
	clientID       string
	clientSecret   string
	session        *mcpsdk.ClientSession
	client         *mcpsdk.Client
	cachedTools    []mcp.ToolDefinition
	toolsFetched   bool
	optionalToken  bool
	onTokenRefresh func(TokenSet)
	refreshMu      sync.Mutex
	connectMu      sync.Mutex
	// A refresh token the issuer rejected is never replayed; Configure clears it.
	rejectedRefreshToken string
	rejectedRefreshErr   error
}

var _ auth.OAuthHandler = (*remote)(nil)

// New creates a remote MCP integration that proxies to the given server URL.
func New(name, serverURL string) mcp.Integration {
	return NewWithOptions(name, serverURL, Options{})
}

func NewOptionalToken(name, serverURL string) mcp.Integration {
	return NewWithOptions(name, serverURL, Options{OptionalToken: true})
}

// NewWithOptions creates a remote MCP integration with a custom endpoint path and refresh hook.
func NewWithOptions(name, serverURL string, opts Options) mcp.Integration {
	endpointPath := strings.TrimSpace(opts.EndpointPath)
	if endpointPath == "" {
		endpointPath = defaultEndpointPath
	}
	return &remote{
		name:           name,
		serverURL:      serverURL,
		endpointPath:   endpointPath,
		optionalToken:  opts.OptionalToken,
		onTokenRefresh: opts.OnTokenRefresh,
	}
}

func (r *remote) Name() string { return r.name }

func (r *remote) Configure(_ context.Context, creds mcp.Credentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	token := creds["access_token"]
	if token == "" && !r.optionalToken {
		return fmt.Errorf("%s: access_token is required", r.name)
	}

	if token != r.token {
		if r.session != nil {
			_ = r.session.Close()
			r.session = nil
		}
		r.token = token
		r.toolsFetched = false
		r.cachedTools = nil
	}
	r.refreshToken = creds["refresh_token"]
	r.clientID = creds["client_id"]
	r.clientSecret = creds["client_secret"]
	r.rejectedRefreshToken = ""
	r.rejectedRefreshErr = nil
	return nil
}

// TokenSource implements auth.OAuthHandler; a nil source leaves requests unauthenticated.
func (r *remote) TokenSource(context.Context) (oauth2.TokenSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.token == "" {
		return nil, nil
	}
	return currentTokenSource{r}, nil
}

type currentTokenSource struct{ r *remote }

func (s currentTokenSource) Token() (*oauth2.Token, error) {
	s.r.mu.RLock()
	defer s.r.mu.RUnlock()
	return &oauth2.Token{AccessToken: s.r.token, TokenType: "Bearer"}, nil
}

// Authorize implements auth.OAuthHandler: the SDK calls it on 401/403 and retries when it returns nil.
func (r *remote) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("%s: authorization failed (%d %s)", r.name, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	rejected := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	return r.refreshAccessToken(ctx, rejected)
}

func (r *remote) connect(ctx context.Context) (*mcpsdk.ClientSession, error) {
	r.mu.RLock()
	if r.session != nil {
		sess := r.session
		r.mu.RUnlock()
		return sess, nil
	}
	r.mu.RUnlock()

	// The SDK handshake calls back into TokenSource/Authorize, which take r.mu,
	// so connects serialize on connectMu and never hold r.mu across Connect.
	r.connectMu.Lock()
	defer r.connectMu.Unlock()

	r.mu.RLock()
	if r.session != nil {
		sess := r.session
		r.mu.RUnlock()
		return sess, nil
	}
	r.mu.RUnlock()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "switchboard",
		Version: version.String(),
	}, nil)

	transport := &mcpsdk.StreamableClientTransport{
		Endpoint:             r.serverURL + r.endpointPath,
		HTTPClient:           &http.Client{Timeout: defaultTimeout},
		OAuthHandler:         r,
		DisableStandaloneSSE: true,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", r.serverURL, err)
	}

	r.mu.Lock()
	r.client = client
	r.session = session
	r.mu.Unlock()
	return session, nil
}

func (r *remote) disconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != nil {
		_ = r.session.Close()
		r.session = nil
	}
}

// Close closes any active session and clears session/client/tool cache.
// It is safe and idempotent; concurrent Configure/Tools/Execute serialize on r.mu.
func (r *remote) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != nil {
		_ = r.session.Close()
		r.session = nil
	}
	r.client = nil
	r.toolsFetched = false
	r.cachedTools = nil
	return nil
}

func (r *remote) Healthy(ctx context.Context) bool {
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
	r.mu.RLock()
	if r.toolsFetched {
		tools := r.cachedTools
		r.mu.RUnlock()
		return tools
	}
	r.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	session, err := r.connect(ctx)
	if err != nil {
		return nil
	}

	result, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		return nil
	}

	tools := convertTools(r.name, result.Tools)

	r.mu.Lock()
	r.cachedTools = tools
	r.toolsFetched = true
	r.mu.Unlock()

	return tools
}

func (r *remote) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	session, err := r.connect(ctx)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	remoteName := strings.TrimPrefix(string(toolName), r.name+"_")
	if args == nil {
		args = map[string]any{}
	}

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      remoteName,
		Arguments: args,
	})
	if err != nil {
		r.disconnect()
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
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
