package integrationcache

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type cacheKey struct {
	TenantID    string
	Integration string
}

type entry struct {
	instance     mcp.Integration
	credHash     string
	lastUsed     time.Time
	configuredAt time.Time
}

// Cache stores per-tenant configured integration instances with LRU eviction and TTL.
type Cache struct {
	mu      sync.RWMutex
	entries map[cacheKey]*entry
	ttl     time.Duration
	maxSize int
	now     func() time.Time
}

// New creates a Cache with the given TTL and max size.
func New(ttl time.Duration, maxSize int) *Cache {
	return &Cache{
		entries: make(map[cacheKey]*entry),
		ttl:     ttl,
		maxSize: maxSize,
		now:     time.Now,
	}
}

// Get returns a cached integration instance for the given tenant and integration name.
// Returns false if not cached, expired, or credentials have changed (credHash mismatch).
func (c *Cache) Get(tenantID, integration, credHash string) (mcp.Integration, bool) {
	c.mu.RLock()
	key := cacheKey{TenantID: tenantID, Integration: integration}
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	now := c.now()
	if now.Sub(e.configuredAt) > c.ttl {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	if credHash != "" && e.credHash != credHash {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	e.lastUsed = now
	c.mu.Unlock()

	return e.instance, true
}

// Put stores a configured integration instance in the cache.
// If the cache is at capacity, the least recently used entry is evicted.
func (c *Cache) Put(tenantID, integration string, instance mcp.Integration, credHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	key := cacheKey{TenantID: tenantID, Integration: integration}

	c.entries[key] = &entry{
		instance:     instance,
		credHash:     credHash,
		lastUsed:     now,
		configuredAt: now,
	}

	if len(c.entries) > c.maxSize {
		c.evictLRU()
	}
}

// Evict removes all cached entries for a tenant.
func (c *Cache) Evict(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k := range c.entries {
		if k.TenantID == tenantID {
			delete(c.entries, k)
		}
	}
}

// Len returns the number of cached entries.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *Cache) evictLRU() {
	var oldest cacheKey
	var oldestTime time.Time
	first := true

	for k, e := range c.entries {
		if first || e.lastUsed.Before(oldestTime) {
			oldest = k
			oldestTime = e.lastUsed
			first = false
		}
	}

	if !first {
		delete(c.entries, oldest)
	}
}

// HashCreds returns a SHA-256 hex digest of a Credentials map.
func HashCreds(creds mcp.Credentials) string {
	h := sha256.New()
	for k, v := range creds {
		_, _ = fmt.Fprintf(h, "%s=%s\n", k, v)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
