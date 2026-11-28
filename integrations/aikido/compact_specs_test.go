package aikido

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// rawReadTools return payloads that compaction would only damage or that
// are already small: documents, positional paths, settings and lookups.
var rawReadTools = map[mcp.ToolName]string{
	"aikido_get_workspace":                   "five fields describing the workspace",
	"aikido_get_sla_settings":                "four severities with a duration each",
	"aikido_get_default_pr_check_config":     "one flat settings object",
	"aikido_get_compliance_overview":         "per-control overview objects with framework-specific keys",
	"aikido_export_code_repo_sbom":           "an SBOM document (CycloneDX, SPDX or CSV); dropping fields corrupts it",
	"aikido_export_container_sbom":           "an SBOM document (CycloneDX, SPDX or CSV); dropping fields corrupts it",
	"aikido_export_virtual_machine_sbom":     "an SBOM document (CycloneDX, SPDX or CSV); dropping fields corrupts it",
	"aikido_get_issue_counts":                "eight counters",
	"aikido_get_issue_reachability":          "call paths are positional nested arrays",
	"aikido_get_code_coverage":               "one coverage summary",
	"aikido_get_custom_rule":                 "the rule YAML is the point of the call",
	"aikido_get_container_registry":          "four fields",
	"aikido_list_cwe":                        "two fields per CWE",
	"aikido_get_package_health":              "a score and summary per package",
	"aikido_get_changelog_summary":           "changes are keyed by version",
	"aikido_get_zen_countries":               "a mode and a country list",
	"aikido_get_zen_threat_lists":            "a short list of threat feeds and their modes",
	"aikido_get_zen_bot_lists":               "a short list of bot feeds and their modes",
	"aikido_list_endpoint_permission_groups": "three fields per group",
	"aikido_list_task_tracking_integrations": "three fields per integration",
	"aikido_get_autofix_settings":            "one flat settings object",
	"aikido_list_autofix_task_issues":        "a list of issue ids",
	"aikido_get_pentest_assessment":          "three fields",
	"aikido_get_pentest_attack_analysis":     "the full analysis (steps, remediation, retests) is the point of the call",
}

func TestFieldCompactionSpecs_Parse(t *testing.T) {
	var file compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &file))
	require.NotEmpty(t, fieldCompactionSpecs)
	assert.Len(t, fieldCompactionSpecs, len(file.Tools), "the loader dropped an entry")
	_, err := compact.Load(compactYAML, compact.Options{Strict: true})
	require.NoError(t, err)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no tool", name)
	}
}

func TestFieldCompactionSpecs_ReadToolsOnly(t *testing.T) {
	for _, ep := range endpoints {
		_, hasSpec := fieldCompactionSpecs[ep.tool]
		_, raw := rawReadTools[ep.tool]
		switch {
		case ep.access != mcp.AccessRead:
			assert.False(t, hasSpec, "mutation tool %s must return its full confirmation", ep.tool)
		case raw:
			assert.False(t, hasSpec, "%s is listed as raw but has a spec", ep.tool)
		default:
			assert.True(t, hasSpec, "read tool %s has no compaction spec and no raw reason", ep.tool)
		}
	}
	for name := range rawReadTools {
		ep, ok := dispatch[name]
		require.True(t, ok, "raw tool %s does not exist", name)
		assert.Equal(t, mcp.AccessRead, ep.access, name)
	}
}

// Paginated tools are wrapped, so their specs must start at the wrapper.
func TestFieldCompactionSpecs_PagedRoots(t *testing.T) {
	roots := map[string]bool{"results": true, "page": true, "per_page": true, "has_next_page": true}
	for name, fields := range fieldCompactionSpecs {
		if dispatch[name].page == nil {
			continue
		}
		for _, field := range fields {
			root, _, _ := strings.Cut(strings.TrimPrefix(field.Spec(), "-"), ".")
			root, _, _ = strings.Cut(root, "[")
			assert.True(t, roots[root], "%s: spec %q does not start at the page wrapper", name, field.Spec())
		}
	}
}

// Several fields of one nested array overwrite each other (see
// docs/field-compaction.md), so a nested array is projected whole or by one
// field.
func TestFieldCompactionSpecs_NoNestedArrayFieldSets(t *testing.T) {
	for name, fields := range fieldCompactionSpecs {
		seen := map[string]string{}
		for _, field := range fields {
			spec := field.Spec()
			if strings.Count(spec, "[]") < 2 {
				continue
			}
			array := spec[:strings.LastIndex(spec, "[]")]
			if previous, dup := seen[array]; dup {
				t.Errorf("%s: %q and %q project the same nested array", name, previous, spec)
			}
			seen[array] = spec
		}
	}
}

func TestMaxBytes_NoCapConfigured(t *testing.T) {
	a := New().(*aikido)
	for name := range fieldCompactionSpecs {
		_, ok := a.MaxBytes(name)
		assert.False(t, ok, "%s declares a max_bytes cap; update this test deliberately", name)
	}
	fields, ok := a.CompactSpec("aikido_list_issue_groups")
	require.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = a.CompactSpec("aikido_ignore_issue")
	assert.False(t, ok)
}

