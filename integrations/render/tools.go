package render

import mcp "github.com/daltoniam/switchboard"

// Parameter descriptions shared by several upstream tools.
const (
	paramWorkspace     = "Render workspace ID (tea-... or usr-...) from render_list_workspaces. Defaults to the workspace_id credential when it is set."
	paramServiceID     = "Render service ID (srv-... for web services, crn-... for cron jobs) from render_list_services."
	paramPostgresID    = "Render Postgres instance ID (dpg-...) from render_list_postgres_instances."
	paramKeyValueID    = "Render Key Value instance ID (red-...) from render_list_key_value."
	paramBlueprintID   = "Render Blueprint ID (exs-...), not a sync ID."
	paramCursor        = "Cursor from the previous page; omit for the first page."
	paramPageLimit     = "Maximum records on this page, 1 to 100."
	paramWorkflowID    = "Render workflow ID from render_list_workflows."
	paramWorkflowVerID = "Workflow version ID from render_list_workflow_versions."
	paramFlowRunID     = "Flow run ID from render_list_flow_runs or render_start_flow_run."
	paramTaskRunID     = "Task run ID from render_list_workflow_runs."
	paramTaskSlug      = "Filter by workflow-slug/task-name, optionally with :version."
	paramFlowSlug      = "Filter by workflow-slug/flow-name, optionally with :version."
	paramResultOffset  = "UTF-8 byte offset; pass nextOffset from the previous page."
	paramResultLimit   = "Maximum result bytes in this page."
	paramResultSHA     = "Result hash from the first page; required when offset is greater than zero."
	paramEnvVars       = "Environment variables as an array of {key, value} objects."
	paramRepo          = "Git repository URL Render can clone, without the branch (pass branch separately)."
	paramBranch        = "Repository branch to deploy; defaults to the repository's default branch."
	paramRegion        = "Region: oregon (default), frankfurt, singapore, ohio, or virginia."
	paramAutoDeploy    = "yes (default) to deploy automatically when the branch updates, or no."
	paramRuntime       = "Runtime: node, python, go, rust, ruby, elixir, or docker."
	paramDockerCommand = "Overrides the image's start command when runtime is docker."
	paramDockerContext = "Docker build context relative to the repository root when runtime is docker (default .)."
	paramDockerfile    = "Dockerfile path relative to the repository root when runtime is docker (default ./Dockerfile)."
	paramLogStart      = "Start time in RFC3339; defaults to 1 hour ago and must be within the last 30 days."
	paramLogEnd        = "End time in RFC3339; defaults to now."
	paramLogDirection  = "backward (newest first, default) or forward (oldest first)."
)

func withWorkspace(params map[string]string) map[string]string {
	params["workspaceId"] = paramWorkspace
	return params
}

