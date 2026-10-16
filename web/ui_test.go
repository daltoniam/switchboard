package web

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/notionmcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

type uiApp struct {
	testing *testing.T
	binary  string
	home    string
	URL     string
	stop    func()
}

func newUIApp(t *testing.T) *uiApp {
	t.Helper()
	binary := os.Getenv("SWITCHBOARD_UI_BINARY")
	if binary == "" {
		t.Skip("run make test-ui to test the development binary in Chromium")
	}
	absolute, err := filepath.Abs(binary)
	require.NoError(t, err)
	info, err := os.Stat(absolute)
	require.NoError(t, err, "build the development binary before running UI tests")
	require.False(t, info.IsDir())
	app := &uiApp{testing: t, binary: absolute, home: t.TempDir()}
	app.start()
	t.Cleanup(func() { app.stop() })
	return app
}

func (app *uiApp) start() {
	app.testing.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(app.testing, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(app.testing, listener.Close())
	app.URL = fmt.Sprintf("http://127.0.0.1:%d", port)
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, app.binary, "--listen-host", "127.0.0.1", "--port", fmt.Sprint(port))
	command.Env = []string{"HOME=" + app.home, "XDG_CONFIG_HOME=" + filepath.Join(app.home, ".config"), "PATH=" + os.Getenv("PATH")}
	logFile, err := os.CreateTemp(app.home, "server-*.log")
	require.NoError(app.testing, err)
	command.Stdout, command.Stderr = logFile, logFile
	require.NoError(app.testing, command.Start())
	var once sync.Once
	app.stop = func() {
		once.Do(func() {
			cancel()
			_ = command.Wait()
			require.NoError(app.testing, logFile.Close())
		})
	}
	app.testing.Cleanup(app.stop)
	client := &http.Client{Timeout: time.Second}
	require.Eventually(app.testing, func() bool {
		response, requestErr := client.Get(app.URL + "/api/health")
		if requestErr != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	}, 30*time.Second, 100*time.Millisecond, "development server must become ready")
}

func (app *uiApp) restart() {
	app.stop()
	app.start()
}

func uiPage(t *testing.T) playwright.Page {
	t.Helper()
	scripts := uiScriptFixtures(t)
	logos := uiLogoFixtures(t)
	driver, err := playwright.Run()
	require.NoError(t, err, "install the Playwright driver with make ui-install")
	t.Cleanup(func() { require.NoError(t, driver.Stop()) })
	browser, err := driver.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	require.NoError(t, err, "Chromium is mandatory for UI verification; run make ui-install")
	t.Cleanup(func() { require.NoError(t, browser.Close()) })
	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &playwright.Size{Width: 1280, Height: 900}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, browserContext.Close()) })
	page, err := browserContext.NewPage()
	require.NoError(t, err)
	page.SetDefaultTimeout(10000)
	require.NoError(t, browserContext.Route("**/*", func(route playwright.Route) {
		if body, ok := scripts[route.Request().URL()]; ok {
			uiFulfillFixture(t, route, body, "text/javascript")
			return
		}
		if logos[route.Request().URL()] {
			body := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`
			uiFulfillFixture(t, route, body, "image/svg+xml")
			return
		}
		target, parseErr := url.Parse(route.Request().URL())
		if parseErr != nil || target.Hostname() != "127.0.0.1" {
			t.Errorf("browser attempted non-fixture request")
			require.NoError(t, route.Abort())
			return
		}
		require.NoError(t, route.Continue())
	}))
	page.OnPageError(func(err error) { t.Errorf("browser JavaScript error: %v", err) })
	page.OnConsole(func(message playwright.ConsoleMessage) {
		if message.Type() == "error" {
			t.Errorf("browser console error: %s", message.Text())
		}
	})
	page.OnResponse(func(response playwright.Response) {
		if response.Status() >= http.StatusBadRequest {
			t.Errorf("browser HTTP error: %d %s", response.Status(), response.URL())
		}
	})
	t.Cleanup(func() {
		if t.Failed() {
			uiCapture(t, page, strings.ReplaceAll(t.Name(), "/", "-")+"-failure")
		}
	})
	return page
}

func uiFulfillFixture(t *testing.T, route playwright.Route, body, contentType string) {
	t.Helper()
	headers := map[string]string{"Access-Control-Allow-Origin": "*"}
	options := playwright.RouteFulfillOptions{
		Body: body, ContentType: playwright.String(contentType), Headers: headers,
	}
	require.NoError(t, route.Fulfill(options))
}

func uiLogoFixtures(t *testing.T) map[string]bool {
	t.Helper()
	css, err := uiAssets.ReadFile("static/style.css")
	require.NoError(t, err)
	pattern := regexp.MustCompile(`https://cdn\.jsdelivr\.net/gh/[\w-]+/[\w-]+@[a-f0-9]{40}/[\w/.-]+\.svg`)
	sources := pattern.FindAllString(string(css), -1)
	require.NotEmpty(t, sources)
	fixtures := make(map[string]bool, len(sources))
	for _, source := range sources {
		fixtures[source] = true
	}
	return fixtures
}

func uiScriptFixtures(t *testing.T) map[string]string {
	t.Helper()
	integrities := map[string]string{
		"https://cdn.jsdelivr.net/npm/htmx.org@2.0.11/dist/htmx.min.js":                              "2OatzQy1H+Zd/IIrjr1TcuDGqLXeHhbooAyJY1KdQMKnr4LZ22k31GBLdYKHmVjg",
		"https://cdn.jsdelivr.net/npm/@tarekraafat/autocomplete.js@10.2.10/dist/autoComplete.min.js": "cLawOb2F3v0Zfax5NIlJRiJLXwaGoV0vezUUxIAvxIZDfx/jXGa8xOYNTXS8/cu4",
	}
	fixtures := make(map[string]string, len(integrities))
	for source, integrity := range integrities {
		fixtures[source] = uiFetchScript(t, source, integrity)
	}
	return fixtures
}

func uiFetchScript(t *testing.T, source, integrity string) string {
	t.Helper()
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(source)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	digest := sha512.Sum384(body)
	require.Equal(t, integrity, base64.StdEncoding.EncodeToString(digest[:]), source)
	return string(body)
}

func uiNavigate(t *testing.T, page playwright.Page, target string) {
	t.Helper()
	response, err := page.Goto(target)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusOK, response.Status())
}

func uiText(t *testing.T, locator playwright.Locator, expected string) {
	t.Helper()
	require.Eventually(t, func() bool {
		text, err := locator.TextContent()
		return err == nil && strings.Contains(text, expected)
	}, 10*time.Second, 100*time.Millisecond, "expected UI text %q", expected)
}

func uiSave(t *testing.T, page playwright.Page) {
	t.Helper()
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save Configuration", Exact: playwright.Bool(true)}).Click())
	uiText(t, page.Locator("body"), "Configuration saved")
}

func uiSearch(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "metabase_list_databases", "integration": "metabase", "limit": 1}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	var response struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &response))
	var names []string
	for _, tool := range response.Tools {
		names = append(names, tool.Name)
	}
	if len(names) > 0 {
		require.Contains(t, names, "metabase_list_databases")
		result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "metabase_list_databases", "arguments": map[string]any{}}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		text, ok = result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.Contains(t, text.Text, "UI fixture database")
	}
	return names
}

