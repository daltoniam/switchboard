package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBreaker_AllowsWhenClosed(t *testing.T) {
	b := newBreaker(5, 30*time.Second)
	assert.True(t, b.allow(), "fresh breaker should allow requests")
}

func TestBreaker_TripsAfterThresholdFailures(t *testing.T) {
	b := newBreaker(3, 30*time.Second)
	for range 3 {
		b.recordFailure()
	}
	assert.False(t, b.allow(), "breaker should be open after threshold failures")
}

func TestBreaker_ResetsOnSuccess(t *testing.T) {
	b := newBreaker(3, 30*time.Second)
	b.recordFailure()
	b.recordFailure()
	b.recordSuccess()
	assert.True(t, b.allow(), "success should reset failure count")
	// Needs threshold more failures to trip again.
	b.recordFailure()
	b.recordFailure()
	assert.True(t, b.allow(), "should still be closed — only 2 failures after reset")
}

func TestBreaker_AllowsAfterCooldown(t *testing.T) {
	b := newBreaker(2, 50*time.Millisecond)
	b.recordFailure()
	b.recordFailure()
	assert.False(t, b.allow(), "should be open")

	time.Sleep(60 * time.Millisecond)
	assert.True(t, b.allow(), "should allow one probe after cooldown (half-open)")
}

func TestBreaker_ClosesAfterHalfOpenSuccess(t *testing.T) {
	b := newBreaker(2, 50*time.Millisecond)
	b.recordFailure()
	b.recordFailure()
	assert.False(t, b.allow(), "should be open")

	time.Sleep(60 * time.Millisecond)
	assert.True(t, b.allow(), "half-open: allows one probe")

	b.recordSuccess()
	assert.True(t, b.allow(), "should be fully closed after half-open success")
	assert.True(t, b.allow(), "still closed — no limit on requests")
}
