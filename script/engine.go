package script

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/dop251/goja"
)

const (
	DefaultTimeout  = 30 * time.Second
	DefaultMaxCalls = 50
	MaxScriptSize   = 64 * 1024
	MaxLogEntries   = 100
)

// Executor looks up an integration by tool name prefix and executes the tool.
type Executor interface {
	Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error)
}

// Engine runs JavaScript scripts with access to integration tools via api.call().
type Engine struct {
	executor Executor
	timeout  time.Duration
	maxCalls int
}

// Option configures an Engine.
type Option func(*Engine)

// WithTimeout sets the maximum execution time for a script.
func WithTimeout(d time.Duration) Option {
	return func(e *Engine) { e.timeout = d }
}

// WithMaxCalls sets the maximum number of api.call() invocations per script.
func WithMaxCalls(n int) Option {
	return func(e *Engine) { e.maxCalls = n }
}

// New creates a script Engine that delegates tool calls to the given Executor.
func New(executor Executor, opts ...Option) *Engine {
	e := &Engine{
		executor: executor,
		timeout:  DefaultTimeout,
		maxCalls: DefaultMaxCalls,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// parseCallArgs extracts the tool name, argument map, and optional options from a goja function call.
// The third argument is optional: api.call(tool, args, {fields: ["id", "title"]})
func parseCallArgs(call goja.FunctionCall) (string, map[string]any, map[string]any) {
	toolName := call.Argument(0).String()

	var args map[string]any
	if len(call.Arguments) > 1 {
		argsVal := call.Argument(1)
		if argsVal != nil && !goja.IsUndefined(argsVal) && !goja.IsNull(argsVal) {
			exported := argsVal.Export()
			if m, ok := exported.(map[string]any); ok {
				args = m
			} else {
				raw, _ := json.Marshal(exported)
				_ = json.Unmarshal(raw, &args)
			}
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	var opts map[string]any
	if len(call.Arguments) > 2 {
		optsVal := call.Argument(2)
		if optsVal != nil && !goja.IsUndefined(optsVal) && !goja.IsNull(optsVal) {
			if m, ok := optsVal.Export().(map[string]any); ok {
				opts = m
			}
		}
	}
	return toolName, args, opts
}

// projectFields applies field projection to result data when opts contains a "fields" key.
// Returns the original data unchanged if no fields option is provided.
func projectFields(data string, opts map[string]any) (string, error) {
	if opts == nil {
		return data, nil
	}
	fieldsRaw, ok := opts["fields"]
	if !ok {
		return data, nil
	}
	fieldSlice, ok := fieldsRaw.([]any)
	if !ok {
		return "", fmt.Errorf("fields option must be an array, got %T", fieldsRaw)
	}
	specs := make([]string, len(fieldSlice))
	for i, f := range fieldSlice {
		s, ok := f.(string)
		if !ok {
			return "", fmt.Errorf("fields[%d] must be a string, got %T", i, f)
		}
		specs[i] = s
	}
	fields, err := mcp.ParseCompactSpecs(specs)
	if err != nil {
		return "", fmt.Errorf("invalid field projection: %w", err)
	}
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return "", fmt.Errorf("field projection parse: %w", err)
	}
	projected := mcp.CompactAny(parsed, fields)
	result, err := json.Marshal(projected)
	if err != nil {
		return "", fmt.Errorf("field projection marshal: %w", err)
	}
	return string(result), nil
}

// parseResult JSON-parses a ToolResult.Data string and returns it as a goja value.
func parseResult(vm *goja.Runtime, data string) goja.Value {
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return vm.ToValue(data)
	}
	return vm.ToValue(parsed)
}

// Run executes a JavaScript script. The script has access to:
//   - api.call(toolName, args[, opts]) — calls an integration tool and returns the parsed JSON result.
//     Optional opts object with fields key applies a secondary field projection: {fields: ["id", "title"]}.
//   - api.tryCall(toolName, args[, opts]) — like call, but returns {ok, data/error} instead of throwing.
//     Also supports the optional opts with field projection.
//   - console.log(...args) — collects log output (available in result on error)
//
// The script's return value is JSON-serialized as the ToolResult.Data.
func (e *Engine) Run(ctx context.Context, source string) (*mcp.ToolResult, error) {
	if len(source) > MaxScriptSize {
		return &mcp.ToolResult{
			Data:    fmt.Sprintf("script too large: %d bytes (max %d)", len(source), MaxScriptSize),
			IsError: true,
		}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	callCount := 0
	var logs []string

	apiObj := vm.NewObject()

	// checkCallLimit enforces context cancellation and the per-script call cap.
	// Both api.call() and api.tryCall() share this limit.
	checkCallLimit := func() {
		if err := ctx.Err(); err != nil {
			panic(vm.NewGoError(fmt.Errorf("script cancelled: %w", err)))
		}
		callCount++
		if callCount > e.maxCalls {
			panic(vm.NewGoError(fmt.Errorf("exceeded maximum of %d api.call() invocations", e.maxCalls)))
		}
	}

	if err := apiObj.Set("call", func(call goja.FunctionCall) goja.Value {
		checkCallLimit()

		toolName, args, opts := parseCallArgs(call)
		if toolName == "" || toolName == "undefined" {
			panic(vm.NewGoError(fmt.Errorf("api.call() requires a tool name as the first argument")))
		}

		result, err := e.executor.Execute(ctx, mcp.ToolName(toolName), args)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("api.call(%q) failed: %w", toolName, err)))
		}
		if result.IsError {
			panic(vm.NewGoError(fmt.Errorf("api.call(%q) returned error: %s", toolName, result.Data)))
		}

		data, err := projectFields(result.Data, opts)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("api.call(%q) field projection: %w", toolName, err)))
		}

		return parseResult(vm, data)
	}); err != nil {
		return nil, fmt.Errorf("failed to set api.call: %w", err)
	}

	if err := apiObj.Set("tryCall", func(call goja.FunctionCall) goja.Value {
		// Context cancellation still kills the script — nothing can execute anyway.
		if err := ctx.Err(); err != nil {
			panic(vm.NewGoError(fmt.Errorf("script cancelled: %w", err)))
		}
		// maxCalls returns an error envelope instead of killing the script,
		// so partial results from earlier calls are preserved.
		callCount++
		if callCount > e.maxCalls {
			return vm.ToValue(map[string]any{"ok": false, "error": fmt.Sprintf("exceeded maximum of %d api.call() invocations", e.maxCalls)})
		}

		toolName, args, opts := parseCallArgs(call)
		if toolName == "" || toolName == "undefined" {
			return vm.ToValue(map[string]any{"ok": false, "error": "api.tryCall() requires a tool name"})
		}

		result, err := e.executor.Execute(ctx, mcp.ToolName(toolName), args)
		if err != nil {
			return vm.ToValue(map[string]any{"ok": false, "error": err.Error()})
		}
		if result.IsError {
			return vm.ToValue(map[string]any{"ok": false, "error": result.Data})
		}

		data := result.Data
		if projected, err := projectFields(data, opts); err != nil {
			return vm.ToValue(map[string]any{"ok": false, "error": err.Error()})
		} else {
			data = projected
		}

		var parsed any
		if jsonErr := json.Unmarshal([]byte(data), &parsed); jsonErr != nil {
			return vm.ToValue(map[string]any{"ok": true, "data": data})
		}
		return vm.ToValue(map[string]any{"ok": true, "data": parsed})
	}); err != nil {
		return nil, fmt.Errorf("failed to set api.tryCall: %w", err)
	}

	if err := vm.Set("api", apiObj); err != nil {
		return nil, fmt.Errorf("failed to set api object: %w", err)
	}

	consoleObj := vm.NewObject()
	if err := consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		if len(logs) >= MaxLogEntries {
			return goja.Undefined()
		}
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		logs = append(logs, strings.Join(parts, " "))
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("failed to set console.log: %w", err)
	}
	if err := vm.Set("console", consoleObj); err != nil {
		return nil, fmt.Errorf("failed to set console object: %w", err)
	}

	go func() {
		<-ctx.Done()
		vm.Interrupt("execution timeout")
	}()

	val, err := vm.RunString(source)
	if err != nil {
		errMsg := err.Error()
		if len(logs) > 0 {
			logData, _ := json.Marshal(logs)
			errMsg += "\nconsole output: " + string(logData)
		}
		return &mcp.ToolResult{Data: errMsg, IsError: true}, nil
	}

	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		if len(logs) > 0 {
			logData, _ := json.Marshal(logs)
			return &mcp.ToolResult{Data: string(logData)}, nil
		}
		return &mcp.ToolResult{Data: "null"}, nil
	}

	exported := val.Export()
	data, err := json.Marshal(exported)
	if err != nil {
		return &mcp.ToolResult{Data: fmt.Sprintf("%v", exported)}, nil
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}
