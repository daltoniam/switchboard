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

// ParseViewArgs is the request-boundary parser. The pipeline consumers
// (processResult, processViewsResult, applyResultProcessing) take
// compact.ViewArgs by value — a struct, not a map — so the old leak
// (caller passes nil and silently dispatches default view) is gone at
// the type level. These tests pin the parse boundary's behavior across
// the legitimate inputs (nil, empty, well-typed, wrong-typed).
func TestParseViewArgs_NilArgs_ReturnsZeroValue(t *testing.T) {
	// Nil args at the parse boundary is legitimate — means "no selection
	// from the LLM, use defaults downstream". This is the single most
	// important case: it's why we lifted parsing to a typed boundary in
	// the first place. Nil and empty map are now equivalent here.
	got := compact.ParseViewArgs(nil)
	if got.View != "" || got.Format != "" {
		t.Errorf("ParseViewArgs(nil) populated fields: %+v", got)
	}
	if got.Err() != nil {
		t.Errorf("ParseViewArgs(nil) err = %v, want nil", got.Err())
	}
}

func TestParseViewArgs_EmptyMap_ReturnsZeroValue(t *testing.T) {
	got := compact.ParseViewArgs(map[string]any{})
	if got.View != "" || got.Format != "" {
		t.Errorf("ParseViewArgs({}) populated fields: %+v", got)
	}
	if got.Err() != nil {
		t.Errorf("ParseViewArgs({}) err = %v, want nil", got.Err())
	}
}

func TestParseViewArgs_WellTypedSelection_PopulatesFields(t *testing.T) {
	got := compact.ParseViewArgs(map[string]any{
		"view":   "full",
		"format": "markdown",
	})
	if got.View != "full" {
		t.Errorf("View = %q, want %q", got.View, "full")
	}
	if got.Format != "markdown" {
		t.Errorf("Format = %q, want %q", got.Format, "markdown")
	}
	if got.Err() != nil {
		t.Errorf("Err = %v, want nil", got.Err())
	}
}

func TestParseViewArgs_NonStringView_EmbedsError(t *testing.T) {
	// LLMs can send weird types. Boundary catches it; downstream sees
	// Err() != nil and emits a view envelope rather than dispatching.
	got := compact.ParseViewArgs(map[string]any{"view": 123})
	if got.Err() == nil {
		t.Fatal("expected Err() != nil for non-string view")
	}
	if got.View != "" {
		t.Errorf("View should not be populated on type error, got %q", got.View)
	}
}

func TestParseViewArgs_NonStringFormat_EmbedsError(t *testing.T) {
	got := compact.ParseViewArgs(map[string]any{
		"view":   "full",
		"format": true,
	})
	if got.Err() == nil {
		t.Fatal("expected Err() != nil for non-string format")
	}
}

// Err is unexported state on ViewArgs — the only way to set it is through
// ParseViewArgs. Pin that contract: a zero-value ViewArgs has no error.
func TestViewArgs_ZeroValue_HasNoError(t *testing.T) {
	var v compact.ViewArgs
	if v.Err() != nil {
		t.Errorf("zero ViewArgs.Err() = %v, want nil", v.Err())
	}
}
