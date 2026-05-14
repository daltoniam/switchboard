package compact_test

import (
	"testing"

	"github.com/daltoniam/switchboard/compact"
)

// ReservedArgs is the single source of truth that bridges the two view-dispatch
// boundaries: the parser (parseViewSelection reads args[ArgView]/args[ArgFormat])
// and the MCP arg validator (skips these keys when a tool exposes a ViewSet).
// Drift between the two would either reject valid input or silently miss a
// reserved arg, so the values are pinned here.
func TestReservedArgs_ReturnsViewAndFormat(t *testing.T) {
	got := compact.ReservedArgs()
	want := []string{"view", "format"}

	if len(got) != len(want) {
		t.Fatalf("ReservedArgs() = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ReservedArgs()[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestReservedArgs_ConstantsMatch(t *testing.T) {
	if compact.ArgView != "view" {
		t.Errorf("ArgView = %q, want %q", compact.ArgView, "view")
	}
	if compact.ArgFormat != "format" {
		t.Errorf("ArgFormat = %q, want %q", compact.ArgFormat, "format")
	}
}
