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

// --- token store ---

// tokenStore holds the current session token and cookie with thread-safe access.
type tokenStore struct {
	mu        sync.RWMutex
	token     string
	cookie    string
	updatedAt time.Time
	source    string
	filePath  string
}

func newTokenStore() *tokenStore {
	home, _ := os.UserHomeDir()
	return &tokenStore{
		filePath: filepath.Join(home, ".slack-mcp-tokens.json"),
	}
}

func (ts *tokenStore) get() (token, cookie string) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.token, ts.cookie
}

func (ts *tokenStore) set(token, cookie string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.token = token
	ts.cookie = cookie
	ts.updatedAt = time.Now()
}

func (ts *tokenStore) info() (token, cookie, source string, updatedAt time.Time) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.token, ts.cookie, ts.source, ts.updatedAt
}

// loadFromFile reads the persisted token file. Overwrites in-memory values
// only if the file exists and contains valid data.
func (ts *tokenStore) loadFromFile() {
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		return
	}
	var f struct {
		Token     string `json:"token"`
		Cookie    string `json:"cookie"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return
	}
	if f.Token == "" {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.token = f.Token
	ts.cookie = f.Cookie
	ts.source = "file"
	if t, err := time.Parse(time.RFC3339, f.UpdatedAt); err == nil {
		ts.updatedAt = t
	} else {
		ts.updatedAt = time.Now()
	}
}

// saveToFile atomically writes the current token/cookie to disk.
func (ts *tokenStore) saveToFile() error {
	ts.mu.RLock()
	tok := ts.token
	cook := ts.cookie
	ts.mu.RUnlock()

	data, err := json.MarshalIndent(map[string]string{
		"token":      tok,
		"cookie":     cook,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}

	tmp := ts.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, ts.filePath)
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

func extractFromChrome() *chromeTokens {
	result, _ := extractFromChromeWithError()
	return result
}

func extractFromChromeWithError() (*chromeTokens, error) {
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
			if t, err := extractTokenFromLevelDB(profile); err != nil {
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

// extractTokenFromLevelDB reads the xoxc-* token from Chrome's localStorage
// LevelDB. Chrome holds a lock on the DB while running, so we copy the files
// to a temp directory first.
func extractTokenFromLevelDB(profilePath string) (string, error) {
	ldbDir := filepath.Join(profilePath, "Local Storage", "leveldb")
	if _, err := os.Stat(ldbDir); err != nil {
		return "", fmt.Errorf("no LevelDB at %s", ldbDir)
	}

	tmpDir, err := os.MkdirTemp("", "slack-ldb-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	entries, err := os.ReadDir(ldbDir)
	if err != nil {
		return "", fmt.Errorf("reading LevelDB dir: %w", err)
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
		return "", fmt.Errorf("opening LevelDB copy: %w", err)
	}
	defer func() { _ = db.Close() }()

	key := "_https://app.slack.com\x00\x01localConfig_v2"
	val, err := db.Get([]byte(key), nil)
	if err != nil {
		return "", fmt.Errorf("key not found in profile %s", filepath.Base(profilePath))
	}

	// LevelDB values for localStorage have a \x01 prefix byte.
	data := val
	if len(data) > 0 && data[0] == 0x01 {
		data = data[1:]
	}

	var cfg struct {
		Teams map[string]struct {
			Token string `json:"token"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parsing localConfig_v2: %w", err)
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

func tokenStatus(_ context.Context, s *slackIntegration, _ map[string]any) (*mcp.ToolResult, error) {
	_, _, source, updatedAt := s.store.info()
	ageHours := 0.0
	if !updatedAt.IsZero() {
		ageHours = math.Round(time.Since(updatedAt).Hours()*10) / 10
	}
	status := "healthy"
	if ageHours > 10 {
		status = "critical"
	} else if ageHours > 6 {
		status = "warning"
	}

	return jsonResult(map[string]any{
		"status":     status,
		"age_hours":  ageHours,
		"source":     source,
		"updated_at": updatedAt.Format(time.RFC3339),
		"auto_refresh": map[string]any{
			"enabled":  true,
			"interval": "4 hours",
			"platform": runtime.GOOS,
			"requires": "Slack tab open in Chrome (macOS only)",
		},
	})
}

func refreshTokens(_ context.Context, s *slackIntegration, _ map[string]any) (*mcp.ToolResult, error) {
	if runtime.GOOS != "darwin" {
		return &mcp.ToolResult{
			Data:    "Auto-refresh is only available on macOS. On other platforms, manually update token/cookie in config or set SLACK_TOKEN/SLACK_COOKIE env vars.",
			IsError: true,
		}, nil
	}

	if ok := s.tryRefresh(); !ok {
		return &mcp.ToolResult{
			Data:    "Could not extract tokens from Chrome. Make sure Chrome is running with a Slack tab open (app.slack.com) and you are logged in.",
			IsError: true,
		}, nil
	}

	// Verify the new tokens work.
	resp, err := s.getClient().AuthTest()
	if err != nil {
		return errResult(fmt.Errorf("extracted tokens but auth failed: %w", err)), nil
	}
	return jsonResult(map[string]any{
		"status":  "refreshed",
		"user":    resp.User,
		"team":    resp.Team,
		"user_id": resp.UserID,
	})
}
