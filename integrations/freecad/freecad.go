package freecad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var (
	_ mcp.Integration                = (*freecad)(nil)
	_ mcp.FieldCompactionIntegration = (*freecad)(nil)
	_ mcp.PlainTextCredentials       = (*freecad)(nil)
)

type freecad struct {
	binary      string // path to freecad binary
	dataDir     string // working directory for FreeCAD files
	mu          sync.Mutex
	execTimeout time.Duration
}

func New() mcp.Integration {
	return &freecad{
		execTimeout: 60 * time.Second,
	}
}

func (f *freecad) Name() string { return "freecad" }

func (f *freecad) PlainTextKeys() []string {
	return []string{"binary_path", "data_dir"}
}

func (f *freecad) Configure(_ context.Context, creds mcp.Credentials) error {
	f.binary = creds["binary_path"]
	if f.binary == "" {
		// Auto-detect from PATH
		path, err := exec.LookPath("freecad")
		if err != nil {
			return fmt.Errorf("freecad: binary_path not set and freecad not found in PATH")
		}
		f.binary = path
	}
	f.dataDir = creds["data_dir"]
	if f.dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("freecad: data_dir not set and cannot determine home directory: %w", err)
		}
		f.dataDir = filepath.Join(home, "FreeCAD")
	}
	if err := os.MkdirAll(f.dataDir, 0o750); err != nil {
		return fmt.Errorf("freecad: cannot create data_dir %q: %w", f.dataDir, err)
	}
	return nil
}

func (f *freecad) Healthy(ctx context.Context) bool {
	if f.binary == "" {
		return false
	}
	_, err := f.runScript(ctx, `
import FreeCAD
import json
print(json.dumps({"version": FreeCAD.Version()[0] + "." + FreeCAD.Version()[1] + "." + FreeCAD.Version()[2]}))
`)
	return err == nil
}

func (f *freecad) Tools() []mcp.ToolDefinition {
	return tools
}

func (f *freecad) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, f, args)
}

func (f *freecad) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- Python execution helpers ---

func (f *freecad) runScript(ctx context.Context, script string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, f.execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.binary, "--console") // #nosec G204 -- binary path comes from trusted config credentials, not user input
	cmd.Stdin = strings.NewReader(script + "\nexit()\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("freecad script error: %s", stderrStr)
		}
		return "", fmt.Errorf("freecad execution error: %w", err)
	}

	// Extract the last JSON line from output (FreeCAD prints banner + progress noise)
	return extractJSON(stdout.String()), nil
}

// extractJSON finds the last line that looks like JSON output from FreeCAD console output.
// FreeCAD prints banners, progress bars, and other noise that we need to skip.
func extractJSON(output string) string {
	lines := strings.Split(output, "\n")
	// Walk backwards to find the last JSON line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		if (strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}")) ||
			(strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
			return line
		}
	}
	// No JSON found — return trimmed output
	return strings.TrimSpace(output)
}

// filePath resolves a file path relative to the data directory.
// If the path is absolute, it's used as-is.
func (f *freecad) filePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(f.dataDir, name)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error)

// scriptResult runs a Python script and returns the output as a ToolResult.
func scriptResult(ctx context.Context, f *freecad, script string) (*mcp.ToolResult, error) {
	out, err := f.runScript(ctx, script)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: out}, nil
}

// jsonScriptResult runs a Python script that outputs JSON and verifies it's valid.
func jsonScriptResult(ctx context.Context, f *freecad, script string) (*mcp.ToolResult, error) {
	out, err := f.runScript(ctx, script)
	if err != nil {
		return mcp.ErrResult(err)
	}
	// Validate JSON
	if json.Valid([]byte(out)) {
		return &mcp.ToolResult{Data: out}, nil
	}
	return &mcp.ToolResult{Data: out}, nil
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Version / info
	mcp.ToolName("freecad_get_version"): getVersion,

	// Document management
	mcp.ToolName("freecad_list_documents"):  listDocuments,
	mcp.ToolName("freecad_create_document"): createDocument,
	mcp.ToolName("freecad_open_document"):   openDocument,
	mcp.ToolName("freecad_get_document"):    getDocument,

	// Objects
	mcp.ToolName("freecad_list_objects"):   listObjects,
	mcp.ToolName("freecad_get_object"):     getObject,
	mcp.ToolName("freecad_delete_object"):  deleteObject,
	mcp.ToolName("freecad_set_placement"):  setPlacement,
	mcp.ToolName("freecad_get_properties"): getProperties,
	mcp.ToolName("freecad_set_property"):   setProperty,

	// Primitives
	mcp.ToolName("freecad_create_box"):      createBox,
	mcp.ToolName("freecad_create_cylinder"): createCylinder,
	mcp.ToolName("freecad_create_sphere"):   createSphere,
	mcp.ToolName("freecad_create_cone"):     createCone,
	mcp.ToolName("freecad_create_torus"):    createTorus,

	// Boolean operations
	mcp.ToolName("freecad_boolean_cut"):    booleanCut,
	mcp.ToolName("freecad_boolean_fuse"):   booleanFuse,
	mcp.ToolName("freecad_boolean_common"): booleanCommon,

	// Shape operations
	mcp.ToolName("freecad_fillet"):  fillet,
	mcp.ToolName("freecad_chamfer"): chamfer,
	mcp.ToolName("freecad_extrude"): extrude,
	mcp.ToolName("freecad_mirror"):  mirror,

	// Measurement / analysis
	mcp.ToolName("freecad_measure_shape"):  measureShape,
	mcp.ToolName("freecad_check_geometry"): checkGeometry,
	mcp.ToolName("freecad_bounding_box"):   boundingBox,

	// Export / import
	mcp.ToolName("freecad_export_step"): exportSTEP,
	mcp.ToolName("freecad_export_stl"):  exportSTL,
	mcp.ToolName("freecad_export_brep"): exportBRep,
	mcp.ToolName("freecad_import_file"): importFile,

	// Python scripting
	mcp.ToolName("freecad_run_script"): runUserScript,
}
