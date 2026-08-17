package project

import (
	"sync"
)

// EventKind identifies a semantic catalog change.
type EventKind string

const (
	EventProjectAdded          EventKind = "project_added"
	EventProjectRemoved        EventKind = "project_removed"
	EventDefinitionChanged     EventKind = "definition_changed"
	EventContextChanged        EventKind = "context_changed"
	EventDiagnosticsChanged    EventKind = "diagnostics_changed"
	EventProjectValidityChanged EventKind = "project_validity_changed"
)

// Event is a transport-neutral catalog change notification.
// It never carries project:// URIs; adapters map IDs to resources.
type Event struct {
	Kind        EventKind
	ProjectID   ProjectID
	OldRevision Revision
	NewRevision Revision
	SourceKind  string
	RootURI     string
	ContextPaths []string
}

// EventBus is a bounded fan-out for catalog invalidations.
type EventBus struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewEventBus constructs an empty bus.
func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[chan Event]struct{})}
}

// Subscribe returns a buffered channel of events. Callers must Unsubscribe.
func (b *EventBus) Subscribe(buffer int) <-chan Event {
	if buffer < 1 {
		buffer = 16
	}
	ch := make(chan Event, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *EventBus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for sub := range b.subs {
		if sub == ch {
			delete(b.subs, sub)
			close(sub)
			return
		}
	}
}

func (b *EventBus) publish(ev Event) {
	if b == nil {
		return
	}
	// Deep-copy path slice so consumers cannot alias bus state.
	if ev.ContextPaths != nil {
		ev.ContextPaths = append([]string(nil), ev.ContextPaths...)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Drop for slow consumers; source truth remains on disk.
		}
	}
}

// SetEventBus attaches a bus used after durable mutations.
func (s *Store) SetEventBus(bus *EventBus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bus = bus
}

// EventBus returns the attached bus, if any.
func (s *Store) EventBus() *EventBus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bus
}

func (s *Store) emit(ev Event) {
	if s.bus == nil {
		return
	}
	s.bus.publish(ev)
}
