package launchdarkly

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        mcp.ToolName("launchdarkly_list_projects"),
		Description: "List LaunchDarkly projects that hold feature flags and environments. Start here for feature flag management, flag rollouts, and finding the project_key needed by every other LaunchDarkly tool.",
		Parameters: map[string]string{
			"query":  "Optional name or key substring filter",
			"limit":  "Page size (default 20, max 100)",
			"offset": "Pagination offset (default 0)",
		},
	},
	{
		Name:        mcp.ToolName("launchdarkly_list_environments"),
		Description: "List LaunchDarkly environments (production, staging, test) for a project, including environment_key and critical flag. Use after list_projects to find the environment_key for flag status checks and toggles.",
		Parameters: map[string]string{
			"project_key": "LaunchDarkly project key",
			"query":       "Optional name or key substring filter",
			"limit":       "Page size (default 20, max 100)",
			"offset":      "Pagination offset (default 0)",
		},
		Required: []string{"project_key"},
	},
	{
		Name:        mcp.ToolName("launchdarkly_list_flags"),
		Description: "List LaunchDarkly feature flags and toggles in a project with per-environment on/off targeting state. Use after list_projects to answer whether a flag or rollout is enabled. Pass environment_key to return only that environment's configuration.",
		Parameters: map[string]string{
			"project_key":     "LaunchDarkly project key",
			"environment_key": "Optional environment key to restrict environment configuration (e.g. production)",
			"query":           "Optional name, key, or description substring filter",
			"tag":             "Optional tag filter",
			"sort":            "Sort field: creationDate, key, name, targetingModifiedDate, type (prefix with - for descending)",
			"limit":           "Page size (default 20, max 100)",
			"offset":          "Pagination offset (default 0)",
		},
		Required: []string{"project_key"},
	},
	{
		Name:        mcp.ToolName("launchdarkly_get_flag"),
		Description: "Get a single LaunchDarkly feature flag by key, including variations, maintainer, and per-environment on/off state, fallthrough, and version. Use after list_flags. Pass environment_key to return only that environment's configuration.",
		Parameters: map[string]string{
			"project_key":     "LaunchDarkly project key",
			"flag_key":        "Feature flag key",
			"environment_key": "Optional environment key to restrict environment configuration (e.g. production)",
		},
		Required: []string{"project_key", "flag_key"},
	},
	{
		Name:        mcp.ToolName("launchdarkly_list_flag_statuses"),
		Description: "List LaunchDarkly flag evaluation statuses (new, active, inactive, launched) for one environment, showing when each feature flag was last requested by SDKs. Use after list_environments to find stale or unused flags; the flag_key is the last path segment of each item's href.",
		Parameters: map[string]string{
			"project_key":     "LaunchDarkly project key",
			"environment_key": "Environment key (e.g. production)",
		},
		Required: []string{"project_key", "environment_key"},
	},
	{
		Name:        mcp.ToolName("launchdarkly_toggle_flag"),
		Description: "Turn a LaunchDarkly feature flag on or off (enable, disable, kill switch) in exactly one environment via semantic patch. Explicit single-flag mutation; does not edit targeting rules. Requires project_key, flag_key, environment_key, and on. Returns a small confirmation (key, environment_key, on, version). Use after get_flag to confirm the current state.",
		Parameters: map[string]string{
			"project_key":     "LaunchDarkly project key",
			"flag_key":        "Feature flag key",
			"environment_key": "Environment key to change (e.g. production). Never defaults",
			"on":              "true to turn the flag on, false to turn it off",
			"comment":         "Optional audit log comment explaining the change",
		},
		Required: []string{"project_key", "flag_key", "environment_key", "on"},
	},
}
