package specimport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression coverage for review findings fixed earlier in this PR's
// history that had no test of their own.

func TestSubstitutePathParams_RejectsDotSegmentsAndEscapesOnce(t *testing.T) {
	bindings := []openAPIParamBinding{{arg: "path_id", wire: "id"}}
	for _, bad := range []string{".", "..", "a/b"} {
		_, err := substitutePathParams("/users/{id}", bindings, map[string]any{"path_id": bad})
		assert.Error(t, err, "path value %q must be rejected", bad)
	}
	got, err := substitutePathParams("/users/{id}", bindings, map[string]any{"path_id": "a b%2F"})
	require.NoError(t, err)
	assert.Equal(t, "/users/a%20b%252F", got, "values are escaped exactly once")
}

func TestParseOpenAPI_AcceptsOpenAPI31TypeArrays(t *testing.T) {
	doc := `{"openapi":"3.1.0","info":{"title":"t","version":"1"},
	  "servers":[{"url":"https://api.example.test"}],
	  "paths":{"/items":{"get":{"operationId":"listItems",
	    "parameters":[{"name":"limit","in":"query","schema":{"type":["integer","null"]}}],
	    "responses":{"200":{"description":"ok"}}}}}}`
	im, err := Parse(KindOpenAPI, "demo", []byte(doc), "")
	require.NoError(t, err)
	require.Len(t, im.Tools(), 1)
	assert.Contains(t, im.Tools()[0].Parameters, "query_limit")
}

func TestExecuteOpenAPI_SendsExplicitEmptyJSONBody(t *testing.T) {
	var gotBody string
	var gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(b), r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), srv.URL)
	require.NoError(t, err)
	in := NewIntegration(im)
	require.NoError(t, in.Configure(context.Background(), mcp.Credentials{}))
	var create mcp.ToolName
	for _, tool := range in.Tools() {
		if strings.HasPrefix(tool.Description, "[write]") && strings.Contains(string(tool.Name), "create") {
			create = tool.Name
		}
	}
	require.NotEmpty(t, create)
	res, err := in.Execute(context.Background(), create, map[string]any{"body": map[string]any{}})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Equal(t, "{}", gotBody, "an explicit empty object is sent, not dropped")
	assert.Equal(t, "application/json", gotType)
}

func TestExecuteOpenAPI_RejectsNonObjectBodyInsteadOfDroppingIt(t *testing.T) {
	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), "https://api.example.test")
	require.NoError(t, err)
	in := NewIntegration(im)
	require.NoError(t, in.Configure(context.Background(), mcp.Credentials{}))
	for _, tool := range in.Tools() {
		if strings.HasPrefix(tool.Description, "[write]") && strings.Contains(string(tool.Name), "create") {
			res, err := in.Execute(context.Background(), tool.Name, map[string]any{"body": "hello"})
			require.NoError(t, err)
			require.True(t, res.IsError)
			assert.Contains(t, res.Data, "body must be a JSON object")
			return
		}
	}
	t.Fatal("fixture has no create tool")
}

func TestRequireArgs_AllowsEmptyStrings(t *testing.T) {
	op := &operation{tool: mcp.ToolDefinition{Required: []string{"name"}}}
	assert.NoError(t, requireArgs(op, map[string]any{"name": ""}), "a required string may be empty")
	assert.Error(t, requireArgs(op, map[string]any{}), "a missing required argument is still rejected")
	assert.Error(t, requireArgs(op, map[string]any{"name": nil}), "null is not a value")
}

func TestParseGraphQL_DefaultedNonNullArgumentIsOptional(t *testing.T) {
	doc := `{"data":{"__schema":{
	  "queryType":{"name":"Query"},"mutationType":null,
	  "types":[{"kind":"OBJECT","name":"Query","fields":[
	    {"name":"items","args":[
	      {"name":"first","defaultValue":"10","type":{"kind":"NON_NULL","name":null,"ofType":{"kind":"SCALAR","name":"Int","ofType":null}}},
	      {"name":"owner","defaultValue":null,"type":{"kind":"NON_NULL","name":null,"ofType":{"kind":"SCALAR","name":"String","ofType":null}}}
	    ],"type":{"kind":"SCALAR","name":"String","ofType":null}}]}]}}}`
	im, err := Parse(KindGraphQL, "demo", []byte(doc), "https://api.example.test/graphql")
	require.NoError(t, err)
	require.Len(t, im.Tools(), 1)
	required := im.Tools()[0].Required
	assert.Contains(t, required, "owner")
	assert.NotContains(t, required, "first", "a non-null argument with a default may be omitted")
}

func TestConfigure_GraphQLEndpointKeepsTrailingSlash(t *testing.T) {
	im, err := Parse(KindGraphQL, "demo", []byte(graphqlFixture), "https://api.example.test/graphql/")
	require.NoError(t, err)
	in := NewIntegration(im)
	require.NoError(t, in.Configure(context.Background(), mcp.Credentials{"base_url": "https://other.example.test/graphql/"}))
	assert.Equal(t, "https://other.example.test/graphql/", in.baseURL)
}

