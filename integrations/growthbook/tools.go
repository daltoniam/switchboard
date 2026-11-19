package growthbook

import mcp "github.com/daltoniam/switchboard"

const (
	limitParam  = "Page size 1..100 (default 20)"
	offsetParam = "Pagination offset (default 0); use nextOffset from the previous page"
)

var tools = []mcp.ToolDefinition{
	{
		Name:        "growthbook_list_features",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook feature flags (features) with per-environment on/off state, default value, value type, tags and targeting rules (force, rollout, experiment). Start here for GrowthBook feature flag management, finding a flag key, and checking whether a flag or rollout is enabled in production. Paginated with limit/offset.",
		Parameters: map[string]string{
			"project_id": "Optional GrowthBook project id (prj_...) from growthbook_list_projects",
			"archived":   "Optional boolean; true includes archived flags (default false)",
			"limit":      limitParam,
			"offset":     offsetParam,
		},
	},
	{
		Name:        "growthbook_get_feature",
		Access:      mcp.AccessRead,
		Description: "Get one GrowthBook feature flag by key: default value, per-environment enabled state, targeting rules with conditions, saved groups, rollout coverage and linked experiments, prerequisites, and the published revision version. Use after growthbook_list_features.",
		Parameters: map[string]string{
			"feature_id": "Feature flag key (id), e.g. checkout-express-pay",
		},
		Required: []string{"feature_id"},
	},
	{
		Name:        "growthbook_get_feature_staleness",
		Access:      mcp.AccessRead,
		Description: "Check whether GrowthBook feature flags are stale (unused, fully rolled out, toggled off, no rules) overall and per environment, with the reason and what each environment evaluates to. Use for flag cleanup and tech-debt removal after growthbook_list_features.",
		Parameters: map[string]string{
			"feature_ids": "Comma-separated feature flag keys (at most 100)",
		},
		Required: []string{"feature_ids"},
	},
	{
		Name:        "growthbook_list_feature_revisions",
		Access:      mcp.AccessRead,
		Description: "List revisions (drafts and published versions) of one GrowthBook feature flag: version, status, author, comment, environments enabled and rules. Defaults to open drafts; pass status=published to see change history. Use after growthbook_get_feature to see what changed or what is pending review.",
		Parameters: map[string]string{
			"feature_id": "Feature flag key (id)",
			"status":     "Optional revision status: draft, published, discarded, approved, changes-requested, pending-review, pending-parent, or all-drafts (default: open drafts)",
			"limit":      limitParam,
			"offset":     offsetParam,
		},
		Required: []string{"feature_id"},
	},
	{
		Name:        "growthbook_list_experiments",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook A/B test experiments with status (draft, running, stopped), variations, traffic split per phase, goal metrics, linked flag keys and the result summary (won, lost, inconclusive). Start here for GrowthBook experiment analysis and finding an experiment id. Paginated with limit/offset.",
		Parameters: map[string]string{
			"project_id": "Optional GrowthBook project id (prj_...)",
			"status":     "Optional status filter: draft, running or stopped",
			"search":     "Optional search text (same syntax as the GrowthBook experiment list filter)",
			"tag":        "Optional comma-separated tags",
			"archived":   "Optional boolean; true returns only archived experiments, false only active ones",
			"limit":      limitParam,
			"offset":     offsetParam,
		},
	},
	{
		Name:        "growthbook_get_experiment",
		Access:      mcp.AccessRead,
		Description: "Get one GrowthBook A/B test experiment by id: hypothesis, status, variations, phases with traffic split and targeting, data source, goal, secondary and guardrail metrics, linked flag keys and result summary. Use after growthbook_list_experiments.",
		Parameters: map[string]string{
			"experiment_id": "Experiment id (exp_...) from growthbook_list_experiments",
		},
		Required: []string{"experiment_id"},
	},
	{
		Name:        "growthbook_get_experiment_results",
		Access:      mcp.AccessRead,
		Description: "Get the latest GrowthBook experiment results (A/B test analysis): users per variation and, per metric, mean, percent change (lift), confidence interval, chance to beat control or p-value, plus health checks (sample ratio mismatch SRM, multiple exposures, power). Use after growthbook_get_experiment to decide a winner. Returns an error when no analysis has run; use growthbook_refresh_experiment_results first.",
		Parameters: map[string]string{
			"experiment_id": "Experiment id (exp_...)",
			"phase":         "Optional zero-based phase index (default: latest phase)",
			"dimension":     "Optional dimension to break results down by, e.g. exp:country or dim_...",
		},
		Required: []string{"experiment_id"},
	},
	{
		Name:        "growthbook_refresh_experiment_results",
		Access:      mcp.AccessWrite,
		Description: "Start a new GrowthBook experiment analysis snapshot, which runs the experiment's queries against the data warehouse and updates its results. Returns the snapshot id and status; poll growthbook_get_snapshot until status is success, then read growthbook_get_experiment_results. Costs warehouse compute.",
		Parameters: map[string]string{
			"experiment_id": "Experiment id (exp_...)",
			"phase":         "Optional zero-based phase index (default: latest phase)",
			"dimension":     "Optional dimension to break results down by, e.g. exp:country",
		},
		Required: []string{"experiment_id"},
	},
	{
		Name:        "growthbook_get_snapshot",
		Access:      mcp.AccessRead,
		Description: "Get the status (running, success, error) of a GrowthBook experiment analysis snapshot. Use after growthbook_refresh_experiment_results to know when new experiment results are ready.",
		Parameters: map[string]string{
			"snapshot_id": "Snapshot id (snp_...) from growthbook_refresh_experiment_results",
		},
		Required: []string{"snapshot_id"},
	},
	{
		Name:        "growthbook_list_fact_metrics",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook fact metrics (the current metric type) used as experiment goals and guardrails: name, metric type (proportion, mean, ratio, quantile, retention), fact table and column, inverse flag. Use to resolve metric ids seen in experiments and results.",
		Parameters: map[string]string{
			"project_id":    "Optional GrowthBook project id (prj_...)",
			"datasource_id": "Optional data source id (ds_...)",
			"fact_table_id": "Optional fact table id (ftb_...)",
			"limit":         limitParam,
			"offset":        offsetParam,
		},
	},
	{
		Name:        "growthbook_get_fact_metric",
		Access:      mcp.AccessRead,
		Description: "Get one GrowthBook fact metric definition: numerator and denominator (fact table, column, filters), conversion window, capping, minimum sample size and percent change thresholds. Use after growthbook_list_fact_metrics.",
		Parameters: map[string]string{
			"metric_id": "Fact metric id (fact__...)",
		},
		Required: []string{"metric_id"},
	},
	{
		Name:        "growthbook_list_metrics",
		Access:      mcp.AccessRead,
		Description: "List legacy GrowthBook SQL metrics (met_... ids, defined before fact metrics) with type (binomial, count, duration, revenue), data source and goal direction. Use when an experiment references a met_ metric id; prefer growthbook_list_fact_metrics for newer metrics.",
		Parameters: map[string]string{
			"project_id":    "Optional GrowthBook project id (prj_...)",
			"datasource_id": "Optional data source id (ds_...)",
			"limit":         limitParam,
			"offset":        offsetParam,
		},
	},
	{
		Name:        "growthbook_list_fact_tables",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook fact tables: the warehouse SQL query, identifier types and columns that fact metrics are built from. Use to understand where a metric's data comes from.",
		Parameters: map[string]string{
			"datasource_id": "Optional data source id (ds_...)",
			"project_id":    "Optional GrowthBook project id (prj_...)",
			"limit":         limitParam,
			"offset":        offsetParam,
		},
	},
	{
		Name:        "growthbook_list_projects",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook projects, which group feature flags, experiments and metrics and control access. Use to find the project_id filter for other GrowthBook tools.",
		Parameters: map[string]string{
			"limit":  limitParam,
			"offset": offsetParam,
		},
	},
	{
		Name:        "growthbook_list_environments",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook environments (production, staging, dev) with whether each is production and its default flag state. Use to find environment ids before growthbook_toggle_feature.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "growthbook_list_sdk_connections",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook SDK connections: which environment, projects and languages each client key serves, and whether payload encryption, remote evaluation or a proxy is on. Use to find which apps receive which feature flags. Encryption and proxy signing keys are never returned.",
		Parameters: map[string]string{
			"project_id": "Optional GrowthBook project id (prj_...)",
			"limit":      limitParam,
			"offset":     offsetParam,
		},
	},
	{
		Name:        "growthbook_list_saved_groups",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook saved groups (reusable audiences and targeting segments: ID lists or conditions) referenced by feature flag rules and experiment targeting. Use to resolve grp_ ids seen in rules.",
		Parameters: map[string]string{
			"limit":  limitParam,
			"offset": offsetParam,
		},
	},
	{
		Name:        "growthbook_get_saved_group",
		Access:      mcp.AccessRead,
		Description: "Get one GrowthBook saved group (audience) including its condition or list of member values. Use after growthbook_list_saved_groups to see exactly who a targeting rule includes.",
		Parameters: map[string]string{
			"saved_group_id": "Saved group id (grp_...)",
		},
		Required: []string{"saved_group_id"},
	},
	{
		Name:        "growthbook_list_attributes",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook targeting attributes (user properties such as id, country, deviceType) with data type, enum values and which one is used for hashing. Use to write or understand targeting conditions.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "growthbook_list_data_sources",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook data sources (warehouse connections) with type, identifier types and experiment assignment queries. Connection credentials are never returned. Use to understand where experiment exposure data comes from.",
		Parameters: map[string]string{
			"project_id": "Optional GrowthBook project id (prj_...)",
			"limit":      limitParam,
			"offset":     offsetParam,
		},
	},
	{
		Name:        "growthbook_list_segments",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook segments (SQL or fact-table defined user subsets) that experiments can be analyzed against.",
		Parameters: map[string]string{
			"datasource_id": "Optional data source id (ds_...)",
			"limit":         limitParam,
			"offset":        offsetParam,
		},
	},
	{
		Name:        "growthbook_list_audit_logs",
		Access:      mcp.AccessRead,
		Description: "List the GrowthBook audit log (change history) for an entity type or one entity: who changed a feature flag, experiment or metric, what event, when, and why. Use to investigate who toggled a flag or changed an experiment.",
		Parameters: map[string]string{
			"entity_type": "Entity type, e.g. feature, experiment, metric, factMetric, savedGroup, sdkConnection, project, environment",
			"entity_id":   "Optional entity id (e.g. a feature flag key); omit for every entity of the type",
			"limit":       "Entries to return 1..100 (default 20)",
			"before":      "Optional cursor: nextCursor from the previous page, or an ISO date-time",
		},
		Required: []string{"entity_type"},
	},
	{
		Name:        "growthbook_list_code_refs",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook code references: where feature flag keys appear in source code (repository, branch, file, line). Use for flag cleanup to find code still using a flag.",
		Parameters: map[string]string{
			"limit":  limitParam,
			"offset": offsetParam,
		},
	},
	{
		Name:        "growthbook_get_feature_code_refs",
		Access:      mcp.AccessRead,
		Description: "Get GrowthBook code references for one feature flag: repositories, branches, files and lines that use the flag key. Use before removing a stale flag.",
		Parameters: map[string]string{
			"feature_id": "Feature flag key (id)",
		},
		Required: []string{"feature_id"},
	},
	{
		Name:        "growthbook_get_settings",
		Access:      mcp.AccessRead,
		Description: "Get GrowthBook organization settings: feature flag key format (featureKeyExample, featureRegexValidator), statistics engine and thresholds, review requirements and kill switch confirmation. Use before growthbook_create_feature to follow the key format.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "growthbook_list_members",
		Access:      mcp.AccessRead,
		Description: "List GrowthBook organization members (users) with name, email, role, teams and project roles. Use to resolve owner ids seen on flags and experiments.",
		Parameters: map[string]string{
			"limit":  limitParam,
			"offset": offsetParam,
		},
	},
	{
		Name:        "growthbook_toggle_feature",
		Access:      mcp.AccessWrite,
		Description: "Turn a GrowthBook feature flag on or off (enable, disable, kill switch) in exactly one environment. Publishes a new revision; does not edit rules. If the API key requires confirmation, returns status pending_confirmation with a link for a person instead of applying it. Use after growthbook_get_feature to confirm the current state.",
		Parameters: map[string]string{
			"feature_id":  "Feature flag key (id)",
			"environment": "Environment id to change (e.g. production); never defaults",
			"enabled":     "true to turn the flag on in that environment, false to turn it off",
			"reason":      "Optional reason recorded in the GrowthBook audit log",
		},
		Required: []string{"feature_id", "environment", "enabled"},
	},
	{
		Name:        "growthbook_create_feature",
		Access:      mcp.AccessWrite,
		Description: "Create a GrowthBook feature flag that starts disabled in every environment, with no rules. Flag keys cannot be changed later; check growthbook_get_settings for the key format and put context in description. Use growthbook_toggle_feature afterwards to turn it on.",
		Parameters: map[string]string{
			"feature_id":    "New flag key: letters, digits and _ . : | - (kebab-case unless settings require otherwise)",
			"value_type":    "boolean, string, number or json",
			"default_value": "Default value as a string, e.g. false, control, 0 or {}",
			"description":   "Optional markdown: what the flag gates, why, rollout plan and links",
			"project_id":    "Optional GrowthBook project id (prj_...)",
			"owner":         "Optional owner email or user id; required when the API key acts as itself",
			"tags":          "Optional comma-separated tags",
		},
		Required: []string{"feature_id", "value_type", "default_value"},
	},
}
