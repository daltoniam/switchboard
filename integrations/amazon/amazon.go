package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	mcp "github.com/daltoniam/switchboard"
)

type amazon struct {
	cookies []*http.Cookie
	client  *http.Client
	domain  string
	baseURL string // test-only override; when set, URL helpers use this prefix instead of domain
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

func (a *amazon) Name() string { return "amazon" }

func (a *amazon) Configure(_ context.Context, creds mcp.Credentials) error {
	raw := creds["cookies"]
	if raw == "" {
		return fmt.Errorf("amazon: cookies is required (JSON array of cookie objects exported from browser)")
	}

	var parsed []cookieJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("amazon: invalid cookies JSON: %w", err)
	}
	if len(parsed) == 0 {
		return fmt.Errorf("amazon: cookies array is empty")
	}

	a.cookies = make([]*http.Cookie, 0, len(parsed))
	for _, c := range parsed {
		a.cookies = append(a.cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	a.domain = detectDomain(parsed)

	if v := creds["domain"]; v != "" {
		a.domain = strings.TrimPrefix(v, "www.")
	}

	return nil
}

func (a *amazon) Healthy(ctx context.Context) bool {
	if len(a.cookies) == 0 {
		return false
	}
	u := fmt.Sprintf("https://www.%s/-/en/gp/css/order-history", a.domain)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return false
	}
	a.setCookies(req)
	req.Header.Set("User-Agent", userAgent)

	resp, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	return !isLoginPage(string(data))
}

func (a *amazon) Tools() []mcp.ToolDefinition {
	return tools
}

func (a *amazon) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, a, args)
}

func (a *amazon) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- HTTP helpers ---

func (a *amazon) setCookies(req *http.Request) {
	for _, c := range a.cookies {
		req.AddCookie(c)
	}
}

func (a *amazon) fetch(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	a.setCookies(req)
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

func isLoginPage(html string) bool {
	return strings.Contains(html, `id="ap_email"`) || strings.Contains(html, `id="signInSubmit"`)
}

func isLoginDoc(doc *goquery.Document) bool {
	return doc.Find("#ap_email").Length() > 0 || doc.Find("#signInSubmit").Length() > 0
}

// --- Cookie parsing ---

type cookieJSON struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
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

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

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

var dispatch = map[string]handlerFunc{
	"amazon_search_products": searchProducts,
	"amazon_get_product":     getProduct,
	"amazon_get_orders":      getOrders,
	"amazon_get_cart":        getCart,
	"amazon_add_to_cart":     addToCart,
	"amazon_clear_cart":      clearCart,
}
