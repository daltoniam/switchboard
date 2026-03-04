package server

import (
	"context"
	"sync"
)

type retryBudgetKey struct{}

type retryBudget struct {
	mu        sync.Mutex
	remaining int
}

// withRetryBudget attaches a retry budget to the context.
// All executeTool calls sharing this context consume from the same budget.
func withRetryBudget(ctx context.Context, max int) context.Context {
	return context.WithValue(ctx, retryBudgetKey{}, &retryBudget{remaining: max})
}

// consumeRetry decrements the retry budget by 1 and returns true if a retry is allowed.
// Returns true if no budget is attached (direct execute, not script).
func consumeRetry(ctx context.Context) bool {
	b, ok := ctx.Value(retryBudgetKey{}).(*retryBudget)
	if !ok {
		return true // no budget — unlimited retries (direct execute path)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining <= 0 {
		return false
	}
	b.remaining--
	return true
}
