package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	mcp "github.com/daltoniam/switchboard"
)

func sqsListQueues(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &sqs.ListQueuesInput{}
	if v := argStr(args, "queue_name_prefix"); v != "" {
		input.QueueNamePrefix = aws.String(v)
	}
	out, err := a.sqsClient.ListQueues(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func sqsGetQueueAttributes(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(argStr(args, "queue_url")),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func sqsSendMessage(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(argStr(args, "queue_url")),
		MessageBody: aws.String(argStr(args, "message_body")),
	}
	if v := argInt32(args, "delay_seconds"); v > 0 {
		input.DelaySeconds = v
	}
	out, err := a.sqsClient.SendMessage(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func sqsReceiveMessage(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(argStr(args, "queue_url")),
	}
	if v := argInt32(args, "max_messages"); v > 0 {
		input.MaxNumberOfMessages = v
	}
	if v := argInt32(args, "wait_time_seconds"); v > 0 {
		input.WaitTimeSeconds = v
	}
	out, err := a.sqsClient.ReceiveMessage(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func sqsDeleteMessage(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := a.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(argStr(args, "queue_url")),
		ReceiptHandle: aws.String(argStr(args, "receipt_handle")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func sqsPurgeQueue(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := a.sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(argStr(args, "queue_url")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}
