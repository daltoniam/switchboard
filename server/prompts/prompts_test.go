package prompts_test

import (
	"testing"

	"github.com/daltoniam/switchboard/server/prompts"
	"github.com/stretchr/testify/require"
)

func TestSearchHintMulti(t *testing.T) {
	want := "These tools span multiple integrations. Use execute with a script to chain them in a single call — intermediate results stay server-side and never enter the conversation."
	require.Equal(t, want, prompts.SearchHintMulti(prompts.Context{}))
}

func TestSearchHintSingle(t *testing.T) {
	want := "Tip: if your task requires multiple tool calls, use execute with a script to chain them in a single call and reduce token usage."
	require.Equal(t, want, prompts.SearchHintSingle(prompts.Context{}))
}

func TestResponseTooLargeHint(t *testing.T) {
	want := "narrow your query (e.g., add a filter, reduce page size, or request fewer fields)"
	require.Equal(t, want, prompts.ResponseTooLargeHint(prompts.Context{}))
}

func TestCircuitBreaker(t *testing.T) {
	cases := []struct {
		name            string
		integration     string
		cooldownSeconds int
		want            string
	}{
		{"typical", "github", 30, `integration "github" temporarily unavailable (circuit breaker open, try again in ~30s). Other integrations still work.`},
		{"empty name", "", 30, `integration "" temporarily unavailable (circuit breaker open, try again in ~30s). Other integrations still work.`},
		{"zero cooldown", "linear", 0, `integration "linear" temporarily unavailable (circuit breaker open, try again in ~0s). Other integrations still work.`},
		{"large cooldown", "datadog", 3600, `integration "datadog" temporarily unavailable (circuit breaker open, try again in ~3600s). Other integrations still work.`},
		{"name with quotes", `a"b`, 5, `integration "a\"b" temporarily unavailable (circuit breaker open, try again in ~5s). Other integrations still work.`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, prompts.CircuitBreaker(prompts.Context{}, tc.integration, tc.cooldownSeconds))
		})
	}
}

func TestSearchSummary(t *testing.T) {
	cases := []struct {
		name  string
		total int
		query string
		want  string
	}{
		{"empty", 0, "", "Found 0 tools"},
		{"no query", 5, "", "Found 5 tools"},
		{"with query", 12, "auth", `Found 12 tools matching "auth"`},
		{"unicode", 3, "café", `Found 3 tools matching "café"`},
		{"japanese", 0, "日本", `Found 0 tools matching "日本"`},
		{"query with quotes", 1, `a"b`, `Found 1 tools matching "a\"b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := prompts.SearchSummary(prompts.Context{}, tc.total, tc.query)
			require.Equal(t, tc.want, got)
		})
	}
}
