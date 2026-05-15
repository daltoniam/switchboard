package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// CharsPerToken is the rough chars-per-token ratio used to convert byte
// savings into estimated LLM token savings. Industry-standard heuristic for
// JSON/English content (Claude, GPT-4, and Llama tokenizers all hover in this
// range). Surfaced as a constant so callers and docs can reference one source
// of truth.
const CharsPerToken = 4

// DefaultInputDollarsPerMTok is the default dollar cost per million input
// tokens, used to estimate dollars saved on the dashboard. Defaults to current
// Claude Sonnet input pricing ($3/MTok). Configurable in Settings.
const DefaultInputDollarsPerMTok = 3.0

// BytesToTokens converts a byte count to an estimated token count using
// CharsPerToken. Treats negative input as zero.
func BytesToTokens(b int64) int64 {
	if b <= 0 {
		return 0
	}
	return b / CharsPerToken
}

// Metrics collects operational metrics for the Switchboard server.
// All methods are safe for concurrent use.
type Metrics struct {
	mu sync.RWMutex

	// Per-tool execution tracking.
	toolCalls   map[ToolName]*toolMetric
	searchCount atomic.Int64
	scriptCount atomic.Int64

	// Per-integration aggregates.
	integrationCalls map[IntegrationName]*integrationMetric
	circuitBreaks    map[IntegrationName]*atomic.Int64

	// Rolling-window samples (bounded). Used for percentage averages and
	// (in the future) recency-weighted stats; counts overflow into the
	// atomic lifetime totals below so persistence preserves accuracy.
	compactionSavings []compactionSample
	markdownRenders   []compactionSample // reuses compactionSample shape

	// Lifetime aggregates for the "context window saved" story. Tracked as
	// atomics so they survive sample-slice trimming and can be persisted.
	compactionBytesBefore atomic.Int64
	compactionBytesAfter  atomic.Int64
	compactionSamplesAll  atomic.Int64

	markdownBytesBefore atomic.Int64
	markdownBytesAfter  atomic.Int64
	markdownSamplesAll  atomic.Int64

	// Catalog avoidance: bytes the LLM never had to see because Switchboard
	// shipped only matching tools from `search` instead of the entire
	// vendor-MCP tool catalog.
	catalogBytesAvoided atomic.Int64
	catalogAvoidedCount atomic.Int64

	// Script intermediates: bytes flowing through api.call() inside a script
	// that never reach the LLM. The "final" bytes are what the script
	// actually returned; "intermediate" is the sum of every api.call() raw
	// response (which the LLM would have had to see if it had called those
	// tools directly).
	scriptIntermediateBytes atomic.Int64
	scriptFinalBytes        atomic.Int64
	scriptSavingsSamples    atomic.Int64

	// Global counters.
	totalExecutions atomic.Int64
	totalErrors     atomic.Int64
	totalRetries    atomic.Int64
	truncations     atomic.Int64
	startTime       time.Time

	// Persistence bookkeeping.
	dirty    atomic.Bool // set on every record; cleared after a successful flush.
	filePath string      // empty disables persistence.
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
	Tool       ToolName
	BeforeSize int
	AfterSize  int
}

// NewMetrics returns an initialized Metrics collector with no persistence.
func NewMetrics() *Metrics {
	return &Metrics{
		toolCalls:        make(map[ToolName]*toolMetric),
		integrationCalls: make(map[IntegrationName]*integrationMetric),
		circuitBreaks:    make(map[IntegrationName]*atomic.Int64),
		startTime:        time.Now(),
	}
}

// RecordExecution records a tool execution with its outcome.
func (m *Metrics) RecordExecution(integration IntegrationName, tool ToolName, duration time.Duration, isError bool, retries int) {
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
	m.dirty.Store(true)
}

// RecordSearch records a search invocation.
func (m *Metrics) RecordSearch() {
	m.searchCount.Add(1)
	m.dirty.Store(true)
}

// RecordScript records a script execution.
func (m *Metrics) RecordScript() {
	m.scriptCount.Add(1)
	m.dirty.Store(true)
}

// RecordCircuitBreak records a circuit breaker trip for an integration.
func (m *Metrics) RecordCircuitBreak(integration IntegrationName) {
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
	m.dirty.Store(true)
}

