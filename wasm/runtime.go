package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	mcp "github.com/daltoniam/switchboard"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime manages the wazero WebAssembly runtime and compiles/instantiates modules.
type Runtime struct {
	rt wazero.Runtime
}

// NewRuntime creates a new wazero runtime with WASI support and host functions.
func NewRuntime(ctx context.Context) (*Runtime, error) {
	rt := wazero.NewRuntime(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	_, err := rt.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithFunc(hostHTTPRequest).
		WithParameterNames("ptr_size").
		Export("host_http_request").
		NewFunctionBuilder().
		WithFunc(hostLog).
		WithParameterNames("ptr", "size").
		Export("host_log").
		Instantiate(ctx)
	if err != nil {
		rt.Close(ctx) //nolint:errcheck
		return nil, fmt.Errorf("wasm: instantiate host module: %w", err)
	}

	return &Runtime{rt: rt}, nil
}

// LoadModule compiles and instantiates a WASM binary, returning a Module
// that implements mcp.Integration.
func (r *Runtime) LoadModule(ctx context.Context, wasmBytes []byte) (*Module, error) {
	compiled, err := r.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("wasm: compile module: %w", err)
	}

	mod, err := r.rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithStartFunctions().
		WithName(""))
	if err != nil {
		return nil, fmt.Errorf("wasm: instantiate module: %w", err)
	}

	// Standard Go wasip1 modules export _rt0_wasm_wasip1 as the entry point.
	// We call it manually to initialize the Go runtime without calling main().
	// TinyGo modules export _initialize instead.
	initFn := mod.ExportedFunction("_rt0_wasm_wasip1")
	if initFn == nil {
		initFn = mod.ExportedFunction("_initialize")
	}
	if initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			mod.Close(ctx) //nolint:errcheck
			return nil, fmt.Errorf("wasm: initialize module: %w", err)
		}
	}

	m := &Module{mod: mod}
	if err := m.resolveExports(); err != nil {
		mod.Close(ctx) //nolint:errcheck
		return nil, err
	}
	m.loadCompactSpecs(ctx)

	return m, nil
}

// Close releases all runtime resources.
func (r *Runtime) Close(ctx context.Context) error {
	return r.rt.Close(ctx)
}

// Module wraps an instantiated WASM module and implements mcp.Integration.
type Module struct {
	mod          api.Module
	nameOverride string
	fnName       api.Function
	fnTools      api.Function
	fnConfig     api.Function
	fnExec       api.Function
	fnHealthy    api.Function
	fnMetadata   api.Function

	fnCompactSpecs api.Function                        // optional: compact_specs() -> u64
	compactSpecs   map[mcp.ToolName][]mcp.CompactField // parsed once after load
}

func (m *Module) resolveExports() error {
	m.fnName = m.mod.ExportedFunction("name")
	if m.fnName == nil {
		return fmt.Errorf("wasm: module does not export 'name'")
	}
	m.fnTools = m.mod.ExportedFunction("tools")
	if m.fnTools == nil {
		return fmt.Errorf("wasm: module does not export 'tools'")
	}
	m.fnConfig = m.mod.ExportedFunction("configure")
	if m.fnConfig == nil {
		return fmt.Errorf("wasm: module does not export 'configure'")
	}
	m.fnExec = m.mod.ExportedFunction("execute")
	if m.fnExec == nil {
		return fmt.Errorf("wasm: module does not export 'execute'")
	}
	m.fnHealthy = m.mod.ExportedFunction("healthy")
	if m.fnHealthy == nil {
		return fmt.Errorf("wasm: module does not export 'healthy'")
	}
	m.fnMetadata = m.mod.ExportedFunction("metadata")
	if m.fnMetadata == nil {
		return fmt.Errorf("wasm: module does not export 'metadata'")
	}

	// compact_specs is optional — modules without it simply skip compaction.
	m.fnCompactSpecs = m.mod.ExportedFunction("compact_specs")
	return nil
}

// loadCompactSpecs calls the guest compact_specs() export (if present),
// parses the returned JSON map into pre-compiled CompactField slices,
// and caches them for the lifetime of the module.
func (m *Module) loadCompactSpecs(ctx context.Context) {
	if m.fnCompactSpecs == nil {
		return
	}
	results, err := m.fnCompactSpecs.Call(ctx)
	if err != nil {
		slog.Warn("wasm: compact_specs call failed", "err", err)
		return
	}
	if len(results) == 0 {
		return
	}

	ptr, size := unpackPtrSize(results[0])
	if size == 0 {
		return
	}
	data, err := readFromGuest(m.mod, ptr, size)
	freeInGuest(ctx, m.mod, ptr)
	if err != nil {
		slog.Warn("wasm: read compact_specs", "err", err)
		return
	}

	var raw map[mcp.ToolName][]string
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Warn("wasm: unmarshal compact_specs", "err", err)
		return
	}

	specs := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, dotSpecs := range raw {
		fields, parseErr := mcp.ParseCompactSpecs(dotSpecs)
		if parseErr != nil {
			slog.Warn("wasm: invalid compact spec", "tool", tool, "err", parseErr)
			continue
		}
		specs[tool] = fields
	}
	m.compactSpecs = specs
}

// CompactSpec implements mcp.FieldCompactionIntegration.
func (m *Module) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	if m.compactSpecs == nil {
		return nil, false
	}
	fields, ok := m.compactSpecs[toolName]
	return fields, ok
}

// Close releases the WASM module instance.
func (m *Module) Close(ctx context.Context) error {
	return m.mod.Close(ctx)
}
