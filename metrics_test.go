package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_RecordExecution(t *testing.T) {
	m := NewMetrics()

	m.RecordExecution(IntegrationName("github"), ToolName("github_list_issues"), 100*time.Millisecond, false, 0)
	m.RecordExecution(IntegrationName("github"), ToolName("github_list_issues"), 200*time.Millisecond, false, 1)
	m.RecordExecution(IntegrationName("github"), ToolName("github_get_issue"), 50*time.Millisecond, true, 0)
	m.RecordExecution(IntegrationName("slack"), ToolName("slack_post_message"), 75*time.Millisecond, false, 0)

	snap := m.Snapshot()

	assert.Equal(t, int64(4), snap.TotalExecutions)
	assert.Equal(t, int64(1), snap.TotalErrors)
	assert.Equal(t, int64(1), snap.TotalRetries)

	require.Contains(t, snap.Tools, "github_list_issues")
	assert.Equal(t, int64(2), snap.Tools["github_list_issues"].Calls)
	assert.Equal(t, int64(0), snap.Tools["github_list_issues"].Errors)
	assert.Equal(t, int64(1), snap.Tools["github_list_issues"].Retries)

	require.Contains(t, snap.Tools, "github_get_issue")
	assert.Equal(t, int64(1), snap.Tools["github_get_issue"].Calls)
	assert.Equal(t, int64(1), snap.Tools["github_get_issue"].Errors)

	require.Contains(t, snap.Integrations, "github")
	assert.Equal(t, int64(3), snap.Integrations["github"].Calls)
	assert.Equal(t, int64(1), snap.Integrations["github"].Errors)

	require.Contains(t, snap.Integrations, "slack")
	assert.Equal(t, int64(1), snap.Integrations["slack"].Calls)
}

func TestMetrics_RecordSearchScript(t *testing.T) {
	m := NewMetrics()

	m.RecordSearch()
	m.RecordSearch()
	m.RecordScript()

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.SearchCount)
	assert.Equal(t, int64(1), snap.ScriptCount)
}

func TestMetrics_RecordCompaction(t *testing.T) {
	m := NewMetrics()

	m.RecordCompaction("github_list_issues", 10000, 3000)
	m.RecordCompaction("slack_search", 5000, 2500)

	snap := m.Snapshot()
	assert.Equal(t, 2, snap.CompactionSamples)
	assert.Equal(t, 64, snap.CompactionSavingsPct) // 100 - int(100*5500/15000) = 100 - 36 = 64
	assert.Equal(t, int64(9500), snap.CompactionBytesSaved)
}

func TestMetrics_RecordCircuitBreak(t *testing.T) {
	m := NewMetrics()

	m.RecordCircuitBreak(IntegrationName("github"))
	m.RecordCircuitBreak(IntegrationName("github"))
	m.RecordCircuitBreak(IntegrationName("slack"))

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.CircuitBreaks["github"])
	assert.Equal(t, int64(1), snap.CircuitBreaks["slack"])
}

func TestMetrics_RecordTruncation(t *testing.T) {
	m := NewMetrics()

	m.RecordTruncation()
	m.RecordTruncation()

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.Truncations)
}

func TestMetrics_TopTools(t *testing.T) {
	m := NewMetrics()

	for range 10 {
		m.RecordExecution(IntegrationName("github"), ToolName("github_list_issues"), time.Millisecond, false, 0)
	}
	for range 5 {
		m.RecordExecution(IntegrationName("slack"), ToolName("slack_post_message"), time.Millisecond, false, 0)
	}
	for range 3 {
		m.RecordExecution(IntegrationName("linear"), ToolName("linear_create_issue"), time.Millisecond, false, 0)
	}

	top := m.TopTools(2)
	require.Len(t, top, 2)
	assert.Equal(t, ToolName("github_list_issues"), top[0].Name)
	assert.Equal(t, int64(10), top[0].Calls)
	assert.Equal(t, ToolName("slack_post_message"), top[1].Name)
	assert.Equal(t, int64(5), top[1].Calls)
}

func TestMetrics_ErrorRate(t *testing.T) {
	tests := []struct {
		name       string
		executions int64
		errors     int64
		wantRate   float64
	}{
		{"no executions", 0, 0, 0},
		{"no errors", 100, 0, 0},
		{"50% error rate", 100, 50, 50},
		{"100% error rate", 10, 10, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := MetricsSnapshot{
				TotalExecutions: tt.executions,
				TotalErrors:     tt.errors,
			}
			assert.InDelta(t, tt.wantRate, snap.ErrorRate(), 0.01)
		})
	}
}

