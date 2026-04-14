package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*integration)(nil)
	_ mcp.FieldCompactionIntegration = (*integration)(nil)
	_ mcp.PlainTextCredentials       = (*integration)(nil)
)

func (a *integration) PlainTextKeys() []string {
	return []string{"access_key_id", "region"}
}

type integration struct {
	cfg          aws.Config
	region       string
	s3Client     *s3.Client
	ec2Client    *ec2.Client
	lambdaClient *lambda.Client
	iamClient    *iam.Client
	cwClient     *cloudwatch.Client
	stsClient    *sts.Client
	ecsClient    *ecs.Client
	snsClient    *sns.Client
	sqsClient    *sqs.Client
	dynamoClient *dynamodb.Client
	cfnClient    *cloudformation.Client
}

func New() mcp.Integration {
	return &integration{}
}

func (a *integration) Name() string { return "aws" }

func (a *integration) Configure(ctx context.Context, creds mcp.Credentials) error {
	region := creds["region"]
	if region == "" {
		region = "us-east-1"
	}
	a.region = region

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	accessKey := creds["access_key_id"]
	secretKey := creds["secret_access_key"]
	if accessKey != "" && secretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, creds["session_token"]),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("aws: failed to load config: %w", err)
	}
	a.cfg = cfg

	a.s3Client = s3.NewFromConfig(cfg)
	a.ec2Client = ec2.NewFromConfig(cfg)
	a.lambdaClient = lambda.NewFromConfig(cfg)
	a.iamClient = iam.NewFromConfig(cfg)
	a.cwClient = cloudwatch.NewFromConfig(cfg)
	a.stsClient = sts.NewFromConfig(cfg)
	a.ecsClient = ecs.NewFromConfig(cfg)
	a.snsClient = sns.NewFromConfig(cfg)
	a.sqsClient = sqs.NewFromConfig(cfg)
	a.dynamoClient = dynamodb.NewFromConfig(cfg)
	a.cfnClient = cloudformation.NewFromConfig(cfg)
	return nil
}

func (a *integration) Healthy(ctx context.Context) bool {
	_, err := a.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	return err == nil
}

func (a *integration) Tools() []mcp.ToolDefinition {
	return tools
}

func (a *integration) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (a *integration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, a, args)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error)

func wrapRetryable(err error) error {
	if err == nil {
		return nil
	}
	var apiErr interface{ ErrorCode() string }
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		switch code {
		case "Throttling", "ThrottlingException", "TooManyRequestsException",
			"RequestLimitExceeded", "ProvisionedThroughputExceededException",
			"RequestThrottled", "SlowDown", "EC2ThrottledException":
			return &mcp.RetryableError{StatusCode: 429, Err: err}
		case "InternalError", "InternalFailure", "ServiceUnavailable":
			return &mcp.RetryableError{StatusCode: 500, Err: err}
		}
	}
	return err
}

func errResult(err error) (*mcp.ToolResult, error) {
	return mcp.ErrResult(wrapRetryable(err))
}

