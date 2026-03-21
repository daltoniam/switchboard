package slack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
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
	_, err := SaveTokensForWeb("", "cookie", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestExtractFromChromeForWeb_PlatformGuard(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("this test validates the non-darwin guard")
	}
	result := ExtractFromChromeForWeb("")
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "macOS")
}

func TestListWorkspacesFromChrome_PlatformGuard(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("this test validates the non-darwin guard")
	}
	_, err := ListWorkspacesFromChrome()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "macOS")
}

// writeSyntheticLevelDB creates a Chrome-like LevelDB profile directory with
// the given localConfig version and JSON teams data.
func writeSyntheticLevelDB(t *testing.T, version string, teams map[string]any) string {
	t.Helper()
	profileDir := t.TempDir()
	ldbDir := filepath.Join(profileDir, "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(ldbDir, 0755))

	db, err := leveldb.OpenFile(ldbDir, nil)
	require.NoError(t, err)

	cfg := map[string]any{"teams": teams}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	key := "_https://app.slack.com\x00\x01" + version
	val := append([]byte{0x01}, data...)
	require.NoError(t, db.Put([]byte(key), val, nil))
	require.NoError(t, db.Close())

	return profileDir
}

func TestExtractTokenFromLevelDB_SingleTeam(t *testing.T) {
	profile := writeSyntheticLevelDB(t, "localConfig_v2", map[string]any{
		"T001": map[string]any{
			"token": "xoxc-single-workspace",
			"name":  "Acme Corp",
			"url":   "https://acme.slack.com",
		},
	})

	token, err := extractTokenFromLevelDB(profile, "")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-single-workspace", token)
}

func TestExtractTokenFromLevelDB_SpecificTeamID(t *testing.T) {
	profile := writeSyntheticLevelDB(t, "localConfig_v2", map[string]any{
		"T001": map[string]any{
			"token": "xoxc-team-one",
			"name":  "Team One",
		},
		"T002": map[string]any{
			"token": "xoxc-team-two",
			"name":  "Team Two",
		},
	})

	token, err := extractTokenFromLevelDB(profile, "T002")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-team-two", token)
}

func TestExtractTokenFromLevelDB_TeamIDNotFound(t *testing.T) {
	profile := writeSyntheticLevelDB(t, "localConfig_v2", map[string]any{
		"T001": map[string]any{
			"token": "xoxc-exists",
			"name":  "Exists",
		},
	})

	_, err := extractTokenFromLevelDB(profile, "T999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "T999")
}

func TestExtractTokenFromLevelDB_SkipsNonXoxcTokens(t *testing.T) {
	profile := writeSyntheticLevelDB(t, "localConfig_v2", map[string]any{
		"T001": map[string]any{
			"token": "xoxb-bot-token",
			"name":  "Bot",
		},
	})

	_, err := extractTokenFromLevelDB(profile, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no xoxc-*")
}

func TestExtractTokenFromLevelDB_FallsBackToLaterVersion(t *testing.T) {
	profile := writeSyntheticLevelDB(t, "localConfig_v5", map[string]any{
		"T001": map[string]any{
			"token": "xoxc-from-v5",
			"name":  "V5 Workspace",
		},
	})

	token, err := extractTokenFromLevelDB(profile, "")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-from-v5", token)
}

func TestReadSlackLocalConfig_MultipleTeams(t *testing.T) {
	profile := writeSyntheticLevelDB(t, "localConfig_v2", map[string]any{
		"T001": map[string]any{
			"token": "xoxc-one",
			"name":  "Workspace One",
			"url":   "https://one.slack.com",
		},
		"T002": map[string]any{
			"token": "xoxc-two",
			"name":  "Workspace Two",
			"url":   "https://two.slack.com",
		},
	})

	cfg, err := readSlackLocalConfig(profile)
	require.NoError(t, err)
	assert.Len(t, cfg.Teams, 2)
	assert.Equal(t, "xoxc-one", cfg.Teams["T001"].Token)
	assert.Equal(t, "Workspace One", cfg.Teams["T001"].Name)
	assert.Equal(t, "https://one.slack.com", cfg.Teams["T001"].URL)
	assert.Equal(t, "xoxc-two", cfg.Teams["T002"].Token)
	assert.Equal(t, "Workspace Two", cfg.Teams["T002"].Name)
}

func TestReadSlackLocalConfig_NoLevelDB(t *testing.T) {
	_, err := readSlackLocalConfig(t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LevelDB")
}