func TestMetrics_Uptime(t *testing.T) {
	m := NewMetrics()
	time.Sleep(10 * time.Millisecond)
	snap := m.Snapshot()
	assert.Greater(t, snap.UptimeSeconds, float64(0))
}

func TestMetrics_CompactionSampleCap(t *testing.T) {
	// Lifetime sample count is tracked atomically and is NOT capped, so
	// snapshot reflects every recorded call. The rolling-window slice
	// (used for any future recency-weighted stats) is still capped at 1000.
	m := NewMetrics()
	for i := range 1100 {
		m.RecordCompaction("tool", 1000+i, 500)
	}
	snap := m.Snapshot()
	assert.Equal(t, 1100, snap.CompactionSamples)

	// Inspect the rolling slice directly to confirm the cap is still enforced.
	m.mu.RLock()
	defer m.mu.RUnlock()
	assert.LessOrEqual(t, len(m.compactionSavings), 1000)
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()
	done := make(chan struct{})

	for range 10 {
		go func() {
			for range 100 {
				m.RecordExecution(IntegrationName("github"), ToolName("github_list_issues"), time.Millisecond, false, 0)
				m.RecordSearch()
				m.RecordScript()
				m.RecordCircuitBreak(IntegrationName("github"))
				m.RecordCompaction("tool", 1000, 500)
				m.RecordTruncation()
				m.Snapshot()
				m.TopTools(3)
			}
			done <- struct{}{}
		}()
	}

	for range 10 {
		<-done
	}

	snap := m.Snapshot()
	assert.Equal(t, int64(1000), snap.TotalExecutions)
}

func TestBytesToTokens(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want int64
	}{
		{"zero", 0, 0},
		{"negative", -100, 0},
		{"small under threshold", 3, 0}, // 3 / 4 = 0
		{"exact", 4, 1},
		{"realistic", 40000, 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BytesToTokens(tt.in))
		})
	}
}

func TestMetrics_RecordCatalogAvoidance(t *testing.T) {
	m := NewMetrics()
	m.RecordCatalogAvoidance(120_000)
	m.RecordCatalogAvoidance(80_000)
	m.RecordCatalogAvoidance(-5) // ignored

	snap := m.Snapshot()
	assert.Equal(t, int64(200_000), snap.CatalogBytesAvoided)
	assert.Equal(t, int64(2), snap.CatalogAvoidedCount)
	assert.Equal(t, int64(50_000), snap.CatalogTokensSaved) // 200K / 4
}

func TestMetrics_RecordMarkdownRender_Lifetime(t *testing.T) {
	m := NewMetrics()
	m.RecordMarkdownRender("notion_get_page", 40000, 4000)
	m.RecordMarkdownRender("gmail_get_message", 10000, 2000)

	snap := m.Snapshot()
	assert.Equal(t, 2, snap.MarkdownSamples)
	assert.Equal(t, int64(50000), snap.MarkdownBytesBefore)
	assert.Equal(t, int64(44000), snap.MarkdownBytesSaved)  // 50K - 6K
	assert.Equal(t, int64(11000), snap.MarkdownTokensSaved) // 44K / 4
	assert.Equal(t, 88, snap.MarkdownSavingsPct)            // 100 - 100*6000/50000 = 88
}

func TestMetrics_RecordScriptSavings(t *testing.T) {
	m := NewMetrics()
	m.RecordScriptSavings(50_000, 200)
	m.RecordScriptSavings(30_000, 100)

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.ScriptSavingsSamples)
	assert.Equal(t, int64(80_000), snap.ScriptIntermediateBytes)
	assert.Equal(t, int64(300), snap.ScriptFinalBytes)
	assert.Equal(t, int64(79_700), snap.ScriptIntermediateHidden) // 80K - 300
}

func TestMetrics_RecordScriptSavings_FinalLargerThanIntermediate(t *testing.T) {
	// Defensive: a script could return more bytes than it pulled in via
	// api.call() (e.g., constructed text). Hidden bytes must clamp to 0.
	m := NewMetrics()
	m.RecordScriptSavings(100, 500)
	snap := m.Snapshot()
	assert.Equal(t, int64(0), snap.ScriptIntermediateHidden)
}

func TestMetrics_TotalSavings_AggregatesAllBuckets(t *testing.T) {
	m := NewMetrics()
	m.RecordCompaction("github_list_issues", 10_000, 2_000)  // saves 8K
	m.RecordMarkdownRender("notion_get_page", 40_000, 4_000) // saves 36K
	m.RecordCatalogAvoidance(120_000)                        // saves 120K
	m.RecordScriptSavings(50_000, 500)                       // hides 49.5K

	snap := m.Snapshot()
	want := int64(8_000 + 36_000 + 120_000 + 49_500)
	assert.Equal(t, want, snap.TotalBytesSaved)
	assert.Equal(t, want/CharsPerToken, snap.TotalTokensSaved)
}

