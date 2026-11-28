// Package onepassword reads and writes 1Password items through the official
// 1Password Go SDK, signed in through the 1Password desktop app or with a
// service account token.
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
	"unicode"

	op "github.com/1password/onepassword-sdk-go"
	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/daltoniam/switchboard/version"
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
	ListItems(ctx context.Context, vaultID string, filter op.ItemListFilter) ([]op.ItemOverview, error)
	GetItem(ctx context.Context, vaultID, itemID string) (op.Item, error)
	CreateItem(ctx context.Context, params op.ItemCreateParams) (op.Item, error)
	PutItem(ctx context.Context, item op.Item) (op.Item, error)
	ArchiveItem(ctx context.Context, vaultID, itemID string) error
	DeleteItem(ctx context.Context, vaultID, itemID string) error
	ResolveSecret(ctx context.Context, reference string) (string, error)
	GeneratePassword(ctx context.Context) (string, error)
}

// auth holds exactly one sign-in method: a 1Password account the desktop app
// is signed in to, or a service account token.
type auth struct {
	account string
	token   string
}

func (a auth) method() string {
	if a.account != "" {
		return "through the 1Password desktop app"
	}
	return "with the service account token"
}

// connectFunc signs in to 1Password.
type connectFunc func(ctx context.Context, a auth) (store, error)

type handlerFunc func(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error)

// session is the signed-in client for one credential set. Identities with
// the same credentials share it, and it outlives reconfiguration while the
// credentials stay, so reloading settings does not prompt again.
type session struct {
	mu      sync.Mutex
	client  store
	lastErr error
}

type onepassword struct {
	connect          connectFunc
	desktopAvailable bool

	mu       sync.Mutex
	top      auth
	named    []identity
	sessions map[auth]*session
}

var (
	_ mcp.Integration                = (*onepassword)(nil)
	_ mcp.FieldCompactionIntegration = (*onepassword)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*onepassword)(nil)
	_ mcp.PlaceholderHints           = (*onepassword)(nil)
	_ mcp.PlainTextCredentials       = (*onepassword)(nil)
	_ io.Closer                      = (*onepassword)(nil)
)

func New() mcp.Integration {
	return &onepassword{connect: connectSDK, desktopAvailable: desktopAppAvailable}
}

// DesktopAppAvailable reports whether this build can sign in through the
// 1Password desktop app.
func DesktopAppAvailable() bool { return desktopAppAvailable }

const maxAccountNameLen = 256

// desktopHint is added to desktop sign-in failures because the SDK's errors
// do not say where the app must run.
const desktopHint = "Switchboard must run on the same computer and user session as the unlocked 1Password app, not in Docker, over SSH or as a system service. " +
	"In 1Password, open Settings > Developer and turn on Integrate with 1Password SDKs, then approve the prompt 1Password shows"

func (*onepassword) Name() string                { return "onepassword" }
func (*onepassword) Tools() []mcp.ToolDefinition { return tools }
func (*onepassword) Placeholders() map[string]string {
	return map[string]string{
		"account_name":          "Account name shown at the top of the 1Password desktop app sidebar",
		"service_account_token": "Optional service account token (ops_...) for headless setups",
	}
}
func (*onepassword) PlainTextKeys() []string { return []string{"account_name"} }

// Configure stores the default identity's sign-in method, which may be empty
// when named identities provide every account; ConfigureIdentities checks
// that at least one exists. Signing in happens on first use, because it
// starts the SDK core and may show a 1Password approval prompt.
func (a *onepassword) Configure(_ context.Context, creds mcp.Credentials) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	next, err := a.parseAuth(creds)
	if err != nil {
		a.top, a.named = auth{}, nil
		a.pruneSessions()
		return fmt.Errorf("onepassword: %w", err)
	}
	a.top = next
	a.pruneSessions()
	return nil
}

// parseAuth reads one sign-in method; no credentials at all give a zero auth.
func (a *onepassword) parseAuth(creds mcp.Credentials) (auth, error) {
	account := strings.TrimSpace(creds["account_name"])
	token := strings.TrimSpace(creds["service_account_token"])
	switch {
	case account == "" && token == "":
		return auth{}, nil
	case account != "" && token != "":
		return auth{}, fmt.Errorf("set account_name or service_account_token, not both; clear one (the OP_ACCOUNT_NAME and OP_SERVICE_ACCOUNT_TOKEN environment variables also set them)")
	case token != "":
		if !strings.HasPrefix(token, "ops_") || strings.ContainsAny(token, " \t\r\n") {
			return auth{}, fmt.Errorf("service_account_token must be a 1Password service account token (ops_...)")
		}
		return auth{token: token}, nil
	}
	if len(account) > maxAccountNameLen || strings.ContainsFunc(account, unicode.IsControl) {
		return auth{}, fmt.Errorf("account_name must be the account name shown in the 1Password app sidebar, or the account ID")
	}
	if !a.desktopAvailable {
		return auth{}, fmt.Errorf("this Switchboard build cannot use the 1Password desktop app, because it was built without CGO. Build Switchboard on this computer with go install or make build, or use a service account token")
	}
	return auth{account: account}, nil
}

