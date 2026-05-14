package teams

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// tenant holds OAuth credentials and resolved identity for a single Microsoft
// Entra tenant (or the magic "_config" tenant for tokens injected via plain
// credentials). The "default" tenant is selected by tenant ID; identity fields
// are populated lazily by the /me probe in Configure.
type tenant struct {
	TenantID     string    `json:"tenant_id"`
	TenantName   string    `json:"tenant_name,omitempty"`
	UserOID      string    `json:"user_oid,omitempty"`
	UserUPN      string    `json:"user_upn,omitempty"`
	UserDisplay  string    `json:"user_display,omitempty"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Source       string    `json:"source,omitempty"` // "device_code", "config", etc.
	UpdatedAt    time.Time `json:"updated_at"`
}

type tokenStore struct {
	mu              sync.RWMutex
	tenants         map[string]*tenant // keyed by TenantID
	defaultTenantID string
	filePath        string
}

func newTokenStore() *tokenStore {
	home, _ := os.UserHomeDir()
	return &tokenStore{
		tenants:  make(map[string]*tenant),
		filePath: filepath.Join(home, ".teams-mcp-tokens.json"),
	}
}

func (ts *tokenStore) get(tenantID string) *tenant {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if tenantID == "" {
		tenantID = ts.defaultTenantID
	}
	tn, ok := ts.tenants[tenantID]
	if !ok {
		return nil
	}
	cp := *tn
	return &cp
}

func (ts *tokenStore) getDefault() *tenant { return ts.get("") }

func (ts *tokenStore) defaultID() string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.defaultTenantID
}

func (ts *tokenStore) setDefault(tenantID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if _, ok := ts.tenants[tenantID]; ok {
		ts.defaultTenantID = tenantID
	}
}

func (ts *tokenStore) all() []*tenant {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]*tenant, 0, len(ts.tenants))
	for _, tn := range ts.tenants {
		cp := *tn
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TenantID < out[j].TenantID })
	return out
}

func (ts *tokenStore) upsert(tn *tenant) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	tn.UpdatedAt = time.Now()
	existing, ok := ts.tenants[tn.TenantID]
	if ok {
		// Preserve identity fields when not supplied on the update.
		if tn.UserOID == "" {
			tn.UserOID = existing.UserOID
		}
		if tn.UserUPN == "" {
			tn.UserUPN = existing.UserUPN
		}
		if tn.UserDisplay == "" {
			tn.UserDisplay = existing.UserDisplay
		}
		if tn.TenantName == "" {
			tn.TenantName = existing.TenantName
		}
	}
	cp := *tn
	ts.tenants[tn.TenantID] = &cp
	if ts.defaultTenantID == "" {
		ts.defaultTenantID = tn.TenantID
	}
}

func (ts *tokenStore) remove(tenantID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.tenants, tenantID)
	if ts.defaultTenantID == tenantID {
		ts.defaultTenantID = ""
		// Promote the lexicographically smallest remaining tenant.
		for id := range ts.tenants {
			if ts.defaultTenantID == "" || id < ts.defaultTenantID {
				ts.defaultTenantID = id
			}
		}
	}
}

// tokenFile is the on-disk shape.
type tokenFile struct {
	Version         int               `json:"version"`
	DefaultTenantID string            `json:"default_tenant_id"`
	Tenants         []*tokenFileEntry `json:"tenants"`
}

type tokenFileEntry struct {
	TenantID     string `json:"tenant_id"`
	TenantName   string `json:"tenant_name,omitempty"`
	UserOID      string `json:"user_oid,omitempty"`
	UserUPN      string `json:"user_upn,omitempty"`
	UserDisplay  string `json:"user_display,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Source       string `json:"source,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// loadFromFile populates the store from disk. Missing or malformed files are
// treated as empty stores (callers don't care; they'll just see "no tenants").
func (ts *tokenStore) loadFromFile() {
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		return
	}
	var tf tokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, entry := range tf.Tenants {
		if entry.AccessToken == "" && entry.RefreshToken == "" {
			continue
		}
		tn := &tenant{
			TenantID:     entry.TenantID,
			TenantName:   entry.TenantName,
			UserOID:      entry.UserOID,
			UserUPN:      entry.UserUPN,
			UserDisplay:  entry.UserDisplay,
			AccessToken:  entry.AccessToken,
			RefreshToken: entry.RefreshToken,
			Source:       entry.Source,
		}
		if t, err := time.Parse(time.RFC3339, entry.ExpiresAt); err == nil {
			tn.ExpiresAt = t
		}
		if t, err := time.Parse(time.RFC3339, entry.UpdatedAt); err == nil {
			tn.UpdatedAt = t
		} else {
			tn.UpdatedAt = time.Now()
		}
		ts.tenants[tn.TenantID] = tn
	}
	if tf.DefaultTenantID != "" {
		if _, ok := ts.tenants[tf.DefaultTenantID]; ok {
			ts.defaultTenantID = tf.DefaultTenantID
		}
	}
	if ts.defaultTenantID == "" {
		for id := range ts.tenants {
			if ts.defaultTenantID == "" || id < ts.defaultTenantID {
				ts.defaultTenantID = id
			}
		}
	}
}

// saveToFile persists the store atomically (write-tmp + rename).
func (ts *tokenStore) saveToFile() error {
	ts.mu.RLock()
	tf := tokenFile{
		Version:         1,
		DefaultTenantID: ts.defaultTenantID,
	}
	for _, tn := range ts.tenants {
		entry := &tokenFileEntry{
			TenantID:     tn.TenantID,
			TenantName:   tn.TenantName,
			UserOID:      tn.UserOID,
			UserUPN:      tn.UserUPN,
			UserDisplay:  tn.UserDisplay,
			AccessToken:  tn.AccessToken,
			RefreshToken: tn.RefreshToken,
			Source:       tn.Source,
			UpdatedAt:    tn.UpdatedAt.UTC().Format(time.RFC3339),
		}
		if !tn.ExpiresAt.IsZero() {
			entry.ExpiresAt = tn.ExpiresAt.UTC().Format(time.RFC3339)
		}
		tf.Tenants = append(tf.Tenants, entry)
	}
	ts.mu.RUnlock()

	sort.Slice(tf.Tenants, func(i, j int) bool {
		return tf.Tenants[i].TenantID < tf.Tenants[j].TenantID
	})

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	tmp := ts.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, ts.filePath)
}
