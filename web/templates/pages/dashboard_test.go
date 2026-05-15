package pages

import (
	"bytes"
	"context"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortByName(t *testing.T) {
	items := []IntegrationSummary{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}
	sorted := sortByName(items)
	assert.Equal(t, "alpha", sorted[0].Name)
	assert.Equal(t, "bravo", sorted[1].Name)
	assert.Equal(t, "charlie", sorted[2].Name)

	assert.Equal(t, "charlie", items[0].Name)
}

func TestLastCheckLabel(t *testing.T) {
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{"zero", time.Time{}, ""},
		{"just now", time.Now().Add(-5 * time.Second), "just now"},
		{"minutes", time.Now().Add(-15 * time.Minute), "15m ago"},
		{"hours", time.Now().Add(-3 * time.Hour), "3h ago"},
		{"days", time.Now().Add(-48 * time.Hour), time.Now().Add(-48 * time.Hour).Format("Jan 2 15:04")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastCheckLabel(tt.when)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1_000, "1.0K"},
		{1_500, "1.5K"},
		{1_000_000, "1.00M"},
		{2_500_000, "2.50M"},
		{1_000_000_000, "1.00B"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, formatTokens(tt.in))
	}
}

func TestSavingsPct(t *testing.T) {
	assert.Equal(t, "0%", savingsPct(0, 0))
	assert.Equal(t, "0%", savingsPct(0, 100))
	assert.Equal(t, "0%", savingsPct(100, 0))
	assert.Equal(t, "50%", savingsPct(50, 100))
	assert.Equal(t, "0.5%", savingsPct(5, 1000))
	assert.Equal(t, "100%", savingsPct(100, 100))
}

func TestHasSavings(t *testing.T) {
	assert.False(t, hasSavings(nil))
	assert.False(t, hasSavings(&mcp.MetricsSnapshot{}))
	assert.True(t, hasSavings(&mcp.MetricsSnapshot{TotalBytesSaved: 1}))
	assert.True(t, hasSavings(&mcp.MetricsSnapshot{CatalogAvoidedCount: 1}))
	assert.True(t, hasSavings(&mcp.MetricsSnapshot{CompactionSamples: 1}))
}

func TestSampleNoun(t *testing.T) {
	assert.Equal(t, "search", sampleNoun("Tool catalog", 1))
	assert.Equal(t, "searches", sampleNoun("Tool catalog", 2))
	assert.Equal(t, "script", sampleNoun("Script intermediates", 1))
	assert.Equal(t, "scripts", sampleNoun("Script intermediates", 5))
	assert.Equal(t, "call", sampleNoun("Response compaction", 1))
	assert.Equal(t, "calls", sampleNoun("Markdown rendering", 3))
}

func renderDashboard(t *testing.T, data DashboardData) string {
	t.Helper()
	var buf bytes.Buffer
	page := layouts.PageData{Title: "Dashboard", CurrentPath: "/"}
	require.NoError(t, Dashboard(page, data).Render(context.Background(), &buf))
	return buf.String()
}

// heroSentinel is text that only appears when the savings hero section renders,
// never in CSS or other markup.
const heroSentinel = "Context window saved by Switchboard"

func TestDashboard_NoMetrics_HidesHero(t *testing.T) {
	html := renderDashboard(t, DashboardData{ConnectedCount: 2})
	assert.NotContains(t, html, heroSentinel)
}

func TestDashboard_ZeroSavings_HidesHero(t *testing.T) {
	html := renderDashboard(t, DashboardData{
		ConnectedCount: 1,
		Metrics:        &mcp.MetricsSnapshot{TotalExecutions: 5},
	})
	assert.NotContains(t, html, heroSentinel)
}

func TestDashboard_RendersHero_WithCatalogOnly(t *testing.T) {
	html := renderDashboard(t, DashboardData{
		ConnectedCount: 1,
		Metrics: &mcp.MetricsSnapshot{
			CatalogBytesAvoided: 40_000,
			CatalogAvoidedCount: 3,
			CatalogTokensSaved:  10_000,
			TotalBytesSaved:     40_000,
			TotalTokensSaved:    10_000,
		},
	})
	assert.Contains(t, html, heroSentinel)
	assert.Contains(t, html, "10.0K") // tokens
	assert.Contains(t, html, "Tool catalog")
	assert.Contains(t, html, "Response compaction")
	assert.Contains(t, html, "Markdown rendering")
	assert.Contains(t, html, "Script intermediates")
	assert.Contains(t, html, "39.1KB") // ~40000 bytes
	assert.NotContains(t, html, "saved at $")
}

func TestDashboard_RendersHero_WithDollars(t *testing.T) {
	html := renderDashboard(t, DashboardData{
		ConnectedCount: 1,
		Metrics: &mcp.MetricsSnapshot{
			CatalogBytesAvoided:   20_000,
			CatalogAvoidedCount:   2,
			CatalogTokensSaved:    5_000,
			CompactionBytesSaved:  10_000,
			CompactionSamples:     4,
			CompactionTokensSaved: 2_500,
			TotalBytesSaved:       30_000,
			TotalTokensSaved:      7_500,
			EstDollarsSaved:       "$0.02",
			DollarsPerMTok:        3.0,
		},
	})
	assert.Contains(t, html, "$0.02")
	assert.Contains(t, html, "$3.00/MTok")
	assert.Contains(t, html, "saved at")
}

func TestDashboard_HeroIncludesAllBucketsByName(t *testing.T) {
	html := renderDashboard(t, DashboardData{
		ConnectedCount: 1,
		Metrics: &mcp.MetricsSnapshot{
			TotalBytesSaved:          1000,
			TotalTokensSaved:         250,
			ScriptIntermediateHidden: 500,
			ScriptSavingsSamples:     1,
			ScriptTokensSaved:        125,
		},
	})
	// All four bucket labels must appear regardless of which has data
	assert.Contains(t, html, "Tool catalog")
	assert.Contains(t, html, "Response compaction")
	assert.Contains(t, html, "Markdown rendering")
	assert.Contains(t, html, "Script intermediates")
	// Foot note about token heuristic
	assert.Contains(t, html, "4 characters per token")
}
