package openproject

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

func TestFieldCompactionSpecs_Parity(t *testing.T) {
	integration := New()
	if len(integration.Tools()) != 2 || len(fieldCompactionSpecs) != 2 {
		t.Fatal("exactly two read tools and compaction specs required")
	}
	compactor, ok := integration.(mcp.FieldCompactionIntegration)
	if !ok {
		t.Fatal("read tools require field compaction")
	}
	for _, tool := range integration.Tools() {
		fields, found := compactor.CompactSpec(tool.Name)
		if !found || len(fields) == 0 {
			t.Fatalf("missing read-tool compaction for %s", tool.Name)
		}
		seen := map[string]bool{}
		for _, field := range fields {
			if field.Spec() == "" || seen[field.Spec()] {
				t.Fatalf("empty or duplicate spec for %s: %s", tool.Name, field.Spec())
			}
			seen[field.Spec()] = true
		}
	}
	if _, found := compactor.CompactSpec("openproject_unknown"); found {
		t.Fatal("unknown tool has compaction spec")
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	compactor, ok := New().(mcp.FieldCompactionIntegration)
	if !ok {
		t.Fatal("read tools require field compaction")
	}
	for name := range fieldCompactionSpecs {
		if dispatch[name] == nil {
			t.Fatalf("orphan compaction spec %s has no read handler", name)
		}
		if fields, found := compactor.CompactSpec(name); !found || len(fields) == 0 {
			t.Fatalf("spec %s not exposed by public compaction API", name)
		}
	}
}

func TestFieldCompactionSpecs_RealisticShapeParity(t *testing.T) {
	cases := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
		body string
	}{
		{"list with continuation", "openproject_list_work_packages", map[string]any{"limit": 1}, collectionFixture(2, 1, 1, 1, workPackageFixture)},
		{"empty list", "openproject_list_work_packages", nil, collectionFixture(0, 0, 1, 20)},
		{"exact detail", "openproject_get_work_package", map[string]any{"id": 42}, workPackageFixture},
		{"nullable detail dates", "openproject_get_work_package", map[string]any{"id": 42}, strings.Replace(workPackageFixture, `"startDate":"2026-10-16","dueDate":"2026-10-20"`, `"startDate":null,"dueDate":null`, 1)},
		{"omitted foreign project", "openproject_list_work_packages", map[string]any{"limit": 1}, collectionFixture(2, 1, 1, 1, strings.Replace(workPackageFixture, "/openproject/api/v3/projects/7", "/openproject/api/v3/projects/8", 1))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, tc.body)
			})
			result := execute(t, integration, tc.tool, tc.args)
			raw := successJSON(t, result)
			compactor, ok := integration.(mcp.FieldCompactionIntegration)
			if !ok {
				t.Fatal("read tools require field compaction")
			}
			fields, found := compactor.CompactSpec(tc.tool)
			if !found {
				t.Fatal("missing compaction spec")
			}
			projected, err := mcp.CompactJSON([]byte(result.Data), fields)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(projected, &got); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"next_page", "startDate", "dueDate"} {
				if raw[key] == nil {
					delete(raw, key)
				}
			}
			if !reflect.DeepEqual(got, raw) {
				t.Fatalf("compaction changed safe read fields beyond shared null omission: raw=%s compact=%s", result.Data, projected)
			}
			assertSafe(t, result.Data)
			assertSafe(t, string(projected))
		})
	}
}

func TestFieldCompactionSpecs_EveryFieldMatchesReadShape(t *testing.T) {
	cases := []struct {
		tool mcp.ToolName
		body map[string]any
	}{
		{"openproject_list_work_packages", listJSON(2, 1, 1, 1, 1, 0, float64(2), summaryJSON())},
		{"openproject_get_work_package", detailJSON()},
	}
	for _, tc := range cases {
		t.Run(string(tc.tool), func(t *testing.T) {
			compactor, ok := New().(mcp.FieldCompactionIntegration)
			if !ok {
				t.Fatal("read tools require field compaction")
			}
			fields, found := compactor.CompactSpec(tc.tool)
			if !found || len(fields) == 0 {
				t.Fatal("missing compaction spec")
			}
			data, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range fields {
				projected, err := mcp.CompactJSON(data, []mcp.CompactField{field})
				if err != nil || string(projected) == "{}" || string(projected) == "null" {
					t.Fatalf("phantom compaction field %s: output=%s err=%v", field.Spec(), projected, err)
				}
			}
		})
	}
}
