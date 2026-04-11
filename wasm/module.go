package wasm

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// SetName overrides the name returned by the WASM module's name() export.
func (m *Module) SetName(name string) {
	m.nameOverride = name
}

// Name implements mcp.Integration.
func (m *Module) Name() string {
	if m.nameOverride != "" {
		return m.nameOverride
	}
	ctx := context.Background()
	results, err := m.fnName.Call(ctx)
	if err != nil {
		return "unknown"
	}
	if len(results) == 0 {
		return "unknown"
	}

	ptr, size := unpackPtrSize(results[0])
	data, err := readFromGuest(m.mod, ptr, size)
	freeInGuest(ctx, m.mod, ptr)
	if err != nil {
		return "unknown"
	}
	return string(data)
}

// Configure implements mcp.Integration.
func (m *Module) Configure(ctx context.Context, creds mcp.Credentials) error {
	credsJSON, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("wasm: marshal credentials: %w", err)
	}

	ptr, size, err := writeToGuest(ctx, m.mod, credsJSON)
	if err != nil {
		return fmt.Errorf("wasm: write credentials: %w", err)
	}
	defer freeInGuest(ctx, m.mod, ptr)

	results, err := m.fnConfig.Call(ctx, packPtrSize(ptr, size))
	if err != nil {
		return fmt.Errorf("wasm: configure call failed: %w", err)
	}
	if len(results) == 0 {
		return nil
	}

	rPtr, rSize := unpackPtrSize(results[0])
	if rSize == 0 {
		return nil
	}
	errData, readErr := readFromGuest(m.mod, rPtr, rSize)
	freeInGuest(ctx, m.mod, rPtr)
	if readErr != nil {
		return fmt.Errorf("wasm: read configure result: %w", readErr)
	}
	if len(errData) > 0 {
		return fmt.Errorf("%s", string(errData))
	}
	return nil
}

// Tools implements mcp.Integration.
func (m *Module) Tools() []mcp.ToolDefinition {
	ctx := context.Background()
	results, err := m.fnTools.Call(ctx)
	if err != nil {
		return nil
	}
	if len(results) == 0 {
		return nil
	}

	ptr, size := unpackPtrSize(results[0])
	data, err := readFromGuest(m.mod, ptr, size)
	freeInGuest(ctx, m.mod, ptr)
	if err != nil {
		return nil
	}

	var tools []mcp.ToolDefinition
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil
	}
	return tools
}

// Execute implements mcp.Integration.
func (m *Module) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	req := struct {
		ToolName string         `json:"tool_name"`
		Args     map[string]any `json:"args"`
	}{
		ToolName: string(toolName),
		Args:     args,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	ptr, size, err := writeToGuest(ctx, m.mod, reqJSON)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}
	defer freeInGuest(ctx, m.mod, ptr)

	results, err := m.fnExec.Call(ctx, packPtrSize(ptr, size))
	if err != nil {
		return nil, fmt.Errorf("wasm: execute call failed: %w", err)
	}
	if len(results) == 0 {
		return &mcp.ToolResult{Data: "empty response from wasm module", IsError: true}, nil
	}

	rPtr, rSize := unpackPtrSize(results[0])
	data, readErr := readFromGuest(m.mod, rPtr, rSize)
	freeInGuest(ctx, m.mod, rPtr)
	if readErr != nil {
		return &mcp.ToolResult{Data: readErr.Error(), IsError: true}, nil
	}

	var result mcp.ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		return &mcp.ToolResult{Data: string(data)}, nil
	}
	return &result, nil
}

// Healthy implements mcp.Integration.
func (m *Module) Healthy(ctx context.Context) bool {
	results, err := m.fnHealthy.Call(ctx)
	if err != nil {
		return false
	}
	if len(results) == 0 {
		return false
	}
	return results[0] == 1
}
