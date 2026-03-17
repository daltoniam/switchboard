package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "amazon", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	cookies := `[{"name":"session-id","value":"123","domain":".amazon.com","path":"/","secure":true,"httpOnly":false}]`
	err := i.Configure(context.Background(), mcp.Credentials{"cookies": cookies})
	assert.NoError(t, err)
}

func TestConfigure_EmailPassword(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"email": "user@example.com", "password": "secret"})
	assert.NoError(t, err)
	a := i.(*amazon)
	assert.Equal(t, "user@example.com", a.email)
	assert.Equal(t, "secret", a.password)
}

func TestConfigure_NeitherEmailNorCookies(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either email+password or cookies is required")
}

func TestConfigure_InvalidJSON(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"cookies": "not json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cookies JSON")
}

func TestConfigure_DomainDetection(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: defaultDomain}
	cookies := `[{"name":"ubid-acbes","value":"x","domain":".amazon.co.uk","path":"/"}]`
	err := a.Configure(context.Background(), mcp.Credentials{"cookies": cookies})
	assert.NoError(t, err)
	assert.Equal(t, "amazon.co.uk", a.domain)
}

func TestConfigure_DomainOverride(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: defaultDomain}
	cookies := `[{"name":"ubid-acbes","value":"x","domain":".amazon.com","path":"/"}]`
	err := a.Configure(context.Background(), mcp.Credentials{
		"cookies": cookies,
		"domain":  "www.amazon.de",
	})
	assert.NoError(t, err)
	assert.Equal(t, "amazon.de", a.domain)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveAmazonPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "amazon_", "tool %s missing amazon_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestSetBrowserService(t *testing.T) {
	i := New()
	a := i.(*amazon)
	assert.Nil(t, a.browserSvc)

	mock := &mockBrowserSvc{}
	SetBrowserService(i, mock)
	assert.Equal(t, mock, a.browserSvc)
}

func TestSetBrowserService_WrongType(t *testing.T) {
	type other struct{ mcp.Integration }
	SetBrowserService(&other{}, &mockBrowserSvc{})
}

func TestConfigure_ResetsBrowserSession(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: defaultDomain}
	mockSess := &mockBrowserSession{}
	a.session = mockSess

	cookies := `[{"name":"session-id","value":"123","domain":".amazon.com","path":"/"}]`
	err := a.Configure(context.Background(), mcp.Credentials{"cookies": cookies})
	require.NoError(t, err)
	assert.Nil(t, a.session)
	assert.True(t, mockSess.closed)
}

func TestConfigure_PopulatesBrowserCookies(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: defaultDomain}
	cookies := `[{"name":"session-id","value":"abc","domain":".amazon.com","path":"/","secure":true,"httpOnly":true}]`
	err := a.Configure(context.Background(), mcp.Credentials{"cookies": cookies})
	require.NoError(t, err)
	require.Len(t, a.browserCookies, 1)
	assert.Equal(t, "session-id", a.browserCookies[0].Name)
	assert.Equal(t, "abc", a.browserCookies[0].Value)
	assert.True(t, a.browserCookies[0].Secure)
	assert.True(t, a.browserCookies[0].HTTPOnly)
	require.Len(t, a.httpCookies, 1)
}

func TestFetchHTTPFallback_NoBrowser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>OK</h1></body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	doc, err := a.fetch(context.Background(), ts.URL+"/test")
	require.NoError(t, err)
	assert.Equal(t, "OK", doc.Find("h1").Text())
}

func TestFetchBrowser_UsesSessionAndPage(t *testing.T) {
	mockPage := &mockBrowserPage{
		html: `<html><body><h1>Browser Content</h1></body></html>`,
	}
	mockSess := &mockBrowserSession{}
	mockSess.pages = append(mockSess.pages, mockPage)

	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: &mockBrowserSvc{session: mockSess},
		session:    mockSess,
	}

	doc, err := a.fetchBrowser(context.Background(), "https://www.amazon.com/s?k=test")
	require.NoError(t, err)
	assert.Equal(t, "Browser Content", doc.Find("h1").Text())
	assert.Equal(t, "https://www.amazon.com/s?k=test", mockSess.pages[len(mockSess.pages)-1].navigatedURL)
}

