package mcp

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stringer implements fmt.Stringer for testing ArgStr.
type stringer struct{ s string }

func (s stringer) String() string { return s.s }

func TestArgInt(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    int
		wantErr string
	}{
		{name: "float64", args: map[string]any{"k": float64(42)}, key: "k", want: 42},
		{name: "float64 truncates", args: map[string]any{"k": float64(3.9)}, key: "k", want: 3},
		{name: "int", args: map[string]any{"k": int(7)}, key: "k", want: 7},
		{name: "int64", args: map[string]any{"k": int64(99)}, key: "k", want: 99},
		{name: "json.Number valid", args: map[string]any{"k": json.Number("123")}, key: "k", want: 123},
		{name: "json.Number invalid", args: map[string]any{"k": json.Number("abc")}, key: "k", wantErr: `parameter "k"`},
		{name: "json.Number fractional", args: map[string]any{"k": json.Number("3.14")}, key: "k", wantErr: `parameter "k"`},
		{name: "string valid", args: map[string]any{"k": "55"}, key: "k", want: 55},
		{name: "string invalid", args: map[string]any{"k": "nope"}, key: "k", wantErr: `parameter "k"`},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: 0},
		{name: "missing key", args: map[string]any{}, key: "k", want: 0},
		{name: "nil map", args: nil, key: "k", want: 0},
		{name: "bool wrong type", args: map[string]any{"k": true}, key: "k", wantErr: `parameter "k": cannot convert bool to int`},
		{name: "slice wrong type", args: map[string]any{"k": []string{"a"}}, key: "k", wantErr: `parameter "k": cannot convert []string to int`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgInt(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Equal(t, 0, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestArgInt64(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    int64
		wantErr string
	}{
		{name: "float64", args: map[string]any{"k": float64(42)}, key: "k", want: 42},
		{name: "int", args: map[string]any{"k": int(7)}, key: "k", want: 7},
		{name: "int64", args: map[string]any{"k": int64(99)}, key: "k", want: 99},
		{name: "json.Number valid", args: map[string]any{"k": json.Number("9999999999")}, key: "k", want: 9999999999},
		{name: "json.Number invalid", args: map[string]any{"k": json.Number("abc")}, key: "k", wantErr: `parameter "k"`},
		{name: "string valid", args: map[string]any{"k": "55"}, key: "k", want: 55},
		{name: "string invalid", args: map[string]any{"k": "nope"}, key: "k", wantErr: `parameter "k"`},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: 0},
		{name: "missing key", args: map[string]any{}, key: "k", want: 0},
		{name: "nil map", args: nil, key: "k", want: 0},
		{name: "bool wrong type", args: map[string]any{"k": true}, key: "k", wantErr: `parameter "k": cannot convert bool to int64`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgInt64(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Equal(t, int64(0), got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestArgInt32(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    int32
		wantErr string
	}{
		{name: "float64", args: map[string]any{"k": float64(42)}, key: "k", want: 42},
		{name: "int", args: map[string]any{"k": int(7)}, key: "k", want: 7},
		{name: "int64", args: map[string]any{"k": int64(99)}, key: "k", want: 99},
		{name: "json.Number valid", args: map[string]any{"k": json.Number("123")}, key: "k", want: 123},
		{name: "json.Number invalid", args: map[string]any{"k": json.Number("abc")}, key: "k", wantErr: `parameter "k"`},
		{name: "string valid", args: map[string]any{"k": "55"}, key: "k", want: 55},
		{name: "string invalid", args: map[string]any{"k": "nope"}, key: "k", wantErr: `parameter "k"`},
		{name: "string overflow", args: map[string]any{"k": "3000000000"}, key: "k", wantErr: `parameter "k"`},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: 0},
		{name: "missing key", args: map[string]any{}, key: "k", want: 0},
		{name: "bool wrong type", args: map[string]any{"k": true}, key: "k", wantErr: `parameter "k": cannot convert bool to int32`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgInt32(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Equal(t, int32(0), got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestArgFloat64(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    float64
		wantErr string
	}{
		{name: "float64", args: map[string]any{"k": float64(3.14)}, key: "k", want: 3.14},
		{name: "int", args: map[string]any{"k": int(7)}, key: "k", want: 7.0},
		{name: "int64", args: map[string]any{"k": int64(99)}, key: "k", want: 99.0},
		{name: "json.Number valid", args: map[string]any{"k": json.Number("2.718")}, key: "k", want: 2.718},
		{name: "json.Number invalid", args: map[string]any{"k": json.Number("abc")}, key: "k", wantErr: `parameter "k"`},
		{name: "string valid", args: map[string]any{"k": "1.5"}, key: "k", want: 1.5},
		{name: "string invalid", args: map[string]any{"k": "nope"}, key: "k", wantErr: `parameter "k"`},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: 0},
		{name: "missing key", args: map[string]any{}, key: "k", want: 0},
		{name: "nil map", args: nil, key: "k", want: 0},
		{name: "bool wrong type", args: map[string]any{"k": true}, key: "k", wantErr: `parameter "k": cannot convert bool to float64`},
		{name: "NaN", args: map[string]any{"k": math.NaN()}, key: "k", want: math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgFloat64(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Equal(t, float64(0), got)
			} else {
				require.NoError(t, err)
				if tt.name == "NaN" {
					assert.True(t, math.IsNaN(got), "expected NaN")
				} else {
					assert.InDelta(t, tt.want, got, 0.0001)
				}
			}
		})
	}
}

func TestArgStr(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    string
		wantErr string
	}{
		{name: "string", args: map[string]any{"k": "hello"}, key: "k", want: "hello"},
		{name: "empty string", args: map[string]any{"k": ""}, key: "k", want: ""},
		{name: "fmt.Stringer", args: map[string]any{"k": stringer{"world"}}, key: "k", want: "world"},
		{name: "float64", args: map[string]any{"k": float64(3.14)}, key: "k", want: "3.14"},
		{name: "float64 integer", args: map[string]any{"k": float64(42)}, key: "k", want: "42"},
		{name: "int", args: map[string]any{"k": int(7)}, key: "k", want: "7"},
		{name: "int64", args: map[string]any{"k": int64(99)}, key: "k", want: "99"},
		{name: "json.Number", args: map[string]any{"k": json.Number("123")}, key: "k", want: "123"},
		{name: "bool", args: map[string]any{"k": true}, key: "k", want: "true"},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: ""},
		{name: "missing key", args: map[string]any{}, key: "k", want: ""},
		{name: "nil map", args: nil, key: "k", want: ""},
		{name: "slice wrong type", args: map[string]any{"k": []int{1, 2}}, key: "k", wantErr: `parameter "k": cannot convert []int to string`},
		{name: "map wrong type", args: map[string]any{"k": map[string]any{"a": 1}}, key: "k", wantErr: `parameter "k": cannot convert map[string]interface {} to string`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgStr(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Equal(t, "", got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestArgBool(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    bool
		wantErr string
	}{
		{name: "true", args: map[string]any{"k": true}, key: "k", want: true},
		{name: "false", args: map[string]any{"k": false}, key: "k", want: false},
		{name: "string true", args: map[string]any{"k": "true"}, key: "k", want: true},
		{name: "string false", args: map[string]any{"k": "false"}, key: "k", want: false},
		{name: "string 1", args: map[string]any{"k": "1"}, key: "k", want: true},
		{name: "string 0", args: map[string]any{"k": "0"}, key: "k", want: false},
		{name: "string invalid", args: map[string]any{"k": "nope"}, key: "k", wantErr: `parameter "k"`},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: false},
		{name: "missing key", args: map[string]any{}, key: "k", want: false},
		{name: "nil map", args: nil, key: "k", want: false},
		{name: "int wrong type", args: map[string]any{"k": 42}, key: "k", wantErr: `parameter "k": cannot convert int to bool`},
		{name: "float64 wrong type", args: map[string]any{"k": 1.0}, key: "k", wantErr: `parameter "k": cannot convert float64 to bool`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgBool(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Equal(t, false, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestArgStrSlice(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    []string
		wantErr string
	}{
		{
			name: "[]any with strings",
			args: map[string]any{"k": []any{"a", "b", "c"}},
			key:  "k",
			want: []string{"a", "b", "c"},
		},
		{
			name:    "[]any with non-string",
			args:    map[string]any{"k": []any{"a", 42, "c"}},
			key:     "k",
			wantErr: `parameter "k": element 1 is int, not string`,
		},
		{
			name: "[]any empty",
			args: map[string]any{"k": []any{}},
			key:  "k",
			want: []string{},
		},
		{
			name: "[]string",
			args: map[string]any{"k": []string{"x", "y"}},
			key:  "k",
			want: []string{"x", "y"},
		},
		{
			name: "string comma-separated",
			args: map[string]any{"k": "a,b,c"},
			key:  "k",
			want: []string{"a", "b", "c"},
		},
		{
			name: "string single value",
			args: map[string]any{"k": "solo"},
			key:  "k",
			want: []string{"solo"},
		},
		{
			name: "string empty",
			args: map[string]any{"k": ""},
			key:  "k",
			want: nil,
		},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: nil},
		{name: "missing key", args: map[string]any{}, key: "k", want: nil},
		{name: "nil map", args: nil, key: "k", want: nil},
		{
			name:    "int wrong type",
			args:    map[string]any{"k": 42},
			key:     "k",
			wantErr: `parameter "k": cannot convert int to []string`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgStrSlice(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestArgMap(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    map[string]any
		wantErr string
	}{
		{
			name: "map[string]any",
			args: map[string]any{"k": map[string]any{"a": 1, "b": "two"}},
			key:  "k",
			want: map[string]any{"a": 1, "b": "two"},
		},
		{
			name: "empty map",
			args: map[string]any{"k": map[string]any{}},
			key:  "k",
			want: map[string]any{},
		},
		{name: "nil value", args: map[string]any{"k": nil}, key: "k", want: nil},
		{name: "missing key", args: map[string]any{}, key: "k", want: nil},
		{name: "nil map", args: nil, key: "k", want: nil},
		{
			name:    "string wrong type",
			args:    map[string]any{"k": "nope"},
			key:     "k",
			wantErr: `parameter "k": cannot convert string to map`,
		},
		{
			name:    "int wrong type",
			args:    map[string]any{"k": 42},
			key:     "k",
			wantErr: `parameter "k": cannot convert int to map`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgMap(tt.args, tt.key)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestArgStr_FloatFormatting verifies that float64 integers are formatted without
// trailing ".0" (e.g., "42" not "42.0") since MCP clients often send numbers.
func TestArgStr_FloatFormatting(t *testing.T) {
	tests := []struct {
		name string
		val  float64
		want string
	}{
		{name: "integer float", val: 42.0, want: "42"},
		{name: "fractional float", val: 3.14, want: "3.14"},
		{name: "zero", val: 0.0, want: "0"},
		{name: "negative integer", val: -5.0, want: "-5"},
		{name: "negative fractional", val: -2.5, want: "-2.5"},
		{name: "large integer", val: 1e6, want: "1000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ArgStr(map[string]any{"k": tt.val}, "k")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestArgInt_Overflow verifies that large float64 values that exceed int range
// are still converted without error (Go truncates) — matching existing behavior.
func TestArgInt_Overflow(t *testing.T) {
	_, err := ArgInt(map[string]any{"k": float64(math.MaxFloat64)}, "k")
	require.NoError(t, err, "overflow should not error — Go truncates float64→int, matching existing adapter behavior")
}

// TestArgInt_JsonNumberFromDecoder verifies json.Number values that come from
// json.Decoder with UseNumber enabled, the real-world source of json.Number.
func TestArgInt_JsonNumberFromDecoder(t *testing.T) {
	var args map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"count": 42}`))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&args))

	got, err := ArgInt(args, "count")
	require.NoError(t, err)
	assert.Equal(t, 42, got)
}

// TestArgFloat64_JsonNumberFromDecoder verifies json.Number from a real decoder.
func TestArgFloat64_JsonNumberFromDecoder(t *testing.T) {
	var args map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"temp": 98.6}`))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&args))

	got, err := ArgFloat64(args, "temp")
	require.NoError(t, err)
	assert.InDelta(t, 98.6, got, 0.0001)
}

// --- Args (accumulating reader) tests ---

func TestArgs_HappyPath(t *testing.T) {
	args := map[string]any{
		"owner":       "acme",
		"repo":        "widgets",
		"pull_number": float64(42),
		"draft":       true,
		"labels":      "bug,fix",
	}
	r := NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	draft := r.Bool("draft")
	labels := r.StrSlice("labels")

	require.NoError(t, r.Err())
	assert.Equal(t, "acme", owner)
	assert.Equal(t, "widgets", repo)
	assert.Equal(t, 42, pull)
	assert.True(t, draft)
	assert.Equal(t, []string{"bug", "fix"}, labels)
}

func TestArgs_FirstErrorWins(t *testing.T) {
	args := map[string]any{
		"count":  "not-a-number",
		"second": "also-bad",
	}
	r := NewArgs(args)
	count := r.Int("count")
	second := r.Int("second")

	require.Error(t, r.Err())
	assert.Contains(t, r.Err().Error(), `parameter "count"`)
	assert.Equal(t, 0, count)
	assert.Equal(t, 0, second)
}

func TestArgs_AfterError_ReturnsZero(t *testing.T) {
	args := map[string]any{
		"bad":  []int{1, 2},
		"good": "hello",
	}
	r := NewArgs(args)
	_ = r.Str("bad")      // errors: []int → string
	good := r.Str("good") // should return "" (short-circuit)

	require.Error(t, r.Err())
	assert.Equal(t, "", good, "after error, subsequent calls return zero")
}

func TestArgs_MissingKeys_NoError(t *testing.T) {
	r := NewArgs(map[string]any{})
	s := r.Str("missing")
	n := r.Int("missing")
	b := r.Bool("missing")

	require.NoError(t, r.Err())
	assert.Equal(t, "", s)
	assert.Equal(t, 0, n)
	assert.False(t, b)
}

func TestOptInt(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		def  int
		want int
	}{
		{name: "present positive", args: map[string]any{"p": float64(5)}, key: "p", def: 1, want: 5},
		{name: "present zero", args: map[string]any{"p": float64(0)}, key: "p", def: 1, want: 1},
		{name: "present negative", args: map[string]any{"p": float64(-1)}, key: "p", def: 1, want: 1},
		{name: "missing key", args: map[string]any{}, key: "p", def: 10, want: 10},
		{name: "wrong type", args: map[string]any{"p": []string{"bad"}}, key: "p", def: 42, want: 42},
		{name: "string coercion", args: map[string]any{"p": "7"}, key: "p", def: 1, want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, OptInt(tt.args, tt.key, tt.def))
		})
	}
}

func TestOptInt64(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		def  int64
		want int64
	}{
		{name: "present positive", args: map[string]any{"p": float64(5)}, key: "p", def: 1, want: 5},
		{name: "present zero", args: map[string]any{"p": float64(0)}, key: "p", def: 1, want: 1},
		{name: "missing key", args: map[string]any{}, key: "p", def: 500, want: 500},
		{name: "wrong type", args: map[string]any{"p": true}, key: "p", def: 99, want: 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, OptInt64(tt.args, tt.key, tt.def))
		})
	}
}

// newArgsVarPattern captures the variable name from "X := mcp.NewArgs(args)".
var newArgsVarPattern = regexp.MustCompile(`(\w+)\s*:=\s*mcp\.NewArgs\(args\)`)

// funcBoundary splits source into per-function chunks at "func " at column 0.
var funcBoundary = regexp.MustCompile(`(?m)^func `)

// TestNewArgs_ErrCheckParity walks all adapter packages and verifies that every
// mcp.NewArgs(args) call has at least one corresponding .Err() check in the same
// function. Without the check, the Args reader silently swallows type-coercion errors.
//
// Checks per-function (not per-file) to avoid false passes where one function's
// extra .Err() masks another function's missing check.
func TestNewArgs_ErrCheckParity(t *testing.T) {
	dirs, err := filepath.Glob("integrations/*")
	require.NoError(t, err)
	dirs = append(dirs, "gcp")

	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			files, err := filepath.Glob(filepath.Join(dir, "*.go"))
			require.NoError(t, err)

			for _, f := range files {
				if strings.HasSuffix(f, "_test.go") {
					continue
				}
				data, err := os.ReadFile(f)
				require.NoError(t, err)

				src := string(data)
				if !strings.Contains(src, "mcp.NewArgs(args)") {
					continue
				}

				// Split into per-function chunks and check each independently.
				chunks := funcBoundary.Split(src, -1)
				for _, chunk := range chunks {
					varNames := newArgsVarPattern.FindAllStringSubmatch(chunk, -1)
					if len(varNames) == 0 {
						continue
					}
					newArgs := len(varNames)
					errCheck := 0
					for _, m := range varNames {
						errCheck += strings.Count(chunk, m[1]+".Err()")
					}
					assert.GreaterOrEqual(t, errCheck, newArgs,
						"%s: function has %d mcp.NewArgs but only %d .Err() checks",
						f, newArgs, errCheck)
				}
			}
		})
	}
}

func TestArgs_OptInt(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		def  int
		want int
	}{
		{name: "present positive", args: map[string]any{"p": float64(5)}, key: "p", def: 1, want: 5},
		{name: "present zero", args: map[string]any{"p": float64(0)}, key: "p", def: 1, want: 1},
		{name: "present negative", args: map[string]any{"p": float64(-1)}, key: "p", def: 1, want: 1},
		{name: "missing key", args: map[string]any{}, key: "p", def: 10, want: 10},
		{name: "nil value", args: map[string]any{"p": nil}, key: "p", def: 10, want: 10},
		{name: "wrong type silently defaults", args: map[string]any{"p": []string{"bad"}}, key: "p", def: 42, want: 42},
		{name: "string coercion", args: map[string]any{"p": "7"}, key: "p", def: 1, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewArgs(tt.args)
			got := r.OptInt(tt.key, tt.def)
			assert.Equal(t, tt.want, got)
			// OptInt should never set an error on the reader.
			assert.NoError(t, r.Err())
		})
	}
}

func TestArgs_OptInt_AfterError(t *testing.T) {
	// If the reader already has an error, OptInt returns the default.
	r := NewArgs(map[string]any{"bad": []int{1}})
	_ = r.Str("bad") // sets error: cannot convert []int to string
	require.Error(t, r.Err())

	got := r.OptInt("page", 99)
	assert.Equal(t, 99, got, "OptInt should return default when reader has prior error")
}

func TestArgs_AllTypes(t *testing.T) {
	args := map[string]any{
		"s":   "hello",
		"i":   float64(1),
		"i32": float64(2),
		"i64": float64(3),
		"f":   float64(1.5),
		"b":   true,
		"sl":  []any{"a", "b"},
		"m":   map[string]any{"k": "v"},
	}
	r := NewArgs(args)
	assert.Equal(t, "hello", r.Str("s"))
	assert.Equal(t, 1, r.Int("i"))
	assert.Equal(t, int32(2), r.Int32("i32"))
	assert.Equal(t, int64(3), r.Int64("i64"))
	assert.InDelta(t, 1.5, r.Float64("f"), 0.001)
	assert.True(t, r.Bool("b"))
	assert.Equal(t, []string{"a", "b"}, r.StrSlice("sl"))
	assert.Equal(t, map[string]any{"k": "v"}, r.Map("m"))
	require.NoError(t, r.Err())
}
