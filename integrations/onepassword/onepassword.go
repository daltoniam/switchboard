// Package onepassword reads and writes 1Password items through the official
// 1Password Go SDK, signed in with a service account token.
package onepassword

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	op "github.com/1password/onepassword-sdk-go"
	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("onepassword", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// store is the part of 1Password the adapter uses. The SDK implements it in
// production; tests use a fake, so handlers run without the SDK's
// WebAssembly core or a network.
type store interface {
	ListVaults(ctx context.Context) ([]op.VaultOverview, error)
	ListItems(ctx context.Context, vaultID string) ([]op.ItemOverview, error)
	GetItem(ctx context.Context, vaultID, itemID string) (op.Item, error)
	CreateItem(ctx context.Context, params op.ItemCreateParams) (op.Item, error)
	PutItem(ctx context.Context, item op.Item) (op.Item, error)
	ResolveSecret(ctx context.Context, reference string) (string, error)
	GeneratePassword(ctx context.Context) (string, error)
}

// connectFunc signs in to 1Password with a service account token.
type connectFunc func(ctx context.Context, token string) (store, error)

type handlerFunc func(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error)

type onepassword struct {
	connect connectFunc

	mu     sync.Mutex
	token  string
	client store
}

var (
	_ mcp.Integration                = (*onepassword)(nil)
	_ mcp.FieldCompactionIntegration = (*onepassword)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*onepassword)(nil)
	_ mcp.PlaceholderHints           = (*onepassword)(nil)
	_ io.Closer                      = (*onepassword)(nil)
)

func New() mcp.Integration { return &onepassword{connect: connectSDK} }

func (*onepassword) Name() string                { return "onepassword" }
func (*onepassword) Tools() []mcp.ToolDefinition { return tools }
func (*onepassword) Placeholders() map[string]string {
	return map[string]string{
		"service_account_token": "Service account token (ops_...) from 1Password Developer > Service accounts",
	}
}

// Configure stores the token. Signing in happens on first use, because it
// starts the SDK's WebAssembly core and calls 1Password.
func (a *onepassword) Configure(_ context.Context, creds mcp.Credentials) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = nil
	a.token = ""
	token := strings.TrimSpace(creds["service_account_token"])
	if token == "" {
		return fmt.Errorf("onepassword: service_account_token is required")
	}
	if !strings.HasPrefix(token, "ops_") || strings.ContainsAny(token, " \t\r\n") {
		return fmt.Errorf("onepassword: service_account_token must be a 1Password service account token (ops_...)")
	}
	a.token = token
	return nil
}

// signedIn returns the signed-in client, signing in once per token.
func (a *onepassword) signedIn(ctx context.Context) (store, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token == "" {
		return nil, fmt.Errorf("onepassword: integration is not configured")
	}
	if a.client == nil {
		client, err := a.connect(ctx, a.token)
		if err != nil {
			return nil, fmt.Errorf("onepassword: cannot sign in with the service account token: %w", err)
		}
		a.client = client
	}
	return a.client, nil
}

// Healthy signs in and lists vaults, which any service account may do.
func (a *onepassword) Healthy(ctx context.Context) bool {
	s, err := a.signedIn(ctx)
	if err != nil {
		return false
	}
	_, err = s.ListVaults(ctx)
	return err == nil
}

func (a *onepassword) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("onepassword: unknown tool: %s", toolName))
	}
	s, err := a.signedIn(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return fn(ctx, s, args)
}

// Close drops the signed-in client; the SDK releases it when it is
// garbage-collected. Safe to call more than once.
func (a *onepassword) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = nil
	return nil
}

func (*onepassword) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (*onepassword) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// sdkErr labels an SDK error and marks rate limits retryable. The SDK's
// messages name the problem without echoing secret values.
func sdkErr(err error) error {
	var limited *op.RateLimitExceededError
	if errors.As(err, &limited) {
		return &mcp.RetryableError{StatusCode: http.StatusTooManyRequests, Err: fmt.Errorf("onepassword: rate limit exceeded: %w", err)}
	}
	return fmt.Errorf("onepassword: %w", err)
}

// sdkStore is the store backed by the 1Password Go SDK.
type sdkStore struct{ client *op.Client }

func connectSDK(ctx context.Context, token string) (store, error) {
	client, err := op.NewClient(ctx,
		op.WithServiceAccountToken(token),
		op.WithIntegrationInfo("Switchboard", op.DefaultIntegrationVersion),
	)
	if err != nil {
		return nil, err
	}
	return sdkStore{client: client}, nil
}

func (s sdkStore) ListVaults(ctx context.Context) ([]op.VaultOverview, error) {
	return s.client.Vaults().List(ctx)
}

func (s sdkStore) ListItems(ctx context.Context, vaultID string) ([]op.ItemOverview, error) {
	return s.client.Items().List(ctx, vaultID)
}

func (s sdkStore) GetItem(ctx context.Context, vaultID, itemID string) (op.Item, error) {
	return s.client.Items().Get(ctx, vaultID, itemID)
}

func (s sdkStore) CreateItem(ctx context.Context, params op.ItemCreateParams) (op.Item, error) {
	return s.client.Items().Create(ctx, params)
}

func (s sdkStore) PutItem(ctx context.Context, item op.Item) (op.Item, error) {
	return s.client.Items().Put(ctx, item)
}

func (s sdkStore) ResolveSecret(ctx context.Context, reference string) (string, error) {
	return s.client.Secrets().Resolve(ctx, reference)
}

func (sdkStore) GeneratePassword(ctx context.Context) (string, error) {
	res, err := op.Secrets.GeneratePassword(ctx, op.NewPasswordRecipeTypeVariantRandom(&op.PasswordRecipeRandomInner{
		IncludeDigits: true, IncludeSymbols: true, Length: 32,
	}))
	if err != nil {
		return "", err
	}
	return res.Password, nil
}
