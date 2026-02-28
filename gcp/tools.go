package gcp

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Resource Manager ─────────────────────────────────────────────
	{
		Name: "gcp_get_project", Description: "Get details about the configured GCP project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_list_projects", Description: "Search for GCP projects the caller has access to",
		Parameters: map[string]string{"query": "Filter query (e.g. name:my-project)"},
	},
	{
		Name: "gcp_list_folders", Description: "List folders under a parent resource",
		Parameters: map[string]string{"parent": "Parent resource name (e.g. organizations/123)"},
		Required:   []string{"parent"},
	},
	{
		Name: "gcp_get_folder", Description: "Get details about a folder",
		Parameters: map[string]string{"folder_id": "Folder ID"},
		Required:   []string{"folder_id"},
	},
	{
		Name: "gcp_get_iam_policy", Description: "Get IAM policy for the configured project",
		Parameters: map[string]string{},
	},

	// ── Cloud Storage ────────────────────────────────────────────────
	{
		Name: "gcp_storage_list_buckets", Description: "List all Cloud Storage buckets in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_storage_get_bucket", Description: "Get metadata for a Cloud Storage bucket",
		Parameters: map[string]string{"bucket": "Bucket name"},
		Required:   []string{"bucket"},
	},
	{
		Name: "gcp_storage_list_objects", Description: "List objects in a Cloud Storage bucket",
		Parameters: map[string]string{"bucket": "Bucket name", "prefix": "Object name prefix filter", "delimiter": "Delimiter for hierarchy (e.g. /)"},
		Required:   []string{"bucket"},
	},
	{
		Name: "gcp_storage_get_object", Description: "Get an object from Cloud Storage (returns metadata and body as text for text types, base64 for binary)",
		Parameters: map[string]string{"bucket": "Bucket name", "object": "Object name/path"},
		Required:   []string{"bucket", "object"},
	},
	{
		Name: "gcp_storage_put_object", Description: "Upload an object to Cloud Storage",
		Parameters: map[string]string{"bucket": "Bucket name", "object": "Object name/path", "body": "Object content (text)", "content_type": "MIME type (default: application/octet-stream)"},
		Required:   []string{"bucket", "object", "body"},
	},
	{
		Name: "gcp_storage_delete_object", Description: "Delete an object from Cloud Storage",
		Parameters: map[string]string{"bucket": "Bucket name", "object": "Object name/path"},
		Required:   []string{"bucket", "object"},
	},
	{
		Name: "gcp_storage_copy_object", Description: "Copy an object within Cloud Storage",
		Parameters: map[string]string{"source_bucket": "Source bucket name", "source_object": "Source object name", "dest_bucket": "Destination bucket name", "dest_object": "Destination object name"},
		Required:   []string{"source_bucket", "source_object", "dest_bucket", "dest_object"},
	},

	// ── Compute Engine ───────────────────────────────────────────────
	{
		Name: "gcp_compute_list_instances", Description: "List Compute Engine instances in a zone",
		Parameters: map[string]string{"zone": "Zone name (e.g. us-central1-a)", "filter": "Filter expression", "max_results": "Maximum number of results"},
		Required:   []string{"zone"},
	},
	{
		Name: "gcp_compute_get_instance", Description: "Get details for a specific Compute Engine instance",
		Parameters: map[string]string{"zone": "Zone name", "instance": "Instance name"},
		Required:   []string{"zone", "instance"},
	},
	{
		Name: "gcp_compute_start_instance", Description: "Start a Compute Engine instance",
		Parameters: map[string]string{"zone": "Zone name", "instance": "Instance name"},
		Required:   []string{"zone", "instance"},
	},
	{
		Name: "gcp_compute_stop_instance", Description: "Stop a Compute Engine instance",
		Parameters: map[string]string{"zone": "Zone name", "instance": "Instance name"},
		Required:   []string{"zone", "instance"},
	},
	{
		Name: "gcp_compute_list_disks", Description: "List persistent disks in a zone",
		Parameters: map[string]string{"zone": "Zone name", "filter": "Filter expression", "max_results": "Maximum number of results"},
		Required:   []string{"zone"},
	},
	{
		Name: "gcp_compute_list_networks", Description: "List VPC networks in the project",
		Parameters: map[string]string{"filter": "Filter expression", "max_results": "Maximum number of results"},
	},
	{
		Name: "gcp_compute_list_subnetworks", Description: "List subnetworks in a region",
		Parameters: map[string]string{"region": "Region name (e.g. us-central1)", "filter": "Filter expression", "max_results": "Maximum number of results"},
		Required:   []string{"region"},
	},
	{
		Name: "gcp_compute_list_firewalls", Description: "List firewall rules in the project",
		Parameters: map[string]string{"filter": "Filter expression", "max_results": "Maximum number of results"},
	},
	{
		Name: "gcp_compute_get_firewall", Description: "Get details for a specific firewall rule",
		Parameters: map[string]string{"firewall": "Firewall rule name"},
		Required:   []string{"firewall"},
	},

	// ── Cloud Functions ──────────────────────────────────────────────
	{
		Name: "gcp_functions_list", Description: "List Cloud Functions in a location",
		Parameters: map[string]string{"location": "Location (e.g. us-central1, or - for all locations)"},
	},
	{
		Name: "gcp_functions_get", Description: "Get details about a Cloud Function",
		Parameters: map[string]string{"name": "Full resource name (projects/PROJECT/locations/LOCATION/functions/NAME)"},
		Required:   []string{"name"},
	},
	{
		Name: "gcp_functions_get_iam_policy", Description: "Get IAM policy for a Cloud Function",
		Parameters: map[string]string{"name": "Full resource name of the function"},
		Required:   []string{"name"},
	},

	// ── IAM ──────────────────────────────────────────────────────────
	{
		Name: "gcp_iam_list_service_accounts", Description: "List service accounts in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_iam_get_service_account", Description: "Get details about a service account",
		Parameters: map[string]string{"email": "Service account email"},
		Required:   []string{"email"},
	},
	{
		Name: "gcp_iam_list_service_account_keys", Description: "List keys for a service account",
		Parameters: map[string]string{"email": "Service account email"},
		Required:   []string{"email"},
	},
	{
		Name: "gcp_iam_list_roles", Description: "List predefined and custom IAM roles",
		Parameters: map[string]string{"show_deleted": "Include deleted roles (true/false)"},
	},
	{
		Name: "gcp_iam_get_role", Description: "Get details about an IAM role",
		Parameters: map[string]string{"name": "Role name (e.g. roles/editor or projects/PROJECT/roles/CUSTOM)"},
		Required:   []string{"name"},
	},

	// ── Cloud Monitoring ─────────────────────────────────────────────
	{
		Name: "gcp_monitoring_list_metric_descriptors", Description: "List available metric descriptors",
		Parameters: map[string]string{"filter": "Metric filter (e.g. metric.type = starts_with(\"compute.googleapis.com\"))"},
	},
	{
		Name: "gcp_monitoring_list_time_series", Description: "Get time series data for a metric",
		Parameters: map[string]string{"filter": "Time series filter (e.g. metric.type=\"compute.googleapis.com/instance/cpu/utilization\")", "start_time": "Start time (RFC3339)", "end_time": "End time (RFC3339, default now)", "alignment_period": "Alignment period in seconds (e.g. 60s)", "per_series_aligner": "Aligner: ALIGN_MEAN, ALIGN_SUM, ALIGN_MAX, ALIGN_MIN, etc."},
		Required:   []string{"filter", "start_time"},
	},
	{
		Name: "gcp_monitoring_list_alert_policies", Description: "List alert policies in the project",
		Parameters: map[string]string{"filter": "Filter expression"},
	},
	{
		Name: "gcp_monitoring_get_alert_policy", Description: "Get details about an alert policy",
		Parameters: map[string]string{"name": "Full resource name of the alert policy"},
		Required:   []string{"name"},
	},
	{
		Name: "gcp_monitoring_list_monitored_resources", Description: "List monitored resource descriptors",
		Parameters: map[string]string{"filter": "Filter expression"},
	},

	// ── Cloud Run ────────────────────────────────────────────────────
	{
		Name: "gcp_run_list_services", Description: "List Cloud Run services in a location",
		Parameters: map[string]string{"location": "Location (e.g. us-central1)"},
		Required:   []string{"location"},
	},
	{
		Name: "gcp_run_get_service", Description: "Get details about a Cloud Run service",
		Parameters: map[string]string{"name": "Full resource name (projects/PROJECT/locations/LOCATION/services/NAME)"},
		Required:   []string{"name"},
	},
	{
		Name: "gcp_run_list_revisions", Description: "List revisions of a Cloud Run service",
		Parameters: map[string]string{"service_name": "Full resource name of the parent service"},
		Required:   []string{"service_name"},
	},
	{
		Name: "gcp_run_get_revision", Description: "Get details about a Cloud Run revision",
		Parameters: map[string]string{"name": "Full resource name of the revision"},
		Required:   []string{"name"},
	},

	// ── Pub/Sub ──────────────────────────────────────────────────────
	{
		Name: "gcp_pubsub_list_topics", Description: "List Pub/Sub topics in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_pubsub_get_topic", Description: "Get configuration for a Pub/Sub topic",
		Parameters: map[string]string{"topic": "Topic ID"},
		Required:   []string{"topic"},
	},
	{
		Name: "gcp_pubsub_publish", Description: "Publish a message to a Pub/Sub topic",
		Parameters: map[string]string{"topic": "Topic ID", "message": "Message data (text)", "attributes": "JSON object of message attributes"},
		Required:   []string{"topic", "message"},
	},
	{
		Name: "gcp_pubsub_list_subscriptions", Description: "List Pub/Sub subscriptions in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_pubsub_get_subscription", Description: "Get configuration for a Pub/Sub subscription",
		Parameters: map[string]string{"subscription": "Subscription ID"},
		Required:   []string{"subscription"},
	},
	{
		Name: "gcp_pubsub_pull", Description: "Pull messages from a Pub/Sub subscription",
		Parameters: map[string]string{"subscription": "Subscription ID", "max_messages": "Maximum messages to pull (default 10)"},
		Required:   []string{"subscription"},
	},

	// ── Firestore ────────────────────────────────────────────────────
	{
		Name: "gcp_firestore_list_collections", Description: "List top-level collections in Firestore",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_firestore_list_documents", Description: "List documents in a Firestore collection",
		Parameters: map[string]string{"collection": "Collection path", "limit": "Maximum number of documents to return"},
		Required:   []string{"collection"},
	},
	{
		Name: "gcp_firestore_get_document", Description: "Get a document from Firestore",
		Parameters: map[string]string{"path": "Document path (e.g. collection/docId)"},
		Required:   []string{"path"},
	},
	{
		Name: "gcp_firestore_set_document", Description: "Create or update a document in Firestore",
		Parameters: map[string]string{"path": "Document path (e.g. collection/docId)", "data": "JSON object with document fields"},
		Required:   []string{"path", "data"},
	},
	{
		Name: "gcp_firestore_delete_document", Description: "Delete a document from Firestore",
		Parameters: map[string]string{"path": "Document path (e.g. collection/docId)"},
		Required:   []string{"path"},
	},
	{
		Name: "gcp_firestore_query", Description: "Query documents in a Firestore collection",
		Parameters: map[string]string{"collection": "Collection path", "where_field": "Field to filter on", "where_op": "Operator: ==, !=, <, <=, >, >=, array-contains, in", "where_value": "Value to compare (JSON-encoded)", "order_by": "Field to order by", "order_dir": "Order direction: asc or desc", "limit": "Maximum number of documents"},
		Required:   []string{"collection"},
	},

	// ── Cloud Logging ────────────────────────────────────────────────
	{
		Name: "gcp_logging_list_entries", Description: "List log entries for the project",
		Parameters: map[string]string{"filter": "Logging filter (e.g. severity>=ERROR)", "order_by": "Order: timestamp asc or timestamp desc (default desc)", "page_size": "Maximum entries to return (default 50)"},
	},
	{
		Name: "gcp_logging_list_log_names", Description: "List log names in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_logging_list_sinks", Description: "List logging sinks in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "gcp_logging_get_sink", Description: "Get details about a logging sink",
		Parameters: map[string]string{"sink_name": "Sink name"},
		Required:   []string{"sink_name"},
	},
}
