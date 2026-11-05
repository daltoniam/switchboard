package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/prometheus"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

func TestUIPrometheus(t *testing.T) {
	binary := os.Getenv("SWITCHBOARD_UI_BINARY")
	if binary == "" {
		t.Skip("set SWITCHBOARD_UI_BINARY to the development binary to run Chromium acceptance")
	}
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	home := t.TempDir()
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer ui-fixture-token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/status/buildinfo":
			_, _ = io.WriteString(writer, `{"status":"success","data":{"version":"3.0"}}`)
		case "/api/v1/query":
			if request.URL.Query().Get("query") != "up" {
				http.Error(writer, "unexpected query", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(writer, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"job":"UI fixture"},"value":[100,"1"]}]}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	var address string
	var stop func()
	start := func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())
		address = fmt.Sprintf("http://127.0.0.1:%d", port)
		ctx, cancel := context.WithCancel(context.Background())
		command := exec.CommandContext(ctx, binary, "--listen-host", "127.0.0.1", "--port", fmt.Sprint(port))
		command.Env = []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "PATH=" + os.Getenv("PATH")}
		require.NoError(t, command.Start())
		var once sync.Once
		stop = func() { once.Do(func() { cancel(); _ = command.Wait() }) }
		t.Cleanup(stop)
		client := &http.Client{Timeout: time.Second}
		require.Eventually(t, func() bool {
			response, err := client.Get(address + "/api/health")
			if err != nil {
				return false
			}
			_ = response.Body.Close()
			return response.StatusCode == http.StatusOK
		}, 30*time.Second, 100*time.Millisecond)
		t.Logf("isolated development URL: %s", address)
	}
	start()
	driver, err := playwright.Run()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, driver.Stop()) })
	browser, err := driver.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, browser.Close()) })
	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &playwright.Size{Width: 1280, Height: 900}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, browserContext.Close()) })
	page, err := browserContext.NewPage()
	require.NoError(t, err)
	page.SetDefaultTimeout(10000)
	scripts := uiScriptFixtures(t)
	logos := uiLogoFixtures(t)
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
		if !strings.HasPrefix(route.Request().URL(), address+"/") {
			t.Error("browser attempted non-fixture request")
			_ = route.Abort()
			return
		}
		_ = route.Continue()
	}))
	page.OnPageError(func(err error) { t.Errorf("browser JavaScript error: %v", err) })
	page.OnConsole(func(message playwright.ConsoleMessage) {
		if message.Type() == "error" {
			t.Errorf("browser console error: %s", message.Text())
		}
	})
	page.OnResponse(func(response playwright.Response) {
		if response.Status() >= 400 {
			t.Errorf("browser HTTP error: %d", response.Status())
		}
	})
	page.OnRequestFailed(func(request playwright.Request) { t.Errorf("browser request failed: %s", request.Failure()) })
	navigate := func(path string) {
		response, err := page.Goto(address + path)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.Status())
	}
	text := func(locator playwright.Locator, expected string) {
		require.Eventually(t, func() bool {
			value, err := locator.TextContent()
			return err == nil && strings.Contains(value, expected)
		}, 10*time.Second, 100*time.Millisecond, "expected %q", expected)
	}
	save := func() {
		require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save Configuration", Exact: playwright.Bool(true)}).Click())
		text(page.Locator("body"), "Configuration saved")
	}
	capture := func(name string) {
		root := os.Getenv("SWITCHBOARD_UI_ARTIFACTS")
		if root == "" {
			root = t.TempDir()
		}
		require.NoError(t, os.MkdirAll(root, 0700))
		_, err := page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(filepath.Join(root, name+".png")), FullPage: playwright.Bool(true), Mask: []playwright.Locator{page.Locator("input")}})
		require.NoError(t, err)
	}
	search := func(enabled bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "prometheus-ui", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: address + "/mcp", DisableStandaloneSSE: true}, nil)
		require.NoError(t, err)
		defer func() { require.NoError(t, session.Close()) }()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "prometheus", "limit": 50}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		var response struct {
			Total int `json:"total"`
		}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].(*mcpsdk.TextContent).Text), &response))
		if !enabled {
			require.Zero(t, response.Total)
			return
		}
		require.Equal(t, 9, response.Total)
		require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "prometheus_query")
		result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "prometheus_query", "arguments": map[string]any{"query": "up"}}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "UI fixture")
	}
	navigate("/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/prometheus"]`)
	text(card, "Disabled")
	search(false)
	require.NoError(t, card.Click())
	text(page.Locator("h1"), "prometheus")
	form := page.Locator("#integration-configuration")
	adapter := prometheus.New()
	for _, key := range []string{"base_url", "username", "password", "bearer_token", "headers", "ca_cert"} {
		t.Run(key, func(t *testing.T) {
			input := form.Locator("#cred_" + key)
			value, err := input.InputValue()
			require.NoError(t, err)
			require.Empty(t, value)
			inputType, err := input.GetAttribute("type")
			require.NoError(t, err)
			expected := "password"
			if key == "base_url" || key == "username" {
				expected = "text"
			}
			require.Equal(t, expected, inputType)
			placeholder, err := input.GetAttribute("placeholder")
			require.NoError(t, err)
			require.Equal(t, adapter.(mcp.PlaceholderHints).Placeholders()[key], placeholder)
			label := key
			if key != "base_url" {
				label += " (optional)"
			}
			text(form.Locator(`label[for="cred_`+key+`"]`), label)
		})
	}
	endpoint, token := form.Locator("#cred_base_url"), form.Locator("#cred_bearer_token")
	require.NoError(t, endpoint.Fill(provider.URL))
	require.NoError(t, token.Fill("ui-fixture-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	save()
	text(page.Locator(".badge").First(), "Connected")
	search(true)
	capture("prometheus-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Close sidebar", Exact: playwright.Bool(true)}).Click())
	capture("prometheus-mobile")
	stop()
	start()
	navigate("/integrations/prometheus")
	text(page.Locator(".badge").First(), "Connected")
	value, err := token.InputValue()
	require.NoError(t, err)
	require.Equal(t, "ui-fixture-token", value)
	search(true)
	require.NoError(t, token.Fill("invalid-fixture-token"))
	save()
	text(page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, endpoint.Fill(""))
	save()
	text(page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, endpoint.Fill(provider.URL))
	require.NoError(t, token.Fill("ui-fixture-token"))
	save()
	text(page.Locator(".badge").First(), "Connected")
	navigate("/integrations")
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all"}).Click())
	text(card, "Connected")
	require.NoError(t, card.Click())
	require.NoError(t, form.Locator(".toggle").Click())
	save()
	text(page.Locator(".badge").First(), "Disabled")
	search(false)
	stop()
	start()
	navigate("/integrations/prometheus")
	text(page.Locator(".badge").First(), "Disabled")
	search(false)
}
