package slack

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/go-sqlite/sqlite3"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"golang.org/x/crypto/pbkdf2"
)

var _ *mcp.ToolResult // type anchor

// --- multi-workspace token store ---

// workspace holds credentials for a single Slack workspace.
type workspace struct {
	TeamID    string    `json:"team_id"`
	TeamName  string    `json:"team_name"`
	Token     string    `json:"token"`
	Cookie    string    `json:"cookie"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

// tokenStore holds credentials for multiple Slack workspaces.
type tokenStore struct {
	mu            sync.RWMutex
	workspaces    map[string]*workspace // keyed by team_id
	defaultTeamID string
	filePath      string
}

func newTokenStore() *tokenStore {
	home, _ := os.UserHomeDir()
	return &tokenStore{
		workspaces: make(map[string]*workspace),
		filePath:   filepath.Join(home, ".slack-mcp-tokens.json"),
	}
}

func (ts *tokenStore) getWorkspace(teamID string) *workspace {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if teamID == "" {
		teamID = ts.defaultTeamID
	}
	ws := ts.workspaces[teamID]
	if ws == nil {
		return nil
	}
	cp := *ws
	return &cp
}

func (ts *tokenStore) getDefault() *workspace {
	return ts.getWorkspace("")
}

func (ts *tokenStore) defaultID() string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.defaultTeamID
}

func (ts *tokenStore) setDefault(teamID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.defaultTeamID = teamID
}

func (ts *tokenStore) allWorkspaces() []*workspace {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]*workspace, 0, len(ts.workspaces))
	for _, ws := range ts.workspaces {
		cp := *ws
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeamName < out[j].TeamName })
	return out
}

func (ts *tokenStore) setWorkspace(ws *workspace) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ws.UpdatedAt = time.Now()
	ts.workspaces[ws.TeamID] = ws
	if ts.defaultTeamID == "" {
		ts.defaultTeamID = ws.TeamID
	}
}

func (ts *tokenStore) removeWorkspace(teamID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.workspaces, teamID)
	if ts.defaultTeamID == teamID {
		ts.defaultTeamID = ""
		for id := range ts.workspaces {
			ts.defaultTeamID = id
			break
		}
	}
}

func (ts *tokenStore) updateTokens(teamID, token, cookie string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ws, ok := ts.workspaces[teamID]
	if !ok {
		ws = &workspace{TeamID: teamID}
		ts.workspaces[teamID] = ws
		if ts.defaultTeamID == "" {
			ts.defaultTeamID = teamID
		}
	}
	ws.Token = token
	ws.Cookie = cookie
	ws.UpdatedAt = time.Now()
}

// --- backward-compatible file persistence ---

// tokenFileV2 is the new multi-workspace file format.
type tokenFileV2 struct {
	Version       int               `json:"version"`
	DefaultTeamID string            `json:"default_team_id"`
	Workspaces    []*tokenFileEntry `json:"workspaces"`
}

type tokenFileEntry struct {
	TeamID    string `json:"team_id"`
	TeamName  string `json:"team_name"`
	Token     string `json:"token"`
	Cookie    string `json:"cookie"`
	Source    string `json:"source"`
	UpdatedAt string `json:"updated_at"`
}

// loadFromFile reads the persisted token file. Handles both the legacy
// single-token format and the new multi-workspace format.
func (ts *tokenStore) loadFromFile() {
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		return
	}

	// Try v2 format first.
	var v2 tokenFileV2
	if err := json.Unmarshal(data, &v2); err == nil && v2.Version == 2 {
		ts.mu.Lock()
		defer ts.mu.Unlock()
		for _, entry := range v2.Workspaces {
			if entry.Token == "" {
				continue
			}
			ws := &workspace{
				TeamID:   entry.TeamID,
				TeamName: entry.TeamName,
				Token:    entry.Token,
				Cookie:   entry.Cookie,
				Source:   entry.Source,
			}
			if t, err := time.Parse(time.RFC3339, entry.UpdatedAt); err == nil {
				ws.UpdatedAt = t
			} else {
				ws.UpdatedAt = time.Now()
			}
			ts.workspaces[ws.TeamID] = ws
		}
		if v2.DefaultTeamID != "" {
			ts.defaultTeamID = v2.DefaultTeamID
		}
		return
	}

	// Fall back to legacy single-token format.
	var legacy struct {
		Token     string `json:"token"`
		Cookie    string `json:"cookie"`
		TeamID    string `json:"team_id"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil || legacy.Token == "" {
		return
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()
	teamID := legacy.TeamID
	if teamID == "" {
		teamID = "_unknown"
	}
	ws := &workspace{
		TeamID: teamID,
		Token:  legacy.Token,
		Cookie: legacy.Cookie,
		Source: "file",
	}
	if t, err := time.Parse(time.RFC3339, legacy.UpdatedAt); err == nil {
		ws.UpdatedAt = t
	} else {
		ws.UpdatedAt = time.Now()
	}
	ts.workspaces[teamID] = ws
	if ts.defaultTeamID == "" {
		ts.defaultTeamID = teamID
	}
}

func (ts *tokenStore) saveToFile() error {
	ts.mu.RLock()
	v2 := tokenFileV2{
		Version:       2,
		DefaultTeamID: ts.defaultTeamID,
	}
	for _, ws := range ts.workspaces {
		v2.Workspaces = append(v2.Workspaces, &tokenFileEntry{
			TeamID:    ws.TeamID,
			TeamName:  ws.TeamName,
			Token:     ws.Token,
			Cookie:    ws.Cookie,
			Source:    ws.Source,
			UpdatedAt: ws.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	ts.mu.RUnlock()

	sort.Slice(v2.Workspaces, func(i, j int) bool {
		return v2.Workspaces[i].TeamID < v2.Workspaces[j].TeamID
	})

	data, err := json.MarshalIndent(v2, "", "  ")
	if err != nil {
		return err
	}

	tmp := ts.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, ts.filePath)
}

// --- backward-compat helpers for single-workspace callers ---

// get returns the default workspace's token and cookie.
func (ts *tokenStore) get() (token, cookie string) {
	ws := ts.getDefault()
	if ws == nil {
		return "", ""
	}
	return ws.Token, ws.Cookie
}

// --- Chrome extraction (macOS only) ---
//
// Reads tokens directly from Chrome's on-disk storage:
//   - Token (xoxc-*): LevelDB localStorage at <profile>/Local Storage/leveldb/
//   - Cookie (xoxd-*): Encrypted SQLite cookie DB at <profile>/Cookies
//
// No AppleScript, no "Allow JavaScript from Apple Events" setting required.

type chromeTokens struct {
	token  string
	cookie string
}

var extractMu sync.Mutex

func extractFromChrome(teamID string) *chromeTokens {
	result, _ := extractFromChromeWithError(teamID)
	return result
}

// extractAllFromChrome extracts tokens for every workspace found in Chrome.
func extractAllFromChrome() ([]*chromeTokens, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("chrome extraction is only available on macOS")
	}

	extractMu.Lock()
	defer extractMu.Unlock()

	profiles, err := findChromeProfiles()
	if err != nil {
		return nil, fmt.Errorf("could not find Chrome profiles: %w", err)
	}

	var cookie string
	for _, profile := range profiles {
		if c, err := extractCookieFromChrome(profile); err == nil {
			cookie = c
			break
		}
	}

	seen := make(map[string]bool)
	var results []*chromeTokens
	for _, profile := range profiles {
		cfg, err := readSlackLocalConfig(profile)
		if err != nil {
			continue
		}
		for id, team := range cfg.Teams {
			if seen[id] || !strings.HasPrefix(team.Token, "xoxc-") {
				continue
			}
			seen[id] = true
			results = append(results, &chromeTokens{
				token:  team.Token,
				cookie: cookie,
			})
		}
	}
	return results, nil
}

func extractFromChromeWithError(teamID string) (*chromeTokens, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("chrome extraction is only available on macOS")
	}

	extractMu.Lock()
	defer extractMu.Unlock()

	profiles, err := findChromeProfiles()
	if err != nil {
		return nil, fmt.Errorf("could not find Chrome profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no Chrome profiles found at ~/Library/Application Support/Google/Chrome/")
	}

	var token, cookie string
	var lastTokenErr, lastCookieErr error

	for _, profile := range profiles {
		if token == "" {
			if t, err := extractTokenFromLevelDB(profile, teamID); err != nil {
				lastTokenErr = err
			} else {
				token = t
			}
		}
		if cookie == "" {
			if c, err := extractCookieFromChrome(profile); err != nil {
				lastCookieErr = err
			} else {
				cookie = c
			}
		}
		if token != "" && cookie != "" {
			break
		}
	}

	if token == "" && cookie == "" {
		msg := "could not extract Slack credentials from Chrome."
		if lastTokenErr != nil {
			msg += " Token: " + lastTokenErr.Error() + "."
		}
		if lastCookieErr != nil {
			msg += " Cookie: " + lastCookieErr.Error() + "."
		}
		msg += " Make sure you are logged in to Slack (app.slack.com) in Chrome."
		return nil, fmt.Errorf("%s", msg)
	}
	if token == "" {
		msg := "found cookie but no xoxc-* token in Chrome localStorage."
		if lastTokenErr != nil {
			msg += " " + lastTokenErr.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if cookie == "" {
		msg := "found token but no xoxd-* cookie in Chrome cookie store."
		if lastCookieErr != nil {
			msg += " " + lastCookieErr.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return &chromeTokens{token: token, cookie: cookie}, nil
}

// findChromeProfiles returns paths to all Chrome profile directories.
func findChromeProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	chromeDir := filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	entries, err := os.ReadDir(chromeDir)
	if err != nil {
		return nil, err
	}

	var profiles []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "Default" || strings.HasPrefix(name, "Profile ") {
			profiles = append(profiles, filepath.Join(chromeDir, name))
		}
	}
	return profiles, nil
}

// slackLocalConfig represents the parsed structure of Chrome's localStorage
// localConfig_v2 (or later versions) for Slack.
type slackLocalConfig struct {
	Teams map[string]struct {
		Token string `json:"token"`
		Name  string `json:"name"`
		URL   string `json:"url"`
	} `json:"teams"`
}

// readSlackLocalConfig reads and parses Chrome's localStorage LevelDB for Slack
// config data from the given profile path. Returns the parsed config.
func readSlackLocalConfig(profilePath string) (*slackLocalConfig, error) {
	ldbDir := filepath.Join(profilePath, "Local Storage", "leveldb")
	if _, err := os.Stat(ldbDir); err != nil {
		return nil, fmt.Errorf("no LevelDB at %s", ldbDir)
	}

	tmpDir, err := os.MkdirTemp("", "slack-ldb-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	entries, err := os.ReadDir(ldbDir)
	if err != nil {
		return nil, fmt.Errorf("reading LevelDB dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "LOCK" {
			continue
		}
		src := filepath.Join(ldbDir, e.Name())
		dst := filepath.Join(tmpDir, e.Name())
		if err := copyFile(src, dst); err != nil {
			continue
		}
	}

	db, err := leveldb.OpenFile(tmpDir, &opt.Options{
		ReadOnly: true,
		Strict:   opt.NoStrict,
	})
	if err != nil {
		return nil, fmt.Errorf("opening LevelDB copy: %w", err)
	}
	defer func() { _ = db.Close() }()

	versions := []string{"localConfig_v2", "localConfig_v3", "localConfig_v4", "localConfig_v5"}
	var val []byte
	for _, v := range versions {
		key := "_https://app.slack.com\x00\x01" + v
		if found, err := db.Get([]byte(key), nil); err == nil {
			val = found
			break
		}
	}
	if val == nil {
		return nil, fmt.Errorf("key not found in profile %s", filepath.Base(profilePath))
	}

	d := val
	if len(d) > 0 && d[0] == 0x01 {
		d = d[1:]
	}

	var cfg slackLocalConfig
	if err := json.Unmarshal(d, &cfg); err != nil {
		return nil, fmt.Errorf("parsing localConfig: %w", err)
	}
	return &cfg, nil
}

// listWorkspacesFromChrome scans all Chrome profiles and returns every Slack
// workspace that has an xoxc-* token.
func listWorkspacesFromChrome() ([]WorkspaceInfo, error) {
	extractMu.Lock()
	defer extractMu.Unlock()

	profiles, err := findChromeProfiles()
	if err != nil {
		return nil, fmt.Errorf("could not find Chrome profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no Chrome profiles found at ~/Library/Application Support/Google/Chrome/")
	}

	seen := make(map[string]bool)
	var workspaces []WorkspaceInfo
	for _, profile := range profiles {
		cfg, err := readSlackLocalConfig(profile)
		if err != nil {
			continue
		}
		for id, team := range cfg.Teams {
			if seen[id] || !strings.HasPrefix(team.Token, "xoxc-") {
				continue
			}
			seen[id] = true
			name := team.Name
			if name == "" {
				name = id
			}
			workspaces = append(workspaces, WorkspaceInfo{
				TeamID: id,
				Name:   name,
				URL:    team.URL,
			})
		}
	}

	if len(workspaces) == 0 {
		return nil, fmt.Errorf("no Slack workspaces with xoxc-* tokens found in Chrome, make sure you are logged in to Slack at app.slack.com")
	}
	return workspaces, nil
}

// extractTokenFromLevelDB reads the xoxc-* token from Chrome's localStorage
// LevelDB. Chrome holds a lock on the DB while running, so we copy the files
// to a temp directory first. If teamID is non-empty, only that workspace's
// token is returned.
func extractTokenFromLevelDB(profilePath, teamID string) (string, error) {
	cfg, err := readSlackLocalConfig(profilePath)
	if err != nil {
		return "", err
	}

	if teamID != "" {
		team, ok := cfg.Teams[teamID]
		if !ok {
			return "", fmt.Errorf("team %s not found in profile %s", teamID, filepath.Base(profilePath))
		}
		if !strings.HasPrefix(team.Token, "xoxc-") {
			return "", fmt.Errorf("team %s has no xoxc-* token in profile %s", teamID, filepath.Base(profilePath))
		}
		return team.Token, nil
	}

	for _, team := range cfg.Teams {
		if strings.HasPrefix(team.Token, "xoxc-") {
			return team.Token, nil
		}
	}
	return "", fmt.Errorf("no xoxc-* token in localConfig_v2 for profile %s", filepath.Base(profilePath))
}

// extractCookieFromChrome reads the xoxd-* session cookie from Chrome's
// encrypted SQLite cookie database. It copies the DB to a temp file (Chrome
// holds a lock while running), reads the Chrome Safe Storage password from
// the macOS Keychain, and decrypts the cookie value using AES-128-CBC.
func extractCookieFromChrome(profilePath string) (string, error) {
	cookiesFile := filepath.Join(profilePath, "Cookies")
	if _, err := os.Stat(cookiesFile); err != nil {
		return "", fmt.Errorf("no Cookies file at %s", cookiesFile)
	}

	tmpDir, err := os.MkdirTemp("", "slack-cookies-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpFile := filepath.Join(tmpDir, "Cookies")
	if err := copyFile(cookiesFile, tmpFile); err != nil {
		return "", fmt.Errorf("copying Cookies DB: %w", err)
	}

	password, err := chromeKeychainPassword()
	if err != nil {
		return "", err
	}

	db, err := sqlite3.Open(tmpFile)
	if err != nil {
		return "", fmt.Errorf("opening Cookies DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	var cookie string
	err = db.VisitTableRecords("cookies", func(_ *int64, rec sqlite3.Record) error {
		if cookie != "" || len(rec.Values) < 6 {
			return nil
		}
		host, _ := rec.Values[1].(string)
		name, _ := rec.Values[3].(string)
		if !strings.Contains(host, "slack.com") || name != "d" {
			return nil
		}
		enc, ok := rec.Values[5].([]byte)
		if !ok || len(enc) < 4 {
			return nil
		}
		val, err := decryptChromeCookie(enc, password)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(val, "xoxd-") {
			cookie = val
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("reading Cookies DB: %w", err)
	}

	if cookie == "" {
		return "", fmt.Errorf("no xoxd-* cookie for slack.com in profile %s", filepath.Base(profilePath))
	}
	return cookie, nil
}

// chromeKeychainPassword retrieves the Chrome Safe Storage encryption key
// from the macOS Keychain via the security command.
func chromeKeychainPassword() (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", "Chrome Safe Storage", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("could not read Chrome Safe Storage from Keychain: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// decryptChromeCookie decrypts a Chrome encrypted_value blob using AES-128-CBC
// with the Chrome Safe Storage password. Handles the v10/v11 prefix and
// Chrome v24+ 32-byte prefix padding.
func decryptChromeCookie(encrypted []byte, password string) (string, error) {
	if len(encrypted) <= 3 {
		return "", fmt.Errorf("too short")
	}
	prefix := string(encrypted[:3])
	if prefix != "v10" && prefix != "v11" {
		return "", fmt.Errorf("unknown prefix: %q", prefix)
	}
	encrypted = encrypted[3:]
	if len(encrypted)%aes.BlockSize != 0 {
		return "", fmt.Errorf("not block-aligned")
	}

	key := pbkdf2.Key([]byte(password), []byte("saltysalt"), 1003, 16, sha1.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := []byte("                ") // 16 spaces
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// PKCS#7 padding removal.
	padLen := int(decrypted[len(decrypted)-1])
	if padLen < 1 || padLen > aes.BlockSize {
		return "", fmt.Errorf("bad padding")
	}
	decrypted = decrypted[:len(decrypted)-padLen]

	// Chrome v24+ prepends 32 bytes of padding before the plaintext.
	// Find the cookie value by scanning for the xoxd- prefix.
	s := string(decrypted)
	if idx := strings.Index(s, "xoxd-"); idx >= 0 {
		return s[idx:], nil
	}
	return s, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}

// --- token management tool handlers ---

func tokenStatus(_ context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	workspaces := s.store.allWorkspaces()
	defaultID := s.store.defaultID()

	type wsStatus struct {
		TeamID    string  `json:"team_id"`
		TeamName  string  `json:"team_name"`
		Status    string  `json:"status"`
		TokenType string  `json:"token_type"`
		AgeHours  float64 `json:"age_hours"`
		Source    string  `json:"source"`
		IsDefault bool    `json:"is_default"`
	}

	var statuses []wsStatus
	for _, ws := range workspaces {
		ageHours := 0.0
		if !ws.UpdatedAt.IsZero() {
			ageHours = math.Round(time.Since(ws.UpdatedAt).Hours()*10) / 10
		}

		tokenType := "unknown"
		if strings.HasPrefix(ws.Token, "xoxp-") {
			tokenType = "oauth_user"
		} else if strings.HasPrefix(ws.Token, "xoxc-") {
			tokenType = "browser_session"
		} else if strings.HasPrefix(ws.Token, "xoxb-") {
			tokenType = "bot"
		}

		status := "healthy"
		if tokenType == "browser_session" {
			if ageHours > 10 {
				status = "critical"
			} else if ageHours > 6 {
				status = "warning"
			}
		}

		statuses = append(statuses, wsStatus{
			TeamID:    ws.TeamID,
			TeamName:  ws.TeamName,
			Status:    status,
			TokenType: tokenType,
			AgeHours:  ageHours,
			Source:    ws.Source,
			IsDefault: ws.TeamID == defaultID,
		})
	}

	refreshInfo := map[string]any{
		"enabled":        true,
		"interval":       "4 hours",
		"chrome_refresh": CanExtractFromChrome(),
		"platform":       runtime.GOOS,
	}

	return mcp.JSONResult(map[string]any{
		"workspace_count": len(statuses),
		"default_team_id": defaultID,
		"workspaces":      statuses,
		"auto_refresh":    refreshInfo,
	})
}

func refreshTokens(_ context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	teamID, _ := mcp.ArgStr(args, "team_id")

	if teamID != "" {
		ws := s.store.getWorkspace(teamID)
		if ws == nil {
			return &mcp.ToolResult{Data: fmt.Sprintf("unknown workspace: %s", teamID), IsError: true}, nil
		}
		if strings.HasPrefix(ws.Token, "xoxp-") {
			return mcp.JSONResult(map[string]any{
				"status": "not_needed",
				"note":   "OAuth tokens (xoxp-) do not expire. No refresh needed.",
			})
		}
	}

	if ok := s.tryRefresh(); !ok {
		return &mcp.ToolResult{
			Data:    "Could not refresh tokens. Tried cookie-based refresh and Chrome extraction. Make sure you have a valid cookie or Chrome is running with Slack open.",
			IsError: true,
		}, nil
	}

	client := s.getClientForTeam(teamID)
	if client == nil {
		return mcp.JSONResult(map[string]any{"status": "refreshed"})
	}
	resp, err := client.AuthTest()
	if err != nil {
		return errResult(fmt.Errorf("refreshed tokens but auth failed: %w", err))
	}
	return mcp.JSONResult(map[string]any{
		"status":  "refreshed",
		"user":    resp.User,
		"team":    resp.Team,
		"user_id": resp.UserID,
	})
}

func listWorkspaces(_ context.Context, s *slackIntegration, _ map[string]any) (*mcp.ToolResult, error) {
	workspaces := s.store.allWorkspaces()
	defaultID := s.store.defaultID()

	type wsInfo struct {
		TeamID    string `json:"team_id"`
		TeamName  string `json:"team_name"`
		IsDefault bool   `json:"is_default"`
	}

	out := make([]wsInfo, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, wsInfo{
			TeamID:    ws.TeamID,
			TeamName:  ws.TeamName,
			IsDefault: ws.TeamID == defaultID,
		})
	}

	return mcp.JSONResult(map[string]any{
		"count":           len(out),
		"default_team_id": defaultID,
		"workspaces":      out,
	})
}
