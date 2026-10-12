package specimport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

func TestMain(m *testing.M) {
	SetAllowPrivateOutboundHostsForTest(true)
	os.Exit(m.Run())
}

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "specimport", name)
}

func TestFixture_OpenAPIPetstoreExecutes(t *testing.T) {
	raw, err := os.ReadFile(testdataPath("petstore_openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"sold":1,"pending":0,"available":3}`)
	}))
	defer srv.Close()

	in, err := Load(mcp.SpecImportConfig{
		Name:     "petstore",
		Kind:     "openapi",
		Spec:     string(raw),
		Endpoint: srv.URL,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(in.Tools()) == 0 {
		t.Fatal("expected tools from petstore spec")
	}
	res, err := in.Execute(context.Background(), "petstore_getinventory", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("error result: %s", res.Data)
	}
	if gotPath != "/store/inventory" {
		t.Errorf("path = %q, want /store/inventory", gotPath)
	}
}

func TestFixture_GraphQLCountriesExecutes(t *testing.T) {
	raw, err := os.ReadFile(testdataPath("countries_graphql.json"))
	if err != nil {
		t.Fatal(err)
	}
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotQuery = body.Query
		_, _ = io.WriteString(w, `{"data":{"continents":[{"code":"AF","name":"Africa"}]}}`)
	}))
	defer srv.Close()

	in, err := Load(mcp.SpecImportConfig{
		Name:     "countries",
		Kind:     "graphql",
		Spec:     string(raw),
		Endpoint: srv.URL,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tool := mcp.ToolName("countries_continents")
	if _, ok := in.opByTool[tool]; !ok {
		t.Fatalf("expected tool %q among %d tools", tool, len(in.Tools()))
	}
	res, err := in.Execute(context.Background(), tool, map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("error result: %s", res.Data)
	}
	if !strings.Contains(gotQuery, "continents") {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestLoad_RejectsPrivateEndpoint(t *testing.T) {
	SetAllowPrivateOutboundHostsForTest(false)
	defer SetAllowPrivateOutboundHostsForTest(true)

	_, err := Load(mcp.SpecImportConfig{
		Name:     "demo",
		Kind:     "openapi",
		Spec:     openAPIFixture,
		Endpoint: "http://127.0.0.1:9999",
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected private host rejection, got %v", err)
	}
}
