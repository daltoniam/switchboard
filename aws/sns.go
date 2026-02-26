package aws

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func snsListTopics(ctx context.Context, a *integration, _ map[string]any) (*mcp.ToolResult, error) {
	out, err := a.snsClient.ListTopics(ctx, &sns.ListTopicsInput{})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func snsGetTopicAttributes(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.snsClient.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
		TopicArn: aws.String(argStr(args, "topic_arn")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func snsListSubscriptions(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	if arn := argStr(args, "topic_arn"); arn != "" {
		out, err := a.snsClient.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
			TopicArn: aws.String(arn),
		})
		if err != nil {
			return errResult(err)
		}
		return jsonResult(out)
	}
	out, err := a.snsClient.ListSubscriptions(ctx, &sns.ListSubscriptionsInput{})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func snsPublish(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &sns.PublishInput{
		TopicArn: aws.String(argStr(args, "topic_arn")),
		Message:  aws.String(argStr(args, "message")),
	}
	if v := argStr(args, "subject"); v != "" {
		input.Subject = aws.String(v)
	}
	out, err := a.snsClient.Publish(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