func TestMetrics_SnapshotWithPricing_Dollars(t *testing.T) {
	m := NewMetrics()
	// 4M bytes saved => 1M tokens => $3 at $3/MTok.
	m.RecordCatalogAvoidance(4_000_000)

	off := m.SnapshotWithPricing(3.0, false)
	assert.Empty(t, off.EstDollarsSaved, "dollars hidden when showDollars=false")
	assert.Zero(t, off.DollarsPerMTok)

	on := m.SnapshotWithPricing(3.0, true)
	assert.Equal(t, "$3.00", on.EstDollarsSaved)
	assert.InDelta(t, 3.0, on.DollarsPerMTok, 0.001)

	// Custom rate (e.g., $15/MTok for a different model).
	pricey := m.SnapshotWithPricing(15.0, true)
	assert.Equal(t, "$15.00", pricey.EstDollarsSaved)
}

func TestMetrics_SnapshotWithPricing_ZeroSavingsNoDollars(t *testing.T) {
	m := NewMetrics()
	snap := m.SnapshotWithPricing(3.0, true)
	assert.Empty(t, snap.EstDollarsSaved)
}

func TestMetrics_Persistence_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	// Write phase.
	m := NewMetrics().WithPersistence(path)
	m.RecordExecution("github", "github_list_issues", 100*time.Millisecond, false, 0)
	m.RecordSearch()
	m.RecordScript()
	m.RecordCompaction("github_list_issues", 10_000, 2_000)
	m.RecordMarkdownRender("notion_get_page", 40_000, 4_000)
	m.RecordCatalogAvoidance(120_000)
	m.RecordScriptSavings(50_000, 500)
	m.RecordTruncation()
	require.NoError(t, m.Flush())

	// Idempotent: a second Flush with no changes should be a no-op (dirty=false).
	require.NoError(t, m.Flush())

	// Load phase.
	m2 := NewMetrics().WithPersistence(path)
	snap := m2.Snapshot()
	assert.Equal(t, int64(1), snap.TotalExecutions)
	assert.Equal(t, int64(1), snap.SearchCount)
	assert.Equal(t, int64(1), snap.ScriptCount)
	assert.Equal(t, int64(1), snap.Truncations)
	assert.Equal(t, int64(8_000), snap.CompactionBytesSaved)
	assert.Equal(t, int64(36_000), snap.MarkdownBytesSaved)
	assert.Equal(t, int64(120_000), snap.CatalogBytesAvoided)
	assert.Equal(t, int64(49_500), snap.ScriptIntermediateHidden)
}

func TestMetrics_Persistence_DirtyFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")
	m := NewMetrics().WithPersistence(path)

	// Nothing recorded yet — Flush is a no-op and no file is written.
	require.NoError(t, m.Flush())
	_, err := readFileOrEmpty(path)
	assert.Error(t, err, "no file should be written when nothing is dirty")

	// Record something, flush, then immediately re-flush.
	m.RecordSearch()
	require.NoError(t, m.Flush())
	stat1, err := fileModTime(path)
	require.NoError(t, err)

	// Second flush with no changes: mtime must not move (dirty cleared by first flush).
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, m.Flush())
	stat2, err := fileModTime(path)
	require.NoError(t, err)
	assert.Equal(t, stat1, stat2, "no-op flush must not touch the file")
}

func TestMetrics_Persistence_NoPath(t *testing.T) {
	m := NewMetrics() // no WithPersistence
	m.RecordSearch()
	assert.NoError(t, m.Flush()) // no-op, no error
}

func TestMetrics_Persistence_CorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")
	require.NoError(t, writeFile(path, []byte("{not json")))

	m := NewMetrics().WithPersistence(path)
	snap := m.Snapshot()
	assert.Zero(t, snap.TotalExecutions)
	assert.Zero(t, snap.SearchCount)
}

func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()
	m.RecordSearch()
	m.RecordCatalogAvoidance(100_000)
	m.RecordExecution("github", "github_list_issues", time.Millisecond, false, 0)

	m.Reset()
	snap := m.Snapshot()
	assert.Zero(t, snap.SearchCount)
	assert.Zero(t, snap.CatalogBytesAvoided)
	assert.Zero(t, snap.TotalExecutions)
	assert.Empty(t, snap.Tools)
	assert.Empty(t, snap.Integrations)
}

// --- helpers ---

func readFileOrEmpty(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
