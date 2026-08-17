package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	mcp "github.com/daltoniam/switchboard"
)

func sqsListQueues(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &sqs.ListQueuesInput{}
	if v := r.Str("queue_name_prefix"); v != "" {
		input.QueueNamePrefix = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.sqsClient.ListQueues(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func sqsGetQueueAttributes(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueURL := r.Str("queue_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func sqsSendMessage(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(r.Str("queue_url")),
		MessageBody: aws.String(r.Str("message_body")),
	}
	if v := r.Int32("delay_seconds"); v > 0 {
		input.DelaySeconds = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.sqsClient.SendMessage(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func sqsReceiveMessage(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(r.Str("queue_url")),
	}
	if v := r.Int32("max_messages"); v > 0 {
		input.MaxNumberOfMessages = v
	}
	if v := r.Int32("wait_time_seconds"); v > 0 {
		input.WaitTimeSeconds = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.sqsClient.ReceiveMessage(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func sqsDeleteMessage(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueURL := r.Str("queue_url")
	receiptHandle := r.Str("receipt_handle")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := a.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func sqsPurgeQueue(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queueURL := r.Str("queue_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := a.sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(queueURL),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}
