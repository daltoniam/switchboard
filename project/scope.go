package project

import (
	"path"

	mcp "github.com/daltoniam/switchboard"
)

// IsToolPermitted implements the RFC-0002 §5 resolution algorithm.
// It returns true if the given tool name is permitted under the scope rule.
func IsToolPermitted(toolName string, rule *ScopeRule) bool {
	if rule == nil {
		return true
	}

	candidate := true
	if len(rule.Allow) > 0 {
		candidate = false
		for _, pattern := range rule.Allow {
			if globMatch(pattern, toolName) {
				candidate = true
				break
			}
		}
	}

	if !candidate {
		return false
	}

	for _, pattern := range rule.Deny {
		if globMatch(pattern, toolName) {
			return false
		}
	}

	return true
}

// FilterTools returns only the tools permitted by the scope rule.
func FilterTools(tools []mcp.ToolDefinition, rule *ScopeRule) []mcp.ToolDefinition {
	if rule == nil {
		return tools
	}
	var result []mcp.ToolDefinition
	for _, t := range tools {
		if IsToolPermitted(t.Name, rule) {
			result = append(result, t)
		}
	}
	return result
}

// ResolveDefaults computes the merged default arguments for a tool name
// by matching against all patterns in the defaults map. Patterns are applied
// in iteration order; later patterns override earlier ones. Agent-provided
// arguments override everything.
func ResolveDefaults(toolName string, rule *ScopeRule, agentArgs map[string]any) map[string]any {
	if rule == nil || len(rule.Defaults) == 0 {
		return agentArgs
	}

	merged := make(map[string]any)

	for pattern, defaults := range rule.Defaults {
		if globMatch(pattern, toolName) {
			for k, v := range defaults {
				merged[k] = v
			}
		}
	}

	for k, v := range agentArgs {
		merged[k] = v
	}

	return merged
}

// ResolveRoleScope computes the effective scope rule for a server
// after applying role overrides per RFC-0004 §3.1:
//   - allow: role REPLACES project-level allow
//   - deny: role APPENDS to project-level deny
//   - defaults: role MERGES with project-level (role wins per key)
func ResolveRoleScope(projectRule *ScopeRule, roleRule *ScopeRule) *ScopeRule {
	if roleRule == nil {
		return projectRule
	}
	if projectRule == nil {
		return roleRule
	}

	result := &ScopeRule{}

	if len(roleRule.Allow) > 0 {
		result.Allow = append(result.Allow, roleRule.Allow...)
	} else {
		result.Allow = append(result.Allow, projectRule.Allow...)
	}

	result.Deny = append(result.Deny, projectRule.Deny...)
	result.Deny = append(result.Deny, roleRule.Deny...)

	result.Defaults = make(map[string]map[string]any)
	for pattern, args := range projectRule.Defaults {
		copied := make(map[string]any)
		for k, v := range args {
			copied[k] = v
		}
		result.Defaults[pattern] = copied
	}
	for pattern, args := range roleRule.Defaults {
		if existing, ok := result.Defaults[pattern]; ok {
			for k, v := range args {
				existing[k] = v
			}
		} else {
			result.Defaults[pattern] = args
		}
	}

	return result
}

// GetEffectiveRule returns the effective scope rule for a given server
// and optional role within a project definition.
func GetEffectiveRule(def *Definition, serverID string, role string) *ScopeRule {
	if def.Tools == nil {
		return nil
	}
	projectRule, ok := def.Tools[serverID]
	if !ok {
		return nil
	}
	if role == "" || def.Agents == nil || def.Agents.Roles == nil {
		return projectRule
	}
	roleDef, ok := def.Agents.Roles[role]
	if !ok || roleDef.ToolOverrides == nil {
		return projectRule
	}
	roleRule, ok := roleDef.ToolOverrides[serverID]
	if !ok {
		return projectRule
	}
	return ResolveRoleScope(projectRule, roleRule)
}

// globMatch matches a tool name against a glob pattern.
// Uses path.Match which supports *, ?, and [] character classes.
func globMatch(pattern, name string) bool {
	matched, _ := path.Match(pattern, name)
	return matched
}
