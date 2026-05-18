package aws

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── STS ──────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("aws_get_caller_identity"), Description: "Get details about the IAM identity making the API call",
		Parameters: []mcp.Parameter{},
	},

	// ── S3 ───────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("aws_s3_list_buckets"), Description: "List all S3 buckets in the account",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("aws_s3_list_objects"), Description: "List objects in an S3 bucket",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("prefix"), Description: "Object key prefix filter"}, {Name: mcp.ParamName("max_keys"), Description: "Maximum number of keys to return (default 1000)"}, {Name: mcp.ParamName("continuation_token"), Description: "Token for pagination"}},
	},
	{
		Name: mcp.ToolName("aws_s3_get_object"), Description: "Get an object from S3 (returns metadata and body as text for text types, base64 for binary)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("key"), Description: "Object key", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_s3_put_object"), Description: "Upload an object to S3",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("key"), Description: "Object key", Required: true}, {Name: mcp.ParamName("body"), Description: "Object content (text)", Required: true}, {Name: mcp.ParamName("content_type"), Description: "MIME type (default: application/octet-stream)"}},
	},
	{
		Name: mcp.ToolName("aws_s3_delete_object"), Description: "Delete an object from S3",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("key"), Description: "Object key", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_s3_head_object"), Description: "Get metadata for an S3 object without downloading it",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("bucket"), Description: "Bucket name", Required: true}, {Name: mcp.ParamName("key"), Description: "Object key", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_s3_copy_object"), Description: "Copy an object within S3",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("source_bucket"), Description: "Source bucket name", Required: true}, {Name: mcp.ParamName("source_key"), Description: "Source object key", Required: true}, {Name: mcp.ParamName("dest_bucket"), Description: "Destination bucket name", Required:

		// ── EC2 ──────────────────────────────────────────────────────────
		true}, {Name: mcp.ParamName("dest_key"), Description: "Destination object key", Required: true}},
	},

	{
		Name: mcp.ToolName("aws_ec2_describe_instances"), Description: "List EC2 instances (servers/VMs) with optional filters. Start here for infrastructure inventory and production servers.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("instance_ids"), Description: "Comma-separated instance IDs"}, {Name: mcp.ParamName("filters"), Description: `JSON array of filters [{"Name":"tag:Env","Values":["prod"]}]`}, {Name: mcp.ParamName("max_results"), Description: "Maximum number of results"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_instance"), Description: "Get details for a specific EC2 instance",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("instance_id"), Description: "Instance ID", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ec2_start_instances"), Description: "Start one or more EC2 instances",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("instance_ids"), Description: "Comma-separated instance IDs to start", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ec2_stop_instances"), Description: "Stop one or more EC2 instances",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("instance_ids"), Description: "Comma-separated instance IDs to stop", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_security_groups"), Description: "List security groups with optional filters",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("group_ids"), Description: "Comma-separated security group IDs"}, {Name: mcp.ParamName("filters"), Description: "JSON array of filters"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_vpcs"), Description: "List VPCs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("vpc_ids"), Description: "Comma-separated VPC IDs"}, {Name: mcp.ParamName("filters"), Description: "JSON array of filters"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_subnets"), Description: "List subnets",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("subnet_ids"), Description: "Comma-separated subnet IDs"}, {Name: mcp.ParamName("filters"), Description: "JSON array of filters"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_images"), Description: "List AMI images",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("image_ids"), Description: "Comma-separated AMI IDs"}, {Name: mcp.ParamName("owners"), Description: "Comma-separated owner IDs or aliases (self, amazon)"}, {Name: mcp.ParamName("filters"), Description: "JSON array of filters"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_volumes"), Description: "List EBS volumes",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("volume_ids"), Description: "Comma-separated volume IDs"}, {Name: mcp.ParamName("filters"), Description: "JSON array of filters"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_addresses"), Description: "List Elastic IP addresses",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("allocation_ids"), Description: "Comma-separated allocation IDs"}, {Name: mcp.ParamName("filters"), Description: "JSON array of filters"}},
	},
	{
		Name: mcp.ToolName("aws_ec2_describe_key_pairs"), Description: "List EC2 key pairs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("key_names"), Description: "Comma-separated key pair names"}},
	},

	// ── Lambda ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("aws_lambda_list_functions"), Description: "List Lambda functions",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("max_items"), Description: "Maximum number of functions to return"}},
	},
	{
		Name: mcp.ToolName("aws_lambda_get_function"), Description: "Get details about a Lambda function",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("function_name"), Description: "Function name or ARN", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_lambda_invoke"), Description: "Invoke (run/trigger) a Lambda function. Use after list_functions to find the function name.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("function_name"), Description: "Function name or ARN", Required: true}, {Name: mcp.ParamName("payload"), Description: "JSON payload to pass to the function"}, {Name: mcp.ParamName("invocation_type"), Description: "RequestResponse (sync, default), Event (async), or DryRun"}},
	},
	{
		Name: mcp.ToolName("aws_lambda_list_event_source_mappings"), Description: "List event source mappings for a function",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("function_name"), Description: "Function name or ARN"}},
	},
	{
		Name: mcp.ToolName("aws_lambda_get_function_configuration"), Description: "Get the configuration of a Lambda function",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("function_name"), Description: "Function name or ARN", Required: true}},
	},

	// ── IAM ──────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("aws_iam_list_users"), Description: "List IAM users. Start here for access audits and finding who has AWS access.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("path_prefix"), Description: "Path prefix for filtering (default /)"}, {Name: mcp.ParamName("max_items"), Description: "Maximum number of users to return"}},
	},
	{
		Name: mcp.ToolName("aws_iam_get_user"), Description: "Get details about an IAM user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "IAM username (omit for current user)"}},
	},
	{
		Name: mcp.ToolName("aws_iam_list_roles"), Description: "List IAM roles",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("path_prefix"), Description: "Path prefix for filtering (default /)"}, {Name: mcp.ParamName("max_items"), Description: "Maximum number of roles to return"}},
	},
	{
		Name: mcp.ToolName("aws_iam_get_role"), Description: "Get details about an IAM role",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("role_name"), Description: "IAM role name", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_iam_list_policies"), Description: "List IAM policies",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("scope"), Description: "Scope: All, AWS, Local (default All)"}, {Name: mcp.ParamName("only_attached"), Description: "Only show attached policies (true/false)"}, {Name: mcp.ParamName("path_prefix"), Description: "Path prefix filter"}, {Name: mcp.ParamName("max_items"), Description: "Maximum number of policies to return"}},
	},
	{
		Name: mcp.ToolName("aws_iam_get_policy"), Description: "Get details about an IAM policy",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("policy_arn"), Description: "Policy ARN", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_iam_list_groups"), Description: "List IAM groups",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("path_prefix"), Description: "Path prefix for filtering"}, {Name: mcp.ParamName("max_items"), Description: "Maximum number of groups to return"}},
	},
	{
		Name: mcp.ToolName("aws_iam_list_attached_role_policies"), Description: "List policies attached to an IAM role",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("role_name"), Description: "IAM role name", Required: true}, {Name: mcp.ParamName("path_prefix"), Description: "Path prefix filter"}},
	},
	{
		Name: mcp.ToolName("aws_iam_list_attached_user_policies"), Description: "List policies attached to an IAM user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "IAM username", Required: true}, {Name: mcp.ParamName("path_prefix"), Description: "Path prefix filter"}},
	},
	{
		Name: mcp.ToolName("aws_iam_list_attached_group_policies"), Description: "List policies attached to an IAM group",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("group_name"), Description: "IAM group name", Required: true}, {Name: mcp.ParamName("path_prefix"), Description:

		// ── CloudWatch ───────────────────────────────────────────────────
		"Path prefix filter"}},
	},

	{
		Name: mcp.ToolName("aws_cloudwatch_list_metrics"), Description: "List CloudWatch metrics",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Metric namespace (e.g. AWS/EC2)"}, {Name: mcp.ParamName("metric_name"), Description: "Metric name filter"}},
	},
	{
		Name: mcp.ToolName("aws_cloudwatch_get_metric_data"), Description: "Get CloudWatch metric data points (time series). Use for monitoring performance over a time range. Use after list_metrics to discover available metrics.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("metric_name"), Description: "Metric name", Required: true}, {Name: mcp.ParamName("namespace"), Description: "Metric namespace", Required: true}, {Name: mcp.ParamName("stat"), Description: "Statistic: Average, Sum, Minimum, Maximum, SampleCount", Required: true}, {Name: mcp.ParamName("period"), Description: "Period in seconds (default 300)"}, {Name: mcp.ParamName("start_time"), Description: "Start time (RFC3339 or relative e.g. -1h)"}, {Name: mcp.ParamName("end_time"), Description: "End time (RFC3339 or relative, default now)"}, {Name: mcp.ParamName("dimensions"), Description: "JSON object of dimension key-value pairs"}},
	},
	{
		Name: mcp.ToolName("aws_cloudwatch_describe_alarms"), Description: "List CloudWatch alarms for active alerts, firing monitors, and threshold warnings. Filter by state (ALARM, OK, INSUFFICIENT_DATA).",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alarm_names"), Description: "Comma-separated alarm names"}, {Name: mcp.ParamName("state_value"), Description: "Filter by state: OK, ALARM, INSUFFICIENT_DATA"}, {Name: mcp.ParamName("max_records"), Description: "Maximum number of alarms to return"}},
	},
	{
		Name: mcp.ToolName("aws_cloudwatch_get_metric_statistics"), Description: "Get statistics for a specific CloudWatch metric",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Metric namespace", Required: true}, {Name: mcp.ParamName("metric_name"), Description: "Metric name", Required: true}, {Name: mcp.ParamName("start_time"), Description: "Start time (RFC3339 or relative e.g. -1h)", Required: true}, {Name: mcp.ParamName("end_time"), Description: "End time (RFC3339)"}, {Name: mcp.ParamName("period"), Description: "Period in seconds", Required: true}, {Name: mcp.ParamName("statistics"),

		// ── ECS ──────────────────────────────────────────────────────────
		Description: "Comma-separated: Average, Sum, Minimum, Maximum, SampleCount", Required: true}, {Name: mcp.ParamName("dimensions"), Description: "JSON object of dimension key-value pairs"}},
	},

	{
		Name: mcp.ToolName("aws_ecs_list_clusters"), Description: "List ECS clusters",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("aws_ecs_describe_clusters"), Description: "Get details about one or more ECS clusters",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("clusters"), Description: "Comma-separated cluster names or ARNs", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ecs_list_services"), Description: "List services (deployed containers) in an ECS cluster. Use after list_clusters to find the cluster name.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cluster"), Description: "Cluster name or ARN", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ecs_describe_services"), Description: "Get details about ECS services including deploy status, health, and running/desired count. Use after list_services.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cluster"), Description: "Cluster name or ARN", Required: true}, {Name: mcp.ParamName("services"), Description: "Comma-separated service names or ARNs", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ecs_list_tasks"), Description: "List tasks in an ECS cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cluster"), Description: "Cluster name or ARN"}, {Name: mcp.ParamName("service_name"), Description: "Filter by service name"}, {Name: mcp.ParamName("desired_status"), Description: "Filter by status: RUNNING, PENDING, STOPPED"}},
	},
	{
		Name: mcp.ToolName("aws_ecs_describe_tasks"), Description: "Get details about one or more ECS tasks",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cluster"), Description: "Cluster name or ARN", Required: true}, {Name: mcp.ParamName("tasks"), Description: "Comma-separated task IDs or ARNs", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_ecs_list_task_definitions"), Description: "List ECS task definition families or revisions",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("family_prefix"), Description: "Task definition family prefix filter"}, {Name: mcp.ParamName("status"), Description: "Filter: ACTIVE or INACTIVE"}},
	},
	{
		Name: mcp.ToolName("aws_ecs_describe_task_definition"), Description: "Get details about an ECS task definition",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("task_definition"), Description: "Task definition family:revision or full ARN", Required: true}},
	},

	// ── SNS ──────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("aws_sns_list_topics"), Description: "List SNS topics",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("aws_sns_get_topic_attributes"), Description: "Get attributes for an SNS topic",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("topic_arn"), Description: "SNS topic ARN", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_sns_list_subscriptions"), Description: "List SNS subscriptions",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("topic_arn"), Description: "Filter by topic ARN"}},
	},
	{
		Name: mcp.ToolName("aws_sns_publish"), Description: "Publish (send) a notification message to an SNS topic. Use after list_topics to find the topic ARN.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("topic_arn"), Description: "SNS topic ARN", Required: true}, {Name: mcp.ParamName("message"), Description: "Message body", Required: true}, {Name: mcp.ParamName("subject"),

		// ── SQS ──────────────────────────────────────────────────────────
		Description: "Message subject (for email subscriptions)"}},
	},

	{
		Name: mcp.ToolName("aws_sqs_list_queues"), Description: "List SQS queues",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("queue_name_prefix"), Description: "Queue name prefix filter"}},
	},
	{
		Name: mcp.ToolName("aws_sqs_get_queue_attributes"), Description: "Get attributes for an SQS queue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("queue_url"), Description: "SQS queue URL", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_sqs_send_message"), Description: "Send a message to an SQS queue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("queue_url"), Description: "SQS queue URL", Required: true}, {Name: mcp.ParamName("message_body"), Description: "Message content", Required: true}, {Name: mcp.ParamName("delay_seconds"), Description: "Delay in seconds (0-900)"}},
	},
	{
		Name: mcp.ToolName("aws_sqs_receive_message"), Description: "Receive messages from an SQS queue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("queue_url"), Description: "SQS queue URL", Required: true}, {Name: mcp.ParamName("max_messages"), Description: "Max messages to receive (1-10, default 1)"}, {Name: mcp.ParamName("wait_time_seconds"), Description: "Long poll wait time (0-20)"}},
	},
	{
		Name: mcp.ToolName("aws_sqs_delete_message"), Description: "Delete a message from an SQS queue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("queue_url"), Description: "SQS queue URL", Required: true}, {Name: mcp.ParamName("receipt_handle"), Description: "Receipt handle from receive", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_sqs_purge_queue"), Description: "Purge all messages from an SQS queue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("queue_url"), Description: "SQS queue URL", Required: true}},
	},

	// ── DynamoDB ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("aws_dynamodb_list_tables"), Description: "List DynamoDB tables",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Maximum number of tables to return"}},
	},
	{
		Name: mcp.ToolName("aws_dynamodb_describe_table"), Description: "Get details about a DynamoDB table",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_dynamodb_get_item"), Description: "Get an item from a DynamoDB table",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}, {Name: mcp.ParamName("key"), Description: `JSON object with key attributes (e.g. {"id":{"S":"123"}})`, Required: true}},
	},
	{
		Name: mcp.ToolName("aws_dynamodb_put_item"), Description: "Put an item into a DynamoDB table",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}, {Name: mcp.ParamName("item"), Description: "JSON object with item attributes in DynamoDB format", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_dynamodb_query"), Description: "Query a DynamoDB table",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}, {Name: mcp.ParamName("key_condition_expression"), Description: "Key condition expression", Required: true}, {Name: mcp.ParamName("expression_attribute_values"), Description: "JSON object of expression attribute values in DynamoDB format", Required: true}, {Name: mcp.ParamName("expression_attribute_names"), Description: "JSON object of expression attribute name placeholders"}, {Name: mcp.ParamName("index_name"), Description: "Secondary index name"}, {Name: mcp.ParamName("limit"), Description: "Maximum number of items to return"}, {Name: mcp.ParamName("scan_index_forward"), Description: "Sort ascending (true, default) or descending (false)"}},
	},
	{
		Name: mcp.ToolName("aws_dynamodb_scan"), Description: "Scan a DynamoDB table",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}, {Name: mcp.ParamName("filter_expression"), Description: "Filter expression"}, {Name: mcp.ParamName("expression_attribute_values"), Description: "JSON object of expression attribute values"}, {Name: mcp.ParamName("expression_attribute_names"), Description: "JSON object of expression attribute name placeholders"}, {Name: mcp.ParamName("limit"), Description: "Maximum number of items to return"}},
	},
	{
		Name: mcp.ToolName("aws_dynamodb_delete_item"), Description: "Delete an item from a DynamoDB table",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}, {Name: mcp.ParamName("key"), Description: "JSON object with key attributes in DynamoDB format",

		// ── CloudFormation ───────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("aws_cloudformation_list_stacks"), Description: "List CloudFormation stacks",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("status_filter"), Description: "Comma-separated status filter (e.g. CREATE_COMPLETE,UPDATE_COMPLETE)"}},
	},
	{
		Name: mcp.ToolName("aws_cloudformation_describe_stack"), Description: "Get details about a CloudFormation stack",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("stack_name"), Description: "Stack name or ID", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_cloudformation_list_stack_resources"), Description: "List resources in a CloudFormation stack",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("stack_name"), Description: "Stack name or ID", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_cloudformation_get_template"), Description: "Get the template for a CloudFormation stack",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("stack_name"), Description: "Stack name or ID", Required: true}},
	},
	{
		Name: mcp.ToolName("aws_cloudformation_describe_stack_events"), Description: "List events for a CloudFormation stack. Use to debug deploy failures, rollbacks, or track infrastructure change history.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("stack_name"), Description: "Stack name or ID", Required: true}},
	},
}
