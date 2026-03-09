package aws

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── STS ──────────────────────────────────────────────────────────
	"aws_get_caller_identity": {"Account", "Arn", "UserId"},

	// ── S3 ───────────────────────────────────────────────────────────
	"aws_s3_list_buckets":  {"Buckets[].Name", "Buckets[].CreationDate"},
	"aws_s3_list_objects":  {"Contents[].Key", "Contents[].Size", "Contents[].LastModified", "Contents[].StorageClass", "IsTruncated", "NextContinuationToken"},
	"aws_s3_get_object":    {"ContentType", "ContentLength", "LastModified", "ETag", "Body"},
	"aws_s3_head_object":   {"ContentType", "ContentLength", "LastModified", "ETag", "Metadata"},

	// ── EC2 ──────────────────────────────────────────────────────────
	"aws_ec2_describe_instances":       {"Reservations[].Instances[].InstanceId", "Reservations[].Instances[].InstanceType", "Reservations[].Instances[].State.Name", "Reservations[].Instances[].PublicIpAddress", "Reservations[].Instances[].PrivateIpAddress", "Reservations[].Instances[].LaunchTime", "Reservations[].Instances[].Tags"},
	"aws_ec2_describe_instance":        {"InstanceId", "InstanceType", "State.Name", "PublicIpAddress", "PrivateIpAddress", "LaunchTime", "Tags", "SecurityGroups", "SubnetId", "VpcId", "Architecture", "Platform"},
	"aws_ec2_describe_security_groups": {"SecurityGroups[].GroupId", "SecurityGroups[].GroupName", "SecurityGroups[].Description", "SecurityGroups[].VpcId", "SecurityGroups[].IpPermissions", "SecurityGroups[].IpPermissionsEgress"},
	"aws_ec2_describe_vpcs":            {"Vpcs[].VpcId", "Vpcs[].CidrBlock", "Vpcs[].State", "Vpcs[].IsDefault", "Vpcs[].Tags"},
	"aws_ec2_describe_subnets":         {"Subnets[].SubnetId", "Subnets[].VpcId", "Subnets[].CidrBlock", "Subnets[].AvailabilityZone", "Subnets[].AvailableIpAddressCount", "Subnets[].Tags"},
	"aws_ec2_describe_images":          {"Images[].ImageId", "Images[].Name", "Images[].State", "Images[].Architecture", "Images[].CreationDate", "Images[].Description"},
	"aws_ec2_describe_volumes":         {"Volumes[].VolumeId", "Volumes[].Size", "Volumes[].State", "Volumes[].VolumeType", "Volumes[].AvailabilityZone", "Volumes[].Attachments[].InstanceId", "Volumes[].Tags"},
	"aws_ec2_describe_addresses":       {"Addresses[].PublicIp", "Addresses[].AllocationId", "Addresses[].InstanceId", "Addresses[].AssociationId", "Addresses[].Domain"},
	"aws_ec2_describe_key_pairs":       {"KeyPairs[].KeyName", "KeyPairs[].KeyPairId", "KeyPairs[].KeyFingerprint", "KeyPairs[].CreateTime"},

	// ── Lambda ───────────────────────────────────────────────────────
	"aws_lambda_list_functions":              {"Functions[].FunctionName", "Functions[].FunctionArn", "Functions[].Runtime", "Functions[].Handler", "Functions[].CodeSize", "Functions[].LastModified", "Functions[].MemorySize", "Functions[].Timeout"},
	"aws_lambda_get_function":                {"Configuration.FunctionName", "Configuration.FunctionArn", "Configuration.Runtime", "Configuration.Handler", "Configuration.CodeSize", "Configuration.LastModified", "Configuration.MemorySize", "Configuration.Timeout", "Configuration.Environment", "Code.Location"},
	"aws_lambda_list_event_source_mappings":  {"EventSourceMappings[].UUID", "EventSourceMappings[].EventSourceArn", "EventSourceMappings[].FunctionArn", "EventSourceMappings[].State", "EventSourceMappings[].BatchSize"},
	"aws_lambda_get_function_configuration":  {"FunctionName", "FunctionArn", "Runtime", "Handler", "CodeSize", "LastModified", "MemorySize", "Timeout", "Environment", "VpcConfig"},

	// ── IAM ──────────────────────────────────────────────────────────
	"aws_iam_list_users":                    {"Users[].UserName", "Users[].UserId", "Users[].Arn", "Users[].CreateDate", "Users[].PasswordLastUsed"},
	"aws_iam_get_user":                      {"User.UserName", "User.UserId", "User.Arn", "User.CreateDate", "User.PasswordLastUsed", "User.Tags"},
	"aws_iam_list_roles":                    {"Roles[].RoleName", "Roles[].RoleId", "Roles[].Arn", "Roles[].CreateDate", "Roles[].Description"},
	"aws_iam_get_role":                      {"Role.RoleName", "Role.RoleId", "Role.Arn", "Role.CreateDate", "Role.Description", "Role.AssumeRolePolicyDocument", "Role.Tags"},
	"aws_iam_list_policies":                 {"Policies[].PolicyName", "Policies[].PolicyId", "Policies[].Arn", "Policies[].CreateDate", "Policies[].AttachmentCount", "Policies[].IsAttachable"},
	"aws_iam_get_policy":                    {"Policy.PolicyName", "Policy.PolicyId", "Policy.Arn", "Policy.CreateDate", "Policy.AttachmentCount", "Policy.Description"},
	"aws_iam_list_groups":                   {"Groups[].GroupName", "Groups[].GroupId", "Groups[].Arn", "Groups[].CreateDate"},
	"aws_iam_list_attached_role_policies":   {"AttachedPolicies[].PolicyName", "AttachedPolicies[].PolicyArn"},
	"aws_iam_list_attached_user_policies":   {"AttachedPolicies[].PolicyName", "AttachedPolicies[].PolicyArn"},
	"aws_iam_list_attached_group_policies":  {"AttachedPolicies[].PolicyName", "AttachedPolicies[].PolicyArn"},

	// ── CloudWatch ───────────────────────────────────────────────────
	"aws_cloudwatch_list_metrics":          {"Metrics[].MetricName", "Metrics[].Namespace", "Metrics[].Dimensions"},
	"aws_cloudwatch_get_metric_data":       {"MetricDataResults[].Id", "MetricDataResults[].Label", "MetricDataResults[].Timestamps", "MetricDataResults[].Values", "MetricDataResults[].StatusCode"},
	"aws_cloudwatch_describe_alarms":       {"MetricAlarms[].AlarmName", "MetricAlarms[].StateValue", "MetricAlarms[].MetricName", "MetricAlarms[].Namespace", "MetricAlarms[].Statistic", "MetricAlarms[].Threshold"},
	"aws_cloudwatch_get_metric_statistics": {"Datapoints[].Timestamp", "Datapoints[].Average", "Datapoints[].Sum", "Datapoints[].Minimum", "Datapoints[].Maximum", "Datapoints[].SampleCount"},

	// ── ECS ──────────────────────────────────────────────────────────
	"aws_ecs_list_clusters":            {"clusterArns"},
	"aws_ecs_describe_clusters":        {"clusters[].clusterName", "clusters[].clusterArn", "clusters[].status", "clusters[].runningTasksCount", "clusters[].activeServicesCount", "clusters[].registeredContainerInstancesCount"},
	"aws_ecs_list_services":            {"serviceArns"},
	"aws_ecs_describe_services":        {"services[].serviceName", "services[].serviceArn", "services[].status", "services[].desiredCount", "services[].runningCount", "services[].taskDefinition", "services[].launchType"},
	"aws_ecs_list_tasks":               {"taskArns"},
	"aws_ecs_describe_tasks":           {"tasks[].taskArn", "tasks[].taskDefinitionArn", "tasks[].lastStatus", "tasks[].desiredStatus", "tasks[].cpu", "tasks[].memory", "tasks[].startedAt", "tasks[].containers[].name", "tasks[].containers[].lastStatus"},
	"aws_ecs_list_task_definitions":     {"taskDefinitionArns"},
	"aws_ecs_describe_task_definition":  {"taskDefinition.taskDefinitionArn", "taskDefinition.family", "taskDefinition.revision", "taskDefinition.status", "taskDefinition.cpu", "taskDefinition.memory", "taskDefinition.containerDefinitions[].name", "taskDefinition.containerDefinitions[].image", "taskDefinition.containerDefinitions[].cpu", "taskDefinition.containerDefinitions[].memory"},

	// ── SNS ──────────────────────────────────────────────────────────
	"aws_sns_list_topics":          {"Topics[].TopicArn"},
	"aws_sns_get_topic_attributes": {"Attributes"},
	"aws_sns_list_subscriptions":   {"Subscriptions[].SubscriptionArn", "Subscriptions[].TopicArn", "Subscriptions[].Protocol", "Subscriptions[].Endpoint"},

	// ── SQS ──────────────────────────────────────────────────────────
	"aws_sqs_list_queues":          {"QueueUrls"},
	"aws_sqs_get_queue_attributes": {"Attributes"},
	"aws_sqs_receive_message":      {"Messages[].MessageId", "Messages[].Body", "Messages[].ReceiptHandle", "Messages[].MD5OfBody"},

	// ── DynamoDB ─────────────────────────────────────────────────────
	"aws_dynamodb_list_tables":    {"TableNames"},
	"aws_dynamodb_describe_table": {"Table.TableName", "Table.TableStatus", "Table.ItemCount", "Table.TableSizeBytes", "Table.KeySchema", "Table.AttributeDefinitions", "Table.ProvisionedThroughput", "Table.CreationDateTime"},
	"aws_dynamodb_get_item":       {"Item"},
	"aws_dynamodb_query":          {"Items", "Count", "ScannedCount"},
	"aws_dynamodb_scan":           {"Items", "Count", "ScannedCount"},

	// ── CloudFormation ───────────────────────────────────────────────
	"aws_cloudformation_list_stacks":          {"StackSummaries[].StackName", "StackSummaries[].StackId", "StackSummaries[].StackStatus", "StackSummaries[].CreationTime", "StackSummaries[].LastUpdatedTime"},
	"aws_cloudformation_describe_stack":       {"Stacks[].StackName", "Stacks[].StackId", "Stacks[].StackStatus", "Stacks[].Parameters", "Stacks[].Outputs", "Stacks[].CreationTime"},
	"aws_cloudformation_list_stack_resources": {"StackResourceSummaries[].LogicalResourceId", "StackResourceSummaries[].PhysicalResourceId", "StackResourceSummaries[].ResourceType", "StackResourceSummaries[].ResourceStatus"},
	"aws_cloudformation_get_template":         {"TemplateBody"},
	"aws_cloudformation_describe_stack_events": {"StackEvents[].EventId", "StackEvents[].ResourceType", "StackEvents[].LogicalResourceId", "StackEvents[].ResourceStatus", "StackEvents[].Timestamp", "StackEvents[].ResourceStatusReason"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("aws: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
