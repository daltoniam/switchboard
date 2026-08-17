package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryBudget_AllowsWithinLimit(t *testing.T) {
	ctx := withRetryBudget(context.Background(), 5)
	for range 5 {
		assert.True(t, consumeRetry(ctx), "should allow retries within budget")
	}
}

func TestRetryBudget_RejectsOverLimit(t *testing.T) {
	ctx := withRetryBudget(context.Background(), 3)
	for range 3 {
		consumeRetry(ctx)
	}
	assert.False(t, consumeRetry(ctx), "should reject when budget exhausted")
}

func TestRetryBudget_MissingBudgetAlwaysAllows(t *testing.T) {
	ctx := context.Background() // no budget attached
	for range 10 {
		assert.True(t, consumeRetry(ctx), "should always allow when no budget in context")
	}
}
