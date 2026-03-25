package mcp

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects operational metrics for the Switchboard server.
// All methods are safe for concurrent use.
type Metrics struct {
	mu sync.RWMutex

	// Per-tool execution tracking.
	toolCalls   map[string]*toolMetric // key: tool name
	searchCount atomic.Int64
	scriptCount atomic.Int64

	// Per-integration aggregates.
	integrationCalls  map[string]*integrationMetric // key: integration name
	circuitBreaks     map[string]*atomic.Int64      // key: integration name
	compactionSavings []compactionSample

	// Global counters.
	totalExecutions atomic.Int64
	totalErrors     atomic.Int64
	totalRetries    atomic.Int64
	truncations     atomic.Int64
	startTime       time.Time
}

type toolMetric struct {
	Calls   atomic.Int64
	Errors  atomic.Int64
	TotalNs atomic.Int64 // sum of latencies in nanoseconds
	Retries atomic.Int64
}

type integrationMetric struct {
	Calls   atomic.Int64
	Errors  atomic.Int64
	TotalNs atomic.Int64
}

type compactionSample struct {
	Tool       string
	BeforeSize int
	AfterSize  int
}

// NewMetrics returns an initialized Metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		toolCalls:        make(map[string]*toolMetric),
		integrationCalls: make(map[string]*integrationMetric),
		circuitBreaks:    make(map[string]*atomic.Int64),
		startTime:        time.Now(),
	}
}

// RecordExecution records a tool execution with its outcome.
func (m *Metrics) RecordExecution(integration, tool string, duration time.Duration, isError bool, retries int) {
	m.totalExecutions.Add(1)
	if isError {
		m.totalErrors.Add(1)
	}
	if retries > 0 {
		m.totalRetries.Add(int64(retries))
	}

	ns := duration.Nanoseconds()

	tm := m.getToolMetric(tool)
	tm.Calls.Add(1)
	if isError {
		tm.Errors.Add(1)
	}
	tm.TotalNs.Add(ns)
	tm.Retries.Add(int64(retries))

	im := m.getIntegrationMetric(integration)
	im.Calls.Add(1)
	if isError {
		im.Errors.Add(1)
	}
	im.TotalNs.Add(ns)
}

// RecordSearch records a search invocation.
func (m *Metrics) RecordSearch() {
	m.searchCount.Add(1)
}

// RecordScript records a script execution.
func (m *Metrics) RecordScript() {
	m.scriptCount.Add(1)
}

// RecordCircuitBreak records a circuit breaker trip for an integration.
func (m *Metrics) RecordCircuitBreak(integration string) {
	m.mu.RLock()
	counter, ok := m.circuitBreaks[integration]
	m.mu.RUnlock()
	if !ok {
		m.mu.Lock()
		counter, ok = m.circuitBreaks[integration]
		if !ok {
			counter = &atomic.Int64{}
			m.circuitBreaks[integration] = counter
		}
		m.mu.Unlock()
	}
	counter.Add(1)
}

// RecordCompaction records a compaction result.
func (m *Metrics) RecordCompaction(tool string, beforeSize, afterSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compactionSavings = append(m.compactionSavings, compactionSample{
		Tool:       tool,
		BeforeSize: beforeSize,
		AfterSize:  afterSize,
	})
	if len(m.compactionSavings) > 1000 {
		m.compactionSavings = m.compactionSavings[len(m.compactionSavings)-1000:]
	}
}

