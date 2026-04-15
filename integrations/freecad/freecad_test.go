package freecad

import (
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "freecad", i.Name())
}

func TestConfigure_AutoDetect(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	// May succeed if freecad is in PATH, may fail if not
	if err != nil {
		assert.Contains(t, err.Error(), "freecad")
	}
}

func TestConfigure_WithBinaryPath(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"binary_path": "/nonexistent/freecad",
		"data_dir":    t.TempDir(),
	})
	// May fail if bridge not found, which is expected in CI
	if err != nil {
		assert.Contains(t, err.Error(), "freecad")
	}
}

func TestConfigure_WithDataDir(t *testing.T) {
	fc := &freecad{}
	dir := t.TempDir()
	err := fc.Configure(context.Background(), mcp.Credentials{
		"binary_path": "/usr/bin/freecad",
		"data_dir":    dir,
	})
	// May fail if bridge not found, which is expected in CI
	if err != nil {
		assert.Contains(t, err.Error(), "freecad")
	} else {
		assert.Equal(t, dir, fc.dataDir)
	}
}

func TestPlainTextKeys(t *testing.T) {
	fc := &freecad{}
	keys := fc.PlainTextKeys()
	assert.Contains(t, keys, "binary_path")
	assert.Contains(t, keys, "data_dir")
}

func TestTools(t *testing.T) {
	i := New()
	tls := i.Tools()
	assert.NotEmpty(t, tls)

	for _, tool := range tls {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHavePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "freecad_", "tool %s missing freecad_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "field compaction spec %s has no dispatch handler", name)
	}
}

func TestCompactSpec(t *testing.T) {
	fc := &freecad{}
	fields, ok := fc.CompactSpec("freecad_list_objects")
	assert.True(t, ok)
	assert.NotEmpty(t, fields)

	_, ok = fc.CompactSpec("freecad_nonexistent")
	assert.False(t, ok)
}

func TestHealthy_NotConfigured(t *testing.T) {
	fc := &freecad{}
	assert.False(t, fc.Healthy(context.Background()))
}

func TestFilePath(t *testing.T) {
	fc := &freecad{dataDir: "/home/user/FreeCAD"}

	t.Run("relative path", func(t *testing.T) {
		assert.Equal(t, "/home/user/FreeCAD/test.FCStd", fc.filePath("test.FCStd"))
	})

	t.Run("absolute path", func(t *testing.T) {
		assert.Equal(t, "/tmp/test.FCStd", fc.filePath("/tmp/test.FCStd"))
	})
}

func TestOptFloat(t *testing.T) {
	t.Run("with value", func(t *testing.T) {
		got := optFloat(map[string]any{"x": float64(42.5)}, "x", 10)
		assert.Equal(t, 42.5, got)
	})

	t.Run("missing uses default", func(t *testing.T) {
		got := optFloat(map[string]any{}, "x", 10)
		assert.Equal(t, float64(10), got)
	})

	t.Run("zero uses default", func(t *testing.T) {
		got := optFloat(map[string]any{"x": float64(0)}, "x", 10)
		assert.Equal(t, float64(10), got)
	})
}

// --- Handler validation tests (required arg checking) ---

func TestCreateDocument_MissingName(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_create_document", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "name is required")
}

func TestOpenDocument_MissingFilePath(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_open_document", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "file_path is required")
}

func TestGetObject_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}

	result, err := fc.Execute(context.Background(), "freecad_get_object", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "file_path is required")

	result, err = fc.Execute(context.Background(), "freecad_get_object", map[string]any{"file_path": "test.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "object_name is required")
}

func TestDeleteObject_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_delete_object", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestExportSTEP_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_export_step", map[string]any{"file_path": "test.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "output_path is required")
}

func TestExportSTL_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_export_stl", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "file_path is required")
}

func TestRunScript_MissingScript(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_run_script", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "script is required")
}

func TestBooleanCut_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_boolean_cut", map[string]any{"file_path": "test.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "base is required")
}

func TestFillet_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_fillet", map[string]any{"file_path": "test.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "object_name is required")
}

func TestSetProperty_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_set_property", map[string]any{
		"file_path":   "test.FCStd",
		"object_name": "Box",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "property is required")
}

func TestMeasureShape_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_measure_shape", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "file_path is required")
}

func TestCheckGeometry_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_check_geometry", map[string]any{"file_path": "t.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "object_name is required")
}

func TestImportFile_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_import_file", map[string]any{"file_path": "t.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "import_path is required")
}

func TestMirror_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_mirror", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "file_path is required")
}

func TestExtrude_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_extrude", map[string]any{"file_path": "t.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "object_name is required")
}

func TestChamfer_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_chamfer", map[string]any{"file_path": "t.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "object_name is required")
}

func TestSetPlacement_MissingArgs(t *testing.T) {
	fc := &freecad{binary: "freecad", dataDir: t.TempDir()}
	result, err := fc.Execute(context.Background(), "freecad_set_placement", map[string]any{"file_path": "t.FCStd"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "object_name is required")
}
