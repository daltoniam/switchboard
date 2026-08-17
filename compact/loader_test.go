package compact_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"

	"github.com/daltoniam/switchboard/compact"
)

func TestLoad_ValidYAML(t *testing.T) {
	data := []byte(`version: 1
tools:
  linear_list_issues:
    spec:
      - issues.nodes[].id
      - issues.nodes[].title
    max_bytes: 50000
  linear_get_issue:
    spec:
      - issue.id
`)
	res, err := compact.Load(data, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs[mcp.ToolName("linear_list_issues")]) != 2 {
		t.Fatalf("want 2 fields, got %d", len(res.Specs[mcp.ToolName("linear_list_issues")]))
	}
	if got := res.MaxBytes[mcp.ToolName("linear_list_issues")]; got != 50000 {
		t.Fatalf("want max_bytes=50000, got %d", got)
	}
	if _, capped := res.MaxBytes[mcp.ToolName("linear_get_issue")]; capped {
		t.Fatal("linear_get_issue has no max_bytes, should not appear in map")
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("want 0 warnings, got %v", res.Warnings)
	}
}

func TestLoad_LenientSkipsBadSpec(t *testing.T) {
	data := []byte(`version: 1
tools:
  good_tool:
    spec: [valid.field]
  bad_tool:
    spec: [""]
`)
	res, _ := compact.Load(data, compact.Options{Strict: false})
	if _, ok := res.Specs[mcp.ToolName("good_tool")]; !ok {
		t.Fatal("good_tool missing")
	}
	if _, ok := res.Specs[mcp.ToolName("bad_tool")]; ok {
		t.Fatal("bad_tool should be skipped")
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("want 1 warning, got %d", len(res.Warnings))
	}
}

func TestLoad_EmptyToolsIsValid(t *testing.T) {
	data := []byte(`version: 1
tools: {}
`)
	res, err := compact.Load(data, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs) != 0 {
		t.Fatal("want 0 specs")
	}
	if len(res.MaxBytes) != 0 {
		t.Fatal("want 0 max_bytes entries")
	}
}

