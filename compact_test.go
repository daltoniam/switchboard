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
			name: "nested array extraction has no objectRoot",
			spec: "repo.labels[].name",
			want: CompactField{path: []string{"repo", "labels[]", "name"}, outputKey: "labels", arrayIdx: 1, arrayKey: "labels", childPath: []string{"name"}},
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
			name:  "empty array omitted",
			input: `{"number":1,"labels":[]}`,
			specs: []string{"number", "labels[].name"},
			want:  `{"number":1}`,
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
			name:  "nested array parent navigates to array",
			input: `{"repo":{"labels":[{"name":"bug","color":"red"},{"name":"P1","color":"blue"}]},"id":1}`,
			specs: []string{"id", "repo.labels[].name"},
			want:  `{"id":1,"labels":["bug","P1"]}`,
		},
		{
			name:  "multi-field from nested array parent",
			input: `{"payload":{"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]},"id":1}`,
			specs: []string{"id", "payload.steps[].name", "payload.steps[].conclusion"},
			want:  `{"id":1,"steps":[{"name":"Build","conclusion":"success"},{"name":"Test","conclusion":"failure"}]}`,
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
			name:  "empty array omitted",
			input: `{"number":1,"labels":[],"title":"bug"}`,
			specs: []string{"number", "title", "labels[].name"},
			want:  `{"number":1,"title":"bug"}`,
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
