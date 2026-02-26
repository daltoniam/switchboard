package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

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

// ExtractFromChromeForWeb triggers Chrome extraction and returns the result.
// This is exported for use by the web UI server.
func ExtractFromChromeForWeb() *ExtractResult {
	if runtime.GOOS != "darwin" {
		return &ExtractResult{
			Success: false,
			Error:   "Chrome extraction is only available on macOS. Use the manual method below.",
		}
	}

	extracted, err := extractFromChromeWithError()
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
  var cookie = document.cookie.split('; ').find(c => c.startsWith('d='));
  var cookieVal = cookie ? cookie.split('=').slice(1).join('=') : '';
  var token = '';
  try { token = JSON.parse(localStorage.localConfig_v2).teams[Object.keys(JSON.parse(localStorage.localConfig_v2).teams)[0]].token; } catch(e) {}
  if (!token) { try { token = JSON.parse(localStorage.localConfig_v3).teams[Object.keys(JSON.parse(localStorage.localConfig_v3).teams)[0]].token; } catch(e) {} }
  if (!token) { try { token = window.boot_data && window.boot_data.api_token; } catch(e) {} }
  if (token && cookieVal) {
    prompt('Copy this entire value and paste it in the web UI:', JSON.stringify({token: token, cookie: cookieVal}));
  } else {
    alert('Could not extract tokens. Make sure you are on a Slack workspace page (app.slack.com).');
  }
})();`
}
