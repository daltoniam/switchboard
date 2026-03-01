package script

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/dop251/goja"
)

const (
	DefaultTimeout  = 30 * time.Second
	DefaultMaxCalls = 50
)

// Executor looks up an integration by tool name prefix and executes the tool.
type Executor interface {
	Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error)
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

// Result is the output of a script execution.
type Result struct {
	Data    string `json:"data"`
	IsError bool   `json:"is_error,omitempty"`
}

// Run executes a JavaScript script. The script has access to:
//   - api.call(toolName, args) — calls an integration tool and returns the parsed JSON result
//   - console.log(...args) — collects log output (available in Result on error)
//
// The script's return value is JSON-serialized as the Result.Data.
func (e *Engine) Run(ctx context.Context, source string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	callCount := 0
	var logs []string

	apiObj := vm.NewObject()
	if err := apiObj.Set("call", func(call goja.FunctionCall) goja.Value {
		if err := ctx.Err(); err != nil {
			panic(vm.NewGoError(fmt.Errorf("script cancelled: %w", err)))
		}

		callCount++
		if callCount > e.maxCalls {
			panic(vm.NewGoError(fmt.Errorf("exceeded maximum of %d api.call() invocations", e.maxCalls)))
		}

		toolName := call.Argument(0).String()
		if toolName == "" || toolName == "undefined" {
			panic(vm.NewGoError(fmt.Errorf("api.call() requires a tool name as the first argument")))
		}

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

		result, err := e.executor.Execute(ctx, toolName, args)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("api.call(%q) failed: %w", toolName, err)))
		}
		if result.IsError {
			panic(vm.NewGoError(fmt.Errorf("api.call(%q) returned error: %s", toolName, result.Data)))
		}

		var parsed any
		if err := json.Unmarshal([]byte(result.Data), &parsed); err != nil {
			return vm.ToValue(result.Data)
		}
		return vm.ToValue(parsed)
	}); err != nil {
		return nil, fmt.Errorf("failed to set api.call: %w", err)
	}

	if err := vm.Set("api", apiObj); err != nil {
		return nil, fmt.Errorf("failed to set api object: %w", err)
	}

	consoleObj := vm.NewObject()
	if err := consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		line := ""
		for i, p := range parts {
			if i > 0 {
				line += " "
			}
			line += p
		}
		logs = append(logs, line)
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
		return &Result{Data: errMsg, IsError: true}, nil
	}

	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		if len(logs) > 0 {
			logData, _ := json.Marshal(logs)
			return &Result{Data: string(logData)}, nil
		}
		return &Result{Data: "null"}, nil
	}

	exported := val.Export()
	data, err := json.Marshal(exported)
	if err != nil {
		return &Result{Data: fmt.Sprintf("%v", exported)}, nil
	}
	return &Result{Data: string(data)}, nil
}
