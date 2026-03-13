package browser

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	"github.com/playwright-community/playwright-go"
)

// service implements mcp.BrowserService backed by Playwright.
type service struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

// New creates a Playwright-backed BrowserService.
// headless controls whether the browser window is visible.
func New(headless bool) (mcp.BrowserService, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("browser: start playwright: %w", err)
	}

	b, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("browser: launch chromium: %w", err)
	}

	return &service{pw: pw, browser: b}, nil
}

func (s *service) NewSession(_ context.Context) (mcp.BrowserSession, error) {
	bctx, err := s.browser.NewContext()
	if err != nil {
		return nil, fmt.Errorf("browser: new context: %w", err)
	}
	return &session{ctx: bctx}, nil
}

func (s *service) Close() error {
	var browserErr, pwErr error
	browserErr = s.browser.Close()
	pwErr = s.pw.Stop()
	if browserErr != nil {
		return fmt.Errorf("browser: close browser: %w", browserErr)
	}
	if pwErr != nil {
		return fmt.Errorf("browser: stop playwright: %w", pwErr)
	}
	return nil
}

// session implements mcp.BrowserSession.
type session struct {
	ctx playwright.BrowserContext
}

func (s *session) NewPage(_ context.Context) (mcp.BrowserPage, error) {
	p, err := s.ctx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("browser: new page: %w", err)
	}
	return &page{p: p}, nil
}

func (s *session) Close() error {
	if err := s.ctx.Close(); err != nil {
		return fmt.Errorf("browser: close context: %w", err)
	}
	return nil
}

// page implements mcp.BrowserPage.
type page struct {
	p playwright.Page
}

func (pg *page) Navigate(_ context.Context, url string) error {
	if _, err := pg.p.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
	}); err != nil {
		return fmt.Errorf("browser: navigate %s: %w", url, err)
	}
	return nil
}

func (pg *page) Fill(_ context.Context, selector, value string) error {
	if err := pg.p.Fill(selector, value); err != nil {
		return fmt.Errorf("browser: fill %s: %w", selector, err)
	}
	return nil
}

func (pg *page) Click(_ context.Context, selector string) error {
	if err := pg.p.Click(selector); err != nil {
		return fmt.Errorf("browser: click %s: %w", selector, err)
	}
	return nil
}

func (pg *page) SelectOption(_ context.Context, selector, value string) error {
	vals := []string{value}
	if _, err := pg.p.SelectOption(selector, playwright.SelectOptionValues{Values: &vals}); err != nil {
		return fmt.Errorf("browser: select option %s: %w", selector, err)
	}
	return nil
}

func (pg *page) InnerText(_ context.Context, selector string) (string, error) {
	text, err := pg.p.InnerText(selector)
	if err != nil {
		return "", fmt.Errorf("browser: inner text %s: %w", selector, err)
	}
	return text, nil
}

func (pg *page) InnerHTML(_ context.Context, selector string) (string, error) {
	html, err := pg.p.InnerHTML(selector)
	if err != nil {
		return "", fmt.Errorf("browser: inner html %s: %w", selector, err)
	}
	return html, nil
}

func (pg *page) Content(_ context.Context) (string, error) {
	content, err := pg.p.Content()
	if err != nil {
		return "", fmt.Errorf("browser: content: %w", err)
	}
	return content, nil
}

func (pg *page) WaitForSelector(_ context.Context, selector string) error {
	if _, err := pg.p.WaitForSelector(selector); err != nil {
		return fmt.Errorf("browser: wait for selector %s: %w", selector, err)
	}
	return nil
}

func (pg *page) Screenshot(_ context.Context) ([]byte, error) {
	data, err := pg.p.Screenshot(playwright.PageScreenshotOptions{Type: playwright.ScreenshotTypePng})
	if err != nil {
		return nil, fmt.Errorf("browser: screenshot: %w", err)
	}
	return data, nil
}

func (pg *page) Evaluate(_ context.Context, expression string, args ...any) (any, error) {
	result, err := pg.p.Evaluate(expression, args...)
	if err != nil {
		return nil, fmt.Errorf("browser: evaluate: %w", err)
	}
	return result, nil
}

func (pg *page) Close() error {
	if err := pg.p.Close(); err != nil {
		return fmt.Errorf("browser: close page: %w", err)
	}
	return nil
}
