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

// refreshResult holds the refreshed token and (optionally rotated) cookie.
type refreshResult struct {
	token  string
	cookie string
}

// refreshViaCookie fetches a fresh xoxc token by loading a Slack workspace
// page with the xoxd session cookie. It also captures any rotated d= cookie
// from Set-Cookie response headers — Slack rotates the session cookie on
// each use, so failing to capture the new value causes subsequent refreshes
// to fail.
func refreshViaCookie(cookie string) (*refreshResult, error) {
	return refreshViaCookieWithClient(nil, "https://app.slack.com", cookie)
}

func refreshViaCookieWithClient(client *http.Client, url, cookie string) (*refreshResult, error) {
	if cookie == "" {
		return nil, nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "d="+cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	if client == nil {
		client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				req.Header.Set("Cookie", "d="+cookie)
				return nil
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Capture the rotated d= cookie from response headers.
	var latestCookie string
	for _, c := range resp.Cookies() {
		if c.Name == "d" && strings.HasPrefix(c.Value, "xoxd-") {
			latestCookie = c.Value
		}
	}
	if latestCookie == "" {
		latestCookie = cookie
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	// Extract xoxc token from the page's boot data JSON.
	if m := xoxcPattern.FindSubmatch(body); len(m) > 1 {
		return &refreshResult{token: string(m[1]), cookie: latestCookie}, nil
	}

	// Try parsing JSON boot_data embedded in the page.
	if idx := strings.Index(string(body), `"api_token":"`); idx >= 0 {
		sub := string(body[idx:])
		start := strings.Index(sub, `"api_token":"`) + len(`"api_token":"`)
		end := strings.Index(sub[start:], `"`)
		if end > 0 {
			tok := sub[start : start+end]
			if strings.HasPrefix(tok, "xoxc-") {
				return &refreshResult{token: tok, cookie: latestCookie}, nil
			}
		}
	}

	return nil, nil
}

// tryRefreshViaCookieForTeam attempts a cookie-based refresh for a specific workspace.
func (s *slackIntegration) tryRefreshViaCookieForTeam(teamID string) bool {
	ws := s.store.getWorkspace(teamID)
	if ws == nil || ws.Cookie == "" {
		return false
	}

	result, err := refreshViaCookie(ws.Cookie)
	if err != nil {
		log.Printf("slack: cookie refresh failed for %s: %v", teamID, err)
		return false
	}
	if result == nil {
		return false
	}

	s.store.updateTokens(teamID, result.token, result.cookie)
	updatedWs := s.store.getWorkspace(teamID)
	if updatedWs != nil {
		s.buildClientForWorkspace(updatedWs)
	}

	// Validate that the refreshed token still belongs to the expected workspace.
	client := s.getClientForTeam(teamID)
	if client != nil {
		resp, err := client.AuthTest()
		if err != nil {
			log.Printf("slack: cookie refresh auth test failed for %s: %v", teamID, err)
			return false
		}
		if resp.TeamID != teamID {
			log.Printf("slack: cookie refresh returned wrong workspace %s (%s), expected %s — rejecting", resp.Team, resp.TeamID, teamID)
			return false
		}
	}

	_ = s.store.saveToFile()
	log.Printf("slack: tokens refreshed via cookie for %s", teamID)
	return true
}

// RefreshStatus returns info about the current refresh capability.
func (s *slackIntegration) RefreshStatus() map[string]any {
	ws := s.store.getDefault()
	if ws == nil {
		return map[string]any{
			"token_type":    "unknown",
			"has_cookie":    false,
			"needs_refresh": false,
		}
	}

	tokenType := "unknown"
	if strings.HasPrefix(ws.Token, "xoxp-") {
		tokenType = "oauth_user"
	} else if strings.HasPrefix(ws.Token, "xoxc-") {
		tokenType = "browser_session"
	} else if strings.HasPrefix(ws.Token, "xoxb-") {
		tokenType = "bot"
	}

	return map[string]any{
		"token_type":         tokenType,
		"has_cookie":         ws.Cookie != "",
		"source":             ws.Source,
		"updated_at":         ws.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		"can_cookie_refresh": ws.Cookie != "" && strings.HasPrefix(ws.Token, "xoxc-"),
		"can_chrome_refresh": CanExtractFromChrome(),
		"oauth_token":        strings.HasPrefix(ws.Token, "xoxp-"),
		"needs_refresh":      strings.HasPrefix(ws.Token, "xoxc-"),
	}
}

// RefreshStatusJSON returns refresh info as a JSON ToolResult.
func RefreshStatusJSON(s *slackIntegration) string {
	data, _ := json.MarshalIndent(s.RefreshStatus(), "", "  ")
	return string(data)
}
