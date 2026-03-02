package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractionSnippet_NotEmpty(t *testing.T) {
	snippet := ExtractionSnippet()
	assert.NotEmpty(t, snippet)
}

func TestExtractionSnippet_ContainsWorkspaceSelection(t *testing.T) {
	snippet := ExtractionSnippet()
	assert.Contains(t, snippet, "Multiple workspaces found", "snippet should handle multiple workspaces")
	assert.Contains(t, snippet, "localConfig_v2", "snippet should try v2")
	assert.Contains(t, snippet, "localConfig_v5", "snippet should try v5")
	assert.Contains(t, snippet, "boot_data", "snippet should fall back to boot_data")
}

func TestExtractionSnippet_NoArrowFunctions(t *testing.T) {
	snippet := ExtractionSnippet()
	assert.NotContains(t, snippet, "=>", "snippet should not use arrow functions for broad browser compatibility")
}

func TestCanExtractFromChrome(t *testing.T) {
	result := CanExtractFromChrome()
	// We can only assert it returns a bool; macOS test runners return true
	assert.IsType(t, true, result)
}

func TestWorkspaceInfo_Fields(t *testing.T) {
	w := WorkspaceInfo{
		TeamID: "T12345",
		Name:   "My Workspace",
		URL:    "https://my.slack.com",
	}
	assert.Equal(t, "T12345", w.TeamID)
	assert.Equal(t, "My Workspace", w.Name)
	assert.Equal(t, "https://my.slack.com", w.URL)
}

func TestExtractResult_Fields(t *testing.T) {
	r := ExtractResult{
		Token:   "xoxc-test",
		Cookie:  "xoxd-test",
		Source:  "chrome",
		Success: true,
	}
	assert.True(t, r.Success)
	assert.Equal(t, "xoxc-test", r.Token)
	assert.Equal(t, "xoxd-test", r.Cookie)
	assert.Equal(t, "chrome", r.Source)
}

func TestSaveTokensForWeb_EmptyToken(t *testing.T) {
	_, err := SaveTokensForWeb("", "cookie")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}