// RecordCompaction records a compaction result.
func (m *Metrics) RecordCompaction(tool ToolName, beforeSize, afterSize int) {
	m.compactionBytesBefore.Add(int64(beforeSize))
	m.compactionBytesAfter.Add(int64(afterSize))
	m.compactionSamplesAll.Add(1)

	m.mu.Lock()
	m.compactionSavings = append(m.compactionSavings, compactionSample{
		Tool:       tool,
		BeforeSize: beforeSize,
		AfterSize:  afterSize,
	})
	if len(m.compactionSavings) > 1000 {
		m.compactionSavings = m.compactionSavings[len(m.compactionSavings)-1000:]
	}
	m.mu.Unlock()
	m.dirty.Store(true)
}

// RecordMarkdownRender records a markdown rendering result.
func (m *Metrics) RecordMarkdownRender(tool ToolName, beforeSize, afterSize int) {
	m.markdownBytesBefore.Add(int64(beforeSize))
	m.markdownBytesAfter.Add(int64(afterSize))
	m.markdownSamplesAll.Add(1)

	m.mu.Lock()
	m.markdownRenders = append(m.markdownRenders, compactionSample{
		Tool:       tool,
		BeforeSize: beforeSize,
		AfterSize:  afterSize,
	})
	if len(m.markdownRenders) > 1000 {
		m.markdownRenders = m.markdownRenders[len(m.markdownRenders)-1000:]
	}
	m.mu.Unlock()
	m.dirty.Store(true)
}

// RecordCatalogAvoidance records bytes that the LLM did NOT receive because
// Switchboard shipped only matching tools via `search` instead of the full
// vendor-MCP tool catalog. Call once per `search` invocation with
// (catalogBytes - resultBytes).
//
// This bucket is the headline savings story for Switchboard vs. connecting
// to multiple official vendor MCPs directly: each direct MCP ships its full
// tool list on every client turn, while Switchboard ships only what was
// asked for.
func (m *Metrics) RecordCatalogAvoidance(bytesAvoided int64) {
	if bytesAvoided <= 0 {
		return
	}
	m.catalogBytesAvoided.Add(bytesAvoided)
	m.catalogAvoidedCount.Add(1)
	m.dirty.Store(true)
}

// RecordScriptSavings records the byte flow of a single script execution.
// `intermediate` is the sum of every api.call() raw response size inside the
// script; `final` is the size of the script's returned value. The difference
// represents JSON that flowed through the server-side script engine but
// never reached the LLM's context window.
func (m *Metrics) RecordScriptSavings(intermediate, final int64) {
	if intermediate <= 0 && final <= 0 {
		return
	}
	if intermediate > 0 {
		m.scriptIntermediateBytes.Add(intermediate)
	}
	if final > 0 {
		m.scriptFinalBytes.Add(final)
	}
	m.scriptSavingsSamples.Add(1)
	m.dirty.Store(true)
}

// RecordTruncation records a response that exceeded the size cap.
func (m *Metrics) RecordTruncation() {
	m.truncations.Add(1)
	m.dirty.Store(true)
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	return m.snapshotWithPricing(DefaultInputDollarsPerMTok, false)
}

// SnapshotWithPricing returns a Snapshot with an optional dollar estimate.
// When showDollars is false the EstDollarsSaved field is left zero so the UI
// can decide whether to render it.
func (m *Metrics) SnapshotWithPricing(dollarsPerMTok float64, showDollars bool) MetricsSnapshot {
	return m.snapshotWithPricing(dollarsPerMTok, showDollars)
}