func uiCapture(t *testing.T, page playwright.Page, name string) {
	t.Helper()
	root := os.Getenv("SWITCHBOARD_UI_ARTIFACTS")
	if root == "" {
		root = t.TempDir()
	}
	require.NoError(t, os.MkdirAll(root, 0700))
	_, err := page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(filepath.Join(root, name+".png")), FullPage: playwright.Bool(true), Mask: []playwright.Locator{page.Locator("input")}})
	require.NoError(t, err)
}

func TestUIIntegrationLifecycle(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-api-key") != "ui-fixture-key" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/user/current":
			_, _ = io.WriteString(writer, `{"id":1,"email":"ui@example.test"}`)
		case "/api/database":
			_, _ = io.WriteString(writer, `{"data":[{"id":1,"name":"UI fixture database","engine":"postgres"}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/metabase"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiSearch(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	key := form.Locator("#cred_api_key")
	endpoint := form.Locator("#cred_url")
	inputType, err := key.GetAttribute("type")
	require.NoError(t, err)
	require.Equal(t, "password", inputType)
	inputType, err = endpoint.GetAttribute("type")
	require.NoError(t, err)
	require.Equal(t, "text", inputType)
	value, err := key.InputValue()
	require.NoError(t, err)
	require.Empty(t, value)
	require.NoError(t, endpoint.Fill(provider.URL))
	require.NoError(t, key.Fill("ui-fixture-key"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NotEmpty(t, uiSearch(t, app))
	uiCapture(t, page, "metabase-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiCapture(t, page, "metabase-mobile")
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/metabase")
	uiText(t, page.Locator(".badge").First(), "Connected")
	value, err = key.InputValue()
	require.NoError(t, err)
	require.Equal(t, "ui-fixture-key", value)
	require.NotEmpty(t, uiSearch(t, app))
	require.NoError(t, key.Fill("invalid-fixture-key"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, key.Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, key.Fill("ui-fixture-key"))
	uiSave(t, page)
	uiNavigate(t, page, app.URL+"/integrations")
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all"}).Click())
	uiText(t, card, "Connected")
	require.NoError(t, card.Click())
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiSearch(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	uiCapture(t, page, "metabase-disabled")
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/metabase")
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiSearch(t, app))
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["metabase"].Enabled)
	require.Equal(t, provider.URL, saved.Integrations["metabase"].Credentials["url"])
}

func TestUINamedIdentities(t *testing.T) {
	_ = newUIApp(t)
	ws, reg, store := setupTestWeb()
	reg.integrations["testint"] = &multiIdentityWebMock{name: "testint", healthy: true}
	app := httptest.NewServer(ws.Handler())
	t.Cleanup(app.Close)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations/testint")
	form := page.Locator(".identity-item-new form")
	require.NoError(t, form.Locator(`[name="identity_id"]`).Fill("work"))
	require.NoError(t, form.Locator(`[name="identity_cred_access_token"]`).Fill("identity-fixture-secret"))
	require.NoError(t, form.Locator(`[name="identity_meta_label"]`).Fill("Work fixture"))
	require.NoError(t, form.Locator(`button[type="submit"]`).Click())
	uiText(t, page.Locator(".identity-list"), "Work fixture")
	secret := page.Locator("#identity-work-cred-access_token")
	value, err := secret.InputValue()
	require.NoError(t, err)
	require.Empty(t, value)
	content, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "identity-fixture-secret")
	edit := page.Locator(".identity-list form").Nth(1)
	require.NoError(t, edit.Locator(`[name="identity_meta_label"]`).Fill("Updated fixture"))
	require.NoError(t, edit.Locator(`button[type="submit"]`).Click())
	uiText(t, page.Locator(".identity-list"), "Updated fixture")
	config, ok := store.GetIntegration("testint")
	require.True(t, ok)
	require.Equal(t, "identity-fixture-secret", config.Identities["work"].Credentials["access_token"])
	uiCapture(t, page, "named-identities")
	page.OnDialog(func(dialog playwright.Dialog) { require.NoError(t, dialog.Accept()) })
	require.NoError(t, page.Locator(".identity-delete").Click())
	uiText(t, page.Locator(".identity-empty"), "No identities configured yet")
	config, ok = store.GetIntegration("testint")
	require.True(t, ok)
	require.Empty(t, config.Identities)
}

func TestUIOAuthSetup(t *testing.T) {
	if os.Getenv("SWITCHBOARD_UI_BINARY") == "" {
		t.Skip("run make test-ui")
	}
	cases := []struct {
		name      string
		cancel    bool
		status    int
		connected bool
	}{
		{name: "connect", status: http.StatusOK, connected: true},
		{name: "cancel", status: http.StatusOK, cancel: true},
		{name: "exchange error", status: http.StatusBadRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := newNotionMCPOAuthFixture(t, test.status)
			ws, reg, store := setupTestWeb()
			integration := notionmcp.New()
			t.Cleanup(func() { require.NoError(t, integration.(io.Closer).Close()) })
			require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": fixture.server.URL, "mcp_access_token": "previous-fixture-token"}))
			reg.integrations["notion-mcp"] = integration
			require.NoError(t, store.SetIntegration("notion-mcp", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"base_url": fixture.server.URL}}))
			app := httptest.NewServer(ws.Handler())
			t.Cleanup(app.Close)
			page := uiPage(t)
			require.NoError(t, page.Route(fixture.server.URL+"/authorize*", func(route playwright.Route) {
				authorize, err := url.Parse(route.Request().URL())
				require.NoError(t, err)
				callback, err := url.Parse(app.URL + "/api/remote/notion-mcp/oauth/callback")
				require.NoError(t, err)
				query := callback.Query()
				query.Set("state", authorize.Query().Get("state"))
				if test.cancel {
					query.Set("error", "access_denied")
				} else {
					query.Set("code", "fixture-code")
				}
				callback.RawQuery = query.Encode()
				require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(http.StatusFound), Headers: map[string]string{"Location": callback.String()}}))
			}))
			uiNavigate(t, page, app.URL+"/integrations/notion-mcp")
			uiText(t, page.Locator("h1"), "Notion MCP Setup")
			require.NoError(t, page.Locator("#notion-mcp-oauth-start").Click())
			if test.connected {
				require.NoError(t, page.Locator(".flash-success").WaitFor())
				message, err := page.Locator(".flash-success").TextContent()
				require.NoError(t, err)
				require.Contains(t, message, "Connected via MCP OAuth")
				config, ok := store.GetIntegration("notion-mcp")
				require.True(t, ok)
				require.True(t, config.Enabled)
				require.Equal(t, "notion-oauth-token", config.Credentials["mcp_access_token"])
				content, err := page.Content()
				require.NoError(t, err)
				require.NotContains(t, content, "notion-oauth-token")
				uiCapture(t, page, "oauth-connected")
				require.NoError(t, page.Locator("#notion-mcp-oauth-start").Click())
				uiText(t, page.Locator(".flash-success"), "Connected via MCP OAuth")
				require.Eventually(t, func() bool {
					fixture.mu.Lock()
					defer fixture.mu.Unlock()
					return fixture.tokenCalls == 2
				}, 10*time.Second, 100*time.Millisecond)
			} else {
				expected := "invalid_grant"
				if test.cancel {
					expected = "access_denied"
				}
				uiText(t, page.Locator(".flash-error").First(), expected)
				config, ok := store.GetIntegration("notion-mcp")
				require.True(t, !ok || !config.Enabled)
				uiText(t, page.Locator(".badge").First(), "Not Connected")
			}
		})
	}
}
