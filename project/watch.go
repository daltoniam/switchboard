package project

import (
	"context"
	"path/filepath"
	"time"
)

// Watch polls the catalog root and emits semantic events after rereading disk.
// A maintained fsnotify watcher is preferred when available; polling is the
// deterministic fallback used by tests and first release.
func (s *Store) Watch(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	prev := s.snapshotGeneration()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			cur := s.snapshotGeneration()
			if cur != prev {
				s.emit(Event{Kind: EventDefinitionChanged, SourceKind: "filesystem"})
				prev = cur
			}
		}
	}
}

func (s *Store) snapshotGeneration() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.rebuildIndex()
	return s.generationDigest()
}

// WatchRoots returns the directories that must exist for watching.
func (s *Store) WatchRoots() []string {
	return []string{s.projectsDir(), filepath.Join(s.configDir, "context")}
}
