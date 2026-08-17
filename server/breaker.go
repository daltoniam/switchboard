package server

import (
	"sync"
	"time"
)

// breaker implements a lightweight circuit breaker per integration.
// States: closed (normal) → open (reject all) → half-open (allow one probe).
// Transitions: threshold consecutive failures → open; cooldown expires → half-open;
// half-open success → closed; half-open failure → open.
type breaker struct {
	mu          sync.Mutex
	failures    int
	lastFailure time.Time
	threshold   int
	cooldown    time.Duration
}

func newBreaker(threshold int, cooldown time.Duration) *breaker {
	return &breaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// allow returns true if the breaker permits a request.
// In half-open state (cooldown expired), allows one probe request.
func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.failures < b.threshold {
		return true // closed
	}
	// Open — check if cooldown has elapsed.
	if time.Since(b.lastFailure) >= b.cooldown {
		return true // half-open: allow one probe
	}
	return false
}

func (b *breaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
}

func (b *breaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFailure = time.Now()
}
