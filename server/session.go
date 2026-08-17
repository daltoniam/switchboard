package server

import (
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	DefaultSessionTTL    = 1 * time.Hour
	MaxBreadcrumbs       = 200
	maxBreadcrumbSummary = 200
)

type Breadcrumb struct {
	Seq       int            `json:"seq"`
	Timestamp time.Time      `json:"timestamp"`
	Tool      mcp.ToolName   `json:"tool"`
	Args      map[string]any `json:"args,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

type Session struct {
	ID          string         `json:"id"`
	Context     map[string]any `json:"context"`
	CreatedAt   time.Time      `json:"created_at"`
	LastUsed    time.Time      `json:"last_used"`
	Breadcrumbs []Breadcrumb   `json:"breadcrumbs,omitempty"`

	mu         sync.RWMutex
	nextSeq    int
	pinned     map[string]*PinnedResult
	nextHandle int
	pinnedSize int
}

func newSession(id string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Context:   make(map[string]any),
		CreatedAt: now,
		LastUsed:  now,
	}
}

func (s *Session) SetContext(pairs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range pairs {
		s.Context[k] = v
	}
	s.LastUsed = time.Now()
}

func (s *Session) GetContext() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]any, len(s.Context))
	for k, v := range s.Context {
		cp[k] = v
	}
	return cp
}

func (s *Session) ClearContext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Context = make(map[string]any)
	s.LastUsed = time.Now()
}

func (s *Session) MergeDefaults(args map[string]any) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Context) == 0 {
		return args
	}
	merged := make(map[string]any, len(args)+len(s.Context))
	for k, v := range s.Context {
		merged[k] = v
	}
	for k, v := range args {
		merged[k] = v
	}
	return merged
}

func (s *Session) AddBreadcrumb(tool mcp.ToolName, args map[string]any, result string, isError bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSeq++
	bc := Breadcrumb{
		Seq:       s.nextSeq,
		Timestamp: time.Now(),
		Tool:      tool,
		Args:      summarizeArgs(args),
		Summary:   truncate(result, maxBreadcrumbSummary),
		IsError:   isError,
	}
	s.Breadcrumbs = append(s.Breadcrumbs, bc)
	if len(s.Breadcrumbs) > MaxBreadcrumbs {
		s.Breadcrumbs = s.Breadcrumbs[len(s.Breadcrumbs)-MaxBreadcrumbs:]
	}
	s.LastUsed = time.Now()
}

func (s *Session) RecentBreadcrumbs(n int, toolFilter mcp.ToolName) []Breadcrumb {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var filtered []Breadcrumb
	for i := len(s.Breadcrumbs) - 1; i >= 0; i-- {
		bc := s.Breadcrumbs[i]
		if toolFilter != "" && bc.Tool != toolFilter {
			continue
		}
		filtered = append(filtered, bc)
		if len(filtered) >= n {
			break
		}
	}
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	return filtered
}

func (s *Session) TotalBreadcrumbs() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Breadcrumbs)
}

func (s *Session) touch() {
	s.LastUsed = time.Now()
}

// SessionStore is the pluggable storage interface for sessions.
// The default implementation is in-memory (MemorySessionStore).
// The commercial version can plug in Postgres, an API shim, etc.
type SessionStore interface {
	GetOrCreate(id string) *Session
	Get(id string) (*Session, bool)
	Save(s *Session) error
	Delete(id string)
}

// MemorySessionStore keeps sessions in memory with TTL-based eviction.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewMemorySessionStore(ttl time.Duration) *MemorySessionStore {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &MemorySessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

func (ss *MemorySessionStore) GetOrCreate(id string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.evictExpired()
	if s, ok := ss.sessions[id]; ok {
		s.touch()
		return s
	}
	s := newSession(id)
	ss.sessions[id] = s
	return s
}

func (ss *MemorySessionStore) Get(id string) (*Session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	s, ok := ss.sessions[id]
	if !ok {
		return nil, false
	}
	if time.Since(s.LastUsed) > ss.ttl {
		return nil, false
	}
	return s, true
}

func (ss *MemorySessionStore) Save(_ *Session) error {
	return nil
}

func (ss *MemorySessionStore) Delete(id string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, id)
}

func (ss *MemorySessionStore) Len() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.sessions)
}

func (ss *MemorySessionStore) evictExpired() {
	now := time.Now()
	for id, s := range ss.sessions {
		if now.Sub(s.LastUsed) > ss.ttl {
			delete(ss.sessions, id)
		}
	}
}

func summarizeArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	summary := make(map[string]any, len(args))
	for k, v := range args {
		switch val := v.(type) {
		case string:
			summary[k] = truncate(val, 100)
		default:
			summary[k] = v
		}
	}
	return summary
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