func TestGraphQLUnionSelectsMemberFields(t *testing.T) {
	doc := `{"data":{"__schema":{
	  "queryType":{"name":"Query"},"mutationType":null,
	  "types":[
	    {"kind":"OBJECT","name":"Query","fields":[{"name":"search","args":[],"type":{"kind":"UNION","name":"Result","ofType":null}}]},
	    {"kind":"UNION","name":"Result","fields":null,"possibleTypes":[{"kind":"OBJECT","name":"User"},{"kind":"OBJECT","name":"Repo"}]},
	    {"kind":"OBJECT","name":"User","fields":[{"name":"login","args":[],"type":{"kind":"SCALAR","name":"String","ofType":null}}]},
	    {"kind":"OBJECT","name":"Repo","fields":[{"name":"fullName","args":[],"type":{"kind":"SCALAR","name":"String","ofType":null}}]}
	  ]}}}`
	im, err := Parse(KindGraphQL, "demo", []byte(doc), "https://api.example.test/graphql")
	require.NoError(t, err)
	raw, err := json.Marshal(im)
	require.NoError(t, err)
	q := string(raw)
	assert.Contains(t, q, "... on User")
	assert.Contains(t, q, "login")
	assert.Contains(t, q, "... on Repo")
	assert.Contains(t, q, "fullName")
}

// A union's members share one selection budget with the rest of the query,
// so a wide union cannot emit members x fields leaf selections.
func TestGraphQLUnionMembersShareTheSelectionBudget(t *testing.T) {
	const members, fieldsPer = 49, 50
	var types []string
	var possible []string
	for m := 0; m < members; m++ {
		var fields []string
		for f := 0; f < fieldsPer; f++ {
			fields = append(fields, fmt.Sprintf(`{"name":"m%df%d","args":[],"type":{"kind":"SCALAR","name":"String","ofType":null}}`, m, f))
		}
		types = append(types, fmt.Sprintf(`{"kind":"OBJECT","name":"M%d","fields":[%s]}`, m, strings.Join(fields, ",")))
		possible = append(possible, fmt.Sprintf(`{"kind":"OBJECT","name":"M%d"}`, m))
	}
	doc := fmt.Sprintf(`{"data":{"__schema":{"queryType":{"name":"Query"},"mutationType":null,"types":[
	  {"kind":"OBJECT","name":"Query","fields":[{"name":"search","args":[],"type":{"kind":"UNION","name":"Wide","ofType":null}}]},
	  {"kind":"UNION","name":"Wide","fields":null,"possibleTypes":[%s]},
	  %s]}}}`, strings.Join(possible, ","), strings.Join(types, ","))

	im, err := Parse(KindGraphQL, "demo", []byte(doc), "https://api.example.test/graphql")
	require.NoError(t, err)
	require.Len(t, im.operations, 1)
	query := im.operations[0].gqlDocument
	leaves := regexp.MustCompile(`\bm\d+f\d+\b`).FindAllString(query, -1)
	assert.NotEmpty(t, leaves, "members still get fields")
	assert.LessOrEqual(t, len(leaves), maxDocumentSelections, "the whole query shares one leaf budget")
}

// Synthesized __typename and fallback leaves count against the same budget,
// so a union of composite-only members stays bounded too.
func TestGraphQLCompositeOnlyUnionStaysWithinTheBudget(t *testing.T) {
	const members, fieldsPer = 49, 50
	types := []string{`{"kind":"OBJECT","name":"Inner","fields":[{"name":"child","args":[],"type":{"kind":"OBJECT","name":"Inner","ofType":null}}]}`}
	var possible []string
	for m := 0; m < members; m++ {
		var fields []string
		for f := 0; f < fieldsPer; f++ {
			fields = append(fields, fmt.Sprintf(`{"name":"m%df%d","args":[],"type":{"kind":"OBJECT","name":"Inner","ofType":null}}`, m, f))
		}
		types = append(types, fmt.Sprintf(`{"kind":"OBJECT","name":"M%d","fields":[%s]}`, m, strings.Join(fields, ",")))
		possible = append(possible, fmt.Sprintf(`{"kind":"OBJECT","name":"M%d"}`, m))
	}
	doc := fmt.Sprintf(`{"data":{"__schema":{"queryType":{"name":"Query"},"mutationType":null,"types":[
	  {"kind":"OBJECT","name":"Query","fields":[{"name":"search","args":[],"type":{"kind":"UNION","name":"Wide","ofType":null}}]},
	  {"kind":"UNION","name":"Wide","fields":null,"possibleTypes":[%s]},
	  %s]}}}`, strings.Join(possible, ","), strings.Join(types, ","))

	im, err := Parse(KindGraphQL, "demo", []byte(doc), "https://api.example.test/graphql")
	require.NoError(t, err)
	require.Len(t, im.operations, 1)
	query := im.operations[0].gqlDocument
	leaves := strings.Count(query, "__typename")
	assert.Greater(t, leaves, 1, "members still get selections")
	assert.LessOrEqual(t, leaves, maxDocumentSelections, "synthesized leaves share the query budget")
	assert.Equal(t, strings.Count(query, "{"), strings.Count(query, "}"), "the query stays well formed")
}
