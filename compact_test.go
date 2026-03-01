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
			name: "nested field",
			spec: "user.login",
			want: CompactField{path: []string{"user", "login"}, outputKey: "user.login", arrayIdx: -1},
		},
		{
			name: "deeply nested field",
			spec: "commit.author.name",
			want: CompactField{path: []string{"commit", "author", "name"}, outputKey: "commit.author.name", arrayIdx: -1},
		},
		{
			name: "array extraction",
			spec: "labels[].name",
			want: CompactField{path: []string{"labels[]", "name"}, outputKey: "labels", arrayIdx: 0, arrayKey: "labels", childPath: []string{"name"}},
		},
		{
			name: "nested array extraction",
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
			name:    "multiple array brackets",
			spec:    "items[].labels[].name",
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
			name:  "deeply nested field",
			input: `{"sha":"abc","commit":{"author":{"name":"Alice","email":"a@b.com","date":"2025-01-01"},"message":"fix"}}`,
			specs: []string{"sha", "commit.author.name", "commit.message"},
			want:  `{"sha":"abc","commit.author.name":"Alice","commit.message":"fix"}`,
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
