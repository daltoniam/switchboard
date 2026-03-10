package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCompactSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    CompactField
		wantErr bool
	}{
		{
			name: "simple field",
			spec: "title",
			want: CompactField{path: []string{"title"}, outputKey: "title", arrayIdx: -1},
		},
		{
			name: "nested field has objectRoot",
			spec: "user.login",
			want: CompactField{path: []string{"user", "login"}, outputKey: "user.login", arrayIdx: -1, objectRoot: "user"},
		},
		{
			name: "deeply nested field has objectRoot from first segment",
			spec: "commit.author.name",
			want: CompactField{path: []string{"commit", "author", "name"}, outputKey: "commit.author.name", arrayIdx: -1, objectRoot: "commit"},
		},
		{
			name: "array extraction has no objectRoot",
			spec: "labels[].name",
			want: CompactField{path: []string{"labels[]", "name"}, outputKey: "labels", arrayIdx: 0, arrayKey: "labels", childPath: []string{"name"}},
		},
		{
			name: "nested array extraction has objectRoot from parent prefix",
			spec: "repo.labels[].name",
			want: CompactField{path: []string{"repo", "labels[]", "name"}, outputKey: "repo.labels", arrayIdx: 1, arrayKey: "labels", childPath: []string{"name"}, objectRoot: "repo"},
		},
		{
			name:    "empty string",
			spec:    "",
			wantErr: true,
		},
		{
			name:    "trailing dot",
			spec:    "user.",
			wantErr: true,
		},
		{
			name:    "leading dot",
			spec:    ".user",
			wantErr: true,
		},
		{
			name:    "double dot",
			spec:    "user..login",
			wantErr: true,
		},
		{
			name:    "trailing array bracket",
			spec:    "labels[]",
			wantErr: true,
		},
		{
			name: "nested array brackets has no objectRoot",
			spec: "items[].labels[].name",
			want: CompactField{
				path: []string{"items[]", "labels[]", "name"}, outputKey: "items",
				arrayIdx: 0, arrayKey: "items", childPath: []string{"labels[]", "name"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompactSpec(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompactJSON_Object(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string // expected JSON (compared as unmarshalled maps for order independence)
	}{
		{
			name:  "keep simple fields",
			input: `{"number":1,"title":"bug","body":"long text","node_id":"abc123"}`,
			specs: []string{"number", "title"},
			want:  `{"number":1,"title":"bug"}`,
		},
		{
			name:  "extract nested field",
			input: `{"number":1,"user":{"login":"rsc","id":999,"avatar_url":"https://..."}}`,
			specs: []string{"number", "user.login"},
			want:  `{"number":1,"user.login":"rsc"}`,
		},
		{
			name:  "two nested fields sharing root produce nested object",
			input: `{"sha":"abc","commit":{"author":{"name":"Alice","email":"a@b.com","date":"2025-01-01"},"message":"fix"}}`,
			specs: []string{"sha", "commit.author.name", "commit.message"},
			want:  `{"sha":"abc","commit":{"author.name":"Alice","message":"fix"}}`,
		},
		{
			name:  "single nested field stays flat",
			input: `{"number":1,"user":{"login":"alice","id":999}}`,
			specs: []string{"number", "user.login"},
			want:  `{"number":1,"user.login":"alice"}`,
		},
		{
			name:  "three nested fields sharing root all grouped",
			input: `{"id":1,"subject":{"title":"PR Review","type":"PullRequest","url":"https://github.com/foo/bar/pull/1"}}`,
			specs: []string{"id", "subject.title", "subject.type", "subject.url"},
			want:  `{"id":1,"subject":{"title":"PR Review","type":"PullRequest","url":"https://github.com/foo/bar/pull/1"}}`,
		},
		{
			name:  "missing field skipped gracefully",
			input: `{"number":1,"title":"bug"}`,
			specs: []string{"number", "title", "nonexistent"},
			want:  `{"number":1,"title":"bug"}`,
		},
		{
			name:  "missing nested field skipped",
			input: `{"number":1}`,
			specs: []string{"number", "user.login"},
			want:  `{"number":1}`,
		},
		{
			name:  "nil fields passes through unchanged",
			input: `{"number":1,"title":"bug","body":"long"}`,
			specs: nil,
			want:  `{"number":1,"title":"bug","body":"long"}`,
		},
		{
			name:  "empty input returns empty",
			input: ``,
			specs: []string{"number"},
			want:  ``,
		},
		{
			name:  "array field extraction",
			input: `{"number":1,"labels":[{"id":1,"name":"bug","color":"red"},{"id":2,"name":"P1","color":"blue"}]}`,
			specs: []string{"number", "labels[].name"},
			want:  `{"number":1,"labels":["bug","P1"]}`,
		},
		{
			name:  "empty array preserved",
			input: `{"number":1,"labels":[]}`,
			specs: []string{"number", "labels[].name"},
			want:  `{"number":1,"labels":[]}`,
		},
		{
			name:  "nested null value preserved",
			input: `{"number":1,"user":null}`,
			specs: []string{"number", "user.login"},
			want:  `{"number":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fields []CompactField
			if tt.specs != nil {
				var err error
				fields, err = ParseCompactSpecs(tt.specs)
				require.NoError(t, err)
			}

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			if tt.want == "" {
				assert.Empty(t, got)
				return
			}

			// Compare as unmarshalled values for order independence
			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestCompactJSON_InvalidJSON(t *testing.T) {
	fields, err := ParseCompactSpecs([]string{"field"})
	require.NoError(t, err)
	_, err = CompactJSON([]byte(`{invalid`), fields)
	assert.Error(t, err)
}

func TestCompactJSON_Array(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name: "array of objects compacted",
			input: `[
				{"number":1,"title":"bug","body":"long","node_id":"x"},
				{"number":2,"title":"feat","body":"longer","node_id":"y"}
			]`,
			specs: []string{"number", "title"},
			want:  `[{"number":1,"title":"bug"},{"number":2,"title":"feat"}]`,
		},
		{
			name:  "empty array",
			input: `[]`,
			specs: []string{"number"},
			want:  `[]`,
		},
		{
			name: "array with nested extraction",
			input: `[
				{"number":1,"user":{"login":"alice","id":1}},
				{"number":2,"user":{"login":"bob","id":2}}
			]`,
			specs: []string{"number", "user.login"},
			want:  `[{"number":1,"user.login":"alice"},{"number":2,"user.login":"bob"}]`,
		},
		{
			name: "array with array field extraction",
			input: `[
				{"number":1,"labels":[{"name":"bug"},{"name":"P1"}]},
				{"number":2,"labels":[{"name":"feat"}]}
			]`,
			specs: []string{"number", "labels[].name"},
			want:  `[{"number":1,"labels":["bug","P1"]},{"number":2,"labels":["feat"]}]`,
		},
		{
			name:  "nil fields passes array through",
			input: `[{"a":1,"b":2}]`,
			specs: nil,
			want:  `[{"a":1,"b":2}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fields []CompactField
			if tt.specs != nil {
				var err error
				fields, err = ParseCompactSpecs(tt.specs)
				require.NoError(t, err)
			}

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestCompactJSON_MultiFieldArray(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name:  "two fields from same array parent produce sub-objects",
			input: `{"id":1,"steps":[{"name":"Build","conclusion":"success","number":1},{"name":"Test","conclusion":"failure","number":2}]}`,
			specs: []string{"id", "steps[].name", "steps[].conclusion"},
			want:  `{"id":1,"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]}`,
		},
		{
			name:  "single field from array still produces flat scalars",
			input: `{"id":1,"labels":[{"name":"bug","color":"red"},{"name":"P1","color":"blue"}]}`,
			specs: []string{"id", "labels[].name"},
			want:  `{"id":1,"labels":["bug","P1"]}`,
		},
		{
			name:  "nested array parent preserves envelope",
			input: `{"repo":{"labels":[{"name":"bug","color":"red"},{"name":"P1","color":"blue"}]},"id":1}`,
			specs: []string{"id", "repo.labels[].name"},
			want:  `{"id":1,"repo":{"labels":["bug","P1"]}}`,
		},
		{
			name:  "multi-field from nested array parent preserves envelope",
			input: `{"payload":{"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]},"id":1}`,
			specs: []string{"id", "payload.steps[].name", "payload.steps[].conclusion"},
			want:  `{"id":1,"payload":{"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]}}`,
		},
		{
			name:  "multi-field array idempotent",
			input: `{"id":1,"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]}`,
			specs: []string{"id", "steps[].name", "steps[].conclusion"},
			want:  `{"id":1,"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]}`,
		},
		{
			name: "nested array within multi-field group",
			input: `{"total_count":2,"items":[
				{"number":1,"labels":[{"name":"bug"},{"name":"P1"}],"title":"fix it"},
				{"number":2,"labels":[{"name":"feat"}],"title":"add it"}
			]}`,
			specs: []string{"total_count", "items[].number", "items[].title", "items[].labels[].name"},
			want:  `{"total_count":2,"items":[{"number":1,"title":"fix it","labels":["bug","P1"]},{"number":2,"title":"add it","labels":["feat"]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestCompactJSON_NestedArrayEnvelope(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name:  "GraphQL envelope: array + sibling pageInfo under same parent",
			input: `{"issues":{"nodes":[{"id":"1","title":"Bug","priority":3},{"id":"2","title":"Feat","priority":1}],"pageInfo":{"hasNextPage":true,"endCursor":"abc"}}}`,
			specs: []string{"issues.nodes[].id", "issues.nodes[].title", "issues.pageInfo.hasNextPage", "issues.pageInfo.endCursor"},
			want:  `{"issues":{"nodes":[{"id":"1","title":"Bug"},{"id":"2","title":"Feat"}],"pageInfo":{"hasNextPage":true,"endCursor":"abc"}}}`,
		},
		{
			name:  "GraphQL envelope: single array field preserves nesting",
			input: `{"projects":{"nodes":[{"id":"p1","name":"Alpha"},{"id":"p2","name":"Beta"}]}}`,
			specs: []string{"projects.nodes[].id", "projects.nodes[].name"},
			want:  `{"projects":{"nodes":[{"id":"p1","name":"Alpha"},{"id":"p2","name":"Beta"}]}}`,
		},
		{
			name:  "flat array (Category A) unchanged",
			input: `{"Buckets":[{"Name":"logs","CreationDate":"2025-01-01"},{"Name":"data","CreationDate":"2025-02-01"}]}`,
			specs: []string{"Buckets[].Name"},
			want:  `{"Buckets":["logs","data"]}`,
		},
		{
			name:  "nested array with scalar sibling at parent level",
			input: `{"data":{"total":42,"items":[{"id":1,"name":"first"},{"id":2,"name":"second"}]}}`,
			specs: []string{"data.total", "data.items[].id", "data.items[].name"},
			want:  `{"data":{"total":42,"items":[{"id":1,"name":"first"},{"id":2,"name":"second"}]}}`,
		},
		{
			name:  "nested empty array preserved under envelope",
			input: `{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false}}}`,
			specs: []string{"issues.nodes[].id", "issues.pageInfo.hasNextPage"},
			want:  `{"issues":{"nodes":[],"pageInfo.hasNextPage":false}}`,
		},
		{
			name:  "nested array with two pageInfo siblings nest under pageInfo",
			input: `{"issues":{"nodes":[{"id":"1","title":"Bug"}],"pageInfo":{"hasNextPage":true,"endCursor":"abc"}}}`,
			specs: []string{"issues.nodes[].id", "issues.nodes[].title", "issues.pageInfo.hasNextPage", "issues.pageInfo.endCursor"},
			want:  `{"issues":{"nodes":[{"id":"1","title":"Bug"}],"pageInfo":{"hasNextPage":true,"endCursor":"abc"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestCompactJSON_NestedArrayEnvelope_Idempotent(t *testing.T) {
	input := `{"issues":{"nodes":[{"id":"1","title":"Bug","priority":3}],"pageInfo":{"hasNextPage":true,"endCursor":"abc"}}}`
	specs := []string{"issues.nodes[].id", "issues.nodes[].title", "issues.pageInfo.hasNextPage", "issues.pageInfo.endCursor"}

	fields, err := ParseCompactSpecs(specs)
	require.NoError(t, err)

	once, err := CompactJSON([]byte(input), fields)
	require.NoError(t, err)

	twice, err := CompactJSON(once, fields)
	require.NoError(t, err)

	assert.JSONEq(t, string(once), string(twice), "compact(compact(x)) != compact(x)")
}

func TestCompactJSON_LeadingWhitespace(t *testing.T) {
	fields, err := ParseCompactSpecs([]string{"number", "title"})
	require.NoError(t, err)

	input := []byte(`  {"number":1,"title":"bug","body":"long"}`)
	got, err := CompactJSON(input, fields)
	require.NoError(t, err)

	var wantVal, gotVal any
	require.NoError(t, json.Unmarshal([]byte(`{"number":1,"title":"bug"}`), &wantVal))
	require.NoError(t, json.Unmarshal(got, &gotVal))
	assert.Equal(t, wantVal, gotVal)
}

func TestCompactJSON_Idempotent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
	}{
		{
			name:  "nested fields survive second pass",
			input: `{"number":1,"user":{"login":"alice","id":99},"commit":{"author":{"name":"Bob"}}}`,
			specs: []string{"number", "user.login", "commit.author.name"},
		},
		{
			name:  "array extraction survives second pass",
			input: `{"id":1,"labels":[{"id":10,"name":"bug"},{"id":20,"name":"P1"}]}`,
			specs: []string{"id", "labels[].name"},
		},
		{
			name:  "top-level array with nested fields",
			input: `[{"number":1,"user":{"login":"alice"}},{"number":2,"user":{"login":"bob"}}]`,
			specs: []string{"number", "user.login"},
		},
		{
			name:  "mixed simple and nested",
			input: `{"title":"bug","state":"open","user":{"login":"alice"},"labels":[{"name":"P1"},{"name":"bug"}]}`,
			specs: []string{"title", "state", "user.login", "labels[].name"},
		},
		{
			name:  "multi-field array extraction survives second pass",
			input: `{"id":1,"steps":[{"name":"Build","conclusion":"success","number":1},{"name":"Test","conclusion":"failure","number":2}]}`,
			specs: []string{"id", "steps[].name", "steps[].conclusion"},
		},
		{
			name:  "object-grouped nested fields survive second pass",
			input: `{"sha":"abc","commit":{"author":{"name":"Alice"},"message":"fix"}}`,
			specs: []string{"sha", "commit.author.name", "commit.message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			once, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			twice, err := CompactJSON(once, fields)
			require.NoError(t, err)

			assert.JSONEq(t, string(once), string(twice),
				"compact(compact(x)) != compact(x)")
		})
	}
}

func TestParseCompactSpec_Exclusion(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    CompactField
		wantErr bool
	}{
		{
			name: "exclusion field",
			spec: "-body",
			want: CompactField{path: []string{"body"}, outputKey: "body", arrayIdx: -1, exclude: true},
		},
		{
			name: "exclusion nested field",
			spec: "-user.avatar_url",
			want: CompactField{path: []string{"user", "avatar_url"}, outputKey: "user.avatar_url", arrayIdx: -1, objectRoot: "user", exclude: true},
		},
		{
			name:    "exclusion empty after dash",
			spec:    "-",
			wantErr: true,
		},
		{
			name:    "exclusion array not supported",
			spec:    "-labels[].name",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompactSpec(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompactJSON_Exclusion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name:  "exclude single field",
			input: `{"number":1,"title":"bug","body":"long text","node_id":"abc123"}`,
			specs: []string{"-body", "-node_id"},
			want:  `{"number":1,"title":"bug"}`,
		},
		{
			name:  "exclude nested field",
			input: `{"number":1,"user":{"login":"rsc","id":999,"avatar_url":"https://..."}}`,
			specs: []string{"-user"},
			want:  `{"number":1}`,
		},
		{
			name:  "mixed include and exclude",
			input: `{"number":1,"title":"bug","body":"long text","node_id":"abc123","state":"open"}`,
			specs: []string{"number", "title", "state", "-body", "-node_id"},
			want:  `{"number":1,"title":"bug","state":"open"}`,
		},
		{
			name:  "exclude on array of objects",
			input: `[{"number":1,"title":"bug","body":"long"},{"number":2,"title":"feat","body":"longer"}]`,
			specs: []string{"-body"},
			want:  `[{"number":1,"title":"bug"},{"number":2,"title":"feat"}]`,
		},
		{
			name:  "exclude-only mode keeps everything else",
			input: `{"a":1,"b":2,"c":3,"d":4}`,
			specs: []string{"-c", "-d"},
			want:  `{"a":1,"b":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestCompactJSON_OmitEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name:  "null value omitted",
			input: `{"number":1,"title":"bug","body":null}`,
			specs: []string{"number", "title", "body"},
			want:  `{"number":1,"title":"bug"}`,
		},
		{
			name:  "empty array preserved for spec-targeted array groups",
			input: `{"number":1,"labels":[],"title":"bug"}`,
			specs: []string{"number", "title", "labels[].name"},
			want:  `{"number":1,"title":"bug","labels":[]}`,
		},
		{
			name:  "empty string preserved",
			input: `{"number":1,"title":""}`,
			specs: []string{"number", "title"},
			want:  `{"number":1,"title":""}`,
		},
		{
			name:  "non-empty array preserved",
			input: `{"number":1,"labels":[{"name":"bug"}]}`,
			specs: []string{"number", "labels[].name"},
			want:  `{"number":1,"labels":["bug"]}`,
		},
		{
			name:  "object-wrapped empty array preserved",
			input: `{"query":"test","total":0,"matches":[]}`,
			specs: []string{"query", "total", "matches[].text", "matches[].user"},
			want:  `{"query":"test","total":0,"matches":[]}`,
		},
		{
			name:  "empty object omitted",
			input: `{"number":1,"metadata":{}}`,
			specs: []string{"number", "metadata"},
			want:  `{"number":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestCompactJSON_PrecomputedGrouping(t *testing.T) {
	input := `[
		{"sha":"a","commit":{"author":{"name":"Alice"},"message":"fix"}},
		{"sha":"b","commit":{"author":{"name":"Bob"},"message":"feat"}}
	]`
	specs := []string{"sha", "commit.author.name", "commit.message"}
	fields, err := ParseCompactSpecs(specs)
	require.NoError(t, err)

	got, err := CompactJSON([]byte(input), fields)
	require.NoError(t, err)

	want := `[{"sha":"a","commit":{"author.name":"Alice","message":"fix"}},{"sha":"b","commit":{"author.name":"Bob","message":"feat"}}]`
	var wantVal, gotVal any
	require.NoError(t, json.Unmarshal([]byte(want), &wantVal))
	require.NoError(t, json.Unmarshal(got, &gotVal))
	assert.Equal(t, wantVal, gotVal)
}

func TestParseCompactSpec_Rename(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    CompactField
		wantErr bool
	}{
		{
			name: "simple rename",
			spec: "user.login:author",
			want: CompactField{path: []string{"user", "login"}, outputKey: "author", arrayIdx: -1, objectRoot: "user"},
		},
		{
			name: "nested rename",
			spec: "commit.author.name:committer",
			want: CompactField{path: []string{"commit", "author", "name"}, outputKey: "committer", arrayIdx: -1, objectRoot: "commit"},
		},
		{
			name:    "empty alias",
			spec:    "user.login:",
			wantErr: true,
		},
		{
			name:    "alias on exclusion",
			spec:    "-field:alias",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompactSpec(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompactJSON_Rename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name:  "rename nested field",
			input: `{"number":1,"user":{"login":"rsc","id":999}}`,
			specs: []string{"number", "user.login:author"},
			want:  `{"number":1,"author":"rsc"}`,
		},
		{
			name:  "rename simple field",
			input: `{"full_name":"golang/go","stargazers_count":100}`,
			specs: []string{"full_name:name", "stargazers_count:stars"},
			want:  `{"name":"golang/go","stars":100}`,
		},
		{
			name:  "rename in array of objects",
			input: `[{"user":{"login":"a"},"number":1},{"user":{"login":"b"},"number":2}]`,
			specs: []string{"number", "user.login:author"},
			want:  `[{"number":1,"author":"a"},{"number":2,"author":"b"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func TestParseCompactSpec_Wildcard(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    CompactField
		wantErr bool
	}{
		{
			name: "wildcard on nested object",
			spec: "user.*",
			want: CompactField{path: []string{"user", "*"}, outputKey: "user", arrayIdx: -1, objectRoot: "user", wildcard: true},
		},
		{
			name:    "bare wildcard",
			spec:    "*",
			wantErr: true,
		},
		{
			name:    "wildcard not terminal",
			spec:    "user.*.name",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompactSpec(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompactJSON_Wildcard(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{
			name:  "wildcard keeps entire sub-object under parent key",
			input: `{"number":1,"user":{"login":"alice","id":99,"avatar_url":"https://..."}}`,
			specs: []string{"number", "user.*"},
			want:  `{"number":1,"user":{"login":"alice","id":99,"avatar_url":"https://..."}}`,
		},
		{
			name:  "wildcard on missing key skipped",
			input: `{"number":1}`,
			specs: []string{"number", "user.*"},
			want:  `{"number":1}`,
		},
		{
			name:  "wildcard with other specs",
			input: `{"number":1,"user":{"login":"alice","id":99},"labels":[{"name":"bug"}]}`,
			specs: []string{"number", "user.*", "labels[].name"},
			want:  `{"number":1,"user":{"login":"alice","id":99},"labels":["bug"]}`,
		},
		{
			name:  "wildcard on array of objects",
			input: `[{"number":1,"meta":{"a":1,"b":2}},{"number":2,"meta":{"a":3,"b":4}}]`,
			specs: []string{"number", "meta.*"},
			want:  `[{"number":1,"meta":{"a":1,"b":2}},{"number":2,"meta":{"a":3,"b":4}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCompactSpecs(tt.specs)
			require.NoError(t, err)

			got, err := CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)

			var wantVal, gotVal any
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantVal))
			require.NoError(t, json.Unmarshal(got, &gotVal))
			assert.Equal(t, wantVal, gotVal)
		})
	}
}

func BenchmarkCompactJSON_ArrayOfObjects(b *testing.B) {
	input := make([]map[string]any, 100)
	for i := range input {
		input[i] = map[string]any{
			"number": i,
			"title":  "issue title",
			"body":   "long body text that should be stripped",
			"user":   map[string]any{"login": "alice", "id": 999, "avatar_url": "https://..."},
			"labels": []any{map[string]any{"name": "bug", "color": "red"}, map[string]any{"name": "P1", "color": "blue"}},
			"commit": map[string]any{"author": map[string]any{"name": "Alice", "email": "a@b.com"}, "message": "fix"},
		}
	}
	data, _ := json.Marshal(input)
	fields, _ := ParseCompactSpecs([]string{"number", "title", "user.login", "labels[].name", "commit.author.name", "commit.message"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CompactJSON(data, fields)
	}
}

// ---------------------------------------------------------------------------
// Compaction ratio benchmarks — realistic API payloads per integration
// ---------------------------------------------------------------------------
//
// Each sub-benchmark builds a payload that mirrors the shape and noise level
// of a real API response, then reports:
//   - ns/op, B/op, allocs/op  (standard Go bench metrics)
//   - input_bytes              (raw payload size)
//   - output_bytes             (compacted payload size)
//   - savings_pct              (percentage reduction)
//
// Run with:  go test -bench=BenchmarkCompactionRatio -benchmem ./...

var benchSink []byte

func benchCompaction(b *testing.B, name string, payload []byte, specs []string) {
	b.Helper()
	var fields []CompactField
	if specs != nil {
		var err error
		fields, err = ParseCompactSpecs(specs)
		if err != nil {
			b.Fatal(err)
		}
	}

	compacted, err := CompactJSON(payload, fields)
	if err != nil {
		b.Fatal(err)
	}

	inputLen := len(payload)
	outputLen := len(compacted)
	savingsPct := 0
	if inputLen > 0 {
		savingsPct = 100 - 100*outputLen/inputLen
	}

	b.SetBytes(int64(inputLen))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSink, _ = CompactJSON(payload, fields)
	}
	b.StopTimer()
	b.ReportMetric(float64(inputLen), "input_bytes")
	b.ReportMetric(float64(outputLen), "output_bytes")
	b.ReportMetric(float64(savingsPct), "savings_%")
}

func BenchmarkCompactionRatio(b *testing.B) {
	b.Run("GitHubIssues30", func(b *testing.B) {
		items := make([]map[string]any, 30)
		for i := range items {
			items[i] = map[string]any{
				"number":     i + 1,
				"title":      "Issue title for testing compaction ratio measurement",
				"state":      "open",
				"html_url":   "https://github.com/org/repo/issues/42",
				"created_at": "2025-01-15T10:30:00Z",
				"updated_at": "2025-03-01T14:22:00Z",
				"comments":   i * 3,
				"body":       "This is a long issue body with detailed description of the bug, reproduction steps, expected behavior, and actual behavior. It contains markdown formatting and code blocks that are typically verbose.",
				"node_id":    "MDU6SXNzdWUxMjM0NTY3OA==",
				"id":         12345678 + i,
				"url":        "https://api.github.com/repos/org/repo/issues/42",
				"locked":     false,
				"user": map[string]any{
					"login":      "developer",
					"id":         999,
					"avatar_url": "https://avatars.githubusercontent.com/u/999?v=4",
					"url":        "https://api.github.com/users/developer",
					"html_url":   "https://github.com/developer",
					"type":       "User",
					"site_admin": false,
				},
				"labels": []any{
					map[string]any{"id": 100, "name": "bug", "color": "d73a4a", "description": "Something isn't working", "default": true},
					map[string]any{"id": 101, "name": "priority:high", "color": "e11d48", "description": "High priority item", "default": false},
				},
				"assignees": []any{
					map[string]any{"login": "alice", "id": 1001, "avatar_url": "https://avatars.githubusercontent.com/u/1001?v=4", "type": "User"},
				},
				"milestone": map[string]any{
					"title":         "v2.0",
					"id":            50,
					"number":        5,
					"state":         "open",
					"description":   "Second major release milestone with breaking changes and new features",
					"open_issues":   12,
					"closed_issues": 8,
					"created_at":    "2025-01-01T00:00:00Z",
					"due_on":        "2025-06-01T00:00:00Z",
				},
				"reactions": map[string]any{
					"url":         "https://api.github.com/repos/org/repo/issues/1/reactions",
					"total_count": 5,
					"+1":          3,
					"-1":          0,
					"laugh":       1,
					"hooray":      0,
					"confused":    0,
					"heart":       1,
					"rocket":      0,
					"eyes":        0,
				},
				"performed_via_github_app": nil,
				"state_reason":             nil,
			}
		}
		data, _ := json.Marshal(items)
		benchCompaction(b, "GitHubIssues30", data, []string{
			"number", "title", "state", "html_url", "created_at", "updated_at",
			"comments", "user.login", "labels[].name", "assignees[].login", "milestone.title",
		})
	})

	b.Run("DatadogLogs50", func(b *testing.B) {
		items := make([]map[string]any, 50)
		for i := range items {
			items[i] = map[string]any{
				"id":        "AQAAAYx1234567890abcdef" + string(rune('A'+i%26)),
				"type":      "log",
				"status":    "error",
				"service":   "api-gateway",
				"host":      "ip-10-0-1-42.ec2.internal",
				"message":   "Connection refused: upstream service timeout after 30s (request_id=abc-123-def)",
				"timestamp": "2025-03-01T14:22:33.456Z",
				"tags":      []any{"env:production", "team:backend", "version:2.1.0"},
				"attributes": map[string]any{
					"http": map[string]any{
						"method":      "POST",
						"url":         "https://api.example.com/v2/webhooks/process",
						"status_code": 502,
						"useragent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
					},
					"network": map[string]any{
						"client": map[string]any{
							"ip": "203.0.113.42",
						},
						"destination": map[string]any{
							"ip":   "10.0.1.99",
							"port": 8080,
						},
					},
					"error": map[string]any{
						"kind":    "ConnectionRefusedError",
						"message": "ECONNREFUSED 10.0.1.99:8080",
						"stack":   "Error: connect ECONNREFUSED 10.0.1.99:8080\n    at TCPConnectWrap.afterConnect [as oncomplete] (net.js:1141:16)\n    at Protocol._enqueue (/app/node_modules/mysql/lib/protocol/Protocol.js:144:48)\n    at Connection.query (/app/node_modules/mysql/lib/Connection.js:198:25)",
					},
					"custom": map[string]any{
						"request_id":    "abc-123-def",
						"trace_id":      "1234567890abcdef",
						"span_id":       "fedcba0987654321",
						"duration_ms":   30042,
						"retry_count":   3,
						"circuit_state": "open",
					},
				},
			}
		}
		data, _ := json.Marshal(items)
		benchCompaction(b, "DatadogLogs50", data, []string{
			"status", "service", "host", "message", "timestamp", "tags",
		})
	})

	b.Run("LinearIssues25", func(b *testing.B) {
		items := make([]map[string]any, 25)
		for i := range items {
			items[i] = map[string]any{
				"id":            "issue-uuid-" + string(rune('a'+i%26)),
				"identifier":    "ENG-" + string(rune('0'+i%10)) + string(rune('0'+i%10)),
				"title":         "Implement feature flag evaluation for new pricing tiers",
				"priority":      2,
				"priorityLabel": "High",
				"estimate":      3,
				"dueDate":       "2025-04-01",
				"createdAt":     "2025-01-15T10:30:00Z",
				"updatedAt":     "2025-03-01T14:22:00Z",
				"url":           "https://linear.app/team/issue/ENG-123",
				"branchName":    "feature/eng-123-pricing-tiers",
				"description":   "We need to implement feature flag evaluation logic that correctly handles the new pricing tier structure. This includes enterprise, pro, and starter tiers with different feature access levels. The implementation should be backward compatible with existing flags.",
				"number":        123 + i,
				"sortOrder":     -500.5 + float64(i)*10,
				"state": map[string]any{
					"id":       "state-uuid",
					"name":     "In Progress",
					"type":     "started",
					"color":    "#f2c94c",
					"position": 1,
				},
				"assignee": map[string]any{
					"id":          "user-uuid",
					"name":        "Alice Developer",
					"email":       "alice@example.com",
					"displayName": "Alice",
					"avatarUrl":   "https://avatars.linear.app/user-uuid",
					"active":      true,
				},
				"labels": []any{
					map[string]any{"id": "label-1", "name": "feature", "color": "#22c55e"},
					map[string]any{"id": "label-2", "name": "backend", "color": "#3b82f6"},
				},
				"project": map[string]any{
					"id":         "project-uuid",
					"name":       "Q1 Platform Improvements",
					"state":      "started",
					"progress":   0.45,
					"startDate":  "2025-01-01",
					"targetDate": "2025-03-31",
				},
				"cycle": map[string]any{
					"id":       "cycle-uuid",
					"name":     "Sprint 12",
					"number":   12,
					"startsAt": "2025-02-24",
					"endsAt":   "2025-03-07",
				},
				"team": map[string]any{
					"id":   "team-uuid",
					"name": "Engineering",
					"key":  "ENG",
				},
				"creator": map[string]any{
					"id":   "creator-uuid",
					"name": "Bob Manager",
				},
			}
		}
		data, _ := json.Marshal(items)
		benchCompaction(b, "LinearIssues25", data, []string{
			"id", "identifier", "title", "state.name", "state.type", "priority",
			"priorityLabel", "assignee.name", "labels[].name", "createdAt",
			"updatedAt", "dueDate", "estimate", "project.name", "cycle.name",
		})
	})

	b.Run("SentryIssues30", func(b *testing.B) {
		items := make([]map[string]any, 30)
		for i := range items {
			items[i] = map[string]any{
				"id":        "12345" + string(rune('0'+i%10)),
				"shortId":   "PROJ-" + string(rune('A'+i%26)),
				"title":     "TypeError: Cannot read properties of undefined (reading 'map')",
				"level":     "error",
				"status":    "unresolved",
				"count":     "1542",
				"userCount": 237,
				"firstSeen": "2025-01-15T10:30:00Z",
				"lastSeen":  "2025-03-01T14:22:00Z",
				"culprit":   "app/components/Dashboard.jsx in renderItems",
				"permalink": "https://sentry.io/organizations/org/issues/12345/",
				"type":      "error",
				"metadata": map[string]any{
					"type":     "TypeError",
					"value":    "Cannot read properties of undefined (reading 'map')",
					"filename": "app/components/Dashboard.jsx",
					"function": "renderItems",
				},
				"assignedTo": map[string]any{
					"id":    "user-1",
					"name":  "Alice Developer",
					"email": "alice@example.com",
					"type":  "user",
				},
				"project": map[string]any{
					"id":       "project-1",
					"name":     "frontend-app",
					"slug":     "frontend-app",
					"platform": "javascript-react",
				},
				"annotations":   []any{},
				"isBookmarked":  false,
				"isSubscribed":  true,
				"hasSeen":       true,
				"isPublic":      false,
				"numComments":   3,
				"statusDetails": map[string]any{},
				"logger":        "",
				"platform":      "javascript",
				"stats": map[string]any{
					"24h": []any{[]any{1709222400, 42}, []any{1709226000, 38}},
				},
			}
		}
		data, _ := json.Marshal(items)
		benchCompaction(b, "SentryIssues30", data, []string{
			"id", "shortId", "title", "level", "status", "count", "userCount",
			"firstSeen", "lastSeen", "assignedTo.name", "project.slug",
		})
	})

	b.Run("AWSInstances20", func(b *testing.B) {
		instances := make([]map[string]any, 20)
		for i := range instances {
			instances[i] = map[string]any{
				"InstanceId":       "i-0abcdef1234567890",
				"InstanceType":     "m5.xlarge",
				"ImageId":          "ami-0123456789abcdef0",
				"LaunchTime":       "2025-01-15T10:30:00Z",
				"PublicIpAddress":  "54.123.45.67",
				"PrivateIpAddress": "10.0.1.42",
				"SubnetId":         "subnet-0123456789abcdef0",
				"VpcId":            "vpc-0123456789abcdef0",
				"Architecture":     "x86_64",
				"Platform":         "linux",
				"RootDeviceType":   "ebs",
				"RootDeviceName":   "/dev/xvda",
				"State": map[string]any{
					"Code": 16,
					"Name": "running",
				},
				"Placement": map[string]any{
					"AvailabilityZone": "us-east-1a",
					"GroupName":        "",
					"Tenancy":          "default",
				},
				"Monitoring": map[string]any{
					"State": "disabled",
				},
				"SecurityGroups": []any{
					map[string]any{"GroupName": "web-servers", "GroupId": "sg-0123456789abcdef0"},
					map[string]any{"GroupName": "ssh-access", "GroupId": "sg-0fedcba9876543210"},
				},
				"BlockDeviceMappings": []any{
					map[string]any{
						"DeviceName": "/dev/xvda",
						"Ebs": map[string]any{
							"VolumeId":            "vol-0123456789abcdef0",
							"Status":              "attached",
							"AttachTime":          "2025-01-15T10:30:00Z",
							"DeleteOnTermination": true,
						},
					},
				},
				"NetworkInterfaces": []any{
					map[string]any{
						"NetworkInterfaceId": "eni-0123456789abcdef0",
						"SubnetId":           "subnet-0123456789abcdef0",
						"VpcId":              "vpc-0123456789abcdef0",
						"PrivateIpAddress":   "10.0.1.42",
						"Status":             "in-use",
						"Attachment": map[string]any{
							"AttachmentId":        "eni-attach-0123456789abcdef0",
							"DeviceIndex":         0,
							"Status":              "attached",
							"DeleteOnTermination": true,
						},
						"Groups": []any{
							map[string]any{"GroupName": "web-servers", "GroupId": "sg-0123456789abcdef0"},
						},
					},
				},
				"Tags": []any{
					map[string]any{"Key": "Name", "Value": "web-server-" + string(rune('0'+i%10))},
					map[string]any{"Key": "Environment", "Value": "production"},
					map[string]any{"Key": "Team", "Value": "platform"},
				},
				"IamInstanceProfile": map[string]any{
					"Arn": "arn:aws:iam::123456789012:instance-profile/web-server-role",
					"Id":  "AIPAXXXXXXXXXXXXXXXXX",
				},
				"EbsOptimized": true,
				"EnaSupport":   true,
				"Hypervisor":   "nitro",
				"MetadataOptions": map[string]any{
					"State":                   "applied",
					"HttpTokens":              "required",
					"HttpPutResponseHopLimit": 1,
					"HttpEndpoint":            "enabled",
				},
			}
		}
		reservations := []map[string]any{
			{"ReservationId": "r-0123456789abcdef0", "OwnerId": "123456789012", "Instances": instances[:10]},
			{"ReservationId": "r-0fedcba9876543210", "OwnerId": "123456789012", "Instances": instances[10:]},
		}
		data, _ := json.Marshal(map[string]any{"Reservations": reservations})
		benchCompaction(b, "AWSInstances20", data, []string{
			"Reservations[].Instances[].InstanceId",
			"Reservations[].Instances[].InstanceType",
			"Reservations[].Instances[].State.Name",
			"Reservations[].Instances[].PublicIpAddress",
			"Reservations[].Instances[].PrivateIpAddress",
			"Reservations[].Instances[].LaunchTime",
			"Reservations[].Instances[].Tags",
		})
	})

	b.Run("ExclusionMode", func(b *testing.B) {
		items := make([]map[string]any, 30)
		for i := range items {
			items[i] = map[string]any{
				"number":                   i + 1,
				"title":                    "Issue title",
				"state":                    "open",
				"body":                     "Very long body with detailed description, markdown, code blocks, and reproduction steps that inflate the token count significantly in real responses.",
				"node_id":                  "MDU6SXNzdWUxMjM0NTY3OA==",
				"url":                      "https://api.github.com/repos/org/repo/issues/1",
				"repository_url":           "https://api.github.com/repos/org/repo",
				"labels_url":               "https://api.github.com/repos/org/repo/issues/1/labels{/name}",
				"comments_url":             "https://api.github.com/repos/org/repo/issues/1/comments",
				"events_url":               "https://api.github.com/repos/org/repo/issues/1/events",
				"performed_via_github_app": nil,
			}
		}
		data, _ := json.Marshal(items)
		benchCompaction(b, "ExclusionMode", data, []string{
			"-body", "-node_id", "-url", "-repository_url", "-labels_url",
			"-comments_url", "-events_url", "-performed_via_github_app",
		})
	})

	b.Run("SingleObject", func(b *testing.B) {
		item := map[string]any{
			"id":        "12345",
			"shortId":   "PROJ-ABC",
			"title":     "TypeError: Cannot read properties of undefined",
			"level":     "error",
			"status":    "unresolved",
			"count":     "1542",
			"userCount": 237,
			"firstSeen": "2025-01-15T10:30:00Z",
			"lastSeen":  "2025-03-01T14:22:00Z",
			"culprit":   "app/components/Dashboard.jsx in renderItems",
			"permalink": "https://sentry.io/organizations/org/issues/12345/",
			"metadata": map[string]any{
				"type":     "TypeError",
				"value":    "Cannot read properties of undefined",
				"filename": "app/components/Dashboard.jsx",
			},
			"assignedTo": map[string]any{
				"id": "user-1", "name": "Alice", "email": "alice@example.com", "type": "user",
			},
			"project": map[string]any{
				"id": "project-1", "name": "frontend-app", "slug": "frontend-app", "platform": "javascript-react",
			},
			"type":          "error",
			"annotations":   []any{},
			"statusDetails": map[string]any{},
		}
		data, _ := json.Marshal(item)
		benchCompaction(b, "SingleObject", data, []string{
			"id", "shortId", "title", "level", "status", "count", "userCount",
			"firstSeen", "lastSeen", "assignedTo.name", "project.slug",
		})
	})

	b.Run("Passthrough", func(b *testing.B) {
		items := make([]map[string]any, 30)
		for i := range items {
			items[i] = map[string]any{"id": i, "name": "item", "status": "active"}
		}
		data, _ := json.Marshal(items)
		benchCompaction(b, "Passthrough", data, nil)
	})
}
