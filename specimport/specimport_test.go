package specimport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

const openAPIFixture = `{
  "openapi": "3.0.0",
  "info": {"title": "Demo API", "description": "test"},
  "servers": [{"url": "https://api.example.com/v1"}],
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "summary": "List users",
        "parameters": [
          {"name": "limit", "in": "query", "description": "max results", "required": false}
        ]
      },
      "post": {
        "operationId": "createUser",
        "summary": "Create a user",
        "requestBody": {"required": true}
      }
    },
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "description": "user id"}
        ]
      },
      "delete": {
        "operationId": "deleteUser",
        "parameters": [
          {"name": "id", "in": "path", "required": true}
        ]
      }
    }
  }
}`

func TestParseOpenAPI(t *testing.T) {
	im, err := Parse(KindOpenAPI, "Demo API", []byte(openAPIFixture), "")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if im.Name != "demo_api" {
		t.Errorf("Name = %q, want demo_api", im.Name)
	}
	if im.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q", im.BaseURL)
	}
	tools := im.Tools()
	if len(tools) != 4 {
		t.Fatalf("got %d tools, want 4: %+v", len(tools), tools)
	}
	byName := map[mcp.ToolName]mcp.ToolDefinition{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}
	if _, ok := byName["demo_api_listusers"]; !ok {
		t.Errorf("missing demo_api_listusers; got %v", byName)
	}
	create := byName["demo_api_createuser"]
	if !strings.HasPrefix(create.Description, "[write]") {
		t.Errorf("createUser should be marked write, desc=%q", create.Description)
	}
	if _, ok := create.Parameters["body"]; !ok {
		t.Errorf("createUser should expose body param")
	}
}

func TestParseOpenAPIEndpointOverride(t *testing.T) {
	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), "https://staging.example.com")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if im.BaseURL != "https://staging.example.com" {
		t.Errorf("override BaseURL = %q", im.BaseURL)
	}
}

func TestParseEmptyAndUnknown(t *testing.T) {
	if _, err := Parse(KindOpenAPI, "x", []byte("   "), ""); err != ErrEmptySpec {
		t.Errorf("empty: got %v, want ErrEmptySpec", err)
	}
	if _, err := Parse("bogus", "x", []byte("{}"), ""); err == nil {
		t.Errorf("unknown kind should error")
	}
	if _, err := Parse(KindOpenAPI, "x", []byte(`{"openapi":"3.0.0","paths":{}}`), ""); err != ErrNoOperations {
		t.Errorf("no ops: got %v, want ErrNoOperations", err)
	}
}

func TestExecuteOpenAPI_GETWithPathAndQuery(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	in := NewIntegration(im)
	if err := in.Configure(context.Background(), mcp.Credentials{credAPIKey: "secret"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	res, err := in.Execute(context.Background(), "demo_getuser", map[string]any{"id": "42"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Data)
	}
	if gotPath != "/users/42" {
		t.Errorf("path = %q, want /users/42", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("auth = %q, want Bearer secret", gotAuth)
	}

	if _, err := in.Execute(context.Background(), "demo_listusers", map[string]any{"limit": 5}); err != nil {
		t.Fatalf("listUsers: %v", err)
	}
	if gotQuery != "limit=5" {
		t.Errorf("query = %q, want limit=5", gotQuery)
	}
}

// TestExecuteSendsUserAgent verifies that every outbound request carries an
// explicit User-Agent. Some upstreams (notably Cloudflare-fronted APIs such as
// curri) reject the default Go user agent as a suspected bot, so the host must
// always set one for imported integrations to reach those endpoints.
func TestExecuteSendsUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	in := NewIntegration(im)
	if err := in.Configure(context.Background(), mcp.Credentials{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if _, err := in.Execute(context.Background(), "demo_getuser", map[string]any{"id": "1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotUA != userAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, userAgent)
	}
}

func TestExecuteOpenAPI_POSTBody(t *testing.T) {
	var gotBody map[string]any
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"new"}`)
	}))
	defer srv.Close()

	im, _ := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), srv.URL)
	in := NewIntegration(im)
	_ = in.Configure(context.Background(), mcp.Credentials{})

	res, err := in.Execute(context.Background(), "demo_createuser", map[string]any{
		"body": map[string]any{"name": "Ada"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("error result: %s", res.Data)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotBody["name"] != "Ada" {
		t.Errorf("body = %v", gotBody)
	}
}

func TestExecuteOpenAPI_MissingRequired(t *testing.T) {
	im, _ := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), "https://api.example.com")
	in := NewIntegration(im)
	_ = in.Configure(context.Background(), mcp.Credentials{})
	res, err := in.Execute(context.Background(), "demo_getuser", map[string]any{})
	if err != nil {
		t.Fatalf("Execute returned go error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Data, "id") {
		t.Errorf("expected missing-required error, got %+v", res)
	}
}

func TestExecuteOpenAPI_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"nope"}`)
	}))
	defer srv.Close()
	im, _ := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), srv.URL)
	in := NewIntegration(im)
	_ = in.Configure(context.Background(), mcp.Credentials{})
	res, err := in.Execute(context.Background(), "demo_getuser", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("Execute go error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Data, "403") {
		t.Errorf("expected 403 error result, got %+v", res)
	}
}