// fixtureArgs fills the required arguments of a tool with plausible values.
func fixtureArgs(ep endpoint) map[string]any {
	args := map[string]any{}
	for _, p := range ep.params {
		if !p.required {
			continue
		}
		switch {
		case len(p.enum) > 0:
			args[p.name] = p.enum[0]
		case p.kind == kindInt:
			args[p.name] = float64(1)
		case p.kind == kindStrings:
			args[p.name] = []any{"a@1.0.0"}
		default:
			args[p.name] = "1"
		}
	}
	return args
}

// documentedResponses reads testdata/responses.json, one response body per
// tool built from the schemas in Aikido's API reference.
func documentedResponses(t *testing.T) map[mcp.ToolName]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "responses.json"))
	require.NoError(t, err)
	var out map[mcp.ToolName]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

func fixtureOutput(t *testing.T, tool mcp.ToolName) any {
	t.Helper()
	body, ok := documentedResponses(t)[tool]
	require.True(t, ok, "testdata/responses.json has no response for %s", tool)
	f := newFake(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	result, err := configured(t, f).Execute(t.Context(), tool, fixtureArgs(dispatch[tool]))
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var out any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	return out
}

func fieldPresent(raw any, field mcp.CompactField) bool {
	source, _, _ := strings.Cut(field.Spec(), ":")
	if !strings.ContainsAny(source, ".[*") {
		object, ok := raw.(map[string]any)
		if ok {
			_, found := object[source]
			return found
		}
		items, ok := raw.([]any)
		if !ok || len(items) == 0 {
			return false
		}
		first, ok := items[0].(map[string]any)
		if !ok {
			return false
		}
		_, found := first[source]
		return found
	}
	return !emptyValue(mcp.CompactAny(raw, []mcp.CompactField{field}))
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, item := range typed {
			if !emptyValue(item) {
				return false
			}
		}
		return true
	case []any:
		for _, item := range typed {
			if !emptyValue(item) {
				return false
			}
		}
		return true
	}
	return false
}

// The responses in testdata/responses.json are built from the response schemas in Aikido's
// API reference, so every spec field must name a documented field; a typo
// would silently strip that field from every response.
func TestFieldCompactionSpecs_MatchDocumentedResponses(t *testing.T) {
	for name, fields := range fieldCompactionSpecs {
		t.Run(string(name), func(t *testing.T) {
			raw := fixtureOutput(t, name)
			for _, field := range fields {
				assert.True(t, fieldPresent(raw, field), "field %q matches nothing in the documented %s response", field.Spec(), name)
			}
			compacted, err := json.Marshal(mcp.CompactAny(raw, fields))
			require.NoError(t, err)
			assert.NotContains(t, []string{"{}", "[]", "null"}, string(compacted))
		})
	}
}

func TestFieldCompaction_KeepsTriageData(t *testing.T) {
	tests := []struct {
		tool   mcp.ToolName
		want   []string
		absent []string
	}{
		{"aikido_list_issue_groups", []string{`"severity":"critical"`, "related_cve_ids", `"locations":[{`, "has_next_page"}, []string{"how_to_fix", "description"}},
		{"aikido_list_issues", []string{"cve_id", "affected_package", "patched_versions", "code_repo_name", "sla_remediate_by"}, []string{"cwe_classes", "original_cvss_severity_score"}},
		{"aikido_get_issue", []string{"how_to_fix", "reachability_status", "ignore_reasons"}, nil},
		{"aikido_list_code_repos", []string{`"name":"sample-name"`, `"labels":[{`, "is_imported"}, nil},
		{"aikido_list_domain_data_leaks", []string{"email", "status"}, []string{"password"}},
		{"aikido_get_zen_event", []string{`"attack":{`, "payload", "ip_address"}, []string{"stack"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			raw := fixtureOutput(t, tt.tool)
			compacted, err := json.Marshal(mcp.CompactAny(raw, fieldCompactionSpecs[tt.tool]))
			require.NoError(t, err)
			for _, want := range tt.want {
				assert.Contains(t, string(compacted), want)
			}
			for _, absent := range tt.absent {
				assert.NotContains(t, string(compacted), absent)
			}
		})
	}
}

// Inside an array a nested path keeps only its leaf name, so
// results[].labels[].name would overwrite results[].name in every row.
func TestFieldCompactionSpecs_NoLeafCollisionsInArrays(t *testing.T) {
	for name, fields := range fieldCompactionSpecs {
		seen := map[string]string{}
		for _, field := range fields {
			array, rest, ok := strings.Cut(field.Spec(), "[].")
			if !ok {
				continue
			}
			rest = strings.ReplaceAll(rest, "[]", "")
			key := array + "/" + rest[strings.LastIndex(rest, ".")+1:]
			if previous, dup := seen[key]; dup {
				t.Errorf("%s: %q and %q both compact to the same key", name, previous, field.Spec())
			}
			seen[key] = field.Spec()
		}
	}
}
