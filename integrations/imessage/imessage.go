// Package imessage reads the local macOS Messages database (iMessage, SMS, and
// RCS conversations) and sends messages through the Messages app.
package imessage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("imessage", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

var (
	_ mcp.Integration                = (*imessage)(nil)
	_ mcp.PlainTextCredentials       = (*imessage)(nil)
	_ mcp.PlaceholderHints           = (*imessage)(nil)
	_ mcp.OptionalCredentials        = (*imessage)(nil)
	_ mcp.FieldCompactionIntegration = (*imessage)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*imessage)(nil)
)

// runtimeGOOS is a variable so tests can exercise the non-macOS paths.
var runtimeGOOS = runtime.GOOS

type imessage struct {
	mu        sync.RWMutex
	db        *sql.DB
	dbPath    string
	msgCols   map[string]bool
	contacts  *contactBook
	allowSend bool
	allowlist map[string]bool
	runScript scriptRunner
	// confirmWait bounds how long send waits to observe the outgoing message
	// in chat.db before reporting it as queued.
	confirmWait time.Duration
}

// New creates an iMessage integration.
func New() mcp.Integration {
	return &imessage{runScript: runOSAScript, confirmWait: 3 * time.Second}
}

func (m *imessage) Name() string { return "imessage" }

func defaultMessagesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Messages")
}

func defaultContactsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "AddressBook")
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func (m *imessage) Configure(ctx context.Context, creds mcp.Credentials) error {
	dbPath := expandHome(strings.TrimSpace(creds["db_path"]))
	if dbPath == "" {
		if runtimeGOOS != "darwin" {
			return fmt.Errorf("imessage: requires macOS (set db_path to read a copied chat.db on other platforms)")
		}
		dbPath = filepath.Join(defaultMessagesDir(), "chat.db")
	}

	allowSend, err := parseBoolCred("allow_send", creds["allow_send"])
	if err != nil {
		return fmt.Errorf("imessage: %w", err)
	}
	allowlist := map[string]bool{}
	for _, entry := range strings.Split(creds["send_allowlist"], ",") {
		if k := normalizeHandle(entry); k != "" {
			allowlist[k] = true
		}
	}

	contactsDir := expandHome(strings.TrimSpace(creds["contacts_dir"]))
	if contactsDir == "" && runtimeGOOS == "darwin" {
		contactsDir = defaultContactsDir()
	}

	db, err := openDB(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("imessage: %w", permissionHint(dbPath, err))
	}
	cols, err := tableColumns(ctx, db, "message")
	if err == nil {
		_, err = db.ExecContext(ctx, "SELECT 1 FROM message LIMIT 1")
	}
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("imessage: %w", permissionHint(dbPath, err))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		_ = m.db.Close()
	}
	m.db = db
	m.dbPath = dbPath
	m.msgCols = cols
	m.contacts = newContactBook(contactsDir)
	m.allowSend = allowSend
	m.allowlist = allowlist
	if m.runScript == nil {
		m.runScript = runOSAScript
	}
	return nil
}

func parseBoolCred(key, v string) (bool, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, v)
	}
	return b, nil
}

func (m *imessage) Healthy(ctx context.Context) bool {
	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()
	if db == nil {
		return false
	}
	_, err := db.ExecContext(ctx, "SELECT 1 FROM message LIMIT 1")
	return err == nil
}

func (m *imessage) Tools() []mcp.ToolDefinition { return tools }

func (m *imessage) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (m *imessage) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := compactResult.MaxBytes[toolName]
	return n, ok
}

func (m *imessage) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.db == nil {
		return &mcp.ToolResult{Data: "imessage: not configured", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, m, args)
}

func (m *imessage) PlainTextKeys() []string {
	return []string{"db_path", "contacts_dir", "allow_send", "send_allowlist"}
}

func (m *imessage) OptionalKeys() []string {
	return []string{"db_path", "contacts_dir", "allow_send", "send_allowlist"}
}

func (m *imessage) Placeholders() map[string]string {
	return map[string]string{
		"db_path":        "~/Library/Messages/chat.db",
		"contacts_dir":   "~/Library/Application Support/AddressBook",
		"allow_send":     "false",
		"send_allowlist": "+15551234567, friend@example.com",
	}
}

// msgCol returns the column expression for an optional message column, or
// the fallback literal when the running macOS schema lacks it.
func (m *imessage) msgCol(name, fallback string) string {
	if m.msgCols[name] {
		return "m." + name
	}
	return fallback
}

type handlerFunc func(context.Context, *imessage, map[string]any) (*mcp.ToolResult, error)