// tools is a reviewed copy of the hosted Render MCP catalog
// (https://mcp.render.com/mcp, source github.com/render-oss/render-mcp-server).
// select_workspace and get_selected_workspace are left out on purpose: they
// keep a workspace in upstream session state, which one Switchboard session
// shares across callers. Pass workspaceId, or set the workspace_id credential.
var tools = []mcp.ToolDefinition{
	// Workspaces and services
	{
		Name:        "render_list_services",
		Description: "List Render services (web services, static sites, private services, background workers, cron jobs) in a Render workspace, with type, region, plan, URL, branch and suspended state. Start here for Render cloud hosting: find service IDs before checking deploys, logs, metrics, events or environment variables.",
		Parameters: withWorkspace(map[string]string{
			"includePreviews": "Include pull request preview services (default false).",
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_list_workspaces",
		Description: "List Render workspaces (owners and teams) the API key can access, with their IDs. Use first when the user has several Render workspaces, then pass workspaceId to other Render tools.",
		Parameters:  map[string]string{},
		Access:      mcp.AccessRead,
	},
	{
		Name:        "render_get_service",
		Description: "Get details of one Render service: type, repo, branch, runtime, plan, region, instance count, URL, health check path, autoscaling and suspension. Use after render_list_services.",
		Parameters: withWorkspace(map[string]string{
			"serviceId": paramServiceID,
		}),
		Required: []string{"serviceId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_create_web_service",
		Description: "Create a public Render web service from a Git repository (Node, Python, Go, Rust, Ruby, Elixir or Docker). Auto-deploys on branch updates by default. Supports a subset of Render settings; prebuilt registry images are not supported.",
		Parameters: withWorkspace(map[string]string{
			"name":           "Unique service name; also used for its onrender.com URL.",
			"runtime":        paramRuntime,
			"repo":           paramRepo,
			"branch":         paramBranch,
			"buildCommand":   "Build command, for example npm run build. Required unless runtime is docker.",
			"startCommand":   "Start command, for example npm start. Required unless runtime is docker.",
			"plan":           "Instance plan, for example free, starter, standard, pro, pro_plus, pro_max or pro_ultra.",
			"region":         paramRegion,
			"autoDeploy":     paramAutoDeploy,
			"envVars":        paramEnvVars,
			"dockerCommand":  paramDockerCommand,
			"dockerContext":  paramDockerContext,
			"dockerfilePath": paramDockerfile,
		}),
		Required: []string{"name", "runtime"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_create_static_site",
		Description: "Create a Render static site (React, Vue, Gatsby and other built HTML, CSS, JS) from a Git repository, served on a global CDN with an onrender.com subdomain.",
		Parameters: withWorkspace(map[string]string{
			"name":         "Unique site name; also used for its onrender.com URL.",
			"buildCommand": "Build command, for example yarn; yarn build.",
			"publishPath":  "Directory of built assets to publish, for example ./build or dist.",
			"repo":         paramRepo,
			"branch":       paramBranch,
			"autoDeploy":   paramAutoDeploy,
			"envVars":      paramEnvVars,
		}),
		Required: []string{"name", "buildCommand"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_create_cron_job",
		Description: "Create a Render cron job: a scheduled task that runs a command on a cron schedule (cleanup, reports, data processing) from a Git repository.",
		Parameters: withWorkspace(map[string]string{
			"name":           "Unique cron job name.",
			"schedule":       "Five-field cron expression, for example 0 * * * * for hourly.",
			"runtime":        paramRuntime,
			"repo":           paramRepo,
			"branch":         paramBranch,
			"buildCommand":   "Build command. Required unless runtime is docker.",
			"startCommand":   "Command each run executes. Required unless runtime is docker.",
			"plan":           "Instance plan, for example starter, standard or pro.",
			"region":         paramRegion,
			"autoDeploy":     paramAutoDeploy,
			"envVars":        paramEnvVars,
			"dockerCommand":  paramDockerCommand,
			"dockerContext":  paramDockerContext,
			"dockerfilePath": paramDockerfile,
		}),
		Required: []string{"name", "schedule", "runtime"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_update_environment_variables",
		Description: "Set or update environment variables (env vars, config, secrets) on a Render service. Merges with existing variables by default; replace=true deletes every variable not in the list. Saving starts a new deploy.",
		Parameters: withWorkspace(map[string]string{
			"serviceId": paramServiceID,
			"envVars":   paramEnvVars,
			"replace":   "true to replace all existing variables with this list; false (default) merges.",
		}),
		Required: []string{"serviceId", "envVars"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        "render_run_cron_job",
		Description: "Run a Render cron job now, outside its schedule, using its currently deployed build. Refuses while a run is in progress unless cancelActiveRun is true. Use render_trigger_deploy first if code changed.",
		Parameters: withWorkspace(map[string]string{
			"serviceId":       "Cron job ID (crn-...) from render_list_services.",
			"cancelActiveRun": "true to cancel an in-progress run so this one can start (default false). Only after the user agrees.",
		}),
		Required: []string{"serviceId"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        "render_cancel_cron_job_run",
		Description: "Cancel the in-progress run of a Render cron job. Stops the run partway; the schedule continues. Confirm with the user first.",
		Parameters: withWorkspace(map[string]string{
			"serviceId": "Cron job ID (crn-...) whose run to cancel.",
		}),
		Required: []string{"serviceId"},
		Access:   mcp.AccessDestructive,
	},

	// Deploys and events
	{
		Name:        "render_list_deploys",
		Description: "List deploys (deployments, releases) of a Render service, newest first, with status, trigger, commit and timestamps. Use to check whether a Render deploy succeeded, failed or is still building. Returns items and next_cursor for older pages.",
		Parameters: withWorkspace(map[string]string{
			"serviceId": paramServiceID,
			"limit":     "Deploys per page (default 20).",
			"cursor":    "next_cursor from the previous page; omit for the newest deploys.",
		}),
		Required: []string{"serviceId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_get_deploy",
		Description: "Get one Render deploy's status, trigger, commit, image and start and finish times. Use after render_list_deploys; read build logs with render_list_logs type build.",
		Parameters: withWorkspace(map[string]string{
			"serviceId": paramServiceID,
			"deployId":  "Deploy ID (dep-...) from render_list_deploys.",
		}),
		Required: []string{"serviceId", "deployId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_trigger_deploy",
		Description: "Trigger a new Render deploy (redeploy, release) of a service, optionally clearing the build cache. Also how to restart a Render service: a redeploy restarts its instances on a fresh build. Not needed after a git push when autoDeploy is on.",
		Parameters: withWorkspace(map[string]string{
			"serviceId":  paramServiceID,
			"clearCache": "true to clear the build cache first (default false).",
		}),
		Required: []string{"serviceId"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        "render_list_events",
		Description: "List a Render service's event history: deploys, builds, restarts, crashes, failures (out of memory, non-zero exit, failed health checks), scaling, suspensions and disk changes, newest first. Use to debug why a Render service went down. First page covers 7 days; returns items and next_cursor.",
		Parameters: withWorkspace(map[string]string{
			"serviceId":  paramServiceID,
			"eventTypes": "Array of event types, for example server_failed, deploy_ended, build_ended, image_pull_failed, server_restarted.",
			"startTime":  "Start of the range in RFC3339; defaults to 7 days ago on the first page.",
			"endTime":    "End of the range in RFC3339; defaults to now.",
			"limit":      "Maximum events to return.",
			"cursor":     "next_cursor from the previous page.",
		}),
		Required: []string{"serviceId"},
		Access:   mcp.AccessRead,
	},

	// Logs and metrics
	{
		Name:        "render_list_logs",
		Description: "Search Render logs (application, request and build logs) for services, cron jobs, Postgres, Key Value and workflows, filtered by text, level, status code, path, host, instance and time. Start here for Render log debugging. When hasMore is true, pass nextStartTime and nextEndTime as startTime and endTime.",
		Parameters: withWorkspace(map[string]string{
			"resource":   "Array of resource IDs (srv-, crn-, dpg-, red- or workflow IDs). All must share a region and owner.",
			"text":       "Array of text filters; wildcards and regex are supported.",
			"level":      "Array of severity levels, for example error or warn.",
			"type":       "Array of log types: app, request or build.",
			"statusCode": "Array of HTTP status code filters for request logs, for example 5*.",
			"path":       "Array of request path filters.",
			"host":       "Array of request host filters.",
			"method":     "Array of request method filters.",
			"instance":   "Array of instance IDs.",
			"task":       "Array of workflow task or flow IDs; requires the workflow ID in resource.",
			"taskRun":    "Array of workflow task run or flow run IDs; requires the workflow ID in resource.",
			"startTime":  paramLogStart,
			"endTime":    paramLogEnd,
			"direction":  paramLogDirection,
			"limit":      "Maximum log lines to return.",
		}),
		Required: []string{"resource"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_list_log_label_values",
		Description: "List the values a Render log label takes (host, instance, level, method, statusCode or type) for given resources, to discover filters for render_list_logs.",
		Parameters: withWorkspace(map[string]string{
			"label":      "Label to list values for: host, instance, level, method, statusCode or type.",
			"resource":   "Array of resource IDs (srv-, crn-, dpg-, red-).",
			"text":       "Array of text filters.",
			"level":      "Array of severity levels.",
			"type":       "Array of log types.",
			"statusCode": "Array of HTTP status code filters.",
			"path":       "Array of request path filters.",
			"host":       "Array of request host filters.",
			"method":     "Array of request method filters.",
			"instance":   "Array of instance IDs.",
			"startTime":  paramLogStart,
			"endTime":    paramLogEnd,
			"direction":  paramLogDirection,
		}),
		Required: []string{"label", "resource"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_get_metrics",
		Description: "Get Render performance metrics time series for a service, Postgres or Key Value instance: CPU and memory usage, limits and targets, instance count, HTTP request counts, latency (p50, p95, p99), bandwidth and database connections. Use for Render monitoring, capacity planning and slow response debugging.",
		Parameters: withWorkspace(map[string]string{
			"resourceId":                   "Service, Postgres or Key Value ID.",
			"metricTypes":                  "Array of metrics: cpu_usage, cpu_limit, cpu_target, memory_usage, memory_limit, memory_target, instance_count, http_request_count, http_latency, bandwidth_usage, active_connections.",
			"startTime":                    "Start in RFC3339; defaults to 1 hour ago, within the last 30 days.",
			"endTime":                      "End in RFC3339; defaults to now.",
			"resolution":                   "Seconds between data points (default 60).",
			"cpuUsageAggregationMethod":    "AVG (default), MAX or MIN for CPU usage.",
			"aggregateHttpRequestCountsBy": "host or statusCode, for http_request_count only.",
			"httpLatencyQuantile":          "Latency quantile such as 0.5, 0.95 (default) or 0.99.",
			"httpHost":                     "Filter HTTP metrics to one request host.",
			"httpPath":                     "Filter HTTP metrics to one request path.",
		}),
		Required: []string{"resourceId", "metricTypes"},
		Access:   mcp.AccessRead,
	},

	// Postgres and Key Value
	{
		Name:        "render_list_postgres_instances",
		Description: "List Render Postgres databases (managed PostgreSQL instances) in a workspace with status, plan, region, version and database name.",
		Parameters:  withWorkspace(map[string]string{}),
		Access:      mcp.AccessRead,
	},
	{
		Name:        "render_get_postgres",
		Description: "Get one Render Postgres database's details: status, plan, version, disk size, high availability, read replicas and maintenance. Use after render_list_postgres_instances.",
		Parameters: withWorkspace(map[string]string{
			"postgresId": paramPostgresID,
		}),
		Required: []string{"postgresId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_query_postgres",
		Description: "Run a read-only SQL query against a Render Postgres database. The query runs in a read-only transaction on a new connection that closes afterward.",
		Parameters: withWorkspace(map[string]string{
			"postgresId": paramPostgresID,
			"sql":        "SQL query to run; it is wrapped in a read-only transaction.",
		}),
		Required: []string{"postgresId", "sql"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_create_postgres",
		Description: "Create a Render Postgres database (managed PostgreSQL). Paid plans bill immediately.",
		Parameters: withWorkspace(map[string]string{
			"name":       "Database name shown in the Render Dashboard.",
			"plan":       "Plan, for example free, basic_256mb, basic_1gb, pro_4gb or accelerated_16gb.",
			"version":    "PostgreSQL major version, for example 16.",
			"region":     paramRegion,
			"diskSizeGb": "Storage in GB: 1 or a multiple of 5. It can grow later but never shrink.",
		}),
		Required: []string{"name"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_list_key_value",
		Description: "List Render Key Value instances (Redis-compatible Valkey cache and queue stores) in a workspace with status, plan, region and version.",
		Parameters:  withWorkspace(map[string]string{}),
		Access:      mcp.AccessRead,
	},
	{
		Name:        "render_get_key_value",
		Description: "Get one Render Key Value (Redis-compatible) instance's details: status, plan, version, eviction policy and persistence. Use after render_list_key_value.",
		Parameters: withWorkspace(map[string]string{
			"keyValueId": paramKeyValueID,
		}),
		Required: []string{"keyValueId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_create_key_value",
		Description: "Create a Render Key Value instance (Redis-compatible cache or queue store). Paid plans bill immediately.",
		Parameters: withWorkspace(map[string]string{
			"name":            "Instance name.",
			"plan":            "Plan, for example free, starter, standard, pro or pro_plus.",
			"region":          paramRegion,
			"maxmemoryPolicy": "Eviction policy: noeviction, allkeys_lru, allkeys_lfu, allkeys_random, volatile_lru, volatile_lfu, volatile_random or volatile_ttl.",
			"persistenceMode": "Persistence: journal_snapshot, snapshot or off.",
		}),
		Required: []string{"name"},
		Access:   mcp.AccessWrite,
	},

	// Blueprints (infrastructure as code, render.yaml)
	{
		Name:        "render_get_blueprint",
		Description: "Get a Render Blueprint (render.yaml infrastructure as code): repo, branch, file path, auto-sync, status, last sync and managed resource IDs. Early access upstream.",
		Parameters: withWorkspace(map[string]string{
			"blueprintId": paramBlueprintID,
		}),
		Required: []string{"blueprintId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_create_blueprint",
		Description: "Create a Render Blueprint from a render.yaml file in a Git repository and plan its first sync. Nothing is created until render_approve_blueprint. Early access upstream.",
		Parameters: withWorkspace(map[string]string{
			"repoUrl":           "https:// or ssh:// repository URL without query or fragment; pass the branch separately.",
			"branch":            "Branch containing the Blueprint file; defaults to the repository's default branch.",
			"path":              "Blueprint file path relative to the repository root (default render.yaml).",
			"name":              "Blueprint name; defaults to the repository name.",
			"autoSync":          "Apply later pushes automatically after the first approved sync (default true).",
			"existingResources": "create_new (default) creates separate resources; adopt takes over matching existing resources.",
		}),
		Required: []string{"repoUrl"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_plan_blueprint",
		Description: "Plan (preview) a Render Blueprint sync from its configured branch and render.yaml without applying changes. Review the plan with the user before render_approve_blueprint. Early access upstream.",
		Parameters: withWorkspace(map[string]string{
			"blueprintId": paramBlueprintID,
		}),
		Required: []string{"blueprintId"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_approve_blueprint",
		Description: "Approve and apply a Render Blueprint's newest planned sync, which can create and update infrastructure. Only after the user authorizes the reviewed plan from render_plan_blueprint. Early access upstream.",
		Parameters: withWorkspace(map[string]string{
			"blueprintId":   paramBlueprintID,
			"manualEnvVars": "Values for the variables listed in sync.requiredManualEnvVars, as {key, serviceName, serviceType, value} objects copied from those entries.",
		}),
		Required: []string{"blueprintId"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        "render_update_blueprint",
		Description: "Update a Render Blueprint's name, branch, file path or auto-sync setting. Changing branch or path also plans a sync. Early access upstream.",
		Parameters: withWorkspace(map[string]string{
			"blueprintId": paramBlueprintID,
			"name":        "New Blueprint name.",
			"branch":      "New Git branch for the Blueprint file.",
			"path":        "New Blueprint file path relative to the repository root.",
			"autoSync":    "true to sync automatically on pushes, false to require manual plans.",
		}),
		Required: []string{"blueprintId"},
		Access:   mcp.AccessDestructive,
	},

	// Workflows (durable task and flow runs)
	{
		Name:        "render_list_workflows",
		Description: "List Render Workflows (durable background task and flow orchestration) in a workspace. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"name":          "Filter by workflow name.",
			"environmentId": "Filter by Render project environment ID.",
			"cursor":        paramCursor,
			"limit":         paramPageLimit,
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_get_workflow",
		Description: "Get a Render Workflow's configuration: repository, branch, runtime, build and run commands.",
		Parameters: withWorkspace(map[string]string{
			"workflowId": paramWorkflowID,
		}),
		Required: []string{"workflowId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_create_workflow",
		Description: "Create a Render Workflow from a Git repository and start its first build unless autoDeploy is off.",
		Parameters: withWorkspace(map[string]string{
			"name":         "Workflow name.",
			"repo":         paramRepo,
			"runtime":      "Runtime: node, python, go, ruby or elixir.",
			"buildCommand": "Command that installs dependencies and builds the workflow.",
			"runCommand":   "Command that registers the workflow's tasks.",
			"branch":       paramBranch,
			"rootDir":      "Repository subdirectory containing the workflow.",
			"region":       paramRegion,
			"autoDeploy":   "commit (default), checksPass, or off.",
			"envVars":      paramEnvVars,
		}),
		Required: []string{"name", "repo", "runtime", "buildCommand", "runCommand"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_list_workflow_versions",
		Description: "List Render Workflow versions (deployments) with build and registration status. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"workflowId": "Filter by workflow ID.",
			"cursor":     paramCursor,
			"limit":      paramPageLimit,
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_get_workflow_version",
		Description: "Get a Render Workflow version's build and registration status.",
		Parameters: withWorkspace(map[string]string{
			"workflowVersionId": paramWorkflowVerID,
		}),
		Required: []string{"workflowVersionId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_create_workflow_version",
		Description: "Deploy a new version of a Render Workflow, changing which code later task runs use. Not needed after a push when autoDeploy is on. Returns acceptance; poll render_get_workflow_version.",
		Parameters: withWorkspace(map[string]string{
			"workflowId": paramWorkflowID,
			"commit":     "Git commit to deploy; defaults to the branch's current commit.",
		}),
		Required: []string{"workflowId"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        "render_list_workflow_tasks",
		Description: "List tasks registered by Render Workflows, with IDs and versions. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"workflowId":        "Filter by workflow ID.",
			"workflowVersionId": "Filter by workflow version ID.",
			"taskSlug":          paramTaskSlug,
			"cursor":            paramCursor,
			"limit":             paramPageLimit,
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_get_workflow_task",
		Description: "Get a registered Render Workflow task's name, workflow ID and version ID.",
		Parameters: withWorkspace(map[string]string{
			"taskId": "Task ID from render_list_workflow_tasks.",
		}),
		Required: []string{"taskId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_list_workflow_runs",
		Description: "List Render Workflow task runs with status, attempts and parent and root run IDs. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"workflowId":        "Filter by workflow ID.",
			"workflowVersionId": "Filter by workflow version ID.",
			"taskSlug":          paramTaskSlug,
			"rootTaskRunId":     "Filter to the run tree with this root run ID.",
			"cursor":            paramCursor,
			"limit":             paramPageLimit,
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_get_workflow_run",
		Description: "Get a Render Workflow task run's status, errors and attempts. Task failure is returned as data.",
		Parameters: withWorkspace(map[string]string{
			"taskRunId": paramTaskRunID,
		}),
		Required: []string{"taskRunId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_get_workflow_run_results",
		Description: "Read a finished Render Workflow task run's results as paged JSON text. Concatenate pages using nextOffset and sha256; nextOffset null marks the end.",
		Parameters: withWorkspace(map[string]string{
			"taskRunId": paramTaskRunID,
			"attempt":   "Zero-indexed attempt number; omit for the run's results.",
			"offset":    paramResultOffset,
			"limit":     paramResultLimit,
			"sha256":    paramResultSHA,
		}),
		Required: []string{"taskRunId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_list_flows",
		Description: "List flows registered by Render Workflows with IDs (flw-...) and slugs, to pick a flow for render_start_flow_run. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"workflowVersionId": "Filter by workflow version ID.",
			"flowSlug":          paramFlowSlug,
			"cursor":            paramCursor,
			"limit":             paramPageLimit,
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_get_flow",
		Description: "Get a registered Render Workflow flow's name, workflow ID and version ID.",
		Parameters: withWorkspace(map[string]string{
			"flowId": "Flow ID (flw-...) from render_list_flows.",
		}),
		Required: []string{"flowId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_start_flow_run",
		Description: "Start a Render Workflow flow run, which runs the user's code. Only when asked. Always pass a fresh idempotencyKey; poll render_get_flow_run until it finishes.",
		Parameters: withWorkspace(map[string]string{
			"flow":           "Exact flow ID (flw-...) from render_list_flows; slugs are not accepted.",
			"input":          "Flow arguments: an array for positional or an object for named arguments.",
			"idempotencyKey": "Fresh UUID; repeating it within 24 hours returns the first run instead of starting another.",
		}),
		Required: []string{"flow"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "render_list_flow_runs",
		Description: "List Render Workflow flow runs with status and timestamps. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"flowId":            "Filter by flow ID.",
			"flowSlug":          paramFlowSlug,
			"workflowId":        "Filter by workflow ID.",
			"workflowVersionId": "Filter by workflow version ID.",
			"status":            "pending, running, succeeded, failed or canceled.",
			"cursor":            paramCursor,
			"limit":             paramPageLimit,
		}),
		Access: mcp.AccessRead,
	},
	{
		Name:        "render_get_flow_run",
		Description: "Get a Render Workflow flow run's status, error details and attempts. Poll after render_start_flow_run.",
		Parameters: withWorkspace(map[string]string{
			"flowRunId": paramFlowRunID,
		}),
		Required: []string{"flowRunId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_get_flow_run_results",
		Description: "Read a finished Render Workflow flow run's results as paged JSON text. Concatenate pages using nextOffset and sha256.",
		Parameters: withWorkspace(map[string]string{
			"flowRunId": paramFlowRunID,
			"offset":    paramResultOffset,
			"limit":     paramResultLimit,
			"sha256":    paramResultSHA,
		}),
		Required: []string{"flowRunId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_list_flow_run_steps",
		Description: "List a Render Workflow flow run's steps in start order with type, status and timestamps. Returns items and nextCursor.",
		Parameters: withWorkspace(map[string]string{
			"flowRunId": paramFlowRunID,
			"cursor":    paramCursor,
			"limit":     paramPageLimit,
		}),
		Required: []string{"flowRunId"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_get_flow_run_step",
		Description: "Get one Render Workflow flow run step's status, result and error.",
		Parameters: withWorkspace(map[string]string{
			"flowRunId": paramFlowRunID,
			"seq":       "The step's seq from render_list_flow_run_steps.",
			"stepKey":   "The step's stepKey, to tell apart steps with the same seq.",
		}),
		Required: []string{"flowRunId", "seq"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "render_cancel_flow_run",
		Description: "Cancel a pending or running Render Workflow flow run. Cannot be undone; poll render_get_flow_run until canceled.",
		Parameters: withWorkspace(map[string]string{
			"flowRunId": paramFlowRunID,
		}),
		Required: []string{"flowRunId"},
		Access:   mcp.AccessDestructive,
	},
}

// dispatch maps each exposed tool to the hosted server's tool name. A tool
// that is not listed is refused before any network call.
var dispatch = map[mcp.ToolName]string{
	"render_list_services":                "list_services",
	"render_list_workspaces":              "list_workspaces",
	"render_get_service":                  "get_service",
	"render_create_web_service":           "create_web_service",
	"render_create_static_site":           "create_static_site",
	"render_create_cron_job":              "create_cron_job",
	"render_update_environment_variables": "update_environment_variables",
	"render_run_cron_job":                 "run_cron_job",
	"render_cancel_cron_job_run":          "cancel_cron_job_run",
	"render_list_deploys":                 "list_deploys",
	"render_get_deploy":                   "get_deploy",
	"render_trigger_deploy":               "trigger_deploy",
	"render_list_events":                  "list_events",
	"render_list_logs":                    "list_logs",
	"render_list_log_label_values":        "list_log_label_values",
	"render_get_metrics":                  "get_metrics",
	"render_list_postgres_instances":      "list_postgres_instances",
	"render_get_postgres":                 "get_postgres",
	"render_query_postgres":               "query_render_postgres",
	"render_create_postgres":              "create_postgres",
	"render_list_key_value":               "list_key_value",
	"render_get_key_value":                "get_key_value",
	"render_create_key_value":             "create_key_value",
	"render_get_blueprint":                "get_blueprint",
	"render_create_blueprint":             "create_blueprint",
	"render_plan_blueprint":               "plan_blueprint",
	"render_approve_blueprint":            "approve_blueprint",
	"render_update_blueprint":             "update_blueprint",
	"render_list_workflows":               "list_workflows",
	"render_get_workflow":                 "get_workflow",
	"render_create_workflow":              "create_workflow",
	"render_list_workflow_versions":       "list_workflow_versions",
	"render_get_workflow_version":         "get_workflow_version",
	"render_create_workflow_version":      "create_workflow_version",
	"render_list_workflow_tasks":          "list_workflow_tasks",
	"render_get_workflow_task":            "get_workflow_task",
	"render_list_workflow_runs":           "list_workflow_runs",
	"render_get_workflow_run":             "get_workflow_run",
	"render_get_workflow_run_results":     "get_workflow_run_results",
	"render_list_flows":                   "list_flows",
	"render_get_flow":                     "get_flow",
	"render_start_flow_run":               "start_flow_run",
	"render_list_flow_runs":               "list_flow_runs",
	"render_get_flow_run":                 "get_flow_run",
	"render_get_flow_run_results":         "get_flow_run_results",
	"render_list_flow_run_steps":          "list_flow_run_steps",
	"render_get_flow_run_step":            "get_flow_run_step",
	"render_cancel_flow_run":              "cancel_flow_run",
}
