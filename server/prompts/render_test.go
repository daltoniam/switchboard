package prompts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// render() must strip exactly one trailing newline so embedded .md.tmpl files
// (which end in \n by editor default) produce strings byte-equal to what the
// SDK would have received from a Go raw-string literal. Tested through every
// Meta accessor — also verifies each accessor's template name in meta.go
// resolves against a real file.
func TestRender_TrimsTrailingNewline(t *testing.T) {
	cases := []struct {
		name string
		fn   func() string
	}{
		{"Meta.Search", Meta.Search},
		{"Meta.Execute", Meta.Execute},
		{"Meta.Session", Meta.Session},
		{"Meta.History", Meta.History},
		{"Meta.Pin", Meta.Pin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			require.NotEmpty(t, got)
			require.False(t, strings.HasSuffix(got, "\n"),
				"render() left trailing newline in %s output", tc.name)
		})
	}
}

// render() must fail loudly on a missing template. A silent empty-string
// return would mean a typo in an accessor's template name produces a blank
// Tool.Description at the wire boundary with no compile-time or test-time
// signal — this is the "total seam stays loud" guarantee.
func TestRender_PanicsOnMissingTemplate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on missing template, got nil")
		}
	}()
	render(metaTmpl, "nonexistent.md.tmpl", nil)
}

// execute.md.tmpl contains 9 literal "}}" sequences inside nested JSON
// examples. With default template delims "{{" "}}", init would have panicked
// and the package wouldn't have loaded. Custom "<%" "%>" delims are
// load-bearing for this codebase; this test confirms they pass "}}" through
// to the output verbatim.
func TestMetaTmpl_CustomDelimsHandleNestedBraces(t *testing.T) {
	got := render(metaTmpl, "execute.md.tmpl", nil)
	require.Contains(t, got, "}}",
		"execute.md.tmpl should contain literal }} from JSON examples; custom delims must be active")
}

// metaTmpl and dynamicTmpl are separate parsers backed by separate embed.FS
// values. A template name that exists only in dynamic/ must not resolve via
// metaTmpl — a future file in meta/ sharing a base name with one in dynamic/
// would otherwise collide silently and cross-link the namespaces.
func TestParsers_AreIsolated(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when looking up dynamic template via metaTmpl")
		}
	}()
	render(metaTmpl, "search_summary.md.tmpl", nil)
}
