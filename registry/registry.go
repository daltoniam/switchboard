package registry

import (
	"fmt"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

type registry struct {
	mu           sync.RWMutex
	integrations map[string]mcp.Integration
	order        []string
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
	r.order = append(r.order, i.Name())
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
	result := make([]mcp.Integration, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.integrations[name])
	}
	return result
}

func (r *registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}
