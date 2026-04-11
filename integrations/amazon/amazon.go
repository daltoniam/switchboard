package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	mcp "github.com/daltoniam/switchboard"
)

type amazon struct {
	email     string
	password  string
	otpSecret string

	browserCookies []mcp.BrowserCookie
	httpCookies    []*http.Cookie // kept for HTTP fallback + tests
	client         *http.Client
	domain         string
	baseURL        string // test-only override; when set, URL helpers use this prefix instead of domain

	browserSvc   mcp.BrowserService
	session      mcp.BrowserSession
	sessionMu    sync.Mutex
	loginMu      sync.Mutex
	loginVersion atomic.Uint64
	loginFunc    func(ctx context.Context) error // overridden in tests
}

var _ mcp.FieldCompactionIntegration = (*amazon)(nil)

const (
	maxResponseSize = 5 * 1024 * 1024 // 5 MB
	defaultDomain   = "amazon.com"
	userAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
)

func New() mcp.Integration {
	return &amazon{
		client: &http.Client{Timeout: 30 * time.Second},
		domain: defaultDomain,
	}
}

// SetBrowserService injects the browser automation service (called from main.go).
func SetBrowserService(i mcp.Integration, svc mcp.BrowserService) {
	if a, ok := i.(*amazon); ok {
		a.sessionMu.Lock()
		a.browserSvc = svc
		a.sessionMu.Unlock()
	}
}

func (a *amazon) Name() string { return "amazon" }

func (a *amazon) Configure(_ context.Context, creds mcp.Credentials) error {
	email := strings.TrimSpace(creds["email"])
	password := creds["password"]
	otpSecret := strings.TrimSpace(creds["otp_secret"])

	domain := defaultDomain
	if v := creds["domain"]; v != "" {
		domain = strings.TrimPrefix(v, "www.")
	}

	var browserCookies []mcp.BrowserCookie
	var httpCookies []*http.Cookie

	raw := creds["cookies"]
	if raw != "" {
		var parsed []cookieJSON
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return fmt.Errorf("amazon: invalid cookies JSON: %w", err)
		}
		browserCookies = make([]mcp.BrowserCookie, 0, len(parsed))
		httpCookies = make([]*http.Cookie, 0, len(parsed))
		for _, c := range parsed {
			bc := mcp.BrowserCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Secure:   c.Secure,
				HTTPOnly: c.HTTPOnly,
			}
			if c.Expires != nil {
				t := time.Unix(int64(*c.Expires), 0)
				bc.Expires = &t
			}
			browserCookies = append(browserCookies, bc)
			httpCookies = append(httpCookies, &http.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Secure:   c.Secure,
				HttpOnly: c.HTTPOnly,
			})
		}
		if domain == defaultDomain {
			domain = detectDomain(parsed)
		}
	}

	if email == "" && len(browserCookies) == 0 {
		return fmt.Errorf("amazon: either email+password or cookies is required")
	}

	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	a.email = email
	a.password = password
	a.otpSecret = otpSecret
	a.domain = domain
	a.browserCookies = browserCookies
	a.httpCookies = httpCookies
	if a.session != nil {
		_ = a.session.Close()
		a.session = nil
	}
	return nil
}

func (a *amazon) Healthy(_ context.Context) bool {
	a.sessionMu.Lock()
	hasEmail := a.email != ""
	hasCookies := len(a.browserCookies) > 0
	a.sessionMu.Unlock()
	return hasEmail || hasCookies
}

func (a *amazon) Tools() []mcp.ToolDefinition {
	return tools
}

func (a *amazon) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, a, args)
}

func (a *amazon) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- Page fetching ---

func (a *amazon) fetch(ctx context.Context, rawURL string) (*goquery.Document, error) {
	if a.browserSvc != nil {
		return a.fetchBrowser(ctx, rawURL)
	}
	return a.fetchHTTP(ctx, rawURL)
}

// openPage navigates a browser page and returns the parsed HTML document.
func (a *amazon) openPage(ctx context.Context, rawURL string) (*goquery.Document, error) {
	sess, err := a.ensureSession(ctx)
	if err != nil {
		return nil, err
	}
	pg, err := sess.NewPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("amazon: new page: %w", err)
	}
	defer pg.Close() //nolint:errcheck

	if err := pg.Navigate(ctx, rawURL); err != nil {
		return nil, fmt.Errorf("amazon: navigate %s: %w", rawURL, err)
	}

	html, err := pg.Content(ctx)
	if err != nil {
		return nil, fmt.Errorf("amazon: get page content: %w", err)
	}

	return goquery.NewDocumentFromReader(strings.NewReader(html))
}

