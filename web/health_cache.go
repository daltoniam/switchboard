package web

import (
	"context"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type healthEntry struct {
	Healthy   bool
	Enabled   bool
	CheckedAt time.Time
}

type healthCache struct {
	mu       sync.RWMutex
	entries  map[string]healthEntry
	services *mcp.Services
}

func newHealthCache(services *mcp.Services) *healthCache {
	return &healthCache{
		entries:  make(map[string]healthEntry),
		services: services,
	}
}

func (hc *healthCache) get(name string) (healthEntry, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	e, ok := hc.entries[name]
	return e, ok
}

func (hc *healthCache) refreshAll(ctx context.Context) {
	integrations := hc.services.Registry.All()

	type result struct {
		name  string
		entry healthEntry
	}

	results := make(chan result, len(integrations))
	var wg sync.WaitGroup

	for _, a := range integrations {
		wg.Add(1)
		go func(a mcp.Integration) {
			defer wg.Done()

			ic, exists := hc.services.Config.GetIntegration(a.Name())
			enabled := exists && ic.Enabled

			var healthy bool
			if exists {
				if err := a.Configure(ctx, ic.Credentials); err == nil {
					healthy = a.Healthy(ctx)
					if healthy && !enabled {
						enabled = true
						ic.Enabled = true
						_ = hc.services.Config.SetIntegration(a.Name(), ic)
					}
				}
			}

			results <- result{
				name: a.Name(),
				entry: healthEntry{
					Healthy:   healthy,
					Enabled:   enabled,
					CheckedAt: time.Now(),
				},
			}
		}(a)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		hc.mu.Lock()
		hc.entries[r.name] = r.entry
		hc.mu.Unlock()
	}
}
