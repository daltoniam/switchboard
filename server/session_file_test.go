package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSessionStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, time.Hour)

	sess := fs.GetOrCreate("test-sess")
	sess.SetContext(map[string]any{"owner": "daltoniam", "repo": "switchboard"})
	sess.AddBreadcrumb("github_list_issues", map[string]any{"state": "open"}, `[{"id":1}]`, false)

	require.NoError(t, fs.Save(sess))

	path := filepath.Join(dir, "test-sess.md")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "# Session: test-sess")
	assert.Contains(t, content, "**owner:** `daltoniam`")
	assert.Contains(t, content, "**repo:** `switchboard`")
	assert.Contains(t, content, "`github_list_issues`")
	assert.Contains(t, content, "<!-- session-data")
	assert.Contains(t, content, "session-data -->")

	fs2 := NewFileSessionStore(dir, time.Hour)
	loaded := fs2.GetOrCreate("test-sess")
	ctx := loaded.GetContext()
	assert.Equal(t, "daltoniam", ctx["owner"])
	assert.Equal(t, "switchboard", ctx["repo"])
	require.Len(t, loaded.Breadcrumbs, 1)
	assert.Equal(t, "github_list_issues", string(loaded.Breadcrumbs[0].Tool))
}

func TestFileSessionStore_Get(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, time.Hour)

	_, ok := fs.Get("nonexistent")
	assert.False(t, ok)

	sess := fs.GetOrCreate("existing")
	sess.SetContext(map[string]any{"k": "v"})
	require.NoError(t, fs.Save(sess))

	got, ok := fs.Get("existing")
	require.True(t, ok)
	assert.Equal(t, "v", got.GetContext()["k"])
}

func TestFileSessionStore_Delete(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, time.Hour)

	sess := fs.GetOrCreate("to-delete")
	require.NoError(t, fs.Save(sess))

	fs.Delete("to-delete")

	_, ok := fs.Get("to-delete")
	assert.False(t, ok)

	_, err := os.Stat(filepath.Join(dir, "to-delete.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileSessionStore_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, 10*time.Millisecond)

	sess := fs.GetOrCreate("expires")
	require.NoError(t, fs.Save(sess))

	time.Sleep(20 * time.Millisecond)

	_, ok := fs.Get("expires")
	assert.False(t, ok)
}

func TestFileSessionStore_GetOrCreateReloadsFromDisk(t *testing.T) {
	dir := t.TempDir()
	fs1 := NewFileSessionStore(dir, time.Hour)

	sess := fs1.GetOrCreate("shared")
	sess.SetContext(map[string]any{"env": "prod"})
	require.NoError(t, fs1.Save(sess))

	fs2 := NewFileSessionStore(dir, time.Hour)
	loaded := fs2.GetOrCreate("shared")
	assert.Equal(t, "prod", loaded.GetContext()["env"])
}

func TestFileSessionStore_BreadcrumbsPreserved(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, time.Hour)

	sess := fs.GetOrCreate("bc-test")
	sess.AddBreadcrumb("tool_a", nil, "result_a", false)
	sess.AddBreadcrumb("tool_b", nil, "result_b", true)
	require.NoError(t, fs.Save(sess))

	fs2 := NewFileSessionStore(dir, time.Hour)
	loaded := fs2.GetOrCreate("bc-test")
	require.Len(t, loaded.Breadcrumbs, 2)
	assert.Equal(t, "tool_a", string(loaded.Breadcrumbs[0].Tool))
	assert.False(t, loaded.Breadcrumbs[0].IsError)
	assert.Equal(t, "tool_b", string(loaded.Breadcrumbs[1].Tool))
	assert.True(t, loaded.Breadcrumbs[1].IsError)
}

func TestFileSessionStore_NextSeqPreserved(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, time.Hour)

	sess := fs.GetOrCreate("seq-test")
	sess.AddBreadcrumb("tool", nil, "r", false)
	sess.AddBreadcrumb("tool", nil, "r", false)
	require.NoError(t, fs.Save(sess))

	fs2 := NewFileSessionStore(dir, time.Hour)
	loaded := fs2.GetOrCreate("seq-test")
	loaded.AddBreadcrumb("tool", nil, "r", false)
	assert.Equal(t, 3, loaded.Breadcrumbs[2].Seq)
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"with..dots", "with_dots"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeID(tt.input))
		})
	}
}

func TestFormatAndParseSessionMarkdown_RoundTrip(t *testing.T) {
	s := newSession("roundtrip")
	s.SetContext(map[string]any{"owner": "test", "count": float64(42)})
	s.AddBreadcrumb("tool_x", map[string]any{"arg": "val"}, `{"ok":true}`, false)
	s.AddBreadcrumb("tool_y", nil, "error msg", true)

	md := formatSessionMarkdown(s)

	parsed := parseSessionMarkdown("roundtrip", md)
	require.NotNil(t, parsed)
	assert.Equal(t, "roundtrip", parsed.ID)
	assert.Equal(t, "test", parsed.GetContext()["owner"])
	assert.Equal(t, float64(42), parsed.GetContext()["count"])
	require.Len(t, parsed.Breadcrumbs, 2)
	assert.Equal(t, "tool_x", string(parsed.Breadcrumbs[0].Tool))
	assert.True(t, parsed.Breadcrumbs[1].IsError)
	assert.Equal(t, 2, parsed.nextSeq)
}

func TestParseSessionMarkdown_InvalidContent(t *testing.T) {
	assert.Nil(t, parseSessionMarkdown("x", "no markers here"))
	assert.Nil(t, parseSessionMarkdown("x", "<!-- session-data\ninvalid json\nsession-data -->"))
}

func TestFileSessionStore_MarkdownReadable(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileSessionStore(dir, time.Hour)

	sess := fs.GetOrCreate("readable")
	sess.SetContext(map[string]any{"project": "switchboard"})
	sess.AddBreadcrumb("github_list_repos", map[string]any{"org": "daltoniam"}, `[{"name":"sb"}]`, false)
	require.NoError(t, fs.Save(sess))

	data, err := os.ReadFile(filepath.Join(dir, "readable.md"))
	require.NoError(t, err)

	content := string(data)
	assert.True(t, strings.HasPrefix(content, "# Session: readable"))
	assert.Contains(t, content, "## Context")
	assert.Contains(t, content, "## Breadcrumbs")
	assert.Contains(t, content, "| # | Time | Tool | Error | Summary |")
}
