package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	MaxPinnedBytes = 5 * 1024 * 1024 // 5MB total pinned data per session
)

type PinnedResult struct {
	Handle    string          `json:"handle"`
	Tool      mcp.ToolName    `json:"tool"`
	Data      json.RawMessage `json:"data"`
	PinnedAt  time.Time       `json:"pinned_at"`
	SizeBytes int             `json:"size_bytes"`
}

func (s *Session) PinResult(tool mcp.ToolName, data string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pinned == nil {
		s.pinned = make(map[string]*PinnedResult)
	}
	s.nextHandle++
	handle := "$" + strconv.Itoa(s.nextHandle)
	size := len(data)

	s.evictPinnedIfNeeded(size)

	pr := &PinnedResult{
		Handle:    handle,
		Tool:      tool,
		Data:      json.RawMessage(data),
		PinnedAt:  time.Now(),
		SizeBytes: size,
	}
	s.pinned[handle] = pr
	s.pinnedSize += size
	s.LastUsed = time.Now()
	return handle
}

func (s *Session) GetPinned(handle string) (*PinnedResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pr, ok := s.pinned[handle]
	return pr, ok
}

func (s *Session) Unpin(handle string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	pr, ok := s.pinned[handle]
	if !ok {
		return false
	}
	s.pinnedSize -= pr.SizeBytes
	delete(s.pinned, handle)
	return true
}

func (s *Session) ListPinned() []*PinnedResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]*PinnedResult, 0, len(s.pinned))
	for _, pr := range s.pinned {
		results = append(results, pr)
	}
	return results
}

func (s *Session) PinnedBytes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pinnedSize
}

func (s *Session) PinnedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pinned)
}

func (s *Session) ResolveRef(ref string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	handle, path := splitRef(ref)
	pr, ok := s.pinned[handle]
	if !ok {
		return nil, fmt.Errorf("no pinned result for handle %q", handle)
	}

	if path == "" {
		var v any
		if err := json.Unmarshal(pr.Data, &v); err != nil {
			return nil, fmt.Errorf("unmarshal pinned data: %w", err)
		}
		return v, nil
	}

	var parsed any
	if err := json.Unmarshal(pr.Data, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal pinned data: %w", err)
	}
	return extractPath(parsed, path)
}

func (s *Session) evictPinnedIfNeeded(incoming int) {
	for s.pinnedSize+incoming > MaxPinnedBytes && len(s.pinned) > 0 {
		var oldest *PinnedResult
		for _, pr := range s.pinned {
			if oldest == nil || pr.PinnedAt.Before(oldest.PinnedAt) {
				oldest = pr
			}
		}
		if oldest == nil {
			break
		}
		s.pinnedSize -= oldest.SizeBytes
		delete(s.pinned, oldest.Handle)
	}
}

func splitRef(ref string) (handle, path string) {
	ref = strings.TrimPrefix(ref, "$")
	dotIdx := strings.IndexByte(ref, '.')
	if dotIdx < 0 {
		return "$" + ref, ""
	}
	return "$" + ref[:dotIdx], ref[dotIdx+1:]
}

func extractPath(data any, path string) (any, error) {
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found in path %q", part, path)
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("expected array index, got %q in path %q", part, path)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds (len %d) in path %q", idx, len(v), path)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot traverse into %T at %q in path %q", current, part, path)
		}
	}
	return current, nil
}

func resolveRefs(sess *Session, args map[string]any) {
	for k, v := range args {
		s, ok := v.(string)
		if !ok || !strings.HasPrefix(s, "$") {
			continue
		}
		if len(s) < 2 {
			continue
		}
		if s[1] < '0' || s[1] > '9' {
			continue
		}
		resolved, err := sess.ResolveRef(s)
		if err == nil {
			args[k] = resolved
		}
	}
}
