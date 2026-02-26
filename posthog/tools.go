package posthog

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Projects ────────────────────────────────────────────────────
	{
		Name: "posthog_list_projects", Description: "List all projects in the PostHog organization",
		Parameters: map[string]string{"limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_project", Description: "Get details of a specific project",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)"},
	},
	{
		Name: "posthog_create_project", Description: "Create a new project",
		Parameters: map[string]string{"name": "Project name"},
		Required:   []string{"name"},
	},
	{
		Name: "posthog_update_project", Description: "Update a project's settings",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "New name"},
	},
	{
		Name: "posthog_delete_project", Description: "Delete a project",
		Parameters: map[string]string{"project_id": "Project ID"},
		Required:   []string{"project_id"},
	},

	// ── Feature Flags ───────────────────────────────────────────────
	{
		Name: "posthog_list_feature_flags", Description: "List feature flags",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "search": "Search by key or name", "active": "Filter by active state (true/false)", "type": "Filter by type: boolean, multivariant, experiment", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_feature_flag", Description: "Get details of a specific feature flag",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "flag_id": "Feature flag ID"},
		Required:   []string{"flag_id"},
	},
	{
		Name: "posthog_create_feature_flag", Description: "Create a new feature flag",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "key": "Feature flag key (unique identifier)", "name": "Human-readable name", "filters": "JSON string of filter/rollout config", "active": "Whether flag is active (true/false)", "ensure_experience_continuity": "Persist flag value for users across sessions (true/false)"},
		Required:   []string{"key"},
	},
	{
		Name: "posthog_update_feature_flag", Description: "Update a feature flag",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "flag_id": "Feature flag ID", "key": "Feature flag key", "name": "Human-readable name", "filters": "JSON string of filter/rollout config", "active": "Whether flag is active (true/false)"},
		Required:   []string{"flag_id"},
	},
	{
		Name: "posthog_delete_feature_flag", Description: "Delete a feature flag (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "flag_id": "Feature flag ID"},
		Required:   []string{"flag_id"},
	},
	{
		Name: "posthog_feature_flag_activity", Description: "Get activity log for a feature flag",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "flag_id": "Feature flag ID", "limit": "Max results", "offset": "Pagination offset"},
		Required:   []string{"flag_id"},
	},

	// ── Cohorts ─────────────────────────────────────────────────────
	{
		Name: "posthog_list_cohorts", Description: "List all cohorts",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_cohort", Description: "Get details of a specific cohort",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "cohort_id": "Cohort ID"},
		Required:   []string{"cohort_id"},
	},
	{
		Name: "posthog_create_cohort", Description: "Create a new cohort",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "Cohort name", "description": "Cohort description", "filters": "JSON string of cohort filters", "is_static": "Whether cohort is static (true/false)"},
		Required:   []string{"name"},
	},
	{
		Name: "posthog_update_cohort", Description: "Update a cohort",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "cohort_id": "Cohort ID", "name": "Cohort name", "description": "Cohort description", "filters": "JSON string of cohort filters"},
		Required:   []string{"cohort_id"},
	},
	{
		Name: "posthog_delete_cohort", Description: "Delete a cohort (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "cohort_id": "Cohort ID"},
		Required:   []string{"cohort_id"},
	},
	{
		Name: "posthog_list_cohort_persons", Description: "List persons in a cohort",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "cohort_id": "Cohort ID", "limit": "Max results", "offset": "Pagination offset"},
		Required:   []string{"cohort_id"},
	},

	// ── Insights ────────────────────────────────────────────────────
	{
		Name: "posthog_list_insights", Description: "List saved insights (trends, funnels, etc.)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "search": "Search by name", "limit": "Max results", "offset": "Pagination offset", "created_by": "Filter by creator user ID"},
	},
	{
		Name: "posthog_get_insight", Description: "Get details of a specific insight",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "insight_id": "Insight ID"},
		Required:   []string{"insight_id"},
	},
	{
		Name: "posthog_create_insight", Description: "Create a new insight",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "Insight name", "description": "Insight description", "filters": "JSON string of insight filters (events, actions, properties, date ranges)", "query": "JSON string of HogQL query definition"},
	},
	{
		Name: "posthog_update_insight", Description: "Update an insight",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "insight_id": "Insight ID", "name": "Insight name", "description": "Insight description", "filters": "JSON string of insight filters", "query": "JSON string of HogQL query definition"},
		Required:   []string{"insight_id"},
	},
	{
		Name: "posthog_delete_insight", Description: "Delete an insight (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "insight_id": "Insight ID"},
		Required:   []string{"insight_id"},
	},

	// ── Persons ─────────────────────────────────────────────────────
	{
		Name: "posthog_list_persons", Description: "List persons (users) tracked by PostHog",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "search": "Search by email or distinct ID", "distinct_id": "Filter by exact distinct ID", "email": "Filter by email", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_person", Description: "Get details of a specific person",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "person_id": "Person UUID"},
		Required:   []string{"person_id"},
	},
	{
		Name: "posthog_delete_person", Description: "Delete a person and their data",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "person_id": "Person UUID"},
		Required:   []string{"person_id"},
	},
	{
		Name: "posthog_update_person_property", Description: "Update a property on a person",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "person_id": "Person UUID", "key": "Property key", "value": "Property value"},
		Required:   []string{"person_id", "key", "value"},
	},
	{
		Name: "posthog_delete_person_property", Description: "Delete a property from a person",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "person_id": "Person UUID", "key": "Property key to remove"},
		Required:   []string{"person_id", "key"},
	},

	// ── Groups ──────────────────────────────────────────────────────
	{
		Name: "posthog_list_groups", Description: "List groups",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "group_type_index": "Group type index (0-based)", "search": "Search by group key or properties", "cursor": "Pagination cursor"},
	},
	{
		Name: "posthog_find_group", Description: "Find a specific group by key",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "group_type_index": "Group type index (0-based)", "group_key": "Group key to find"},
		Required:   []string{"group_type_index", "group_key"},
	},

	// ── Annotations ─────────────────────────────────────────────────
	{
		Name: "posthog_list_annotations", Description: "List annotations (markers on charts)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "search": "Search by content", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_annotation", Description: "Get details of a specific annotation",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "annotation_id": "Annotation ID"},
		Required:   []string{"annotation_id"},
	},
	{
		Name: "posthog_create_annotation", Description: "Create a new annotation",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "content": "Annotation text", "date_marker": "ISO 8601 date for the annotation marker", "scope": "Scope: dashboard_item, project, organization"},
		Required:   []string{"content", "date_marker"},
	},
	{
		Name: "posthog_update_annotation", Description: "Update an annotation",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "annotation_id": "Annotation ID", "content": "Annotation text", "date_marker": "ISO 8601 date", "scope": "Scope: dashboard_item, project, organization"},
		Required:   []string{"annotation_id"},
	},
	{
		Name: "posthog_delete_annotation", Description: "Delete an annotation",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "annotation_id": "Annotation ID"},
		Required:   []string{"annotation_id"},
	},

	// ── Dashboards ──────────────────────────────────────────────────
	{
		Name: "posthog_list_dashboards", Description: "List dashboards",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_dashboard", Description: "Get details of a specific dashboard",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "dashboard_id": "Dashboard ID"},
		Required:   []string{"dashboard_id"},
	},
	{
		Name: "posthog_create_dashboard", Description: "Create a new dashboard",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "Dashboard name", "description": "Dashboard description", "pinned": "Pin dashboard (true/false)", "tags": "Comma-separated tags"},
		Required:   []string{"name"},
	},
	{
		Name: "posthog_update_dashboard", Description: "Update a dashboard",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "dashboard_id": "Dashboard ID", "name": "Dashboard name", "description": "Dashboard description", "pinned": "Pin dashboard (true/false)", "tags": "Comma-separated tags"},
		Required:   []string{"dashboard_id"},
	},
	{
		Name: "posthog_delete_dashboard", Description: "Delete a dashboard (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "dashboard_id": "Dashboard ID"},
		Required:   []string{"dashboard_id"},
	},

	// ── Actions ─────────────────────────────────────────────────────
	{
		Name: "posthog_list_actions", Description: "List actions (event groupings)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_action", Description: "Get details of a specific action",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "action_id": "Action ID"},
		Required:   []string{"action_id"},
	},
	{
		Name: "posthog_create_action", Description: "Create a new action",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "Action name", "description": "Action description", "steps": "JSON array of action step definitions", "tags": "Comma-separated tags"},
		Required:   []string{"name"},
	},
	{
		Name: "posthog_update_action", Description: "Update an action",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "action_id": "Action ID", "name": "Action name", "description": "Action description", "steps": "JSON array of action step definitions", "tags": "Comma-separated tags"},
		Required:   []string{"action_id"},
	},
	{
		Name: "posthog_delete_action", Description: "Delete an action (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "action_id": "Action ID"},
		Required:   []string{"action_id"},
	},

	// ── Events ──────────────────────────────────────────────────────
	{
		Name: "posthog_list_events", Description: "List captured events",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "event": "Filter by event name", "person_id": "Filter by person UUID", "distinct_id": "Filter by distinct ID", "properties": "JSON string of property filters", "before": "ISO 8601 timestamp upper bound", "after": "ISO 8601 timestamp lower bound", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_event", Description: "Get details of a specific event",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "event_id": "Event UUID"},
		Required:   []string{"event_id"},
	},

	// ── Experiments ─────────────────────────────────────────────────
	{
		Name: "posthog_list_experiments", Description: "List A/B test experiments",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_experiment", Description: "Get details of a specific experiment",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "experiment_id": "Experiment ID"},
		Required:   []string{"experiment_id"},
	},
	{
		Name: "posthog_create_experiment", Description: "Create a new experiment",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "Experiment name", "description": "Experiment description", "feature_flag_key": "Feature flag key to use for experiment", "start_date": "ISO 8601 start date", "end_date": "ISO 8601 end date", "filters": "JSON string of experiment filters"},
		Required:   []string{"name", "feature_flag_key"},
	},
	{
		Name: "posthog_update_experiment", Description: "Update an experiment",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "experiment_id": "Experiment ID", "name": "Experiment name", "description": "Experiment description", "start_date": "ISO 8601 start date", "end_date": "ISO 8601 end date"},
		Required:   []string{"experiment_id"},
	},
	{
		Name: "posthog_delete_experiment", Description: "Delete an experiment (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "experiment_id": "Experiment ID"},
		Required:   []string{"experiment_id"},
	},

	// ── Surveys ─────────────────────────────────────────────────────
	{
		Name: "posthog_list_surveys", Description: "List surveys",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "limit": "Max results", "offset": "Pagination offset"},
	},
	{
		Name: "posthog_get_survey", Description: "Get details of a specific survey",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "survey_id": "Survey ID"},
		Required:   []string{"survey_id"},
	},
	{
		Name: "posthog_create_survey", Description: "Create a new survey",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "name": "Survey name", "description": "Survey description", "type": "Survey type", "questions": "JSON array of question definitions", "targeting_flag_filters": "JSON string of targeting filters"},
		Required:   []string{"name"},
	},
	{
		Name: "posthog_update_survey", Description: "Update a survey",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "survey_id": "Survey ID", "name": "Survey name", "description": "Survey description", "questions": "JSON array of question definitions"},
		Required:   []string{"survey_id"},
	},
	{
		Name: "posthog_delete_survey", Description: "Delete a survey (soft delete)",
		Parameters: map[string]string{"project_id": "Project ID (defaults to configured project)", "survey_id": "Survey ID"},
		Required:   []string{"survey_id"},
	},
}
