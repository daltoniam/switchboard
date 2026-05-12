package compactyaml_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	mcp "github.com/daltoniam/switchboard"

	"github.com/daltoniam/switchboard/compactyaml"
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
	res, err := compactyaml.Load(data, compactyaml.Options{Strict: true})
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
	res, _ := compactyaml.Load(data, compactyaml.Options{Strict: false})
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
	res, err := compactyaml.Load(data, compactyaml.Options{Strict: true})
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
			_, err := compactyaml.Load([]byte(tc.yaml), compactyaml.Options{Strict: true})
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
	res, err := compactyaml.LoadWithOverlay("linear", embedded, compactyaml.Options{Strict: false})
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
	res, err := compactyaml.LoadWithOverlay("linear", embedded, compactyaml.Options{Strict: true})
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
	res, err := compactyaml.LoadWithOverlay("linear", embedded, compactyaml.Options{Strict: true})
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
			res, err := compactyaml.LoadWithOverlay("linear", []byte(tc.embedded), compactyaml.Options{Strict: true})
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
	_, err := compactyaml.LoadWithOverlay("linear", embedded, compactyaml.Options{Strict: true})
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
	res, err := compactyaml.LoadWithOverlay("linear", embedded, compactyaml.Options{Strict: false})
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
