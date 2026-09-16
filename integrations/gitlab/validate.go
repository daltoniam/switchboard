package gitlab

import (
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func validateArgs(name mcp.ToolName, args map[string]any) error {
	var def mcp.ToolDefinition
	for _, t := range tools {
		if t.Name == name {
			def = t
			break
		}
	}
	for _, key := range def.Required {
		v, ok := args[key]
		if !ok || v == nil {
			return fmt.Errorf("gitlab: %s is required", key)
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			return fmt.Errorf("gitlab: %s is required", key)
		}
	}
	for key, value := range args {
		if _, ok := def.Parameters[key]; !ok {
			return fmt.Errorf("gitlab: unknown parameter %q", key)
		}
		if value == nil {
			return fmt.Errorf("gitlab: %s must not be null", key)
		}
		switch key {
		case "page", "per_page", "merge_request_iid", "issue_iid", "pipeline_id", "job_id":
			if _, err := mcp.ArgInt(map[string]any{key: value}, key); err != nil {
				return fmt.Errorf("gitlab: %s: %w", key, err)
			}
			if key == "per_page" {
				n, _ := mcp.ArgInt(map[string]any{key: value}, key)
				if n > 100 {
					return fmt.Errorf("gitlab: per_page maximum is 100")
				}
			}
		}
	}
	return nil
}
