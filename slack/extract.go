package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// WorkspaceInfo describes a Slack workspace found in Chrome's localStorage.
type WorkspaceInfo struct {
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
	URL    string `json:"url"`
}

// ExtractResult holds the result of a token extraction attempt.
type ExtractResult struct {
	Token   string `json:"token"`
	Cookie  string `json:"cookie"`
	Source  string `json:"source"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// TokenInfo holds current token status for the web UI.
type TokenInfo struct {
	HasToken  bool      `json:"has_token"`
	HasCookie bool      `json:"has_cookie"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
	AgeHours  float64   `json:"age_hours"`
	Status    string    `json:"status"`
}

// ListWorkspacesFromChrome returns all Slack workspaces found in Chrome's localStorage.
// Exported for use by the web UI to let the user pick which workspace to extract.
func ListWorkspacesFromChrome() ([]WorkspaceInfo, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("chrome extraction is only available on macOS")
	}
	return listWorkspacesFromChrome()
}

// ExtractFromChromeForWeb triggers Chrome extraction and returns the result.
// If teamID is non-empty, only the token for that workspace is extracted.
// This is exported for use by the web UI server.
func ExtractFromChromeForWeb(teamID string) *ExtractResult {
	if runtime.GOOS != "darwin" {
		return &ExtractResult{
			Success: false,
			Error:   "Chrome extraction is only available on macOS. Use the manual method below.",
		}
	}

	extracted, err := extractFromChromeWithError(teamID)
	if err != nil {
		return &ExtractResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &ExtractResult{
		Token:   extracted.token,
		Cookie:  extracted.cookie,
		Source:  "chrome",
		Success: true,
	}
}

// SaveTokensForWeb saves the given token/cookie to the persistent file
// and returns token info. Exported for use by the web UI server.
func SaveTokensForWeb(token, cookie string) (*TokenInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	home, _ := os.UserHomeDir()
	fp := filepath.Join(home, ".slack-mcp-tokens.json")

	data, err := json.MarshalIndent(map[string]string{
		"token":      token,
		"cookie":     cookie,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	tmp := fp + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, fp); err != nil {
		return nil, err
	}

	return &TokenInfo{
		HasToken:  true,
		HasCookie: cookie != "",
		Source:    "web_setup",
		UpdatedAt: time.Now(),
		AgeHours:  0,
		Status:    "healthy",
	}, nil
}

// GetTokenInfoForWeb reads the current token status from the persistent file.
// Exported for use by the web UI server.
func GetTokenInfoForWeb() *TokenInfo {
	home, _ := os.UserHomeDir()
	fp := filepath.Join(home, ".slack-mcp-tokens.json")

	data, err := os.ReadFile(fp)
	if err != nil {
		return &TokenInfo{Status: "no_tokens"}
	}

	var f struct {
		Token     string `json:"token"`
		Cookie    string `json:"cookie"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &f); err != nil || f.Token == "" {
		return &TokenInfo{Status: "no_tokens"}
	}

	info := &TokenInfo{
		HasToken:  true,
		HasCookie: f.Cookie != "",
		Source:    "file",
	}

	if t, err := time.Parse(time.RFC3339, f.UpdatedAt); err == nil {
		info.UpdatedAt = t
		info.AgeHours = float64(int(time.Since(t).Hours()*10)) / 10
	}

	if info.AgeHours > 10 {
		info.Status = "critical"
	} else if info.AgeHours > 6 {
		info.Status = "warning"
	} else {
		info.Status = "healthy"
	}

	return info
}

// CanExtractFromChrome returns true if the current platform supports
// automatic Chrome extraction (macOS only).
func CanExtractFromChrome() bool {
	return runtime.GOOS == "darwin"
}

// ExtractionSnippet returns the JavaScript snippet users should run in
// their browser console to extract tokens manually.
func ExtractionSnippet() string {
	return `(function() {
  var cookie = document.cookie.split('; ').find(function(c) { return c.startsWith('d='); });
  var cookieVal = cookie ? cookie.split('=').slice(1).join('=') : '';
  var teams = null;
  var versions = ['localConfig_v2', 'localConfig_v3', 'localConfig_v4', 'localConfig_v5'];
  for (var i = 0; i < versions.length; i++) {
    try {
      var raw = localStorage.getItem(versions[i]);
      if (raw) { var parsed = JSON.parse(raw); if (parsed && parsed.teams) { teams = parsed.teams; break; } }
    } catch(e) {}
  }
  if (!teams && window.boot_data && window.boot_data.api_token) {
    if (cookieVal) {
      prompt('Copy this entire value and paste it in the web UI:', JSON.stringify({token: window.boot_data.api_token, cookie: cookieVal}));
    } else { alert('Found token but no cookie. Make sure you are on app.slack.com.'); }
    return;
  }
  if (!teams) { alert('Could not find Slack config. Make sure you are on a Slack workspace page (app.slack.com).'); return; }
  var ids = Object.keys(teams);
  var entries = [];
  for (var j = 0; j < ids.length; j++) {
    var t = teams[ids[j]];
    if (t && t.token && t.token.indexOf('xoxc-') === 0) {
      entries.push({id: ids[j], name: t.name || ids[j], url: t.url || '', token: t.token});
    }
  }
  if (entries.length === 0) { alert('No xoxc- tokens found. Make sure you are signed in.'); return; }
  var chosen = entries[0];
  if (entries.length > 1) {
    var msg = 'Multiple workspaces found. Enter the number:\n';
    for (var k = 0; k < entries.length; k++) { msg += (k+1) + '. ' + entries[k].name + '\n'; }
    var pick = prompt(msg);
    if (!pick) return;
    var idx = parseInt(pick, 10) - 1;
    if (idx >= 0 && idx < entries.length) chosen = entries[idx];
  }
  if (chosen.token && cookieVal) {
    prompt('Copy this entire value and paste it in the web UI:', JSON.stringify({token: chosen.token, cookie: cookieVal}));
  } else {
    alert('Could not extract tokens. Make sure you are on a Slack workspace page (app.slack.com).');
  }
})();`
}
