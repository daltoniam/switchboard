package posthog

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Projects ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("posthog_list_projects"), Description: "List all projects in the PostHog organization. Start here to discover projects.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_project"), Description: "Get details of a specific project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}},
	},
	{
		Name: mcp.ToolName("posthog_create_project"), Description: "Create a new project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Project name", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_update_project"), Description: "Update a project's settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "New name"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_project"), Description: "Delete a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID", Required: true}},
	},

	// ── Feature Flags ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("posthog_list_feature_flags"), Description: "List feature flags for rollout targeting and release management. Filter by active state, type, or experiment.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("search"), Description: "Search by key or name"}, {Name: mcp.ParamName("active"), Description: "Filter by active state (true/false)"}, {Name: mcp.ParamName("type"), Description: "Filter by type: boolean, multivariant, experiment"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_feature_flag"), Description: "Get details of a specific feature flag",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("flag_id"), Description: "Feature flag ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_feature_flag"), Description: "Create a new feature flag",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("key"), Description: "Feature flag key (unique identifier)", Required: true}, {Name: mcp.ParamName("name"), Description: "Human-readable name"}, {Name: mcp.ParamName("filters"), Description: "JSON string of filter/rollout config"}, {Name: mcp.ParamName("active"), Description: "Whether flag is active (true/false)"}, {Name: mcp.ParamName("ensure_experience_continuity"), Description: "Persist flag value for users across sessions (true/false)"}},
	},
	{
		Name: mcp.ToolName("posthog_update_feature_flag"), Description: "Update a feature flag",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("flag_id"), Description: "Feature flag ID", Required: true}, {Name: mcp.ParamName("key"), Description: "Feature flag key"}, {Name: mcp.ParamName("name"), Description: "Human-readable name"}, {Name: mcp.ParamName("filters"), Description: "JSON string of filter/rollout config"}, {Name: mcp.ParamName("active"), Description: "Whether flag is active (true/false)"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_feature_flag"), Description: "Delete a feature flag (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("flag_id"), Description: "Feature flag ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_feature_flag_activity"), Description: "Get activity log for a feature flag",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("flag_id"), Description: "Feature flag ID", Required: true}, {Name: mcp.ParamName("limit"),

		// ── Cohorts ─────────────────────────────────────────────────────
		Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},

	{
		Name: mcp.ToolName("posthog_list_cohorts"), Description: "List all user cohorts for audience segmentation and analytics targeting",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_cohort"), Description: "Get details of a specific cohort",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("cohort_id"), Description: "Cohort ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_cohort"), Description: "Create a new cohort",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "Cohort name", Required: true}, {Name: mcp.ParamName("description"), Description: "Cohort description"}, {Name: mcp.ParamName("filters"), Description: "JSON string of cohort filters"}, {Name: mcp.ParamName("is_static"), Description: "Whether cohort is static (true/false)"}},
	},
	{
		Name: mcp.ToolName("posthog_update_cohort"), Description: "Update a cohort",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("cohort_id"), Description: "Cohort ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Cohort name"}, {Name: mcp.ParamName("description"), Description: "Cohort description"}, {Name: mcp.ParamName("filters"), Description: "JSON string of cohort filters"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_cohort"), Description: "Delete a cohort (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("cohort_id"), Description: "Cohort ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_list_cohort_persons"), Description: "List persons in a cohort",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("cohort_id"), Description: "Cohort ID", Required: true}, {Name: mcp.ParamName("limit"),

		// ── Insights ────────────────────────────────────────────────────
		Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},

	{
		Name: mcp.ToolName("posthog_list_insights"), Description: "List saved product analytics insights (trends, funnels, retention, etc.). View reports, charts, and metrics.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("search"), Description: "Search by name"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}, {Name: mcp.ParamName("created_by"), Description: "Filter by creator user ID"}},
	},
	{
		Name: mcp.ToolName("posthog_get_insight"), Description: "Get details of a specific product analytics insight, including chart data for trends, funnels, and retention",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("insight_id"), Description: "Insight ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_insight"), Description: "Create a new insight",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "Insight name"}, {Name: mcp.ParamName("description"), Description: "Insight description"}, {Name: mcp.ParamName("filters"), Description: "JSON string of insight filters (events, actions, properties, date ranges)"}, {Name: mcp.ParamName("query"), Description: "JSON string of HogQL query definition"}},
	},
	{
		Name: mcp.ToolName("posthog_update_insight"), Description: "Update an insight",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("insight_id"), Description: "Insight ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Insight name"}, {Name: mcp.ParamName("description"), Description: "Insight description"}, {Name: mcp.ParamName("filters"), Description: "JSON string of insight filters"}, {Name: mcp.ParamName("query"), Description: "JSON string of HogQL query definition"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_insight"), Description: "Delete an insight (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("insight_id"), Description: "Insight ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_query"), Description: "Run a HogQL (SQL-like) query synchronously and return inline results. Use this for ad-hoc analytics without persisting an insight. Returns columns, results rows, and execution metadata.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("query"), Description: "HogQL query text, e.g. SELECT count() FROM events WHERE timestamp > now() - INTERVAL 7 DAY", Required: true}, {Name: mcp.ParamName("client_query_id"), Description: "Optional client-supplied identifier to correlate the request in PostHog logs"}, {Name: mcp.ParamName("refresh"), Description: "Optional cache mode: 'blocking' (default-ish, wait for fresh result), 'force_blocking', 'lazy_async', 'force_cache'"}},
	},

	// ── Persons ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("posthog_list_persons"), Description: "List persons (users and customers) tracked by PostHog analytics. Search by email, distinct ID, or visitor profile.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("search"), Description: "Search by email or distinct ID"}, {Name: mcp.ParamName("distinct_id"), Description: "Filter by exact distinct ID"}, {Name: mcp.ParamName("email"), Description: "Filter by email"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_person"), Description: "Get details of a specific person",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("person_id"), Description: "Person UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_delete_person"), Description: "Delete a person and their data",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("person_id"), Description: "Person UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_update_person_property"), Description: "Update a property on a person",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("person_id"), Description: "Person UUID", Required: true}, {Name: mcp.ParamName("key"), Description: "Property key", Required: true}, {Name: mcp.ParamName("value"), Description: "Property value", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_delete_person_property"), Description: "Delete a property from a person",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("person_id"), Description: "Person UUID", Required: true}, {Name: mcp.

		// ── Groups ──────────────────────────────────────────────────────
		ParamName("key"), Description: "Property key to remove", Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_groups"), Description: "List groups",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("group_type_index"), Description: "Group type index (0-based)"}, {Name: mcp.ParamName("search"), Description: "Search by group key or properties"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("posthog_find_group"), Description: "Find a specific group by key",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("group_type_index"), Description: "Group type index (0-based)", Required: true}, {Name: mcp.ParamName("group_key"),

		// ── Annotations ─────────────────────────────────────────────────
		Description: "Group key to find", Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_annotations"), Description: "List annotations (markers on charts)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("search"), Description: "Search by content"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_annotation"), Description: "Get details of a specific annotation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("annotation_id"), Description: "Annotation ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_annotation"), Description: "Create a new annotation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("content"), Description: "Annotation text", Required: true}, {Name: mcp.ParamName("date_marker"), Description: "ISO 8601 date for the annotation marker", Required: true}, {Name: mcp.ParamName("scope"), Description: "Scope: dashboard_item, project, organization"}},
	},
	{
		Name: mcp.ToolName("posthog_update_annotation"), Description: "Update an annotation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("annotation_id"), Description: "Annotation ID", Required: true}, {Name: mcp.ParamName("content"), Description: "Annotation text"}, {Name: mcp.ParamName("date_marker"), Description: "ISO 8601 date"}, {Name: mcp.ParamName("scope"), Description: "Scope: dashboard_item, project, organization"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_annotation"), Description: "Delete an annotation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("annotation_id"), Description: "Annotation ID",

		// ── Dashboards ──────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_dashboards"), Description: "List PostHog product analytics dashboards for metrics overview and monitoring",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_dashboard"), Description: "Get details of a specific dashboard",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_dashboard"), Description: "Create a new dashboard",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "Dashboard name", Required: true}, {Name: mcp.ParamName("description"), Description: "Dashboard description"}, {Name: mcp.ParamName("pinned"), Description: "Pin dashboard (true/false)"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}},
	},
	{
		Name: mcp.ToolName("posthog_update_dashboard"), Description: "Update a dashboard",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Dashboard name"}, {Name: mcp.ParamName("description"), Description: "Dashboard description"}, {Name: mcp.ParamName("pinned"), Description: "Pin dashboard (true/false)"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_dashboard"), Description: "Delete a dashboard (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID",

		// ── Actions ─────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_actions"), Description: "List actions (custom event groupings) for analytics tracking and conversion metrics",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_action"), Description: "Get details of a specific action",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("action_id"), Description: "Action ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_action"), Description: "Create a new action",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "Action name", Required: true}, {Name: mcp.ParamName("description"), Description: "Action description"}, {Name: mcp.ParamName("steps"), Description: "JSON array of action step definitions"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}},
	},
	{
		Name: mcp.ToolName("posthog_update_action"), Description: "Update an action",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("action_id"), Description: "Action ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Action name"}, {Name: mcp.ParamName("description"), Description: "Action description"}, {Name: mcp.ParamName("steps"), Description: "JSON array of action step definitions"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_action"), Description: "Delete an action (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("action_id"), Description:

		// ── Events ──────────────────────────────────────────────────────
		"Action ID", Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_events"), Description: "List captured product analytics events. View user behavior tracking and activity data.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("event"), Description: "Filter by event name"}, {Name: mcp.ParamName("person_id"), Description: "Filter by person UUID"}, {Name: mcp.ParamName("distinct_id"), Description: "Filter by distinct ID"}, {Name: mcp.ParamName("properties"), Description: "JSON string of property filters"}, {Name: mcp.ParamName("before"), Description: "ISO 8601 timestamp upper bound"}, {Name: mcp.ParamName("after"), Description: "ISO 8601 timestamp lower bound"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_event"), Description: "Get details of a specific event",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("event_id"), Description:

		// ── Experiments ─────────────────────────────────────────────────
		"Event UUID", Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_experiments"), Description: "List A/B test experiments for conversion optimization. View variant results and analytics.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_experiment"), Description: "Get details of a specific experiment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("experiment_id"), Description: "Experiment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_experiment"), Description: "Create a new experiment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "Experiment name", Required: true}, {Name: mcp.ParamName("description"), Description: "Experiment description"}, {Name: mcp.ParamName("feature_flag_key"), Description: "Feature flag key to use for experiment", Required: true}, {Name: mcp.ParamName("start_date"), Description: "ISO 8601 start date"}, {Name: mcp.ParamName("end_date"), Description: "ISO 8601 end date"}, {Name: mcp.ParamName("filters"), Description: "JSON string of experiment filters"}},
	},
	{
		Name: mcp.ToolName("posthog_update_experiment"), Description: "Update an experiment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("experiment_id"), Description: "Experiment ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Experiment name"}, {Name: mcp.ParamName("description"), Description: "Experiment description"}, {Name: mcp.ParamName("start_date"), Description: "ISO 8601 start date"}, {Name: mcp.ParamName("end_date"), Description: "ISO 8601 end date"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_experiment"), Description: "Delete an experiment (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("experiment_id"), Description: "Experiment ID",

		// ── Surveys ─────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("posthog_list_surveys"), Description: "List surveys",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("limit"), Description: "Max results"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("posthog_get_survey"), Description: "Get details of a specific survey",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("survey_id"), Description: "Survey ID", Required: true}},
	},
	{
		Name: mcp.ToolName("posthog_create_survey"), Description: "Create a new survey",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("name"), Description: "Survey name", Required: true}, {Name: mcp.ParamName("description"), Description: "Survey description"}, {Name: mcp.ParamName("type"), Description: "Survey type"}, {Name: mcp.ParamName("questions"), Description: "JSON array of question definitions"}, {Name: mcp.ParamName("targeting_flag_filters"), Description: "JSON string of targeting filters"}},
	},
	{
		Name: mcp.ToolName("posthog_update_survey"), Description: "Update a survey",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("survey_id"), Description: "Survey ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Survey name"}, {Name: mcp.ParamName("description"), Description: "Survey description"}, {Name: mcp.ParamName("questions"), Description: "JSON array of question definitions"}},
	},
	{
		Name: mcp.ToolName("posthog_delete_survey"), Description: "Delete a survey (soft delete)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project ID (uses default if configured, otherwise required)"}, {Name: mcp.ParamName("survey_id"), Description: "Survey ID", Required: true}},
	},
}