func TestLoad_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "missing version",
			yaml: `tools: { x: { spec: [a] } }`,
		},
		{
			name: "invalid spec syntax in strict mode",
			yaml: `version: 1
tools:
  bad_tool:
    spec: [""]
`,
		},
		{
			name: "negative max_bytes",
			yaml: `version: 1
tools:
  t:
    spec: [a.b]
    max_bytes: -1
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compact.Load([]byte(tc.yaml), compact.Options{Strict: true})
			if err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

func TestLoadWithOverlay_PerToolMerge(t *testing.T) {
	dir := t.TempDir()
	overlay := `version: 1
tools:
  shared_tool:
    spec:
      - override.field
  override_only:
    spec:
      - new.field
`
	if err := os.WriteFile(filepath.Join(dir, "linear.yaml"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHBOARD_COMPACT_DIR", dir)

	embedded := []byte(`version: 1
tools:
  shared_tool:
    spec:
      - embedded.field
  embedded_only:
    spec:
      - embedded.other
`)
	res, err := compact.LoadWithOverlay("linear", embedded, compact.Options{Strict: false})
	if err != nil {
		t.Fatal(err)
	}

	// shared_tool overridden: compare via re-parse
	want, err := mcp.ParseCompactSpecs([]string{"override.field"})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Specs[mcp.ToolName("shared_tool")]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("shared_tool: want %v, got %v", want, got)
	}

	// embedded_only preserved
	if _, ok := res.Specs[mcp.ToolName("embedded_only")]; !ok {
		t.Error("embedded_only missing")
	}

	// override_only present + warning emitted
	if _, ok := res.Specs[mcp.ToolName("override_only")]; !ok {
		t.Error("override_only missing")
	}
	if len(res.Warnings) != 1 {
		t.Errorf("want 1 warning for override-only tool, got %d", len(res.Warnings))
	}
}

func TestLoadWithOverlay_NoEnvFallback(t *testing.T) {
	os.Unsetenv("SWITCHBOARD_COMPACT_DIR") //nolint:errcheck
	embedded := []byte(`version: 1
tools:
  foo:
    spec:
      - a.b
`)
	res, err := compact.LoadWithOverlay("linear", embedded, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs) != 1 {
		t.Fatal("want 1 spec from embedded")
	}
}

func TestLoadWithOverlay_MissingFileFallsThrough(t *testing.T) {
	dir := t.TempDir() // empty dir
	t.Setenv("SWITCHBOARD_COMPACT_DIR", dir)
	embedded := []byte(`version: 1
tools:
  foo:
    spec:
      - a.b
`)
	res, err := compact.LoadWithOverlay("linear", embedded, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs) != 1 {
		t.Fatal("want 1 spec from embedded")
	}
	if len(res.Warnings) != 0 {
		t.Fatal("absence of overlay file is not a warning")
	}
}

func TestLoadWithOverlay_MaxBytesMerge(t *testing.T) {
	cases := []struct {
		name            string
		embedded        string
		overlay         string
		wantMaxBytes    int  // 0 means absent
		wantMaxBytesSet bool // whether key should exist in the map
	}{
		{
			name: "overlay adds cap to uncapped embedded tool",
			embedded: `version: 1
tools:
  foo:
    spec: [a.b]
`,
			overlay: `version: 1
tools:
  foo:
    spec: [a.b]
    max_bytes: 5000
`,
			wantMaxBytes:    5000,
			wantMaxBytesSet: true,
		},
		{
			name: "overlay removes cap by omitting max_bytes",
			embedded: `version: 1
tools:
  foo:
    spec: [a.b]
    max_bytes: 5000
`,
			overlay: `version: 1
tools:
  foo:
    spec: [a.b]
`,
			wantMaxBytes:    0,
			wantMaxBytesSet: false,
		},
		{
			name: "overlay changes max_bytes value",
			embedded: `version: 1
tools:
  foo:
    spec: [a.b]
    max_bytes: 5000
`,
			overlay: `version: 1
tools:
  foo:
    spec: [a.b]
    max_bytes: 10000
`,
			wantMaxBytes:    10000,
			wantMaxBytesSet: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "linear.yaml"), []byte(tc.overlay), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("SWITCHBOARD_COMPACT_DIR", dir)
			res, err := compact.LoadWithOverlay("linear", []byte(tc.embedded), compact.Options{Strict: true})
			if err != nil {
				t.Fatal(err)
			}
			got, ok := res.MaxBytes[mcp.ToolName("foo")]
			if ok != tc.wantMaxBytesSet {
				t.Fatalf("MaxBytes presence: want %v, got %v", tc.wantMaxBytesSet, ok)
			}
			if got != tc.wantMaxBytes {
				t.Fatalf("MaxBytes value: want %d, got %d", tc.wantMaxBytes, got)
			}
		})
	}
}

func TestLoadWithOverlay_StrictErrorsOnBadOverlay(t *testing.T) {
	dir := t.TempDir()
	bad := []byte(`version: 99
tools: {}
`)
	if err := os.WriteFile(filepath.Join(dir, "linear.yaml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHBOARD_COMPACT_DIR", dir)
	embedded := []byte(`version: 1
tools:
  foo:
    spec: [a.b]
`)
	_, err := compact.LoadWithOverlay("linear", embedded, compact.Options{Strict: true})
	if err == nil {
		t.Fatal("want error in strict mode for bad overlay")
	}
}

func TestLoadWithOverlay_LenientWarnsOnBadOverlay(t *testing.T) {
	dir := t.TempDir()
	bad := []byte(`version: 99
tools: {}
`)
	if err := os.WriteFile(filepath.Join(dir, "linear.yaml"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHBOARD_COMPACT_DIR", dir)
	embedded := []byte(`version: 1
tools:
  foo:
    spec: [a.b]
`)
	res, err := compact.LoadWithOverlay("linear", embedded, compact.Options{Strict: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("want warning for bad overlay in lenient mode")
	}
	if _, ok := res.Specs[mcp.ToolName("foo")]; !ok {
		t.Fatal("base specs should be preserved when overlay fails")
	}
}

// ── Multi-view loading ──────────────────────────────────────────────

func TestLoad_MultiView_HappyPath(t *testing.T) {
	data := []byte(`version: 1
tools:
  notion_get_page_content:
    views:
      toc:
        spec:
          - page.title
          - blocks[].properties.title
        hint: "Just titles."
        formats: [json]
      full:
        spec:
          - page.id
          - page.title
          - blocks[].id
          - blocks[].properties
        hint: "Whole tree."
        formats: [json, markdown]
        max_bytes: 50000
    default:
      view: toc
      format: json
`)
	res, err := compact.Load(data, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}

	tool := mcp.ToolName("notion_get_page_content")

	vs, ok := res.Views[tool]
	if !ok {
		t.Fatal("Views[notion_get_page_content] missing")
	}

	if vs.Default.View != "toc" || vs.Default.Format != compact.FormatJSON {
		t.Errorf("default mismatch: got view=%q format=%q", vs.Default.View, vs.Default.Format)
	}

	if _, ok := vs.Views["toc"]; !ok {
		t.Error("toc view missing")
	}
	if _, ok := vs.Views["full"]; !ok {
		t.Error("full view missing")
	}

	if vs.Renderers["toc"][compact.FormatJSON] == nil {
		t.Error("toc+json renderer not resolved")
	}
	if vs.Renderers["full"][compact.FormatMarkdown] == nil {
		t.Error("full+markdown renderer not resolved")
	}
	if _, has := vs.Renderers["toc"][compact.FormatMarkdown]; has {
		t.Error("toc should NOT have markdown renderer (not declared)")
	}

	// Back-compat: Specs and MaxBytes populated from default view.
	if got := res.Specs[tool]; len(got) == 0 {
		t.Error("Specs[tool] should mirror default-view spec for back-compat")
	}
	if got := res.MaxBytes[tool]; got != 0 {
		t.Errorf("toc has no max_bytes; expected 0 in flat map, got %d", got)
	}
}

func TestLoad_MultiView_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "spec and views set together",
			yaml: `version: 1
tools:
  bad:
    spec: [a.b]
    views:
      v:
        spec: [c.d]
        formats: [json]
    default:
      view: v
      format: json
`,
		},
		{
			name: "views without default",
			yaml: `version: 1
tools:
  bad:
    views:
      v:
        spec:
          - a.b
        formats: [json]
`,
		},
		{
			name: "default.view does not exist",
			yaml: `version: 1
tools:
  bad:
    views:
      v:
        spec:
          - a.b
        formats: [json]
    default:
      view: nonexistent
      format: json
`,
		},
		{
			name: "default.format not in default view's formats",
			yaml: `version: 1
tools:
  bad:
    views:
      v:
        spec:
          - a.b
        formats: [json]
    default:
      view: v
      format: markdown
`,
		},
		{
			name: "format with no framework default and no custom renderer",
			yaml: `version: 1
tools:
  bad:
    views:
      v:
        spec:
          - a.b
        formats: [json, text]
    default:
      view: v
      format: json
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compact.Load([]byte(tc.yaml), compact.Options{Strict: true})
			if err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

func TestLoad_MultiView_CustomRendererResolves(t *testing.T) {
	// Adapter provides a custom renderer for (tool, view, format) — load succeeds.
	customCalled := false
	customRenderer := func(projected any) ([]byte, error) {
		customCalled = true
		return []byte("custom"), nil
	}

	data := []byte(`version: 1
tools:
  ok:
    views:
      v:
        spec: [a.b]
        formats: [json, text]
    default:
      view: v
      format: json
`)
	res, err := compact.Load(data, compact.Options{
		Strict: true,
		Renderers: map[compact.RenderKey]compact.Renderer{
			{Tool: mcp.ToolName("ok"), View: compact.ViewName("v"), Format: compact.FormatText}: customRenderer,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	vs := res.Views[mcp.ToolName("ok")]
	r := vs.Renderers["v"][compact.FormatText]
	if r == nil {
		t.Fatal("custom text renderer not registered")
	}
	if _, err := r(nil); err != nil || !customCalled {
		t.Errorf("custom renderer not invoked through the registry: called=%v err=%v", customCalled, err)
	}
}

// ── Framework default renderers ──────────────────────────────────────

func TestFrameworkRenderer_JSONRoundtrip(t *testing.T) {
	data := []byte(`version: 1
tools:
  t:
    views:
      v:
        spec: [a.b]
        formats: [json]
    default:
      view: v
      format: json
`)
	res, err := compact.Load(data, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	r := res.Views[mcp.ToolName("t")].Renderers["v"][compact.FormatJSON]

	got, err := r(map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"k":"v"}` {
		t.Errorf("json renderer output: got %s", got)
	}
}

func TestFrameworkRenderer_MarkdownShapes(t *testing.T) {
	data := []byte(`version: 1
tools:
  t:
    views:
      v:
        spec: [a]
        formats: [markdown]
    default:
      view: v
      format: markdown
`)
	res, _ := compact.Load(data, compact.Options{Strict: true})
	r := res.Views[mcp.ToolName("t")].Renderers["v"][compact.FormatMarkdown]

	cases := []struct {
		name     string
		input    any
		contains string
	}{
		{"flat map gets definition list", map[string]any{"title": "Foo"}, "- **title**: Foo"},
		{"array of homogeneous maps becomes table", []any{map[string]any{"a": "1"}, map[string]any{"a": "2"}}, "| a |"},
		{"empty array", []any{}, "_(empty)_"},
		{"nil value", nil, "_(none)_"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := r(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tc.contains) {
				t.Errorf("expected output to contain %q, got: %s", tc.contains, out)
			}
		})
	}
}

func TestLoadWithOverlay_MultiView_WholeToolReplacement(t *testing.T) {
	dir := t.TempDir()
	overlay := `version: 1
tools:
  notion_get_page_content:
    views:
      summary:
        spec:
          - page.id
          - page.title
        formats: [json]
    default:
      view: summary
      format: json
`
	if err := os.WriteFile(filepath.Join(dir, "notion.yaml"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHBOARD_COMPACT_DIR", dir)

	embedded := []byte(`version: 1
tools:
  notion_get_page_content:
    views:
      toc:
        spec:
          - page.title
        formats: [json]
      full:
        spec:
          - page.id
          - page.title
          - blocks[].id
        formats: [json, markdown]
    default:
      view: toc
      format: json
`)
	res, err := compact.LoadWithOverlay("notion", embedded, compact.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}

	vs := res.Views[mcp.ToolName("notion_get_page_content")]
	// Overlay's whole-tool replacement: only "summary" exists, not the embedded "toc" or "full".
	if _, has := vs.Views["summary"]; !has {
		t.Error("summary view (from overlay) missing")
	}
	if _, has := vs.Views["toc"]; has {
		t.Error("toc view (embedded) should be GONE — whole-tool replacement")
	}
	if _, has := vs.Views["full"]; has {
		t.Error("full view (embedded) should be GONE — whole-tool replacement")
	}
	if vs.Default.View != "summary" {
		t.Errorf("default should be from overlay: got %q want summary", vs.Default.View)
	}
}

func TestLoad_MultiView_LenientSkipsBadView(t *testing.T) {
	// In lenient mode, a bad view (e.g. negative max_bytes) is skipped via warning.
	data := []byte(`version: 1
tools:
  partly_bad:
    views:
      good:
        spec: [a.b]
        formats: [json]
      bad:
        spec: [c.d]
        formats: [json]
        max_bytes: -1
    default:
      view: good
      format: json
`)
	res, err := compact.Load(data, compact.Options{Strict: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.Views[mcp.ToolName("partly_bad")]; ok {
		t.Fatal("partly_bad should be skipped entirely (whole-tool failure on any bad view)")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected a warning for the bad view")
	}
}