func TestFetchBrowser_LoginPageDetected(t *testing.T) {
	mockPage := &mockBrowserPage{
		html: `<html><body><input id="ap_email"><input id="signInSubmit"></body></html>`,
	}
	mockSess := &mockBrowserSession{}
	mockSess.pages = append(mockSess.pages, mockPage)

	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: &mockBrowserSvc{session: mockSess},
		session:    mockSess,
	}

	_, err := a.fetchBrowser(context.Background(), "https://www.amazon.com/orders")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestGetSession_InjectsCookies(t *testing.T) {
	mockSess := &mockBrowserSession{}
	mockSvc := &mockBrowserSvc{session: mockSess}
	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: mockSvc,
		browserCookies: []mcp.BrowserCookie{
			{Name: "session-id", Value: "abc", Domain: ".amazon.com"},
		},
	}

	sess, err := a.ensureSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, mockSess, sess)
	require.Len(t, mockSess.cookies, 1)
	assert.Equal(t, "session-id", mockSess.cookies[0].Name)
}

func TestGetSession_ReusesExisting(t *testing.T) {
	mockSess := &mockBrowserSession{}
	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: &mockBrowserSvc{},
		session:    mockSess,
	}

	sess, err := a.ensureSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, mockSess, sess)
}

// --- TOTP tests ---

func TestGenerateTOTP(t *testing.T) {
	code, err := generateTOTP("JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	assert.Len(t, code, 6)
	assert.Regexp(t, `^\d{6}$`, code)
}

func TestGenerateTOTP_InvalidSecret(t *testing.T) {
	_, err := generateTOTP("!!!invalid!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base32")
}

func TestGenerateTOTP_PaddedSecret(t *testing.T) {
	code, err := generateTOTP("JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	codeSpaced, err := generateTOTP("JBSW Y3DP EHPK 3PXP")
	require.NoError(t, err)
	assert.Equal(t, code, codeSpaced)
}

// --- Login tests ---

func TestLogin_MissingCredentials(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: "amazon.com", browserSvc: &mockBrowserSvc{}}
	err := a.login(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email and password are required")
}

func TestFetchBrowser_AutoLoginOnExpiredSession(t *testing.T) {
	loginPage := `<html><body><input id="ap_email"><input id="signInSubmit"></body></html>`
	loggedInPage := `<html><body><h1>Products</h1></body></html>`
	callCount := 0

	mockSess := &mockBrowserSession{}
	mockSess.pages = []*mockBrowserPage{
		{html: loginPage},
		{html: loggedInPage},
	}

	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		email:      "user@example.com",
		password:   "secret",
		browserSvc: &mockBrowserSvc{session: mockSess},
		session:    mockSess,
		loginFunc: func(ctx context.Context) error {
			callCount++
			return nil
		},
	}

	doc, err := a.fetchBrowser(context.Background(), "https://www.amazon.com/s?k=test")
	require.NoError(t, err)
	assert.Equal(t, "Products", doc.Find("h1").Text())
	assert.Equal(t, 1, callCount)
}

// --- Mock browser types ---

type mockBrowserSvc struct {
	sessionErr error
	session    *mockBrowserSession
}

func (m *mockBrowserSvc) NewSession(_ context.Context) (mcp.BrowserSession, error) {
	if m.sessionErr != nil {
		return nil, m.sessionErr
	}
	if m.session == nil {
		m.session = &mockBrowserSession{}
	}
	return m.session, nil
}

func (m *mockBrowserSvc) Close() error { return nil }

type mockBrowserSession struct {
	closed     bool
	cookies    []mcp.BrowserCookie
	cookiesErr error
	pages      []*mockBrowserPage
	pageIdx    int
}

func (m *mockBrowserSession) AddCookies(_ context.Context, cookies []mcp.BrowserCookie) error {
	if m.cookiesErr != nil {
		return m.cookiesErr
	}
	m.cookies = cookies
	return nil
}

func (m *mockBrowserSession) NewPage(_ context.Context) (mcp.BrowserPage, error) {
	if m.pageIdx < len(m.pages) {
		pg := m.pages[m.pageIdx]
		m.pageIdx++
		return pg, nil
	}
	pg := &mockBrowserPage{}
	m.pages = append(m.pages, pg)
	m.pageIdx++
	return pg, nil
}

func (m *mockBrowserSession) Close() error {
	m.closed = true
	return nil
}

type mockBrowserPage struct {
	navigatedURL  string
	html          string
	postClickHTML string
	clicked       bool
	closed        bool
	clickErr      error
}

func (m *mockBrowserPage) Navigate(_ context.Context, url string) error {
	m.navigatedURL = url
	return nil
}

func (m *mockBrowserPage) Content(_ context.Context) (string, error) {
	html := m.html
	if m.clicked && m.postClickHTML != "" {
		html = m.postClickHTML
	}
	if html == "" {
		return "<html><body></body></html>", nil
	}
	return html, nil
}

func (m *mockBrowserPage) Click(_ context.Context, _ string) error {
	m.clicked = true
	return m.clickErr
}
func (m *mockBrowserPage) Fill(_ context.Context, _, _ string) error { return nil }
func (m *mockBrowserPage) SelectOption(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockBrowserPage) InnerText(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockBrowserPage) InnerHTML(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockBrowserPage) WaitForSelector(_ context.Context, _ string) error { return nil }
func (m *mockBrowserPage) Screenshot(_ context.Context) ([]byte, error)      { return nil, nil }
func (m *mockBrowserPage) Evaluate(_ context.Context, _ string, _ ...any) (any, error) {
	return nil, nil
}

func (m *mockBrowserPage) Close() error {
	m.closed = true
	return nil
}

func TestExecute_UnknownTool(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: "amazon.com"}
	result, err := a.Execute(context.Background(), "amazon_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestFetch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("User-Agent"), "Mozilla")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Test</h1></body></html>`))
	}))
	defer ts.Close()

	a := &amazon{
		client:      ts.Client(),
		domain:      "amazon.com",
		baseURL:     ts.URL,
		httpCookies: []*http.Cookie{{Name: "test", Value: "val"}},
	}

	doc, err := a.fetch(context.Background(), ts.URL+"/test")
	require.NoError(t, err)
	assert.Equal(t, "Test", doc.Find("h1").Text())
}

