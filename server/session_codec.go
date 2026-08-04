package server

import (
	"encoding/json"
	"fmt"
	"time"
)

// sessionSnapshot is the durable wire form of a Session, including pins.
// Used by external SessionStore backends (Redis, etc.) and by tests.
type sessionSnapshot struct {
	ID          string                   `json:"id"`
	Context     map[string]any           `json:"context"`
	CreatedAt   time.Time                `json:"created_at"`
	LastUsed    time.Time                `json:"last_used"`
	Breadcrumbs []Breadcrumb             `json:"breadcrumbs,omitempty"`
	NextSeq     int                      `json:"next_seq"`
	Pinned      map[string]*PinnedResult `json:"pinned,omitempty"`
	NextHandle  int                      `json:"next_handle"`
	PinnedSize  int                      `json:"pinned_size"`
}

// EncodeSession serializes a Session (including pins) to JSON for durable stores.
func EncodeSession(s *Session) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("encode session: nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := sessionSnapshot{
		ID:          s.ID,
		Context:     s.Context,
		CreatedAt:   s.CreatedAt,
		LastUsed:    s.LastUsed,
		Breadcrumbs: s.Breadcrumbs,
		NextSeq:     s.nextSeq,
		NextHandle:  s.nextHandle,
		PinnedSize:  s.pinnedSize,
	}
	if len(s.pinned) > 0 {
		snap.Pinned = make(map[string]*PinnedResult, len(s.pinned))
		for k, v := range s.pinned {
			snap.Pinned[k] = v
		}
	}
	if snap.Context == nil {
		snap.Context = map[string]any{}
	}
	return json.Marshal(snap)
}

// DecodeSession rebuilds a Session from EncodeSession JSON.
// id is the store key; if the payload carries a different ID it is ignored
// in favor of the store key so key renames stay consistent.
func DecodeSession(id string, data []byte) (*Session, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("decode session: empty payload")
	}
	var snap sessionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	if id == "" {
		id = snap.ID
	}
	if id == "" {
		return nil, fmt.Errorf("decode session: missing id")
	}
	s := &Session{
		ID:          id,
		Context:     snap.Context,
		CreatedAt:   snap.CreatedAt,
		LastUsed:    snap.LastUsed,
		Breadcrumbs: snap.Breadcrumbs,
		nextSeq:     snap.NextSeq,
		pinned:      snap.Pinned,
		nextHandle:  snap.NextHandle,
		pinnedSize:  snap.PinnedSize,
	}
	if s.Context == nil {
		s.Context = make(map[string]any)
	}
	if s.pinned == nil {
		s.pinned = make(map[string]*PinnedResult)
	}
	// Recompute pinned size if the snapshot omitted it or drifted.
	if s.pinnedSize == 0 && len(s.pinned) > 0 {
		for _, pr := range s.pinned {
			if pr != nil {
				s.pinnedSize += pr.SizeBytes
			}
		}
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	if s.LastUsed.IsZero() {
		s.LastUsed = s.CreatedAt
	}
	return s, nil
}
