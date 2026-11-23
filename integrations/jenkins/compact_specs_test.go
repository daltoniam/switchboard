package jenkins

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_Parity(t *testing.T) {
	var sf compact.SpecFile
	if err := yaml.Unmarshal(compactYAML, &sf); err != nil {
		t.Fatal(err)
	}
	if len(sf.Tools) != 3 || len(fieldCompactionSpecs) != len(sf.Tools) {
		t.Fatal("every read must have one compaction spec")
	}
	j := New().(*jenkins)
	for _, tool := range j.Tools() {
		fields, ok := j.CompactSpec(tool.Name)
		if !ok || len(fields) == 0 {
			t.Fatalf("missing fields for %s", tool.Name)
		}
	}
	if _, ok := j.CompactSpec("jenkins_unknown"); ok {
		t.Fatal("unknown tool has compaction spec")
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		if dispatch[name] == nil {
			t.Fatalf("orphan spec %s", name)
		}
	}
}

func TestFieldCompactionSpecs_RealisticShapeParity(t *testing.T) {
	cases := []struct {
		tool mcp.ToolName
		args map[string]any
		body string
	}{
		{"jenkins_list_jobs", map[string]any{"limit": 1}, jobsFixture},
		{"jenkins_get_job", map[string]any{"job_path": []string{"team", "feature/foo"}, "limit": 2}, jobFixture},
		{"jenkins_get_build", map[string]any{"job_path": []string{"team", "feature/foo"}, "build_number": 42}, buildFixture},
	}
	for _, tc := range cases {
		t.Run(string(tc.tool), func(t *testing.T) {
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			result := execute(t, j, tc.tool, tc.args)
			raw := successJSON(t, result)
			fields, ok := j.CompactSpec(tc.tool)
			if !ok {
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
			switch tc.tool {
			case "jenkins_get_build":
				delete(raw, "result")
			case "jenkins_get_job":
				builds := raw["builds"].([]any)
				delete(builds[0].(map[string]any), "result")
			}
			if !reflect.DeepEqual(got, raw) {
				t.Fatalf("compaction changed safe raw/script fields beyond shared null omission: raw=%s compact=%s", result.Data, projected)
			}
			if strings.Contains(string(projected), "PRIVATE") {
				t.Fatal("private provider data leaked")
			}
			for _, field := range fields {
				shapeData := strings.ReplaceAll(result.Data, `"result":null`, `"result":"SUCCESS"`)
				one, err := mcp.CompactJSON([]byte(shapeData), []mcp.CompactField{field})
				if err != nil || string(one) == "{}" || string(one) == "null" {
					t.Fatalf("phantom field %s output=%s err=%v", field.Spec(), one, err)
				}
			}
		})
	}
}