// RecordTruncation records a response that exceeded the size cap.
func (m *Metrics) RecordTruncation() {
	m.truncations.Add(1)
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := MetricsSnapshot{
		UptimeSeconds:   time.Since(m.startTime).Seconds(),
		TotalExecutions: m.totalExecutions.Load(),
		TotalErrors:     m.totalErrors.Load(),
		TotalRetries:    m.totalRetries.Load(),
		SearchCount:     m.searchCount.Load(),
		ScriptCount:     m.scriptCount.Load(),
		Truncations:     m.truncations.Load(),
		Tools:           make(map[string]ToolSnapshot),
		Integrations:    make(map[string]IntegrationSnapshot),
		CircuitBreaks:   make(map[string]int64),
	}

	for name, tm := range m.toolCalls {
		calls := tm.Calls.Load()
		avgNs := int64(0)
		if calls > 0 {
			avgNs = tm.TotalNs.Load() / calls
		}
		s.Tools[name] = ToolSnapshot{
			Calls:        calls,
			Errors:       tm.Errors.Load(),
			AvgLatencyMs: float64(avgNs) / 1e6,
			Retries:      tm.Retries.Load(),
		}
	}

	for name, im := range m.integrationCalls {
		calls := im.Calls.Load()
		avgNs := int64(0)
		if calls > 0 {
			avgNs = im.TotalNs.Load() / calls
		}
		s.Integrations[name] = IntegrationSnapshot{
			Calls:        calls,
			Errors:       im.Errors.Load(),
			AvgLatencyMs: float64(avgNs) / 1e6,
		}
	}

	for name, counter := range m.circuitBreaks {
		s.CircuitBreaks[name] = counter.Load()
	}

	// Compaction summary.
	var totalBefore, totalAfter int64
	for _, cs := range m.compactionSavings {
		totalBefore += int64(cs.BeforeSize)
		totalAfter += int64(cs.AfterSize)
	}
	s.CompactionSamples = len(m.compactionSavings)
	if totalBefore > 0 {
		s.CompactionSavingsPct = 100 - int(100*totalAfter/totalBefore)
	}
	s.CompactionBytesSaved = totalBefore - totalAfter

	return s
}

// TopTools returns the N most-called tools, sorted by call count descending.
func (m *Metrics) TopTools(n int) []ToolRank {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ranks := make([]ToolRank, 0, len(m.toolCalls))
	for name, tm := range m.toolCalls {
		ranks = append(ranks, ToolRank{
			Name:  name,
			Calls: tm.Calls.Load(),
		})
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		if ranks[i].Calls != ranks[j].Calls {
			return ranks[i].Calls > ranks[j].Calls
		}
		return ranks[i].Name < ranks[j].Name
	})
	if n > 0 && n < len(ranks) {
		ranks = ranks[:n]
	}
	return ranks
}

func (m *Metrics) getToolMetric(tool string) *toolMetric {
	m.mu.RLock()
	tm, ok := m.toolCalls[tool]
	m.mu.RUnlock()
	if ok {
		return tm
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tm, ok = m.toolCalls[tool]
	if !ok {
		tm = &toolMetric{}
		m.toolCalls[tool] = tm
	}
	return tm
}

func (m *Metrics) getIntegrationMetric(integration string) *integrationMetric {
	m.mu.RLock()
	im, ok := m.integrationCalls[integration]
	m.mu.RUnlock()
	if ok {
		return im
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	im, ok = m.integrationCalls[integration]
	if !ok {
		im = &integrationMetric{}
		m.integrationCalls[integration] = im
	}
	return im
}

// MetricsSnapshot is a point-in-time copy of all metrics, safe to read without locks.
type MetricsSnapshot struct {
	UptimeSeconds        float64                        `json:"uptime_seconds"`
	TotalExecutions      int64                          `json:"total_executions"`
	TotalErrors          int64                          `json:"total_errors"`
	TotalRetries         int64                          `json:"total_retries"`
	SearchCount          int64                          `json:"search_count"`
	ScriptCount          int64                          `json:"script_count"`
	Truncations          int64                          `json:"truncations"`
	CompactionSamples    int                            `json:"compaction_samples"`
	CompactionSavingsPct int                            `json:"compaction_savings_pct"`
	CompactionBytesSaved int64                          `json:"compaction_bytes_saved"`
	Tools                map[string]ToolSnapshot        `json:"tools"`
	Integrations         map[string]IntegrationSnapshot `json:"integrations"`
	CircuitBreaks        map[string]int64               `json:"circuit_breaks"`
}

// ErrorRate returns the error rate as a percentage (0-100).
func (s MetricsSnapshot) ErrorRate() float64 {
	if s.TotalExecutions == 0 {
		return 0
	}
	return float64(s.TotalErrors) / float64(s.TotalExecutions) * 100
}

// ToolSnapshot holds metrics for a single tool.
type ToolSnapshot struct {
	Calls        int64   `json:"calls"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Retries      int64   `json:"retries"`
}

// IntegrationSnapshot holds metrics for a single integration.
type IntegrationSnapshot struct {
	Calls        int64   `json:"calls"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// ToolRank pairs a tool name with its call count for ranking.
type ToolRank struct {
	Name  string `json:"name"`
	Calls int64  `json:"calls"`
}
