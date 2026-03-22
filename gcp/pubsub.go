package gcp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func pubsubListTopics(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	var topics []map[string]any
	it := g.pubsubClient.Topics(ctx)
	for {
		t, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		cfg, err := t.Config(ctx)
		if err != nil {
			topics = append(topics, map[string]any{"id": t.ID()})
			continue
		}
		topics = append(topics, map[string]any{
			"id":              t.ID(),
			"retention":       cfg.RetentionDuration,
			"message_storage": cfg.MessageStoragePolicy.AllowedPersistenceRegions,
		})
	}
	return mcp.JSONResult(topics)
}

func pubsubGetTopic(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	topicName := r.Str("topic")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	topic := g.pubsubClient.Topic(topicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return errResult(err)
	}
	if !exists {
		return &mcp.ToolResult{Data: "topic not found", IsError: true}, nil
	}
	cfg, err := topic.Config(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id":              topic.ID(),
		"retention":       cfg.RetentionDuration,
		"message_storage": cfg.MessageStoragePolicy.AllowedPersistenceRegions,
		"labels":          cfg.Labels,
		"schema_settings": cfg.SchemaSettings,
	})
}

func pubsubPublish(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	topicName := r.Str("topic")
	message := r.Str("message")
	attrsStr := r.Str("attributes")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	topic := g.pubsubClient.Topic(topicName)

	msg := &pubsub.Message{
		Data: []byte(message),
	}
	if v := attrsStr; v != "" {
		var attrs map[string]string
		if err := json.Unmarshal([]byte(v), &attrs); err != nil {
			return errResult(err)
		}
		msg.Attributes = attrs
	}

	result := topic.Publish(ctx, msg)
	serverID, err := result.Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"message_id": serverID})
}

func pubsubListSubscriptions(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	var subs []map[string]any
	it := g.pubsubClient.Subscriptions(ctx)
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		cfg, err := s.Config(ctx)
		if err != nil {
			subs = append(subs, map[string]any{"id": s.ID()})
			continue
		}
		subs = append(subs, map[string]any{
			"id":                      s.ID(),
			"topic":                   cfg.Topic.ID(),
			"ack_deadline":            cfg.AckDeadline.String(),
			"retention":               cfg.RetentionDuration.String(),
			"enable_message_ordering": cfg.EnableMessageOrdering,
		})
	}
	return mcp.JSONResult(subs)
}

func pubsubGetSubscription(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	subName := r.Str("subscription")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	sub := g.pubsubClient.Subscription(subName)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return errResult(err)
	}
	if !exists {
		return &mcp.ToolResult{Data: "subscription not found", IsError: true}, nil
	}
	cfg, err := sub.Config(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id":                      sub.ID(),
		"topic":                   cfg.Topic.ID(),
		"ack_deadline":            cfg.AckDeadline.String(),
		"retention":               cfg.RetentionDuration.String(),
		"enable_message_ordering": cfg.EnableMessageOrdering,
		"filter":                  cfg.Filter,
		"labels":                  cfg.Labels,
	})
}

func pubsubPull(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	subName := r.Str("subscription")
	maxMessages := r.Int("max_messages")
	timeout := r.Int("timeout")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	sub := g.pubsubClient.Subscription(subName)

	if maxMessages <= 0 {
		maxMessages = 10
	}
	sub.ReceiveSettings.MaxOutstandingMessages = maxMessages
	sub.ReceiveSettings.Synchronous = true

	var messages []map[string]any
	var mu sync.Mutex

	if timeout <= 0 {
		timeout = 10
	}

	pullCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	err := sub.Receive(pullCtx, func(_ context.Context, msg *pubsub.Message) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, map[string]any{
			"id":           msg.ID,
			"data":         string(msg.Data),
			"attributes":   msg.Attributes,
			"publish_time": msg.PublishTime,
		})
		msg.Ack()
		if len(messages) >= maxMessages {
			cancel()
		}
	})
	if err != nil && pullCtx.Err() == nil {
		return errResult(err)
	}

	return mcp.JSONResult(messages)
}
