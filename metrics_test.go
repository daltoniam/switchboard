package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_RecordExecution(t *testing.T) {
	m := NewMetrics()

	m.RecordExecution("github", "github_list_issues", 100*time.Millisecond, false, 0)
	m.RecordExecution("github", "github_list_issues", 200*time.Millisecond, false, 1)
	m.RecordExecution("github", "github_get_issue", 50*time.Millisecond, true, 0)
	m.RecordExecution("slack", "slack_post_message", 75*time.Millisecond, false, 0)

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

	m.RecordCircuitBreak("github")
	m.RecordCircuitBreak("github")
	m.RecordCircuitBreak("slack")

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
		m.RecordExecution("github", "github_list_issues", time.Millisecond, false, 0)
	}
	for range 5 {
		m.RecordExecution("slack", "slack_post_message", time.Millisecond, false, 0)
	}
	for range 3 {
		m.RecordExecution("linear", "linear_create_issue", time.Millisecond, false, 0)
	}

	top := m.TopTools(2)
	require.Len(t, top, 2)
	assert.Equal(t, "github_list_issues", top[0].Name)
	assert.Equal(t, int64(10), top[0].Calls)
	assert.Equal(t, "slack_post_message", top[1].Name)
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
	assert.Greater(t, snap.Uptime, time.Duration(0))
}

func TestMetrics_CompactionSampleCap(t *testing.T) {
	m := NewMetrics()
	for i := range 1100 {
		m.RecordCompaction("tool", 1000+i, 500)
	}
	snap := m.Snapshot()
	assert.Equal(t, 1000, snap.CompactionSamples)
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()
	done := make(chan struct{})

	for range 10 {
		go func() {
			for range 100 {
				m.RecordExecution("github", "github_list_issues", time.Millisecond, false, 0)
				m.RecordSearch()
				m.RecordScript()
				m.RecordCircuitBreak("github")
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