func TestFetch_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer ts.Close()

	a := &amazon{client: ts.Client(), domain: "amazon.com", baseURL: ts.URL}
	_, err := a.fetch(context.Background(), ts.URL+"/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amazon HTTP error (500)")
}

func TestFetch_LoginPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><input id="ap_email"><input id="signInSubmit"></body></html>`))
	}))
	defer ts.Close()

	a := &amazon{client: ts.Client(), domain: "amazon.com", baseURL: ts.URL}
	_, err := a.fetch(context.Background(), ts.URL+"/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

// --- Result helper tests ---

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key":"value"`)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

// --- Domain detection tests ---

func TestDetectDomain(t *testing.T) {
	tests := []struct {
		name     string
		cookies  []cookieJSON
		expected string
	}{
		{"amazon.com", []cookieJSON{{Domain: ".amazon.com"}}, "amazon.com"},
		{"amazon.co.uk", []cookieJSON{{Domain: ".amazon.co.uk"}}, "amazon.co.uk"},
		{"amazon.de", []cookieJSON{{Domain: ".amazon.de"}}, "amazon.de"},
		{"fallback with amazon in domain", []cookieJSON{{Domain: "www.amazon.es"}}, "www.amazon.es"},
		{"rejects overly broad match", []cookieJSON{{Domain: ".myamazondomain.com"}}, defaultDomain},
		{"no match", []cookieJSON{{Domain: ".example.com"}}, defaultDomain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, detectDomain(tt.cookies))
		})
	}
}

func TestIsLoginPage(t *testing.T) {
	assert.True(t, isLoginPage(`<input id="ap_email">`))
	assert.True(t, isLoginPage(`<input id="signInSubmit">`))
	assert.False(t, isLoginPage(`<div>Hello</div>`))
}

// --- URL helper tests ---

func TestPrefix(t *testing.T) {
	a := &amazon{domain: "amazon.com"}
	assert.Equal(t, "https://www.amazon.com", a.prefix())

	a.baseURL = "http://localhost:8080"
	assert.Equal(t, "http://localhost:8080", a.prefix())
}

func TestProductURL(t *testing.T) {
	a := &amazon{domain: "amazon.com"}
	assert.Equal(t, "https://www.amazon.com/-/en/gp/product/B0CHXKM5GK", a.productURL("B0CHXKM5GK"))
}

