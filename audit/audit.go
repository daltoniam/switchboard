package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// Entry records a single tool execution event.
type Entry struct {
	TenantID    string
	UserID      string
	ToolName    string
	Integration string
	ArgsHash    string
	IsError     bool
	Timestamp   time.Time
}

// Logger records tool execution events for compliance and debugging.
type Logger interface {
	Log(ctx context.Context, entry Entry)
	Close() error
}

// Noop is a no-op audit logger used in local mode.
type Noop struct{}

func (Noop) Log(context.Context, Entry) {}
func (Noop) Close() error               { return nil }

// HashArgs returns a SHA-256 hex digest of the JSON-serialized args.
// Returns empty string on marshal failure.
func HashArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	data, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
