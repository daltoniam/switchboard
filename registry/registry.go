package registry

import (
	"fmt"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

type registry struct {
	mu           sync.RWMutex
	integrations map[string]mcp.Integration
}

// New returns a new Registry implementation.
func New() mcp.Registry {
	return &registry{
		integrations: make(map[string]mcp.Integration),
	}
}

func (r *registry) Register(i mcp.Integration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.integrations[i.Name()]; exists {
		return fmt.Errorf("integration %q already registered", i.Name())
	}
	r.integrations[i.Name()] = i
	return nil
}

func (r *registry) Get(name string) (mcp.Integration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.integrations[name]
	return i, ok
}

func (r *registry) All() []mcp.Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]mcp.Integration, 0, len(r.integrations))
	for _, i := range r.integrations {
		result = append(result, i)
	}
	return result
}

func (r *registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		names = append(names, name)
	}
	return names
}
