package specimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

// Syncer reconciles a set of spec-import integrations in a registry against
// the desired list declared in config. It is the seam that makes spec imports
// live-reloadable: call Sync at startup and again whenever config changes, and
// the registry converges to exactly the enabled entries in config.
//
// Syncer owns only the integrations it registered (tracked by name and
// fingerprint). It never touches built-in integrations, so an unrelated config
// change can't disturb them.
type Syncer struct {
	reg mcp.Registry

	mu sync.Mutex
	// managed maps a registered integration name to the fingerprint of the
	// config entry that produced it. Used to detect no-op vs. changed entries
	// and to know which registry entries are ours to remove.
	managed map[string]string
}

// NewSyncer returns a Syncer bound to a registry.
func NewSyncer(reg mcp.Registry) *Syncer {
	return &Syncer{reg: reg, managed: make(map[string]string)}
}

// SyncResult reports what a Sync call changed, so callers can log a concise
// summary and decide whether to refresh downstream state (e.g., search index).
type SyncResult struct {
	Added   []string
	Updated []string
	Removed []string
	Errors  []error
}

// Changed reports whether the registry membership changed, signaling callers
// to refresh the search index.
func (r SyncResult) Changed() bool {
	return len(r.Added) > 0 || len(r.Updated) > 0 || len(r.Removed) > 0
}

// Sync reconciles the registry to the given desired config entries. Disabled
// entries are treated as absent. Entries that fail to load are skipped and
// their errors collected; a bad entry never blocks the others and never leaves
// a stale integration registered under its name.
func (s *Syncer) Sync(desired []mcp.SpecImportConfig) SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res SyncResult
	wanted := make(map[string]mcp.SpecImportConfig)
	for _, cfg := range desired {
		if !cfg.Enabled {
			continue
		}
		name := SanitizedName(cfg.Name)
		wanted[name] = cfg
	}

	// Remove managed integrations that are no longer wanted.
	for name := range s.managed {
		if _, keep := wanted[name]; !keep {
			s.reg.Unregister(name)
			delete(s.managed, name)
			res.Removed = append(res.Removed, name)
		}
	}

	// Add or update wanted integrations.
	for name, cfg := range wanted {
		fp := fingerprint(cfg)
		if prev, ok := s.managed[name]; ok && prev == fp {
			continue // unchanged, leave registered as-is
		}
		in, err := Load(cfg)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("spec import %q: %w", cfg.Name, err))
			continue
		}
		changed := false
		if _, ok := s.managed[name]; ok {
			// Config changed — replace the registered instance.
			s.reg.Unregister(name)
			changed = true
		}
		if err := s.reg.Register(in); err != nil {
			// A name collision with a non-managed integration lands here.
			res.Errors = append(res.Errors, fmt.Errorf("spec import %q: %w", cfg.Name, err))
			delete(s.managed, name)
			continue
		}
		s.managed[name] = fp
		if changed {
			res.Updated = append(res.Updated, name)
		} else {
			res.Added = append(res.Added, name)
		}
	}
	return res
}

// Managed reports whether the syncer currently owns an integration by its
// registered name.
func (s *Syncer) Managed(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.managed[name]
	return ok
}

// fingerprint produces a stable hash of the fields that affect the resulting
// integration. Two configs with the same fingerprint yield an identical
// integration, so Sync can skip re-loading unchanged entries.
func fingerprint(cfg mcp.SpecImportConfig) string {
	// Marshal a normalized view: field order is fixed by the struct, and
	// json.Marshal sorts map keys, so credential ordering is stable.
	b, err := json.Marshal(struct {
		Kind        string          `json:"kind"`
		Spec        string          `json:"spec"`
		Path        string          `json:"path"`
		Endpoint    string          `json:"endpoint"`
		Credentials mcp.Credentials `json:"credentials"`
	}{
		Kind:        cfg.Kind,
		Spec:        cfg.Spec,
		Path:        cfg.Path,
		Endpoint:    cfg.Endpoint,
		Credentials: cfg.Credentials,
	})
	if err != nil {
		// Marshal of plain strings/maps cannot fail; fall back to a value that
		// forces a reload rather than silently treating entries as equal.
		return cfg.Kind + "|" + cfg.Spec + "|" + cfg.Path + "|" + cfg.Endpoint
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
