package audit

import (
	"context"
	"testing"
)

func TestNoop_Log(t *testing.T) {
	var l Noop
	l.Log(context.Background(), Entry{TenantID: "t1", ToolName: "test"})
	if err := l.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
}

func TestHashArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "nil args", args: nil, want: ""},
		{name: "empty args", args: map[string]any{}, want: ""},
		{name: "with args", args: map[string]any{"key": "value"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashArgs(tt.args)
			if tt.want != "" && got != tt.want {
				t.Errorf("HashArgs() = %q, want %q", got, tt.want)
			}
			if tt.name == "with args" {
				if got == "" {
					t.Error("HashArgs() returned empty for non-empty args")
				}
				if len(got) != 64 {
					t.Errorf("HashArgs() len = %d, want 64 (sha256 hex)", len(got))
				}
				got2 := HashArgs(tt.args)
				if got != got2 {
					t.Error("HashArgs() not deterministic")
				}
			}
		})
	}
}