func (m *Metrics) snapshotWithPricing(dollarsPerMTok float64, showDollars bool) MetricsSnapshot {
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
		s.Tools[string(name)] = ToolSnapshot{
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
		s.Integrations[string(name)] = IntegrationSnapshot{
			Calls:        calls,
			Errors:       im.Errors.Load(),
			AvgLatencyMs: float64(avgNs) / 1e6,
		}
	}

	for name, counter := range m.circuitBreaks {
		s.CircuitBreaks[string(name)] = counter.Load()
	}

	// Compaction lifetime totals (atomics survive sample-slice trimming).
	cBefore := m.compactionBytesBefore.Load()
	cAfter := m.compactionBytesAfter.Load()
	s.CompactionSamples = int(m.compactionSamplesAll.Load())
	s.CompactionBytesBefore = cBefore
	s.CompactionBytesSaved = cBefore - cAfter
	if cBefore > 0 {
		s.CompactionSavingsPct = 100 - int(100*cAfter/cBefore)
	}

	// Markdown rendering lifetime totals.
	mBefore := m.markdownBytesBefore.Load()
	mAfter := m.markdownBytesAfter.Load()
	s.MarkdownSamples = int(m.markdownSamplesAll.Load())
	s.MarkdownBytesBefore = mBefore
	s.MarkdownBytesSaved = mBefore - mAfter
	if mBefore > 0 {
		s.MarkdownSavingsPct = 100 - int(100*mAfter/mBefore)
	}

	// Catalog avoidance.
	s.CatalogBytesAvoided = m.catalogBytesAvoided.Load()
	s.CatalogAvoidedCount = m.catalogAvoidedCount.Load()

	// Script intermediates: the bytes that flowed through api.call() but
	// never ended up in the LLM-visible final return value.
	scriptIntermediate := m.scriptIntermediateBytes.Load()
	scriptFinal := m.scriptFinalBytes.Load()
	s.ScriptIntermediateBytes = scriptIntermediate
	s.ScriptFinalBytes = scriptFinal
	s.ScriptIntermediateHidden = scriptIntermediate - scriptFinal
	if s.ScriptIntermediateHidden < 0 {
		s.ScriptIntermediateHidden = 0
	}
	s.ScriptSavingsSamples = m.scriptSavingsSamples.Load()

	// Aggregate context-window savings (the headline number).
	s.TotalBytesSaved = s.CompactionBytesSaved + s.MarkdownBytesSaved + s.CatalogBytesAvoided + s.ScriptIntermediateHidden
	s.TotalTokensSaved = BytesToTokens(s.TotalBytesSaved)
	s.CompactionTokensSaved = BytesToTokens(s.CompactionBytesSaved)
	s.MarkdownTokensSaved = BytesToTokens(s.MarkdownBytesSaved)
	s.CatalogTokensSaved = BytesToTokens(s.CatalogBytesAvoided)
	s.ScriptTokensSaved = BytesToTokens(s.ScriptIntermediateHidden)

	if showDollars && dollarsPerMTok > 0 && s.TotalTokensSaved > 0 {
		dollars := (float64(s.TotalTokensSaved) / 1_000_000.0) * dollarsPerMTok
		s.EstDollarsSaved = fmt.Sprintf("$%.2f", dollars)
		s.DollarsPerMTok = dollarsPerMTok
	}

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

func (m *Metrics) getToolMetric(tool ToolName) *toolMetric {
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

func (m *Metrics) getIntegrationMetric(integration IntegrationName) *integrationMetric {
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
	UptimeSeconds   float64 `json:"uptime_seconds"`
	TotalExecutions int64   `json:"total_executions"`
	TotalErrors     int64   `json:"total_errors"`
	TotalRetries    int64   `json:"total_retries"`
	SearchCount     int64   `json:"search_count"`
	ScriptCount     int64   `json:"script_count"`
	Truncations     int64   `json:"truncations"`

	// Compaction (field-projection on JSON tool responses).
	CompactionSamples     int   `json:"compaction_samples"`
	CompactionBytesBefore int64 `json:"compaction_bytes_before"`
	CompactionBytesSaved  int64 `json:"compaction_bytes_saved"`
	CompactionSavingsPct  int   `json:"compaction_savings_pct"`
	CompactionTokensSaved int64 `json:"compaction_tokens_saved"`

	// Markdown rendering (HTML/JSON document → markdown).
	MarkdownSamples     int   `json:"markdown_samples"`
	MarkdownBytesBefore int64 `json:"markdown_bytes_before"`
	MarkdownBytesSaved  int64 `json:"markdown_bytes_saved"`
	MarkdownSavingsPct  int   `json:"markdown_savings_pct"`
	MarkdownTokensSaved int64 `json:"markdown_tokens_saved"`

	// Catalog avoidance (search returning matching tools vs. full vendor catalog).
	CatalogBytesAvoided int64 `json:"catalog_bytes_avoided"`
	CatalogAvoidedCount int64 `json:"catalog_avoided_count"`
	CatalogTokensSaved  int64 `json:"catalog_tokens_saved"`

	// Script intermediates (api.call() results that never reached the LLM).
	ScriptIntermediateBytes  int64 `json:"script_intermediate_bytes"`
	ScriptFinalBytes         int64 `json:"script_final_bytes"`
	ScriptIntermediateHidden int64 `json:"script_intermediate_hidden"`
	ScriptSavingsSamples     int64 `json:"script_savings_samples"`
	ScriptTokensSaved        int64 `json:"script_tokens_saved"`

	// Aggregate "context window saved" — the headline number.
	TotalBytesSaved  int64 `json:"total_bytes_saved"`
	TotalTokensSaved int64 `json:"total_tokens_saved"`

	// Optional dollar estimate (populated only when Settings → Show Dollar Estimate is on).
	EstDollarsSaved string  `json:"est_dollars_saved,omitempty"`
	DollarsPerMTok  float64 `json:"dollars_per_mtok,omitempty"`

	Tools         map[string]ToolSnapshot        `json:"tools"`
	Integrations  map[string]IntegrationSnapshot `json:"integrations"`
	CircuitBreaks map[string]int64               `json:"circuit_breaks"`
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
	Name  ToolName `json:"name"`
	Calls int64    `json:"calls"`
}

// --- Persistence -----------------------------------------------------------
//
// Metrics are persisted as a flat lifetime-totals document. We deliberately
// do NOT persist the rolling sample slices (compactionSavings, markdownRenders)
// or per-tool/per-integration call counts — those would balloon the file with
// tool churn. The persisted document is small (<1KB) and only flushed when
// dirty, so it does not produce a steady stream of disk writes.

// persistedMetrics is the on-disk schema. Keep field tags stable.
type persistedMetrics struct {
	Version int `json:"version"`

	TotalExecutions int64 `json:"total_executions"`
	TotalErrors     int64 `json:"total_errors"`
	TotalRetries    int64 `json:"total_retries"`
	SearchCount     int64 `json:"search_count"`
	ScriptCount     int64 `json:"script_count"`
	Truncations     int64 `json:"truncations"`

	CompactionBytesBefore int64 `json:"compaction_bytes_before"`
	CompactionBytesAfter  int64 `json:"compaction_bytes_after"`
	CompactionSamplesAll  int64 `json:"compaction_samples_all"`

	MarkdownBytesBefore int64 `json:"markdown_bytes_before"`
	MarkdownBytesAfter  int64 `json:"markdown_bytes_after"`
	MarkdownSamplesAll  int64 `json:"markdown_samples_all"`

	CatalogBytesAvoided int64 `json:"catalog_bytes_avoided"`
	CatalogAvoidedCount int64 `json:"catalog_avoided_count"`

	ScriptIntermediateBytes int64 `json:"script_intermediate_bytes"`
	ScriptFinalBytes        int64 `json:"script_final_bytes"`
	ScriptSavingsSamples    int64 `json:"script_savings_samples"`

	SavedAt time.Time `json:"saved_at"`
}

const persistedMetricsVersion = 1

// WithPersistence configures the metrics collector to load lifetime totals
// from `path` (if present) and to flush back to that path on Flush() or
// shutdown. Pass an empty path to disable persistence.
//
// Returns the same Metrics for chaining. Logs (via the standard logger) but
// does not fail if the file cannot be read — a corrupt or missing file simply
// starts fresh.
func (m *Metrics) WithPersistence(path string) *Metrics {
	m.filePath = path
	if path == "" {
		return m
	}
	m.loadFromDisk()
	// Loading does not count as dirty.
	m.dirty.Store(false)
	return m
}

func (m *Metrics) loadFromDisk() {
	if m.filePath == "" {
		return
	}
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return // missing file is fine
	}
	var p persistedMetrics
	if err := json.Unmarshal(data, &p); err != nil {
		return // corrupt file: ignore and start fresh
	}
	m.totalExecutions.Store(p.TotalExecutions)
	m.totalErrors.Store(p.TotalErrors)
	m.totalRetries.Store(p.TotalRetries)
	m.searchCount.Store(p.SearchCount)
	m.scriptCount.Store(p.ScriptCount)
	m.truncations.Store(p.Truncations)

	m.compactionBytesBefore.Store(p.CompactionBytesBefore)
	m.compactionBytesAfter.Store(p.CompactionBytesAfter)
	m.compactionSamplesAll.Store(p.CompactionSamplesAll)

	m.markdownBytesBefore.Store(p.MarkdownBytesBefore)
	m.markdownBytesAfter.Store(p.MarkdownBytesAfter)
	m.markdownSamplesAll.Store(p.MarkdownSamplesAll)

	m.catalogBytesAvoided.Store(p.CatalogBytesAvoided)
	m.catalogAvoidedCount.Store(p.CatalogAvoidedCount)

	m.scriptIntermediateBytes.Store(p.ScriptIntermediateBytes)
	m.scriptFinalBytes.Store(p.ScriptFinalBytes)
	m.scriptSavingsSamples.Store(p.ScriptSavingsSamples)
}

// Flush writes lifetime totals to disk, but only when the dirty flag is set.
// Safe to call on a timer — it is a no-op when nothing has changed. Uses an
// atomic write (tmp file + rename) so a crash mid-write cannot corrupt the
// existing file.
func (m *Metrics) Flush() error {
	if m.filePath == "" {
		return nil
	}
	if !m.dirty.Load() {
		return nil
	}
	p := persistedMetrics{
		Version:                 persistedMetricsVersion,
		TotalExecutions:         m.totalExecutions.Load(),
		TotalErrors:             m.totalErrors.Load(),
		TotalRetries:            m.totalRetries.Load(),
		SearchCount:             m.searchCount.Load(),
		ScriptCount:             m.scriptCount.Load(),
		Truncations:             m.truncations.Load(),
		CompactionBytesBefore:   m.compactionBytesBefore.Load(),
		CompactionBytesAfter:    m.compactionBytesAfter.Load(),
		CompactionSamplesAll:    m.compactionSamplesAll.Load(),
		MarkdownBytesBefore:     m.markdownBytesBefore.Load(),
		MarkdownBytesAfter:      m.markdownBytesAfter.Load(),
		MarkdownSamplesAll:      m.markdownSamplesAll.Load(),
		CatalogBytesAvoided:     m.catalogBytesAvoided.Load(),
		CatalogAvoidedCount:     m.catalogAvoidedCount.Load(),
		ScriptIntermediateBytes: m.scriptIntermediateBytes.Load(),
		ScriptFinalBytes:        m.scriptFinalBytes.Load(),
		ScriptSavingsSamples:    m.scriptSavingsSamples.Load(),
		SavedAt:                 time.Now(),
	}
	data, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create metrics dir: %w", err)
	}
	tmp := m.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write metrics tmp: %w", err)
	}
	if err := os.Rename(tmp, m.filePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename metrics: %w", err)
	}
	m.dirty.Store(false)
	return nil
}