func (a *amazon) fetchBrowser(ctx context.Context, rawURL string) (*goquery.Document, error) {
	doc, err := a.openPage(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	if isLoginDoc(doc) && a.email != "" {
		versionBefore := a.loginVersion.Load()
		a.loginMu.Lock()
		// If another goroutine logged in while we waited, skip re-login.
		if a.loginVersion.Load() != versionBefore {
			a.loginMu.Unlock()
			return a.fetchBrowserPage(ctx, rawURL)
		}
		doLogin := a.login
		if a.loginFunc != nil {
			doLogin = a.loginFunc
		}
		loginErr := doLogin(ctx)
		if loginErr == nil {
			a.loginVersion.Add(1)
		}
		a.loginMu.Unlock()
		if loginErr != nil {
			return nil, loginErr
		}
		return a.fetchBrowserPage(ctx, rawURL)
	}

	if isLoginDoc(doc) {
		return nil, fmt.Errorf("amazon: not logged in — please configure email+password or update cookies")
	}

	return doc, nil
}

func (a *amazon) fetchBrowserPage(ctx context.Context, rawURL string) (*goquery.Document, error) {
	doc, err := a.openPage(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if isLoginDoc(doc) {
		return nil, fmt.Errorf("amazon: login failed — still redirected to sign-in page")
	}
	return doc, nil
}

func (a *amazon) fetchHTTP(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	a.setHTTPCookies(req)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("amazon HTTP error (%d): %s", resp.StatusCode, string(body))
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("amazon: failed to parse HTML: %w", err)
	}

	if isLoginDoc(doc) {
		return nil, fmt.Errorf("amazon: not logged in — please update your cookies")
	}

	return doc, nil
}

// --- Browser page helpers (for mutations that need click/fill) ---

func (a *amazon) withPage(ctx context.Context, rawURL string, fn func(ctx context.Context, pg mcp.BrowserPage) error) error {
	sess, err := a.ensureSession(ctx)
	if err != nil {
		return err
	}
	pg, err := sess.NewPage(ctx)
	if err != nil {
		return fmt.Errorf("amazon: new page: %w", err)
	}
	defer pg.Close() //nolint:errcheck

	if err := pg.Navigate(ctx, rawURL); err != nil {
		return fmt.Errorf("amazon: navigate %s: %w", rawURL, err)
	}

	html, err := pg.Content(ctx)
	if err != nil {
		return fmt.Errorf("amazon: read page: %w", err)
	}
	if isLoginPage(html) {
		return fmt.Errorf("amazon: session expired — not logged in")
	}

	return fn(ctx, pg)
}

// --- HTTP helpers (fallback) ---

func (a *amazon) setHTTPCookies(req *http.Request) {
	for _, c := range a.httpCookies {
		req.AddCookie(c)
	}
}

func isLoginPage(html string) bool {
	return strings.Contains(html, `id="ap_email"`) || strings.Contains(html, `id="signInSubmit"`)
}

func isLoginDoc(doc *goquery.Document) bool {
	return doc.Find("#ap_email").Length() > 0 || doc.Find("#signInSubmit").Length() > 0
}

// --- Cookie parsing ---

type cookieJSON struct {
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	Domain   string   `json:"domain"`
	Path     string   `json:"path"`
	Secure   bool     `json:"secure"`
	HTTPOnly bool     `json:"httpOnly"`
	Expires  *float64 `json:"expirationDate"`
}

func detectDomain(cookies []cookieJSON) string {
	for _, c := range cookies {
		d := strings.TrimPrefix(c.Domain, ".")
		if strings.HasPrefix(d, "amazon.") {
			return d
		}
	}
	for _, c := range cookies {
		d := strings.TrimPrefix(c.Domain, ".")
		if strings.Contains(d, ".amazon.") || strings.HasSuffix(d, ".amazon") {
			return d
		}
	}
	return defaultDomain
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, a *amazon, args map[string]any) (*mcp.ToolResult, error)

// --- URL helpers ---

func (a *amazon) prefix() string {
	if a.baseURL != "" {
		return a.baseURL
	}
	return fmt.Sprintf("https://www.%s", a.domain)
}

func (a *amazon) productURL(asin string) string {
	return fmt.Sprintf("%s/-/en/gp/product/%s", a.prefix(), url.PathEscape(asin))
}

func (a *amazon) searchURL(term string) string {
	return fmt.Sprintf("%s/s?k=%s", a.prefix(), url.QueryEscape(term))
}

func (a *amazon) cartURL() string {
	return fmt.Sprintf("%s/-/en/gp/cart/view.html?ref_=nav_cart", a.prefix())
}

func (a *amazon) ordersURL() string {
	return fmt.Sprintf("%s/-/en/gp/css/order-history", a.prefix())
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("amazon_search_products"): searchProducts,
	mcp.ToolName("amazon_get_product"):     getProduct,
	mcp.ToolName("amazon_get_orders"):      getOrders,
	mcp.ToolName("amazon_get_cart"):        getCart,
	mcp.ToolName("amazon_add_to_cart"):     addToCart,
	mcp.ToolName("amazon_clear_cart"):      clearCart,
}
