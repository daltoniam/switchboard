package specimport

import (
	"encoding/json"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

// Imported tools declare whether they change data, so the rest of
// Switchboard (and MCP clients) can treat writes differently from reads.
func TestImportedTools_DeclareAccess(t *testing.T) {
	tests := []struct {
		name     string
		kind     SpecKind
		doc      string
		endpoint string
	}{
		{"openapi", KindOpenAPI, openAPIFixture, ""},
		{"graphql", KindGraphQL, graphqlFixture, "https://api.example.test/graphql"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			im, err := Parse(tt.kind, "demo", []byte(tt.doc), tt.endpoint)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			check := func(label string, tools []mcp.ToolDefinition) {
				var reads, writes int
				for _, tool := range tools {
					write := strings.HasPrefix(tool.Description, "[write]")
					switch {
					case write && tool.Access != mcp.AccessWrite:
						t.Errorf("%s: %s is a write but declares Access %q", label, tool.Name, tool.Access)
					case !write && tool.Access != mcp.AccessRead:
						t.Errorf("%s: %s is a read but declares Access %q", label, tool.Name, tool.Access)
					}
					if write {
						writes++
					} else {
						reads++
					}
				}
				if reads == 0 || writes == 0 {
					t.Fatalf("%s: fixture must cover reads and writes, got %d reads and %d writes", label, reads, writes)
				}
			}
			check("parsed", im.Tools())

			raw, err := json.Marshal(im)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var back Imported
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			check("rehydrated", back.Tools())
		})
	}
}
