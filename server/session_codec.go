package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
// Field snapshots are copied under the session lock so marshal does not block
// concurrent PinResult/SetContext on large pin payloads.
func EncodeSession(s *Session) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("encode session: nil")
	}
	s.mu.RLock()
	snap := sessionSnapshot{
		ID:         s.ID,
		CreatedAt:  s.CreatedAt,
		LastUsed:   s.LastUsed,
		NextSeq:    s.nextSeq,
		NextHandle: s.nextHandle,
		PinnedSize: s.pinnedSize,
	}
	if len(s.Context) > 0 {
		snap.Context = make(map[string]any, len(s.Context))
		for k, v := range s.Context {
			snap.Context[k] = v
		}
	} else {
		snap.Context = map[string]any{}
	}
	if len(s.Breadcrumbs) > 0 {
		snap.Breadcrumbs = make([]Breadcrumb, len(s.Breadcrumbs))
		copy(snap.Breadcrumbs, s.Breadcrumbs)
	}
	if len(s.pinned) > 0 {
		snap.Pinned = make(map[string]*PinnedResult, len(s.pinned))
		for k, v := range s.pinned {
			if v == nil {
				continue
			}
			// Deep-copy pin payload so marshal is independent of the live map.
			cp := *v
			if len(v.Data) > 0 {
				cp.Data = bytes.Clone(v.Data)
			}
			snap.Pinned[k] = &cp
		}
	}
	s.mu.RUnlock()

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
	// Recover nextHandle so the next PinResult does not reuse $N.
	if s.nextHandle == 0 && len(s.pinned) > 0 {
		for h := range s.pinned {
			if strings.HasPrefix(h, "$") {
				if n, err := strconv.Atoi(h[1:]); err == nil && n > s.nextHandle {
					s.nextHandle = n
				}
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
