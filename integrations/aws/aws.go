package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

func (a *integration) Configure(creds mcp.Credentials) error {
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

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
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

func (a *integration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, a, args)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error)

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argInt32(args map[string]any, key string) int32 {
	switch v := args[key].(type) {
	case float64:
		return int32(v)
	case int:
		return int32(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 32)
		return int32(n)
	}
	return 0
}

func argInt64(args map[string]any, key string) int64 {
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
}

var dispatch = map[string]handlerFunc{
	// STS
	"aws_get_caller_identity": getCallerIdentity,

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
	"aws_lambda_list_functions":             lambdaListFunctions,
	"aws_lambda_get_function":               lambdaGetFunction,
	"aws_lambda_invoke":                     lambdaInvoke,
	"aws_lambda_list_event_source_mappings": lambdaListEventSourceMappings,
	"aws_lambda_get_function_configuration": lambdaGetFunctionConfiguration,

	// IAM
	"aws_iam_list_users":                   iamListUsers,
	"aws_iam_get_user":                     iamGetUser,
	"aws_iam_list_roles":                   iamListRoles,
	"aws_iam_get_role":                     iamGetRole,
	"aws_iam_list_policies":                iamListPolicies,
	"aws_iam_get_policy":                   iamGetPolicy,
	"aws_iam_list_groups":                  iamListGroups,
	"aws_iam_list_attached_role_policies":  iamListAttachedRolePolicies,
	"aws_iam_list_attached_user_policies":  iamListAttachedUserPolicies,
	"aws_iam_list_attached_group_policies": iamListAttachedGroupPolicies,

	// CloudWatch
	"aws_cloudwatch_list_metrics":          cwListMetrics,
	"aws_cloudwatch_get_metric_data":       cwGetMetricData,
	"aws_cloudwatch_describe_alarms":       cwDescribeAlarms,
	"aws_cloudwatch_get_metric_statistics": cwGetMetricStatistics,

	// ECS
	"aws_ecs_list_clusters":            ecsListClusters,
	"aws_ecs_describe_clusters":        ecsDescribeClusters,
	"aws_ecs_list_services":            ecsListServices,
	"aws_ecs_describe_services":        ecsDescribeServices,
	"aws_ecs_list_tasks":               ecsListTasks,
	"aws_ecs_describe_tasks":           ecsDescribeTasks,
	"aws_ecs_list_task_definitions":    ecsListTaskDefinitions,
	"aws_ecs_describe_task_definition": ecsDescribeTaskDefinition,

	// SNS
	"aws_sns_list_topics":          snsListTopics,
	"aws_sns_get_topic_attributes": snsGetTopicAttributes,
	"aws_sns_list_subscriptions":   snsListSubscriptions,
	"aws_sns_publish":              snsPublish,

	// SQS
	"aws_sqs_list_queues":          sqsListQueues,
	"aws_sqs_get_queue_attributes": sqsGetQueueAttributes,
	"aws_sqs_send_message":         sqsSendMessage,
	"aws_sqs_receive_message":      sqsReceiveMessage,
	"aws_sqs_delete_message":       sqsDeleteMessage,
	"aws_sqs_purge_queue":          sqsPurgeQueue,

	// DynamoDB
	"aws_dynamodb_list_tables":    dynamoListTables,
	"aws_dynamodb_describe_table": dynamoDescribeTable,
	"aws_dynamodb_get_item":       dynamoGetItem,
	"aws_dynamodb_put_item":       dynamoPutItem,
	"aws_dynamodb_query":          dynamoQuery,
	"aws_dynamodb_scan":           dynamoScan,
	"aws_dynamodb_delete_item":    dynamoDeleteItem,

	// CloudFormation
	"aws_cloudformation_list_stacks":           cfnListStacks,
	"aws_cloudformation_describe_stack":        cfnDescribeStack,
	"aws_cloudformation_list_stack_resources":  cfnListStackResources,
	"aws_cloudformation_get_template":          cfnGetTemplate,
	"aws_cloudformation_describe_stack_events": cfnDescribeStackEvents,
}
