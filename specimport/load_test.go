package specimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

func TestLoad_InlineOpenAPI(t *testing.T) {
	in, err := Load(mcp.SpecImportConfig{
		Name:    "Demo API",
		Kind:    "openapi",
		Spec:    openAPIFixture,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if in.Name() != "demo_api" {
		t.Errorf("Name = %q, want demo_api", in.Name())
	}
	if len(in.Tools()) != 4 {
		t.Errorf("got %d tools, want 4", len(in.Tools()))
	}
}

func TestLoad_FromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")
	if err := os.WriteFile(path, []byte(openAPIFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := Load(mcp.SpecImportConfig{
		Name:    "Demo API",
		Kind:    "openapi",
		Path:    path,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if in.Name() != "demo_api" {
		t.Errorf("Name = %q, want demo_api", in.Name())
	}
}

func TestLoad_KindCaseInsensitive(t *testing.T) {
	if _, err := Load(mcp.SpecImportConfig{
		Name: "Demo",
		Kind: "OpenAPI",
		Spec: openAPIFixture,
	}); err != nil {
		t.Fatalf("Load with mixed-case kind: %v", err)
	}
}

func TestLoad_EndpointOverrideForGraphQL(t *testing.T) {
	in, err := Load(mcp.SpecImportConfig{
		Name:     "gql",
		Kind:     "graphql",
		Spec:     graphqlFixture,
		Endpoint: "https://api.example.com/graphql",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(in.Tools()) == 0 {
		t.Error("expected graphql tools")
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     mcp.SpecImportConfig
		wantSub string
	}{
		{
			name:    "missing name",
			cfg:     mcp.SpecImportConfig{Kind: "openapi", Spec: openAPIFixture},
			wantSub: "missing a name",
		},
		{
			name:    "no spec or path",
			cfg:     mcp.SpecImportConfig{Name: "x", Kind: "openapi"},
			wantSub: "needs a spec or path",
		},
		{
			name:    "both spec and path",
			cfg:     mcp.SpecImportConfig{Name: "x", Kind: "openapi", Spec: openAPIFixture, Path: "/tmp/x"},
			wantSub: "not both",
		},
		{
			name:    "unknown kind",
			cfg:     mcp.SpecImportConfig{Name: "x", Kind: "soap", Spec: openAPIFixture},
			wantSub: "unknown spec kind",
		},
		{
			name:    "missing file",
			cfg:     mcp.SpecImportConfig{Name: "x", Kind: "openapi", Path: "/no/such/spec.json"},
			wantSub: "read spec file",
		},
		{
			name:    "graphql without endpoint",
			cfg:     mcp.SpecImportConfig{Name: "x", Kind: "graphql", Spec: graphqlFixture},
			wantSub: "endpoint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}