// Reset clears all lifetime totals and marks dirty. Used by the "reset" UI
// action. Does not touch the on-disk file until the next Flush.
func (m *Metrics) Reset() {
	m.totalExecutions.Store(0)
	m.totalErrors.Store(0)
	m.totalRetries.Store(0)
	m.searchCount.Store(0)
	m.scriptCount.Store(0)
	m.truncations.Store(0)
	m.compactionBytesBefore.Store(0)
	m.compactionBytesAfter.Store(0)
	m.compactionSamplesAll.Store(0)
	m.markdownBytesBefore.Store(0)
	m.markdownBytesAfter.Store(0)
	m.markdownSamplesAll.Store(0)
	m.catalogBytesAvoided.Store(0)
	m.catalogAvoidedCount.Store(0)
	m.scriptIntermediateBytes.Store(0)
	m.scriptFinalBytes.Store(0)
	m.scriptSavingsSamples.Store(0)

	m.mu.Lock()
	m.compactionSavings = nil
	m.markdownRenders = nil
	m.toolCalls = make(map[ToolName]*toolMetric)
	m.integrationCalls = make(map[IntegrationName]*integrationMetric)
	m.circuitBreaks = make(map[IntegrationName]*atomic.Int64)
	m.startTime = time.Now()
	m.mu.Unlock()

	m.dirty.Store(true)
}