func TestIsWrite(t *testing.T) {
	im, _ := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), "https://x.test")
	in := NewIntegration(im)
	if in.IsWrite("demo_getuser") {
		t.Error("GET should not be write")
	}
	if !in.IsWrite("demo_deleteuser") {
		t.Error("DELETE should be write")
	}
}

func TestConfigureCustomHeaderAndScheme(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Api-Key")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	im, _ := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), srv.URL)
	in := NewIntegration(im)
	if err := in.Configure(context.Background(), mcp.Credentials{
		credAPIKey: "k", credHeader: "X-Api-Key", credScheme: "",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Execute(context.Background(), "demo_getuser", map[string]any{"id": "1"}); err != nil {
		t.Fatal(err)
	}
	if gotHeader != "k" {
		t.Errorf("X-Api-Key = %q, want k (no scheme)", gotHeader)
	}
}

const graphqlFixture = `{
  "data": {
    "__schema": {
      "queryType": {"name": "Query"},
      "mutationType": {"name": "Mutation"},
      "types": [
        {
          "kind": "OBJECT",
          "name": "Query",
          "fields": [
            {
              "name": "user",
              "description": "fetch a user",
              "args": [
                {"name": "id", "type": {"kind": "NON_NULL", "ofType": {"kind": "SCALAR", "name": "ID"}}}
              ],
              "type": {"kind": "OBJECT", "name": "User"}
            }
          ]
        },
        {
          "kind": "OBJECT",
          "name": "Mutation",
          "fields": [
            {
              "name": "createUser",
              "args": [
                {"name": "name", "type": {"kind": "SCALAR", "name": "String"}}
              ],
              "type": {"kind": "OBJECT", "name": "User"}
            }
          ]
        },
        {
          "kind": "OBJECT",
          "name": "User",
          "fields": [
            {"name": "id", "args": [], "type": {"kind": "NON_NULL", "ofType": {"kind": "SCALAR", "name": "ID"}}},
            {"name": "name", "args": [], "type": {"kind": "SCALAR", "name": "String"}},
            {"name": "role", "args": [], "type": {"kind": "ENUM", "name": "Role"}},
            {"name": "manager", "args": [], "type": {"kind": "OBJECT", "name": "User"}},
            {"name": "org", "args": [], "type": {"kind": "OBJECT", "name": "Org"}},
            {"name": "secret", "args": [{"name": "key", "type": {"kind": "NON_NULL", "ofType": {"kind": "SCALAR", "name": "String"}}}], "type": {"kind": "SCALAR", "name": "String"}}
          ]
        },
        {
          "kind": "OBJECT",
          "name": "Org",
          "fields": [
            {"name": "id", "args": [], "type": {"kind": "NON_NULL", "ofType": {"kind": "SCALAR", "name": "ID"}}},
            {"name": "slug", "args": [], "type": {"kind": "SCALAR", "name": "String"}}
          ]
        }
      ]
    }
  }
}`

func TestParseGraphQL(t *testing.T) {
	im, err := Parse(KindGraphQL, "gql", []byte(graphqlFixture), "https://api.example.com/graphql")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tools := im.Tools()
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	in := NewIntegration(im)
	if !in.IsWrite("gql_createuser") {
		t.Error("mutation should be write")
	}
	if in.IsWrite("gql_user") {
		t.Error("query should not be write")
	}
}

func TestParseGraphQLRequiresEndpoint(t *testing.T) {
	if _, err := Parse(KindGraphQL, "gql", []byte(graphqlFixture), ""); err == nil {
		t.Error("graphql import without endpoint should error")
	}
}

func TestExecuteGraphQL(t *testing.T) {
	var gotReq struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_, _ = io.WriteString(w, `{"data":{"user":{"id":"1"}}}`)
	}))
	defer srv.Close()
	im, _ := Parse(KindGraphQL, "gql", []byte(graphqlFixture), srv.URL)
	in := NewIntegration(im)
	_ = in.Configure(context.Background(), mcp.Credentials{credAPIKey: "t"})
	res, err := in.Execute(context.Background(), "gql_user", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("error result: %s", res.Data)
	}
	if !strings.Contains(gotReq.Query, "query($id: ID!)") {
		t.Errorf("query document = %q", gotReq.Query)
	}
	if gotReq.Variables["id"] != "1" {
		t.Errorf("variables = %v", gotReq.Variables)
	}
}