func TestSearchURL(t *testing.T) {
	a := &amazon{domain: "amazon.com"}
	u := a.searchURL("wireless headphones")
	assert.Contains(t, u, "amazon.com/s?k=wireless")
	assert.Contains(t, u, "headphones")
}

func TestCartURL(t *testing.T) {
	a := &amazon{domain: "amazon.co.uk"}
	assert.Contains(t, a.cartURL(), "amazon.co.uk")
	assert.Contains(t, a.cartURL(), "cart/view.html")
}

func TestOrdersURL(t *testing.T) {
	a := &amazon{domain: "amazon.de"}
	assert.Contains(t, a.ordersURL(), "amazon.de")
	assert.Contains(t, a.ordersURL(), "order-history")
}

// --- newTestAmazon helper ---

func newTestAmazon(ts *httptest.Server) *amazon {
	return &amazon{
		client:  ts.Client(),
		domain:  "amazon.com",
		baseURL: ts.URL,
	}
}

// --- Handler integration tests ---

func TestSearchProducts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "k=laptop")
		_, _ = w.Write([]byte(`<html><body>
			<div role="listitem" data-asin="B0CHXKM5GK">
				<h2 aria-label="Test Laptop"><span>Test Laptop</span></h2>
				<h2 class="a-size-mini"><span class="a-size-base-plus a-color-base">BrandX</span></h2>
				<span class="a-price" data-a-size="xl"><span class="a-offscreen">$999.99</span></span>
				<i class="a-icon-star-mini"><span class="a-icon-alt">4.5 out of 5 stars</span></i>
				<img class="s-image" src="https://img.example.com/laptop.jpg">
				<i class="a-icon-prime"></i>
			</div>
		</body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_search_products", map[string]any{
		"search_term": "laptop",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "B0CHXKM5GK")
	assert.Contains(t, result.Data, "Test Laptop")
	assert.Contains(t, result.Data, "999.99")

	var products []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &products))
	assert.Len(t, products, 1)
	assert.Equal(t, true, products[0]["is_prime_eligible"])
}

func TestSearchProducts_MissingTerm(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: "amazon.com"}
	result, err := a.Execute(context.Background(), "amazon_search_products", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "search_term is required")
}

func TestGetProduct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<span id="productTitle">Amazing Widget</span>
			<div class="priceToPay"><span class="a-offscreen">$29.99</span></div>
			<div id="productOverview_feature_div">Great product overview</div>
			<div id="featurebullets_feature_div">Feature 1 Feature 2</div>
			<div id="averageCustomerReviews"><span class="a-size-small a-color-base">4.7</span></div>
			<div id="acrCustomerReviewLink"><span>1,234 ratings</span></div>
			<div id="main-image-container"><img class="a-dynamic-image" src="https://img.example.com/widget.jpg"></div>
		</body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_get_product", map[string]any{
		"asin": "B0CHXKM5GK",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Amazing Widget")
	assert.Contains(t, result.Data, "29.99")
	assert.Contains(t, result.Data, "4.7")
}

func TestGetProduct_InvalidASIN(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: "amazon.com"}
	result, err := a.Execute(context.Background(), "amazon_get_product", map[string]any{
		"asin": "short",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "asin must be exactly 10 uppercase alphanumeric characters")
}

func TestGetOrders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<div class="order-card">
				<span class="yohtmlc-order-id"><span>Order #</span><span>123-456-789</span></span>
				<div class="order-header__header-list-item"><span class="a-size-base">March 1, 2025</span></div>
				<div class="delivery-box__primary-text">Delivered</div>
				<div class="item-box">
					<a class="yohtmlc-product-title" href="/dp/B0CHXKM5GK/ref=123">Test Product</a>
					<div class="product-image"><img src="https://img.example.com/prod.jpg"></div>
				</div>
			</div>
		</body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_get_orders", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "123-456-789")
	assert.Contains(t, result.Data, "Test Product")
	assert.Contains(t, result.Data, "B0CHXKM5GK")
}

func TestGetCart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<div id="sc-active-cart">
				<div data-asin="B0CHXKM5GK">
					<a class="sc-product-title"><span class="a-truncate-full">Wireless Mouse</span></a>
					<div class="apex-price-to-pay-value"><span class="a-offscreen">$24.99</span></div>
					<span data-a-selector="value">2</span>
					<img class="sc-product-image" src="https://img.example.com/mouse.jpg">
					<span class="sc-product-availability">In Stock</span>
				</div>
			</div>
			<span id="sc-subtotal-amount-activecart"><span class="sc-price">$49.98</span></span>
			<span id="sc-subtotal-label-activecart">Subtotal (2 items)</span>
		</body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_get_cart", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Wireless Mouse")
	assert.Contains(t, result.Data, "24.99")
	assert.Contains(t, result.Data, "B0CHXKM5GK")

	var cart map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &cart))
	assert.Equal(t, false, cart["is_empty"])
	assert.Equal(t, float64(2), cart["total_items"])
}

func TestGetCart_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>Your Amazon Cart is empty</body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_get_cart", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var cart map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &cart))
	assert.Equal(t, true, cart["is_empty"])
}

func TestAddToCart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`<html><body>
				<form id="addToCart" action="/gp/product/handle-buy-box">
					<input type="hidden" name="ASIN" value="B0CHXKM5GK">
					<input type="hidden" name="offerListingID" value="abc123">
				</form>
			</body></html>`))
		} else {
			_, _ = w.Write([]byte(`<html><body><div id="sw-atc-confirmation">Added to cart</div></body></html>`))
		}
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_add_to_cart", map[string]any{
		"asin": "B0CHXKM5GK",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestAddToCart_InvalidASIN(t *testing.T) {
	a := &amazon{client: &http.Client{}, domain: "amazon.com"}
	result, err := a.Execute(context.Background(), "amazon_add_to_cart", map[string]any{
		"asin": "short",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "asin must be exactly 10 uppercase alphanumeric characters")
}

func TestAddToCartBrowser(t *testing.T) {
	productPage := `<html><body><button id="add-to-cart-button">Add to Cart</button></body></html>`
	confirmPage := `<html><body><div id="sw-atc-confirmation">Added to cart</div></body></html>`

	mockSess := &mockBrowserSession{}
	mockSess.pages = []*mockBrowserPage{
		{html: productPage, postClickHTML: confirmPage},
	}

	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: &mockBrowserSvc{session: mockSess},
		session:    mockSess,
	}

	result, err := addToCartBrowser(context.Background(), a, "B0CHXKM5GK")
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "added to cart")
}

func TestClearCartBrowser(t *testing.T) {
	cartWithItem := `<html><body><div id="sc-active-cart"><div data-asin="B0CHXKM5GK"></div></div></body></html>`
	emptyCart := `<html><body><div id="sc-active-cart"></div></body></html>`

	mockPg := &mockBrowserPage{html: cartWithItem, postClickHTML: emptyCart}
	mockSess := &mockBrowserSession{}
	mockSess.pages = []*mockBrowserPage{mockPg}

	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: &mockBrowserSvc{session: mockSess},
		session:    mockSess,
	}

	result, err := clearCartBrowser(context.Background(), a)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Removed 1 item")
}

func TestClearCartBrowser_AlreadyEmpty(t *testing.T) {
	emptyCart := `<html><body><div id="sc-active-cart"></div></body></html>`

	mockSess := &mockBrowserSession{}
	mockSess.pages = []*mockBrowserPage{{html: emptyCart}}

	a := &amazon{
		client:     &http.Client{},
		domain:     "amazon.com",
		browserSvc: &mockBrowserSvc{session: mockSess},
		session:    mockSess,
	}

	result, err := clearCartBrowser(context.Background(), a)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "already empty")
}

func TestClearCart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`<html><body>
				<div id="sc-active-cart">
					<div data-asin="B0CHXKM5GK">
						<form action="/cart/delete">
							<input type="hidden" name="item" value="abc">
						</form>
					</div>
				</div>
			</body></html>`))
		} else {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`<html><body>deleted</body></html>`))
		}
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_clear_cart", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Removed")
}

func TestClearCart_AlreadyEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div id="sc-active-cart"></div></body></html>`))
	}))
	defer ts.Close()

	a := newTestAmazon(ts)
	result, err := a.Execute(context.Background(), "amazon_clear_cart", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "already empty")
}
