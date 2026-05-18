package gcp

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Resource Manager ─────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcp_get_project"), Description: "Get details about the configured GCP project. Start here to verify project access.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_list_projects"), Description: "Search for GCP projects the caller has access to",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Filter query (e.g. name:my-project)"}},
	},
	{
		Name: mcp.ToolName("gcp_list_folders"), Description: "List folders under a parent resource",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("parent"), Description: "Parent resource name (e.g. organizations/123)", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_get_folder"), Description: "Get details about a folder",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("folder_id"), Description: "Folder ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_get_iam_policy"), Description: "Get IAM policy for the configured project",
		Parameters: []mcp.Parameter{},
	},

	// ── Cloud Storage ────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcp_storage_list_buckets"), Description: "List all Cloud Storage buckets in the project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Maximum number of buckets to return (default 1000)"}},
	},
	{
		Name: mcp.ToolName("gcp_storage_get_bucket"), Description: "Get metadata for a Cloud Storage bucket",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_storage_list_objects"), Description: "List objects in a Cloud Storage bucket (default limit 1000)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("prefix"), Description: "Object name prefix filter"}, {Name: mcp.ParamName("delimiter"), Description: "Delimiter for hierarchy (e.g. /)"}, {Name: mcp.ParamName("limit"), Description: "Maximum total results to return (default 1000)"}},
	},
	{
		Name: mcp.ToolName("gcp_storage_get_object"), Description: "Get an object from Cloud Storage (max 10MB; returns metadata and body as text for text types, base64 for binary)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("object"), Description: "Object name/path", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_storage_put_object"), Description: "Upload an object to Cloud Storage",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("object"), Description: "Object name/path", Required: true}, {Name: mcp.ParamName("body"), Description: "Object content (text)", Required: true}, {Name: mcp.ParamName("content_type"), Description: "MIME type (default: application/octet-stream)"}},
	},
	{
		Name: mcp.ToolName("gcp_storage_delete_object"), Description: "Delete an object from Cloud Storage",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("object"), Description: "Object name/path", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_storage_copy_object"), Description: "Copy an object within Cloud Storage",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("source_bucket"), Description: "Source bucket name", Required: true}, {Name: mcp.ParamName("source_object"), Description: "Source object name", Required: true}, {Name: mcp.ParamName("dest_bucket"), Description: "Destination bucket name", Required: true}, {Name:

		// ── Compute Engine ───────────────────────────────────────────────
		mcp.ParamName("dest_object"), Description: "Destination object name", Required: true}},
	},

	{
		Name: mcp.ToolName("gcp_compute_list_instances"), Description: "List Compute Engine VM instances (virtual machines) in a zone. View production server infrastructure. Default limit 500.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("zone"), Description: "Zone name (e.g. us-central1-a)", Required: true}, {Name: mcp.ParamName("filter"), Description: "Filter expression"}, {Name: mcp.ParamName("max_results"), Description: "Page size for API requests"}, {Name: mcp.ParamName("limit"), Description: "Maximum total results to return (default 500)"}},
	},
	{
		Name: mcp.ToolName("gcp_compute_get_instance"), Description: "Get details for a specific Compute Engine instance",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("zone"), Description: "Zone name", Required: true}, {Name: mcp.ParamName("instance"), Description: "Instance name", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_compute_start_instance"), Description: "Start a Compute Engine instance",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("zone"), Description: "Zone name", Required: true}, {Name: mcp.ParamName("instance"), Description: "Instance name", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_compute_stop_instance"), Description: "Stop a Compute Engine instance",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("zone"), Description: "Zone name", Required: true}, {Name: mcp.ParamName("instance"), Description: "Instance name", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_compute_list_disks"), Description: "List persistent disks in a zone (default limit 500)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("zone"), Description: "Zone name", Required: true}, {Name: mcp.ParamName("filter"), Description: "Filter expression"}, {Name: mcp.ParamName("max_results"), Description: "Page size for API requests"}, {Name: mcp.ParamName("limit"), Description: "Maximum total results to return (default 500)"}},
	},
	{
		Name: mcp.ToolName("gcp_compute_list_networks"), Description: "List VPC networks in the project (default limit 500)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: "Filter expression"}, {Name: mcp.ParamName("max_results"), Description: "Page size for API requests"}, {Name: mcp.ParamName("limit"), Description: "Maximum total results to return (default 500)"}},
	},
	{
		Name: mcp.ToolName("gcp_compute_list_subnetworks"), Description: "List subnetworks in a region (default limit 500)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("region"), Description: "Region name (e.g. us-central1)", Required: true}, {Name: mcp.ParamName("filter"), Description: "Filter expression"}, {Name: mcp.ParamName("max_results"), Description: "Page size for API requests"}, {Name: mcp.ParamName("limit"), Description: "Maximum total results to return (default 500)"}},
	},
	{
		Name: mcp.ToolName("gcp_compute_list_firewalls"), Description: "List firewall rules in the project (default limit 500)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: "Filter expression"}, {Name: mcp.ParamName("max_results"), Description: "Page size for API requests"}, {Name: mcp.ParamName("limit"), Description: "Maximum total results to return (default 500)"}},
	},
	{
		Name: mcp.ToolName("gcp_compute_get_firewall"), Description: "Get details for a specific firewall rule",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("firewall"), Description: "Firewall rule name", Required: true}},
	},

	// ── Cloud Functions ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcp_functions_list"), Description: "List Cloud Functions (serverless) in a location. View deployed functions and their configurations.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("location"), Description: "Location (e.g. us-central1, or - for all locations)"}},
	},
	{
		Name: mcp.ToolName("gcp_functions_get"), Description: "Get details about a Cloud Function",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Full resource name (projects/PROJECT/locations/LOCATION/functions/NAME)", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_functions_get_iam_policy"), Description: "Get IAM policy for a Cloud Function",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Full resource name of the function", Required: true}},
	},

	// ── IAM ──────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcp_iam_list_service_accounts"), Description: "List service accounts (automation identities) in the project. Review access credentials and security configuration.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_iam_get_service_account"), Description: "Get details about a service account",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("email"), Description: "Service account email", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_iam_list_service_account_keys"), Description: "List keys for a service account",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("email"), Description: "Service account email", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_iam_list_roles"), Description: "List predefined and custom IAM roles",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("show_deleted"), Description: "Include deleted roles (true/false)"}},
	},
	{
		Name: mcp.ToolName("gcp_iam_get_role"), Description: "Get details about an IAM role",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Role name (e.g. roles/editor or projects/PROJECT/roles/CUSTOM)", Required: true}},
	},

	// ── Cloud Monitoring ─────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcp_monitoring_list_metric_descriptors"), Description: "List available metric descriptors",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: `Metric filter (e.g. metric.type = starts_with("compute.googleapis.com"))`}},
	},
	{
		Name: mcp.ToolName("gcp_monitoring_list_time_series"), Description: "Get Cloud Monitoring time series data for a metric. Query production performance, CPU, memory, and custom metric graphs.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: `Time series filter (e.g. metric.type="compute.googleapis.com/instance/cpu/utilization")`, Required: true}, {Name: mcp.ParamName("start_time"), Description: "Start time (RFC3339)", Required: true}, {Name: mcp.ParamName("end_time"), Description: "End time (RFC3339, default now)"}, {Name: mcp.ParamName("alignment_period"), Description: "Alignment period in seconds (e.g. 60s)"}, {Name: mcp.ParamName("per_series_aligner"), Description: "Aligner: ALIGN_MEAN, ALIGN_SUM, ALIGN_MAX, ALIGN_MIN, etc."}},
	},
	{
		Name: mcp.ToolName("gcp_monitoring_list_alert_policies"), Description: "List Cloud Monitoring alert policies for production threshold warnings and notifications",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: "Filter expression"}},
	},
	{
		Name: mcp.ToolName("gcp_monitoring_get_alert_policy"), Description: "Get details about an alert policy",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Full resource name of the alert policy", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_monitoring_list_monitored_resources"), Description: "List monitored resource descriptors",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description:

		// ── Cloud Run ────────────────────────────────────────────────────
		"Filter expression"}},
	},

	{
		Name: mcp.ToolName("gcp_run_list_services"), Description: "List Cloud Run serverless container services in a location. View production deployments and configurations.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("location"), Description: "Location (e.g. us-central1)", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_run_get_service"), Description: "Get details about a Cloud Run service",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Full resource name (projects/PROJECT/locations/LOCATION/services/NAME)", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_run_list_revisions"), Description: "List revisions of a Cloud Run service",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service_name"), Description: "Full resource name of the parent service", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_run_get_revision"), Description: "Get details about a Cloud Run revision",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Full resource name of the revision", Required: true}},
	},

	// ── Pub/Sub ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcp_pubsub_list_topics"), Description: "List Pub/Sub topics in the project",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_pubsub_get_topic"), Description: "Get configuration for a Pub/Sub topic",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("topic"), Description: "Topic ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_pubsub_publish"), Description: "Publish a message to a Pub/Sub topic",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("topic"), Description: "Topic ID", Required: true}, {Name: mcp.ParamName("message"), Description: "Message data (text)", Required: true}, {Name: mcp.ParamName("attributes"), Description: "JSON object of message attributes"}},
	},
	{
		Name: mcp.ToolName("gcp_pubsub_list_subscriptions"), Description: "List Pub/Sub subscriptions in the project",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_pubsub_get_subscription"), Description: "Get configuration for a Pub/Sub subscription",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("subscription"), Description: "Subscription ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_pubsub_pull"), Description: "Pull messages from a Pub/Sub subscription",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("subscription"), Description: "Subscription ID", Required: true}, {Name: mcp.ParamName("max_messages"), Description: "Maximum messages to pull (default 10)"}, {Name: mcp.ParamName("timeout"), Description:

		// ── Firestore ────────────────────────────────────────────────────
		"Timeout in seconds to wait for messages (default 10)"}},
	},

	{
		Name: mcp.ToolName("gcp_firestore_list_collections"), Description: "List top-level collections in Firestore",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_firestore_list_documents"), Description: "List documents in a Firestore collection",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("collection"), Description: "Collection path", Required: true}, {Name: mcp.ParamName("limit"), Description: "Maximum number of documents to return"}},
	},
	{
		Name: mcp.ToolName("gcp_firestore_get_document"), Description: "Get a document from Firestore",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("path"), Description: "Document path (e.g. collection/docId)", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_firestore_set_document"), Description: "Create or update a document in Firestore",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("path"), Description: "Document path (e.g. collection/docId)", Required: true}, {Name: mcp.ParamName("data"), Description: "JSON object with document fields", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_firestore_delete_document"), Description: "Delete a document from Firestore",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("path"), Description: "Document path (e.g. collection/docId)", Required: true}},
	},
	{
		Name: mcp.ToolName("gcp_firestore_query"), Description: "Query documents in a Firestore collection",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("collection"), Description: "Collection path", Required: true}, {Name: mcp.ParamName("where_field"), Description: "Field to filter on"}, {Name: mcp.ParamName("where_op"), Description: "Operator: ==, !=, <, <=, >, >=, array-contains, in"}, {Name: mcp.ParamName("where_value"), Description: `Value to compare (JSON-encoded: use 123 for numbers, "\"text\"" for strings, true/false for booleans)`}, {Name: mcp.ParamName("order_by"),

		// ── Cloud Logging ────────────────────────────────────────────────
		Description: "Field to order by"}, {Name: mcp.ParamName("order_dir"), Description: "Order direction: asc or desc"}, {Name: mcp.ParamName("limit"), Description: "Maximum number of documents"}},
	},

	{
		Name: mcp.ToolName("gcp_logging_list_entries"), Description: "List Cloud Logging entries for the project. Search production logs for errors, debugging, and observability.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: "Logging filter (e.g. severity>=ERROR)"}, {Name: mcp.ParamName("order_by"), Description: "Order: timestamp asc or timestamp desc (default desc)"}, {Name: mcp.ParamName("page_size"), Description: "Maximum entries to return (default 50)"}},
	},
	{
		Name: mcp.ToolName("gcp_logging_list_log_names"), Description: "List log names in the project",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_logging_list_sinks"), Description: "List logging sinks in the project",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("gcp_logging_get_sink"), Description: "Get details about a logging sink",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sink_name"), Description: "Sink name", Required: true}},
	},
}