// TestGraphQLSelectionSet verifies the parser synthesizes a default
// selection set for object-returning fields: scalar/enum leaves are selected,
// nested objects recurse one level, cyclic and depth-exhausted references
// fall back to __typename, and fields requiring arguments are skipped.
func TestGraphQLSelectionSet(t *testing.T) {
	im, err := Parse(KindGraphQL, "gql", []byte(graphqlFixture), "https://x.test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	in := NewIntegration(im)

	doc := in.opByTool["gql_user"].gqlDocument
	// The User object must get a selection set with its scalar/enum leaves.
	for _, want := range []string{"user(id: $id)", " id ", " name ", " role "} {
		if !strings.Contains(doc, want) {
			t.Errorf("user document missing %q: %s", want, doc)
		}
	}
	// Nested Org object recurses one level into its own scalars.
	if !strings.Contains(doc, "org { id slug }") {
		t.Errorf("expected nested org selection, got: %s", doc)
	}
	// The self-referential manager (User->User) must be cycle-guarded to
	// __typename rather than recursing forever.
	if !strings.Contains(doc, "manager { __typename }") {
		t.Errorf("expected cyclic manager to reduce to __typename, got: %s", doc)
	}
	// `secret` requires an argument we cannot supply, so it must not appear
	// in the auto-generated selection set.
	if strings.Contains(doc, "secret") {
		t.Errorf("field requiring args should be skipped, got: %s", doc)
	}
}

// TestGraphQLScalarFieldNoSelectionSet verifies a field whose return type is
// already a scalar gets no selection set (which would be a syntax error).
func TestGraphQLScalarFieldNoSelectionSet(t *testing.T) {
	const fixture = `{"data":{"__schema":{
      "queryType":{"name":"Query"},
      "types":[{"kind":"OBJECT","name":"Query","fields":[
        {"name":"version","args":[],"type":{"kind":"SCALAR","name":"String"}}
      ]}]
    }}}`
	im, err := Parse(KindGraphQL, "gql", []byte(fixture), "https://x.test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc := NewIntegration(im).opByTool["gql_version"].gqlDocument
	if doc != "query { version }" {
		t.Errorf("scalar field should have no selection set, got: %q", doc)
	}
}

// TestGraphQLUnknownReturnTypeFallback verifies an object field whose return
// type is not present in the introspected types still gets a valid (non-empty)
// selection set via __typename, rather than an invalid bare field.
func TestGraphQLUnknownReturnTypeFallback(t *testing.T) {
	const fixture = `{"data":{"__schema":{
      "queryType":{"name":"Query"},
      "types":[{"kind":"OBJECT","name":"Query","fields":[
        {"name":"thing","args":[],"type":{"kind":"OBJECT","name":"Missing"}}
      ]}]
    }}}`
	im, err := Parse(KindGraphQL, "gql", []byte(fixture), "https://x.test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc := NewIntegration(im).opByTool["gql_thing"].gqlDocument
	if doc != "query { thing { __typename } }" {
		t.Errorf("unknown object type should fall back to __typename, got: %q", doc)
	}
}

// TestExecuteGraphQLEnvelopeError verifies that a GraphQL response with a
// top-level errors array and null data (the standard failure shape, served
// over HTTP 200) is promoted to an error ToolResult instead of being treated
// as a successful payload.
func TestExecuteGraphQLEnvelopeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HTTP 200 with a GraphQL-level error and null data.
		_, _ = io.WriteString(w, `{"data":null,"errors":[{"message":"Field must have selections"},{"message":"second"}]}`)
	}))
	defer srv.Close()
	im, _ := Parse(KindGraphQL, "gql", []byte(graphqlFixture), srv.URL)
	in := NewIntegration(im)
	_ = in.Configure(context.Background(), mcp.Credentials{credAPIKey: "t"})
	res, err := in.Execute(context.Background(), "gql_user", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("Execute returned go error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result for GraphQL envelope error, got: %s", res.Data)
	}
	if !strings.Contains(res.Data, "Field must have selections") {
		t.Errorf("error should carry the upstream message, got: %s", res.Data)
	}
	if !strings.Contains(res.Data, "and 1 more") {
		t.Errorf("error should note additional errors, got: %s", res.Data)
	}
}

// TestExecuteGraphQLPartialDataKept verifies that a response carrying BOTH
// data and errors (a partial success) is NOT flagged as an error — the
// resolved data is still returned to the caller.
func TestExecuteGraphQLPartialDataKept(t *testing.T) {
	const partial = `{"data":{"user":{"id":"1"}},"errors":[{"message":"field x failed"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, partial)
	}))
	defer srv.Close()
	im, _ := Parse(KindGraphQL, "gql", []byte(graphqlFixture), srv.URL)
	in := NewIntegration(im)
	_ = in.Configure(context.Background(), mcp.Credentials{credAPIKey: "t"})
	res, err := in.Execute(context.Background(), "gql_user", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("partial success should not be an error result: %s", res.Data)
	}
	if res.Data != partial {
		t.Errorf("partial response body should pass through unchanged, got: %s", res.Data)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"Demo API":        "demo_api",
		"  Foo--Bar  ":    "foo_bar",
		"":                "spec",
		"GET /users/{id}": "get_users_id",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