// pruneSessions drops signed-in clients no identity uses any more. The
// caller holds a.mu.
func (a *onepassword) pruneSessions() {
	used := map[auth]bool{}
	for _, ident := range a.identitiesLocked() {
		used[ident.auth] = true
	}
	for au := range a.sessions {
		if !used[au] {
			delete(a.sessions, au)
		}
	}
}

func (a *onepassword) sessionLocked(au auth) *session {
	if a.sessions == nil {
		a.sessions = map[auth]*session{}
	}
	s, ok := a.sessions[au]
	if !ok {
		s = &session{}
		a.sessions[au] = s
	}
	return s
}

// signedIn returns the signed-in client for the selected identity, signing
// in once per credential set. Sign-ins for different identities do not wait
// on each other, so a pending desktop approval never blocks a service
// account.
func (a *onepassword) signedIn(ctx context.Context, id string) (store, error) {
	a.mu.Lock()
	ident, err := a.resolveLocked(id)
	if err != nil {
		a.mu.Unlock()
		return nil, err
	}
	sess := a.sessionLocked(ident.auth)
	a.mu.Unlock()
	return a.signIn(ctx, ident.auth, sess)
}

func (a *onepassword) signIn(ctx context.Context, au auth, sess *session) (store, error) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.client == nil {
		client, err := a.connect(ctx, au)
		if err != nil {
			err = fmt.Errorf("onepassword: cannot sign in %s: %w", au.method(), err)
			if au.account != "" {
				err = fmt.Errorf("%w. %s", err, desktopHint)
			}
			sess.lastErr = err
			return nil, err
		}
		sess.client = client
		sess.lastErr = nil
	}
	return sess.client, nil
}

// SignInError returns why the last sign-in or health check failed, or "" when
// it succeeded, so the setup page can say what to fix. With several
// identities each failure names its identity.
func (a *onepassword) SignInError() string {
	a.mu.Lock()
	all := a.identitiesLocked()
	sessions := make([]*session, len(all))
	for i, ident := range all {
		sessions[i] = a.sessions[ident.auth]
	}
	a.mu.Unlock()
	var msgs []string
	for i, sess := range sessions {
		if sess == nil {
			continue
		}
		sess.mu.Lock()
		err := sess.lastErr
		sess.mu.Unlock()
		if err == nil {
			continue
		}
		if len(all) == 1 {
			return err.Error()
		}
		msgs = append(msgs, fmt.Sprintf("identity %q: %s", all[i].id, err))
	}
	return strings.Join(msgs, "\n")
}

// Healthy signs in to every identity and lists its vaults, which any
// account or service account may do.
func (a *onepassword) Healthy(ctx context.Context) bool {
	a.mu.Lock()
	all := a.identitiesLocked()
	sessions := make([]*session, len(all))
	for i, ident := range all {
		sessions[i] = a.sessionLocked(ident.auth)
	}
	a.mu.Unlock()
	healthy := len(all) > 0
	for i, sess := range sessions {
		s, err := a.signIn(ctx, all[i].auth, sess)
		if err == nil {
			_, err = s.ListVaults(ctx)
			sess.mu.Lock()
			sess.lastErr = nil
			if err != nil {
				sess.lastErr = sdkErr(err)
			}
			sess.mu.Unlock()
		}
		if err != nil {
			healthy = false
		}
	}
	return healthy
}

func (a *onepassword) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if fn, ok := adapterDispatch[toolName]; ok {
		return fn(a)
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("onepassword: unknown tool: %s", toolName))
	}
	id, args, err := identityArg(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	s, err := a.signedIn(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return fn(ctx, s, args)
}

// Close drops every signed-in client; the SDK releases them when they are
// garbage-collected. Safe to call more than once.
func (a *onepassword) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = nil
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

func connectSDK(ctx context.Context, a auth) (store, error) {
	signIn := op.WithServiceAccountToken(a.token)
	if a.account != "" {
		if err := processDesktop.bind(a.account); err != nil {
			return nil, err
		}
		signIn = op.WithDesktopAppIntegration(a.account)
	}
	client, err := op.NewClient(ctx, signIn, op.WithIntegrationInfo("Switchboard", version.String()))
	if err != nil {
		return nil, err
	}
	return sdkStore{client: client}, nil
}

func (s sdkStore) ListVaults(ctx context.Context) ([]op.VaultOverview, error) {
	return s.client.Vaults().List(ctx)
}

func (s sdkStore) ListItems(ctx context.Context, vaultID string, filter op.ItemListFilter) ([]op.ItemOverview, error) {
	return s.client.Items().List(ctx, vaultID, filter)
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

func (s sdkStore) ArchiveItem(ctx context.Context, vaultID, itemID string) error {
	return s.client.Items().Archive(ctx, vaultID, itemID)
}

func (s sdkStore) DeleteItem(ctx context.Context, vaultID, itemID string) error {
	return s.client.Items().Delete(ctx, vaultID, itemID)
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
