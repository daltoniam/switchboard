package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mcp "github.com/daltoniam/switchboard"
)

func vaultsVarShow(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	name := ra.Str("name")
	vault := ra.Str("vault")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if vault == "" {
		vault = "default"
	}

	cmdArgs := []string{"vaults", "vars", "show", name, "--vault", vault, "--output", "json"}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		if output == "" {
			return mcp.ErrResult(fmt.Errorf("vault var show: no output from CLI"))
		}
		return &mcp.ToolResult{Data: output}, nil
	}
	return mcp.JSONResult(parsed)
}

func vaultsVarSet(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	name := ra.Str("name")
	value := ra.Str("value")
	vault := ra.Str("vault")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if vault == "" {
		vault = "default"
	}

	envFile, err := writeEnvFile(name, value)
	if err != nil {
		return mcp.ErrResult(err)
	}
	defer func() { _ = os.Remove(envFile) }()

	cmdArgs := []string{"vaults", "vars", "set", "--file", envFile, "--vault", vault, "--output", "json"}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	if output == "" {
		return mcp.JSONResult(map[string]any{
			"status": "set",
			"name":   name,
			"vault":  vault,
		})
	}
	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return &mcp.ToolResult{Data: output}, nil
	}
	return mcp.JSONResult(parsed)
}

func vaultsVarDelete(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	name := ra.Str("name")
	vault := ra.Str("vault")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if vault == "" {
		vault = "default"
	}

	cmdArgs := []string{"vaults", "vars", "delete", name, "--vault", vault, "--yes", "--output", "json"}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	if output == "" {
		return mcp.JSONResult(map[string]any{
			"status": "deleted",
			"name":   name,
			"vault":  vault,
		})
	}
	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return &mcp.ToolResult{Data: output}, nil
	}
	return mcp.JSONResult(parsed)
}

func vaultsSecretSet(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	name := ra.Str("name")
	value := ra.Str("value")
	vault := ra.Str("vault")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if vault == "" {
		vault = "default"
	}

	envFile, err := writeEnvFile(name, value)
	if err != nil {
		return mcp.ErrResult(err)
	}
	defer func() { _ = os.Remove(envFile) }()

	cmdArgs := []string{"vaults", "secrets", "set", "--file", envFile, "--vault", vault, "--output", "json"}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	if output == "" {
		return mcp.JSONResult(map[string]any{
			"status": "set",
			"name":   name,
			"vault":  vault,
		})
	}
	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return &mcp.ToolResult{Data: output}, nil
	}
	return mcp.JSONResult(parsed)
}

func vaultsSecretDelete(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	name := ra.Str("name")
	vault := ra.Str("vault")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if vault == "" {
		vault = "default"
	}

	cmdArgs := []string{"vaults", "secrets", "delete", name, "--vault", vault, "--yes", "--output", "json"}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	if output == "" {
		return mcp.JSONResult(map[string]any{
			"status": "deleted",
			"name":   name,
			"vault":  vault,
		})
	}
	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return &mcp.ToolResult{Data: output}, nil
	}
	return mcp.JSONResult(parsed)
}

func writeEnvFile(name, value string) (string, error) {
	f, err := os.CreateTemp("", "rwx-vault-*.env")
	if err != nil {
		return "", fmt.Errorf("create temp env file: %w", err)
	}
	_, err = fmt.Fprintf(f, "%s=%s\n", name, value)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write env file: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close env file: %w", closeErr)
	}
	return f.Name(), nil
}
