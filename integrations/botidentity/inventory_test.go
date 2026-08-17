package botidentity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestInventory(t *testing.T) *inventory {
	t.Helper()
	dir := t.TempDir()
	return &inventory{path: filepath.Join(dir, "bots.json")}
}

func TestInventory_AddAndGet(t *testing.T) {
	inv := newTestInventory(t)
	err := inv.add(&BotEntry{ID: "test-bot", Name: "Test Bot", Slack: SlackIdentity{Enabled: true}})
	require.NoError(t, err)

	entry, err := inv.get("test-bot")
	require.NoError(t, err)
	assert.Equal(t, "Test Bot", entry.Name)
	assert.True(t, entry.Slack.Enabled)
	assert.False(t, entry.CreatedAt.IsZero())
}

func TestInventory_AddDuplicate(t *testing.T) {
	inv := newTestInventory(t)
	require.NoError(t, inv.add(&BotEntry{ID: "bot-1", Name: "Bot", GitHub: GitHubIdentity{Enabled: true}}))
	err := inv.add(&BotEntry{ID: "bot-1", Name: "Bot 2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInventory_List(t *testing.T) {
	inv := newTestInventory(t)
	require.NoError(t, inv.add(&BotEntry{ID: "a", Name: "A", GitHub: GitHubIdentity{Enabled: true}}))
	require.NoError(t, inv.add(&BotEntry{ID: "b", Name: "B", Slack: SlackIdentity{Enabled: true}}))

	bots, err := inv.list()
	require.NoError(t, err)
	assert.Len(t, bots, 2)
}

func TestInventory_ListEmpty(t *testing.T) {
	inv := newTestInventory(t)
	bots, err := inv.list()
	require.NoError(t, err)
	assert.Empty(t, bots)
}

func TestInventory_Update(t *testing.T) {
	inv := newTestInventory(t)
	require.NoError(t, inv.add(&BotEntry{ID: "bot-1", Name: "Old Name", Slack: SlackIdentity{Enabled: true}}))

	err := inv.update("bot-1", func(e *BotEntry) {
		e.Name = "New Name"
		e.LogoPath = "/tmp/logo.png"
	})
	require.NoError(t, err)

	entry, err := inv.get("bot-1")
	require.NoError(t, err)
	assert.Equal(t, "New Name", entry.Name)
	assert.Equal(t, "/tmp/logo.png", entry.LogoPath)
}

func TestInventory_UpdateNotFound(t *testing.T) {
	inv := newTestInventory(t)
	err := inv.update("missing", func(e *BotEntry) {})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInventory_Remove(t *testing.T) {
	inv := newTestInventory(t)
	require.NoError(t, inv.add(&BotEntry{ID: "bot-1", Name: "Bot"}))
	require.NoError(t, inv.remove("bot-1"))

	_, err := inv.get("bot-1")
	assert.Error(t, err)
}

func TestInventory_RemoveNotFound(t *testing.T) {
	inv := newTestInventory(t)
	err := inv.remove("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInventory_GetNotFound(t *testing.T) {
	inv := newTestInventory(t)
	_, err := inv.get("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInventory_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bots.json")

	inv1 := &inventory{path: path}
	require.NoError(t, inv1.add(&BotEntry{
		ID:   "persist-bot",
		Name: "Persist",
		Slack: SlackIdentity{
			Enabled:  true,
			BotToken: "xoxb-secret",
		},
		Creds: map[string]string{"custom": "val"},
	}))

	inv2 := &inventory{path: path}
	entry, err := inv2.get("persist-bot")
	require.NoError(t, err)
	assert.Equal(t, "Persist", entry.Name)
	assert.Equal(t, "xoxb-secret", entry.Slack.BotToken)
	assert.Equal(t, "val", entry.Creds["custom"])
}

func TestInventory_FilePermissions(t *testing.T) {
	inv := newTestInventory(t)
	require.NoError(t, inv.add(&BotEntry{ID: "bot", Name: "Bot"}))

	info, err := os.Stat(inv.path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestInventory_Platforms(t *testing.T) {
	e := &BotEntry{
		Slack:  SlackIdentity{Enabled: true},
		GitHub: GitHubIdentity{Enabled: true},
	}
	assert.Equal(t, []string{"slack", "github"}, e.platforms())

	e2 := &BotEntry{Slack: SlackIdentity{Enabled: true}}
	assert.Equal(t, []string{"slack"}, e2.platforms())

	e3 := &BotEntry{}
	assert.Empty(t, e3.platforms())
}

func TestInventory_UnifiedBot(t *testing.T) {
	inv := newTestInventory(t)
	require.NoError(t, inv.add(&BotEntry{
		ID:   "deploy-bot",
		Name: "Deploy Bot",
		Slack: SlackIdentity{
			Enabled:      true,
			AppID:        "A123",
			ClientID:     "c123",
			ClientSecret: "s3cr3t",
			BotToken:     "xoxb-tok",
		},
		GitHub: GitHubIdentity{
			Enabled: true,
			AppID:   "456",
			AppSlug: "deploy-bot-app",
		},
	}))

	entry, err := inv.get("deploy-bot")
	require.NoError(t, err)
	assert.Equal(t, "A123", entry.Slack.AppID)
	assert.Equal(t, "456", entry.GitHub.AppID)
	assert.Equal(t, []string{"slack", "github"}, entry.platforms())
}

func TestMaskValue(t *testing.T) {
	assert.Equal(t, "", maskValue(""))
	assert.Equal(t, "***", maskValue("short"))
	assert.Equal(t, "xoxb...4567", maskValue("xoxb-1234567"))
}
