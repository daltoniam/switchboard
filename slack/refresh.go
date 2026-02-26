package slack

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var xoxcPattern = regexp.MustCompile(`"token"\s*:\s*"(xoxc-[a-zA-Z0-9-]+)"`)

// refreshViaCookie fetches a fresh xoxc token by loading a Slack workspace
// page with the xoxd session cookie. This avoids needing to read Chrome's
// LevelDB on disk and works even if Chrome is closed.
func refreshViaCookie(cookie string) (newToken string, err error) {
	if cookie == "" {
		return "", nil
	}

	req, err := http.NewRequest("GET", "https://app.slack.com", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", "d="+cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := (&http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			req.Header.Set("Cookie", "d="+cookie)
			return nil
		},
	}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", err
	}

	// Extract xoxc token from the page's boot data JSON.
	if m := xoxcPattern.FindSubmatch(body); len(m) > 1 {
		return string(m[1]), nil
	}

	// Try parsing JSON boot_data embedded in the page.
	if idx := strings.Index(string(body), `"api_token":"`); idx >= 0 {
		sub := string(body[idx:])
		start := strings.Index(sub, `"api_token":"`) + len(`"api_token":"`)
		end := strings.Index(sub[start:], `"`)
		if end > 0 {
			tok := sub[start : start+end]
			if strings.HasPrefix(tok, "xoxc-") {
				return tok, nil
			}
		}
	}

	return "", nil
}

// tryRefreshViaCookie attempts a cookie-based refresh and falls back to Chrome extraction.
func (s *slackIntegration) tryRefreshViaCookie() bool {
	_, cookie := s.store.get()
	if cookie == "" {
		return false
	}

	token, err := refreshViaCookie(cookie)
	if err != nil {
		log.Printf("slack: cookie refresh failed: %v", err)
		return false
	}
	if token == "" {
		return false
	}

	s.store.set(token, cookie)
	_ = s.store.saveToFile()
	s.buildClient(token, cookie)
	log.Println("slack: tokens refreshed via cookie")
	return true
}

// RefreshStatus returns info about the current refresh capability.
func (s *slackIntegration) RefreshStatus() map[string]any {
	tok, cookie := s.store.get()
	_, _, source, updatedAt := s.store.info()

	tokenType := "unknown"
	if strings.HasPrefix(tok, "xoxp-") {
		tokenType = "oauth_user"
	} else if strings.HasPrefix(tok, "xoxc-") {
		tokenType = "browser_session"
	} else if strings.HasPrefix(tok, "xoxb-") {
		tokenType = "bot"
	}

	return map[string]any{
		"token_type":        tokenType,
		"has_cookie":        cookie != "",
		"source":            source,
		"updated_at":        updatedAt.Format("2006-01-02T15:04:05Z"),
		"can_cookie_refresh": cookie != "" && strings.HasPrefix(tok, "xoxc-"),
		"can_chrome_refresh": CanExtractFromChrome(),
		"oauth_token":       strings.HasPrefix(tok, "xoxp-"),
		"needs_refresh":     strings.HasPrefix(tok, "xoxc-"),
	}
}

// RefreshStatusJSON returns refresh info as a JSON ToolResult.
func RefreshStatusJSON(s *slackIntegration) string {
	data, _ := json.MarshalIndent(s.RefreshStatus(), "", "  ")
	return string(data)
}
