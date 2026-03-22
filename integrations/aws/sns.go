package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	mcp "github.com/daltoniam/switchboard"
)

func snsListTopics(ctx context.Context, a *integration, _ map[string]any) (*mcp.ToolResult, error) {
	out, err := a.snsClient.ListTopics(ctx, &sns.ListTopicsInput{})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func snsGetTopicAttributes(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	topicArn := r.Str("topic_arn")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.snsClient.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
		TopicArn: aws.String(topicArn),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func snsListSubscriptions(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	arn := r.Str("topic_arn")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if arn != "" {
		out, err := a.snsClient.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
			TopicArn: aws.String(arn),
		})
		if err != nil {
			return errResult(err)
		}
		return mcp.JSONResult(out)
	}
	out, err := a.snsClient.ListSubscriptions(ctx, &sns.ListSubscriptionsInput{})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func snsPublish(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &sns.PublishInput{
		TopicArn: aws.String(r.Str("topic_arn")),
		Message:  aws.String(r.Str("message")),
	}
	if v := r.Str("subject"); v != "" {
		input.Subject = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.snsClient.Publish(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}
