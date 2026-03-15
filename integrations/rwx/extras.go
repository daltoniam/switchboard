package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getArtifacts(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "run_id"))
	runURL := fmt.Sprintf("%s/mint/%s/runs/%s", rwxAPIBase, rwxOrg, id)
	download := argBool(args, "download")

	cmdArgs := []string{"artifacts", id, "--output", "json"}
	if !download {
		cmdArgs = append(cmdArgs, "--list")
	}
	if key := argStr(args, "artifact_key"); key != "" && download {
		cmdArgs = append(cmdArgs, "--key", key)
	}

	output, err := r.runRWXCommand(cmdArgs, 0)
	if err != nil {
		return errResult(err)
	}

	var parsed struct {
		RunID     string `json:"run_id"`
		Artifacts []struct {
			Key       string `json:"key"`
			TaskKey   string `json:"task_key"`
			SizeBytes int    `json:"size_bytes"`
			Path      string `json:"path"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return errResult(fmt.Errorf("parse artifacts output: %w", err))
	}

	action := "listed"
	if download {
		action = "downloaded"
	}

	resp := map[string]any{
		"run_id":    id,
		"url":       runURL,
		"action":    action,
		"artifacts": parsed.Artifacts,
		"count":     len(parsed.Artifacts),
	}
	if !download {
		resp["hint"] = "Set download=true to download artifacts"
	}
	return jsonResult(resp)
}

func validateWorkflow(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	filePath := argStr(args, "file_path")
	if filePath == "" {
		filePath = ".rwx/ci.yml"
	}

	output, err := r.runRWXCommand([]string{"lint", filePath, "--output", "json"}, 30000)
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && len(exitErr.Stderr) > 0 {
			return jsonResult(map[string]any{
				"isValid": false,
				"errors": []map[string]string{
					{"severity": "error", "message": string(exitErr.Stderr)},
				},
				"warnings": []any{},
			})
		}
		if output != "" {
			return &mcp.ToolResult{Data: output, IsError: true}, nil
		}
		return errResult(err)
	}

	return rawResult(output)
}

func verifyCLI(_ context.Context, r *rwx, _ map[string]any) (*mcp.ToolResult, error) {
	check := r.getRWXCLIVersion()

	if !check.installed {
		return jsonResult(map[string]any{
			"status":               "not_installed",
			"message":              fmt.Sprintf("rwx CLI is not installed. Please install version >= %s.", minRWXVersion),
			"install_instructions": "Visit https://github.com/rwx-research/rwx-cli/releases or use: brew install rwx-research/tap/rwx",
		})
	}

	if !check.meetsMinimum {
		return jsonResult(map[string]any{
			"status":               "outdated",
			"current_version":      check.version,
			"required_version":     minRWXVersion,
			"message":              fmt.Sprintf("rwx CLI version %s is below minimum required version %s. Please upgrade.", check.version, minRWXVersion),
			"install_instructions": "Visit https://github.com/rwx-research/rwx-cli/releases or use: brew upgrade rwx-research/tap/rwx",
		})
	}

	return jsonResult(map[string]any{
		"status":  "ready",
		"version": check.version,
		"message": fmt.Sprintf("rwx CLI version %s is installed and ready.", check.version),
	})
}

// --- CLI version check ---

type rwxVersionCheck struct {
	installed    bool
	version      string
	meetsMinimum bool
}

func (r *rwx) getRWXCLIVersion() rwxVersionCheck {
	output, err := exec.Command(r.cliPath, "--version").CombinedOutput() // #nosec G204 -- resolved binary path
	if err != nil {
		return rwxVersionCheck{installed: false}
	}

	versionStr := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(versionStr)
	if match == nil {
		return rwxVersionCheck{installed: true}
	}

	version := match[1]
	return rwxVersionCheck{
		installed:    true,
		version:      version,
		meetsMinimum: isVersionGTE(version, minRWXVersion),
	}
}

func isVersionGTE(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var va, vb int
		if i < len(partsA) {
			_, _ = fmt.Sscanf(partsA[i], "%d", &va)
		}
		if i < len(partsB) {
			_, _ = fmt.Sscanf(partsB[i], "%d", &vb)
		}
		if va > vb {
			return true
		}
		if va < vb {
			return false
		}
	}
	return true
}
