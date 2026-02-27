package aws

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── STS ──────────────────────────────────────────────────────────
	{
		Name: "aws_get_caller_identity", Description: "Get details about the IAM identity making the API call",
		Parameters: map[string]string{},
	},

	// ── S3 ───────────────────────────────────────────────────────────
	{
		Name: "aws_s3_list_buckets", Description: "List all S3 buckets in the account",
		Parameters: map[string]string{},
	},
	{
		Name: "aws_s3_list_objects", Description: "List objects in an S3 bucket",
		Parameters: map[string]string{"bucket": "Bucket name", "prefix": "Object key prefix filter", "max_keys": "Maximum number of keys to return (default 1000)", "continuation_token": "Token for pagination"},
		Required:   []string{"bucket"},
	},
	{
		Name: "aws_s3_get_object", Description: "Get an object from S3 (returns metadata and body as text for text types, base64 for binary)",
		Parameters: map[string]string{"bucket": "Bucket name", "key": "Object key"},
		Required:   []string{"bucket", "key"},
	},
	{
		Name: "aws_s3_put_object", Description: "Upload an object to S3",
		Parameters: map[string]string{"bucket": "Bucket name", "key": "Object key", "body": "Object content (text)", "content_type": "MIME type (default: application/octet-stream)"},
		Required:   []string{"bucket", "key", "body"},
	},
	{
		Name: "aws_s3_delete_object", Description: "Delete an object from S3",
		Parameters: map[string]string{"bucket": "Bucket name", "key": "Object key"},
		Required:   []string{"bucket", "key"},
	},
	{
		Name: "aws_s3_head_object", Description: "Get metadata for an S3 object without downloading it",
		Parameters: map[string]string{"bucket": "Bucket name", "key": "Object key"},
		Required:   []string{"bucket", "key"},
	},
	{
		Name: "aws_s3_copy_object", Description: "Copy an object within S3",
		Parameters: map[string]string{"source_bucket": "Source bucket name", "source_key": "Source object key", "dest_bucket": "Destination bucket name", "dest_key": "Destination object key"},
		Required:   []string{"source_bucket", "source_key", "dest_bucket", "dest_key"},
	},

	// ── EC2 ──────────────────────────────────────────────────────────
	{
		Name: "aws_ec2_describe_instances", Description: "List EC2 instances with optional filters",
		Parameters: map[string]string{"instance_ids": "Comma-separated instance IDs", "filters": "JSON array of filters [{\"Name\":\"tag:Env\",\"Values\":[\"prod\"]}]", "max_results": "Maximum number of results"},
	},
	{
		Name: "aws_ec2_describe_instance", Description: "Get details for a specific EC2 instance",
		Parameters: map[string]string{"instance_id": "Instance ID"},
		Required:   []string{"instance_id"},
	},
	{
		Name: "aws_ec2_start_instances", Description: "Start one or more EC2 instances",
		Parameters: map[string]string{"instance_ids": "Comma-separated instance IDs to start"},
		Required:   []string{"instance_ids"},
	},
	{
		Name: "aws_ec2_stop_instances", Description: "Stop one or more EC2 instances",
		Parameters: map[string]string{"instance_ids": "Comma-separated instance IDs to stop"},
		Required:   []string{"instance_ids"},
	},
	{
		Name: "aws_ec2_describe_security_groups", Description: "List security groups with optional filters",
		Parameters: map[string]string{"group_ids": "Comma-separated security group IDs", "filters": "JSON array of filters"},
	},
	{
		Name: "aws_ec2_describe_vpcs", Description: "List VPCs",
		Parameters: map[string]string{"vpc_ids": "Comma-separated VPC IDs", "filters": "JSON array of filters"},
	},
	{
		Name: "aws_ec2_describe_subnets", Description: "List subnets",
		Parameters: map[string]string{"subnet_ids": "Comma-separated subnet IDs", "filters": "JSON array of filters"},
	},
	{
		Name: "aws_ec2_describe_images", Description: "List AMI images",
		Parameters: map[string]string{"image_ids": "Comma-separated AMI IDs", "owners": "Comma-separated owner IDs or aliases (self, amazon)", "filters": "JSON array of filters"},
	},
	{
		Name: "aws_ec2_describe_volumes", Description: "List EBS volumes",
		Parameters: map[string]string{"volume_ids": "Comma-separated volume IDs", "filters": "JSON array of filters"},
	},
	{
		Name: "aws_ec2_describe_addresses", Description: "List Elastic IP addresses",
		Parameters: map[string]string{"allocation_ids": "Comma-separated allocation IDs", "filters": "JSON array of filters"},
	},
	{
		Name: "aws_ec2_describe_key_pairs", Description: "List EC2 key pairs",
		Parameters: map[string]string{"key_names": "Comma-separated key pair names"},
	},

	// ── Lambda ───────────────────────────────────────────────────────
	{
		Name: "aws_lambda_list_functions", Description: "List Lambda functions",
		Parameters: map[string]string{"max_items": "Maximum number of functions to return"},
	},
	{
		Name: "aws_lambda_get_function", Description: "Get details about a Lambda function",
		Parameters: map[string]string{"function_name": "Function name or ARN"},
		Required:   []string{"function_name"},
	},
	{
		Name: "aws_lambda_invoke", Description: "Invoke a Lambda function",
		Parameters: map[string]string{"function_name": "Function name or ARN", "payload": "JSON payload to pass to the function", "invocation_type": "RequestResponse (sync, default), Event (async), or DryRun"},
		Required:   []string{"function_name"},
	},
	{
		Name: "aws_lambda_list_event_source_mappings", Description: "List event source mappings for a function",
		Parameters: map[string]string{"function_name": "Function name or ARN"},
	},
	{
		Name: "aws_lambda_get_function_configuration", Description: "Get the configuration of a Lambda function",
		Parameters: map[string]string{"function_name": "Function name or ARN"},
		Required:   []string{"function_name"},
	},

	// ── IAM ──────────────────────────────────────────────────────────
	{
		Name: "aws_iam_list_users", Description: "List IAM users",
		Parameters: map[string]string{"path_prefix": "Path prefix for filtering (default /)", "max_items": "Maximum number of users to return"},
	},
	{
		Name: "aws_iam_get_user", Description: "Get details about an IAM user",
		Parameters: map[string]string{"username": "IAM username (omit for current user)"},
	},
	{
		Name: "aws_iam_list_roles", Description: "List IAM roles",
		Parameters: map[string]string{"path_prefix": "Path prefix for filtering (default /)", "max_items": "Maximum number of roles to return"},
	},
	{
		Name: "aws_iam_get_role", Description: "Get details about an IAM role",
		Parameters: map[string]string{"role_name": "IAM role name"},
		Required:   []string{"role_name"},
	},
	{
		Name: "aws_iam_list_policies", Description: "List IAM policies",
		Parameters: map[string]string{"scope": "Scope: All, AWS, Local (default All)", "only_attached": "Only show attached policies (true/false)", "path_prefix": "Path prefix filter", "max_items": "Maximum number of policies to return"},
	},
	{
		Name: "aws_iam_get_policy", Description: "Get details about an IAM policy",
		Parameters: map[string]string{"policy_arn": "Policy ARN"},
		Required:   []string{"policy_arn"},
	},
	{
		Name: "aws_iam_list_groups", Description: "List IAM groups",
		Parameters: map[string]string{"path_prefix": "Path prefix for filtering", "max_items": "Maximum number of groups to return"},
	},
	{
		Name: "aws_iam_list_attached_role_policies", Description: "List policies attached to an IAM role",
		Parameters: map[string]string{"role_name": "IAM role name", "path_prefix": "Path prefix filter"},
		Required:   []string{"role_name"},
	},
	{
		Name: "aws_iam_list_attached_user_policies", Description: "List policies attached to an IAM user",
		Parameters: map[string]string{"username": "IAM username", "path_prefix": "Path prefix filter"},
		Required:   []string{"username"},
	},
	{
		Name: "aws_iam_list_attached_group_policies", Description: "List policies attached to an IAM group",
		Parameters: map[string]string{"group_name": "IAM group name", "path_prefix": "Path prefix filter"},
		Required:   []string{"group_name"},
	},

	// ── CloudWatch ───────────────────────────────────────────────────
	{
		Name: "aws_cloudwatch_list_metrics", Description: "List CloudWatch metrics",
		Parameters: map[string]string{"namespace": "Metric namespace (e.g. AWS/EC2)", "metric_name": "Metric name filter"},
	},
	{
		Name: "aws_cloudwatch_get_metric_data", Description: "Get CloudWatch metric data points",
		Parameters: map[string]string{"metric_name": "Metric name", "namespace": "Metric namespace", "stat": "Statistic: Average, Sum, Minimum, Maximum, SampleCount", "period": "Period in seconds (default 300)", "start_time": "Start time (RFC3339 or relative e.g. -1h)", "end_time": "End time (RFC3339 or relative, default now)", "dimensions": "JSON object of dimension key-value pairs"},
		Required:   []string{"metric_name", "namespace", "stat"},
	},
	{
		Name: "aws_cloudwatch_describe_alarms", Description: "List CloudWatch alarms",
		Parameters: map[string]string{"alarm_names": "Comma-separated alarm names", "state_value": "Filter by state: OK, ALARM, INSUFFICIENT_DATA", "max_records": "Maximum number of alarms to return"},
	},
	{
		Name: "aws_cloudwatch_get_metric_statistics", Description: "Get statistics for a specific CloudWatch metric",
		Parameters: map[string]string{"namespace": "Metric namespace", "metric_name": "Metric name", "start_time": "Start time (RFC3339 or relative e.g. -1h)", "end_time": "End time (RFC3339)", "period": "Period in seconds", "statistics": "Comma-separated: Average, Sum, Minimum, Maximum, SampleCount", "dimensions": "JSON object of dimension key-value pairs"},
		Required:   []string{"namespace", "metric_name", "start_time", "period", "statistics"},
	},

	// ── ECS ──────────────────────────────────────────────────────────
	{
		Name: "aws_ecs_list_clusters", Description: "List ECS clusters",
		Parameters: map[string]string{},
	},
	{
		Name: "aws_ecs_describe_clusters", Description: "Get details about one or more ECS clusters",
		Parameters: map[string]string{"clusters": "Comma-separated cluster names or ARNs"},
		Required:   []string{"clusters"},
	},
	{
		Name: "aws_ecs_list_services", Description: "List services in an ECS cluster",
		Parameters: map[string]string{"cluster": "Cluster name or ARN"},
		Required:   []string{"cluster"},
	},
	{
		Name: "aws_ecs_describe_services", Description: "Get details about one or more ECS services",
		Parameters: map[string]string{"cluster": "Cluster name or ARN", "services": "Comma-separated service names or ARNs"},
		Required:   []string{"cluster", "services"},
	},
	{
		Name: "aws_ecs_list_tasks", Description: "List tasks in an ECS cluster",
		Parameters: map[string]string{"cluster": "Cluster name or ARN", "service_name": "Filter by service name", "desired_status": "Filter by status: RUNNING, PENDING, STOPPED"},
	},
	{
		Name: "aws_ecs_describe_tasks", Description: "Get details about one or more ECS tasks",
		Parameters: map[string]string{"cluster": "Cluster name or ARN", "tasks": "Comma-separated task IDs or ARNs"},
		Required:   []string{"cluster", "tasks"},
	},
	{
		Name: "aws_ecs_list_task_definitions", Description: "List ECS task definition families or revisions",
		Parameters: map[string]string{"family_prefix": "Task definition family prefix filter", "status": "Filter: ACTIVE or INACTIVE"},
	},
	{
		Name: "aws_ecs_describe_task_definition", Description: "Get details about an ECS task definition",
		Parameters: map[string]string{"task_definition": "Task definition family:revision or full ARN"},
		Required:   []string{"task_definition"},
	},

	// ── SNS ──────────────────────────────────────────────────────────
	{
		Name: "aws_sns_list_topics", Description: "List SNS topics",
		Parameters: map[string]string{},
	},
	{
		Name: "aws_sns_get_topic_attributes", Description: "Get attributes for an SNS topic",
		Parameters: map[string]string{"topic_arn": "SNS topic ARN"},
		Required:   []string{"topic_arn"},
	},
	{
		Name: "aws_sns_list_subscriptions", Description: "List SNS subscriptions",
		Parameters: map[string]string{"topic_arn": "Filter by topic ARN"},
	},
	{
		Name: "aws_sns_publish", Description: "Publish a message to an SNS topic",
		Parameters: map[string]string{"topic_arn": "SNS topic ARN", "message": "Message body", "subject": "Message subject (for email subscriptions)"},
		Required:   []string{"topic_arn", "message"},
	},

	// ── SQS ──────────────────────────────────────────────────────────
	{
		Name: "aws_sqs_list_queues", Description: "List SQS queues",
		Parameters: map[string]string{"queue_name_prefix": "Queue name prefix filter"},
	},
	{
		Name: "aws_sqs_get_queue_attributes", Description: "Get attributes for an SQS queue",
		Parameters: map[string]string{"queue_url": "SQS queue URL"},
		Required:   []string{"queue_url"},
	},
	{
		Name: "aws_sqs_send_message", Description: "Send a message to an SQS queue",
		Parameters: map[string]string{"queue_url": "SQS queue URL", "message_body": "Message content", "delay_seconds": "Delay in seconds (0-900)"},
		Required:   []string{"queue_url", "message_body"},
	},
	{
		Name: "aws_sqs_receive_message", Description: "Receive messages from an SQS queue",
		Parameters: map[string]string{"queue_url": "SQS queue URL", "max_messages": "Max messages to receive (1-10, default 1)", "wait_time_seconds": "Long poll wait time (0-20)"},
		Required:   []string{"queue_url"},
	},
	{
		Name: "aws_sqs_delete_message", Description: "Delete a message from an SQS queue",
		Parameters: map[string]string{"queue_url": "SQS queue URL", "receipt_handle": "Receipt handle from receive"},
		Required:   []string{"queue_url", "receipt_handle"},
	},
	{
		Name: "aws_sqs_purge_queue", Description: "Purge all messages from an SQS queue",
		Parameters: map[string]string{"queue_url": "SQS queue URL"},
		Required:   []string{"queue_url"},
	},

	// ── DynamoDB ─────────────────────────────────────────────────────
	{
		Name: "aws_dynamodb_list_tables", Description: "List DynamoDB tables",
		Parameters: map[string]string{"limit": "Maximum number of tables to return"},
	},
	{
		Name: "aws_dynamodb_describe_table", Description: "Get details about a DynamoDB table",
		Parameters: map[string]string{"table_name": "Table name"},
		Required:   []string{"table_name"},
	},
	{
		Name: "aws_dynamodb_get_item", Description: "Get an item from a DynamoDB table",
		Parameters: map[string]string{"table_name": "Table name", "key": "JSON object with key attributes (e.g. {\"id\":{\"S\":\"123\"}})"},
		Required:   []string{"table_name", "key"},
	},
	{
		Name: "aws_dynamodb_put_item", Description: "Put an item into a DynamoDB table",
		Parameters: map[string]string{"table_name": "Table name", "item": "JSON object with item attributes in DynamoDB format"},
		Required:   []string{"table_name", "item"},
	},
	{
		Name: "aws_dynamodb_query", Description: "Query a DynamoDB table",
		Parameters: map[string]string{"table_name": "Table name", "key_condition_expression": "Key condition expression", "expression_attribute_values": "JSON object of expression attribute values in DynamoDB format", "expression_attribute_names": "JSON object of expression attribute name placeholders", "index_name": "Secondary index name", "limit": "Maximum number of items to return", "scan_index_forward": "Sort ascending (true, default) or descending (false)"},
		Required:   []string{"table_name", "key_condition_expression", "expression_attribute_values"},
	},
	{
		Name: "aws_dynamodb_scan", Description: "Scan a DynamoDB table",
		Parameters: map[string]string{"table_name": "Table name", "filter_expression": "Filter expression", "expression_attribute_values": "JSON object of expression attribute values", "expression_attribute_names": "JSON object of expression attribute name placeholders", "limit": "Maximum number of items to return"},
		Required:   []string{"table_name"},
	},
	{
		Name: "aws_dynamodb_delete_item", Description: "Delete an item from a DynamoDB table",
		Parameters: map[string]string{"table_name": "Table name", "key": "JSON object with key attributes in DynamoDB format"},
		Required:   []string{"table_name", "key"},
	},

	// ── CloudFormation ───────────────────────────────────────────────
	{
		Name: "aws_cloudformation_list_stacks", Description: "List CloudFormation stacks",
		Parameters: map[string]string{"status_filter": "Comma-separated status filter (e.g. CREATE_COMPLETE,UPDATE_COMPLETE)"},
	},
	{
		Name: "aws_cloudformation_describe_stack", Description: "Get details about a CloudFormation stack",
		Parameters: map[string]string{"stack_name": "Stack name or ID"},
		Required:   []string{"stack_name"},
	},
	{
		Name: "aws_cloudformation_list_stack_resources", Description: "List resources in a CloudFormation stack",
		Parameters: map[string]string{"stack_name": "Stack name or ID"},
		Required:   []string{"stack_name"},
	},
	{
		Name: "aws_cloudformation_get_template", Description: "Get the template for a CloudFormation stack",
		Parameters: map[string]string{"stack_name": "Stack name or ID"},
		Required:   []string{"stack_name"},
	},
	{
		Name: "aws_cloudformation_describe_stack_events", Description: "List events for a CloudFormation stack",
		Parameters: map[string]string{"stack_name": "Stack name or ID"},
		Required:   []string{"stack_name"},
	},
}
