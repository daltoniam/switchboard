package project

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
)

func TestIsToolPermitted(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		rule     *ScopeRule
		expected bool
	}{
		{
			name:     "nil rule permits all",
			tool:     "anything",
			rule:     nil,
			expected: true,
		},
		{
			name:     "empty rule permits all",
			tool:     "anything",
			rule:     &ScopeRule{},
			expected: true,
		},
		{
			name:     "allow matches",
			tool:     "github_list_issues",
			rule:     &ScopeRule{Allow: []string{"github_*"}},
			expected: true,
		},
		{
			name:     "allow does not match",
			tool:     "datadog_search_logs",
			rule:     &ScopeRule{Allow: []string{"github_*"}},
			expected: false,
		},
		{
			name:     "deny overrides allow",
			tool:     "github_delete_repo",
			rule:     &ScopeRule{Allow: []string{"github_*"}, Deny: []string{"github_delete_*"}},
			expected: false,
		},
		{
			name:     "deny without allow",
			tool:     "github_delete_repo",
			rule:     &ScopeRule{Deny: []string{"github_delete_*"}},
			expected: false,
		},
		{
			name:     "deny without allow permits non-matching",
			tool:     "github_list_issues",
			rule:     &ScopeRule{Deny: []string{"github_delete_*"}},
			expected: true,
		},
		{
			name:     "multiple allow patterns",
			tool:     "linear_list_issues",
			rule:     &ScopeRule{Allow: []string{"github_*", "linear_*"}},
			expected: true,
		},
		{
			name:     "wildcard deny",
			tool:     "postgres_drop_table",
			rule:     &ScopeRule{Allow: []string{"postgres_*"}, Deny: []string{"*_drop_*"}},
			expected: false,
		},
		{
			name:     "exact match allow",
			tool:     "datadog_search_logs",
			rule:     &ScopeRule{Allow: []string{"datadog_search_logs"}},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsToolPermitted(tt.tool, tt.rule))
		})
	}
}

func TestFilterTools(t *testing.T) {
	tools := []mcp.ToolDefinition{
		{Name: "github_list_issues"},
		{Name: "github_delete_repo"},
		{Name: "linear_list_issues"},
		{Name: "datadog_search_logs"},
	}

	rule := &ScopeRule{
		Allow: []string{"github_*", "linear_*"},
		Deny:  []string{"github_delete_*"},
	}

	result := FilterTools(tools, rule)
	assert.Len(t, result, 2)
	assert.Equal(t, "github_list_issues", result[0].Name)
	assert.Equal(t, "linear_list_issues", result[1].Name)
}

func TestFilterTools_NilRule(t *testing.T) {
	tools := []mcp.ToolDefinition{
		{Name: "github_list_issues"},
	}
	result := FilterTools(tools, nil)
	assert.Len(t, result, 1)
}

func TestResolveDefaults(t *testing.T) {
	rule := &ScopeRule{
		Defaults: map[string]map[string]any{
			"github_*":      {"owner": "myorg"},
			"github_list_*": {"owner": "myorg", "per_page": "50"},
		},
	}

	t.Run("merged defaults", func(t *testing.T) {
		args := map[string]any{"state": "open"}
		result := ResolveDefaults("github_list_issues", rule, args)
		assert.Equal(t, "myorg", result["owner"])
		assert.Equal(t, "50", result["per_page"])
		assert.Equal(t, "open", result["state"])
	})

	t.Run("agent overrides defaults", func(t *testing.T) {
		args := map[string]any{"owner": "other", "state": "open"}
		result := ResolveDefaults("github_list_issues", rule, args)
		assert.Equal(t, "other", result["owner"])
		assert.Equal(t, "50", result["per_page"])
		assert.Equal(t, "open", result["state"])
	})

	t.Run("no matching defaults", func(t *testing.T) {
		args := map[string]any{"query": "test"}
		result := ResolveDefaults("datadog_search_logs", rule, args)
		assert.Equal(t, "test", result["query"])
		assert.Len(t, result, 1)
	})

	t.Run("nil rule", func(t *testing.T) {
		args := map[string]any{"a": "1"}
		result := ResolveDefaults("any_tool", nil, args)
		assert.Equal(t, args, result)
	})
}

func TestResolveRoleScope(t *testing.T) {
	t.Run("role replaces allow", func(t *testing.T) {
		project := &ScopeRule{
			Allow: []string{"github_*", "linear_*"},
			Deny:  []string{"github_delete_*"},
		}
		role := &ScopeRule{
			Allow: []string{"github_*"},
		}

		result := ResolveRoleScope(project, role)
		assert.Equal(t, []string{"github_*"}, result.Allow)
		assert.Equal(t, []string{"github_delete_*"}, result.Deny)
	})

	t.Run("role appends deny", func(t *testing.T) {
		project := &ScopeRule{
			Deny: []string{"github_delete_*"},
		}
		role := &ScopeRule{
			Deny: []string{"github_merge_*"},
		}

		result := ResolveRoleScope(project, role)
		assert.Equal(t, []string{"github_delete_*", "github_merge_*"}, result.Deny)
	})

	t.Run("role merges defaults", func(t *testing.T) {
		project := &ScopeRule{
			Defaults: map[string]map[string]any{
				"github_*": {"owner": "base-org", "repo": "base-repo"},
			},
		}
		role := &ScopeRule{
			Defaults: map[string]map[string]any{
				"github_*": {"owner": "role-org"},
			},
		}

		result := ResolveRoleScope(project, role)
		assert.Equal(t, "role-org", result.Defaults["github_*"]["owner"])
		assert.Equal(t, "base-repo", result.Defaults["github_*"]["repo"])
	})

	t.Run("nil project rule", func(t *testing.T) {
		role := &ScopeRule{Allow: []string{"github_*"}}
		result := ResolveRoleScope(nil, role)
		assert.Equal(t, role, result)
	})

	t.Run("nil role rule", func(t *testing.T) {
		project := &ScopeRule{Allow: []string{"github_*"}}
		result := ResolveRoleScope(project, nil)
		assert.Equal(t, project, result)
	})
}

func TestGetEffectiveRule(t *testing.T) {
	def := &Definition{
		Version: "1",
		Name:    "test",
		Tools: map[string]*ScopeRule{
			"switchboard": {
				Allow: []string{"github_*", "linear_*"},
				Deny:  []string{"github_delete_*"},
			},
		},
		Agents: &AgentsConfig{
			Roles: map[string]*RoleDefinition{
				"reviewer": {
					ToolOverrides: map[string]*ScopeRule{
						"switchboard": {
							Allow: []string{"github_get_*", "github_list_*"},
							Deny:  []string{"github_create_*"},
						},
					},
				},
			},
		},
	}

	t.Run("no role", func(t *testing.T) {
		rule := GetEffectiveRule(def, "switchboard", "")
		assert.Equal(t, []string{"github_*", "linear_*"}, rule.Allow)
	})

	t.Run("with role", func(t *testing.T) {
		rule := GetEffectiveRule(def, "switchboard", "reviewer")
		assert.Equal(t, []string{"github_get_*", "github_list_*"}, rule.Allow)
		assert.Equal(t, []string{"github_delete_*", "github_create_*"}, rule.Deny)
	})

	t.Run("unknown server", func(t *testing.T) {
		rule := GetEffectiveRule(def, "unknown", "")
		assert.Nil(t, rule)
	})

	t.Run("unknown role", func(t *testing.T) {
		rule := GetEffectiveRule(def, "switchboard", "unknown")
		assert.Equal(t, []string{"github_*", "linear_*"}, rule.Allow)
	})
}
