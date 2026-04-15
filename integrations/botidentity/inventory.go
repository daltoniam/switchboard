package botidentity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const inventoryFile = "bots.json"

type SlackIdentity struct {
	Enabled      bool   `json:"enabled"`
	AppID        string `json:"app_id,omitempty"`
	BotUserID    string `json:"bot_user_id,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	BotToken     string `json:"bot_token,omitempty"`
	WebhookURL   string `json:"webhook_url,omitempty"`
	InstallURL   string `json:"install_url,omitempty"`
}

type GitHubIdentity struct {
	Enabled       bool   `json:"enabled"`
	AppID         string `json:"app_id,omitempty"`
	AppSlug       string `json:"app_slug,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
	PrivateKey    string `json:"private_key,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty"`
}

type BotEntry struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	LogoPath  string            `json:"logo_path,omitempty"`
	Slack     SlackIdentity     `json:"slack"`
	GitHub    GitHubIdentity    `json:"github"`
	Creds     map[string]string `json:"credentials,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (e *BotEntry) platforms() []string {
	var p []string
	if e.Slack.Enabled {
		p = append(p, "slack")
	}
	if e.GitHub.Enabled {
		p = append(p, "github")
	}
	return p
}

type inventory struct {
	mu   sync.Mutex
	path string
	Bots []*BotEntry `json:"bots"`
}

func newInventory() *inventory {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &inventory{
		path: filepath.Join(home, ".config", "switchboard", inventoryFile),
	}
}

func (inv *inventory) save() error {
	dir := filepath.Dir(inv.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create inventory dir: %w", err)
	}
	data, err := json.MarshalIndent(inv.Bots, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(inv.path, data, 0600)
}

func (inv *inventory) add(entry *BotEntry) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if err := inv.loadLocked(); err != nil {
		return err
	}
	for _, b := range inv.Bots {
		if b.ID == entry.ID {
			return fmt.Errorf("bot %q already exists", entry.ID)
		}
	}
	now := time.Now().UTC()
	entry.CreatedAt = now
	entry.UpdatedAt = now
	inv.Bots = append(inv.Bots, entry)
	return inv.save()
}

func (inv *inventory) get(id string) (*BotEntry, error) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if err := inv.loadLocked(); err != nil {
		return nil, err
	}
	for _, b := range inv.Bots {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, fmt.Errorf("bot %q not found", id)
}

func (inv *inventory) list() ([]*BotEntry, error) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if err := inv.loadLocked(); err != nil {
		return nil, err
	}
	return inv.Bots, nil
}

func (inv *inventory) update(id string, fn func(*BotEntry)) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if err := inv.loadLocked(); err != nil {
		return err
	}
	for _, b := range inv.Bots {
		if b.ID == id {
			fn(b)
			b.UpdatedAt = time.Now().UTC()
			return inv.save()
		}
	}
	return fmt.Errorf("bot %q not found", id)
}

func (inv *inventory) remove(id string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if err := inv.loadLocked(); err != nil {
		return err
	}
	for i, b := range inv.Bots {
		if b.ID == id {
			inv.Bots = append(inv.Bots[:i], inv.Bots[i+1:]...)
			return inv.save()
		}
	}
	return fmt.Errorf("bot %q not found", id)
}

func (inv *inventory) loadLocked() error {
	data, err := os.ReadFile(inv.path)
	if err != nil {
		if os.IsNotExist(err) {
			inv.Bots = nil
			return nil
		}
		return fmt.Errorf("read inventory: %w", err)
	}
	return json.Unmarshal(data, &inv.Bots)
}

func (inv *inventory) logoDir() string {
	return filepath.Join(filepath.Dir(inv.path), "logos")
}

func (inv *inventory) saveLogo(botID string, data []byte, ext string) (string, error) {
	safe := filepath.Base(botID)
	if safe == "." || safe == "/" || safe == string(filepath.Separator) {
		return "", fmt.Errorf("invalid bot ID for logo filename: %q", botID)
	}
	dir := inv.logoDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create logos dir: %w", err)
	}
	path := filepath.Clean(filepath.Join(dir, safe+ext))
	if !strings.HasPrefix(path, filepath.Clean(dir)+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid bot ID for logo filename: %q", botID)
	}
	if err := os.WriteFile(path, data, 0600); err != nil { // #nosec G703 -- path validated above
		return "", fmt.Errorf("write logo: %w", err)
	}
	return path, nil
}