var dispatch = map[mcp.ToolName]handlerFunc{
	// STS
	mcp.ToolName("aws_get_caller_identity"): getCallerIdentity,

	// S3
	"aws_s3_list_buckets":  s3ListBuckets,
	"aws_s3_list_objects":  s3ListObjects,
	"aws_s3_get_object":    s3GetObject,
	"aws_s3_put_object":    s3PutObject,
	"aws_s3_delete_object": s3DeleteObject,
	"aws_s3_head_object":   s3HeadObject,
	"aws_s3_copy_object":   s3CopyObject,

	// EC2
	"aws_ec2_describe_instances":       ec2DescribeInstances,
	"aws_ec2_describe_instance":        ec2DescribeInstance,
	"aws_ec2_start_instances":          ec2StartInstances,
	"aws_ec2_stop_instances":           ec2StopInstances,
	"aws_ec2_describe_security_groups": ec2DescribeSecurityGroups,
	"aws_ec2_describe_vpcs":            ec2DescribeVPCs,
	"aws_ec2_describe_subnets":         ec2DescribeSubnets,
	"aws_ec2_describe_images":          ec2DescribeImages,
	"aws_ec2_describe_volumes":         ec2DescribeVolumes,
	"aws_ec2_describe_addresses":       ec2DescribeAddresses,
	"aws_ec2_describe_key_pairs":       ec2DescribeKeyPairs,

	// Lambda
	mcp.ToolName("aws_lambda_list_functions"):             lambdaListFunctions,
	mcp.ToolName("aws_lambda_get_function"):               lambdaGetFunction,
	mcp.ToolName("aws_lambda_invoke"):                     lambdaInvoke,
	mcp.ToolName("aws_lambda_list_event_source_mappings"): lambdaListEventSourceMappings,
	mcp.ToolName("aws_lambda_get_function_configuration"): lambdaGetFunctionConfiguration,

	// IAM
	mcp.ToolName("aws_iam_list_users"):                   iamListUsers,
	mcp.ToolName("aws_iam_get_user"):                     iamGetUser,
	mcp.ToolName("aws_iam_list_roles"):                   iamListRoles,
	mcp.ToolName("aws_iam_get_role"):                     iamGetRole,
	mcp.ToolName("aws_iam_list_policies"):                iamListPolicies,
	mcp.ToolName("aws_iam_get_policy"):                   iamGetPolicy,
	mcp.ToolName("aws_iam_list_groups"):                  iamListGroups,
	mcp.ToolName("aws_iam_list_attached_role_policies"):  iamListAttachedRolePolicies,
	mcp.ToolName("aws_iam_list_attached_user_policies"):  iamListAttachedUserPolicies,
	mcp.ToolName("aws_iam_list_attached_group_policies"): iamListAttachedGroupPolicies,

	// CloudWatch
	mcp.ToolName("aws_cloudwatch_list_metrics"):          cwListMetrics,
	mcp.ToolName("aws_cloudwatch_get_metric_data"):       cwGetMetricData,
	mcp.ToolName("aws_cloudwatch_describe_alarms"):       cwDescribeAlarms,
	mcp.ToolName("aws_cloudwatch_get_metric_statistics"): cwGetMetricStatistics,

	// ECS
	mcp.ToolName("aws_ecs_list_clusters"):            ecsListClusters,
	mcp.ToolName("aws_ecs_describe_clusters"):        ecsDescribeClusters,
	mcp.ToolName("aws_ecs_list_services"):            ecsListServices,
	mcp.ToolName("aws_ecs_describe_services"):        ecsDescribeServices,
	mcp.ToolName("aws_ecs_list_tasks"):               ecsListTasks,
	mcp.ToolName("aws_ecs_describe_tasks"):           ecsDescribeTasks,
	mcp.ToolName("aws_ecs_list_task_definitions"):    ecsListTaskDefinitions,
	mcp.ToolName("aws_ecs_describe_task_definition"): ecsDescribeTaskDefinition,

	// SNS
	mcp.ToolName("aws_sns_list_topics"):          snsListTopics,
	mcp.ToolName("aws_sns_get_topic_attributes"): snsGetTopicAttributes,
	mcp.ToolName("aws_sns_list_subscriptions"):   snsListSubscriptions,
	mcp.ToolName("aws_sns_publish"):              snsPublish,

	// SQS
	mcp.ToolName("aws_sqs_list_queues"):          sqsListQueues,
	mcp.ToolName("aws_sqs_get_queue_attributes"): sqsGetQueueAttributes,
	mcp.ToolName("aws_sqs_send_message"):         sqsSendMessage,
	mcp.ToolName("aws_sqs_receive_message"):      sqsReceiveMessage,
	mcp.ToolName("aws_sqs_delete_message"):       sqsDeleteMessage,
	mcp.ToolName("aws_sqs_purge_queue"):          sqsPurgeQueue,

	// DynamoDB
	mcp.ToolName("aws_dynamodb_list_tables"):    dynamoListTables,
	mcp.ToolName("aws_dynamodb_describe_table"): dynamoDescribeTable,
	mcp.ToolName("aws_dynamodb_get_item"):       dynamoGetItem,
	mcp.ToolName("aws_dynamodb_put_item"):       dynamoPutItem,
	mcp.ToolName("aws_dynamodb_query"):          dynamoQuery,
	mcp.ToolName("aws_dynamodb_scan"):           dynamoScan,
	mcp.ToolName("aws_dynamodb_delete_item"):    dynamoDeleteItem,

	// CloudFormation
	mcp.ToolName("aws_cloudformation_list_stacks"):           cfnListStacks,
	mcp.ToolName("aws_cloudformation_describe_stack"):        cfnDescribeStack,
	mcp.ToolName("aws_cloudformation_list_stack_resources"):  cfnListStackResources,
	mcp.ToolName("aws_cloudformation_get_template"):          cfnGetTemplate,
	mcp.ToolName("aws_cloudformation_describe_stack_events"): cfnDescribeStackEvents,
}
