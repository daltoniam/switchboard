package registry

import (
	"fmt"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

type registry struct {
	mu           sync.RWMutex
	integrations map[string]mcp.Integration
	factories    map[string]mcp.IntegrationFactory
	order        []string
}

// New returns a new Registry implementation.
func New() mcp.Registry {
	return &registry{
		integrations: make(map[string]mcp.Integration),
		factories:    make(map[string]mcp.IntegrationFactory),
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

func (r *registry) RegisterFactory(name string, factory mcp.IntegrationFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("factory %q already registered", name)
	}
	r.factories[name] = factory
	return nil
}

func (r *registry) Get(name string) (mcp.Integration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.integrations[name]
	return i, ok
}

// NewInstance creates a new unconfigured integration instance using the registered factory.
// Returns an error if no factory is registered for the given name.
func (r *registry) NewInstance(name string) (mcp.Integration, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no factory registered for integration %q", name)
	}
	return factory(), nil
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
