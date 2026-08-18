package project

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevision_DeterministicJCS(t *testing.T) {
	left := &Definition{
		Version: "1",
		Name:    "acme",
		Resources: map[string]Resource{
			"main": {Type: ResourceTypeRepo, Path: "/tmp/repo"},
		},
		Additional: map[string]json.RawMessage{
			"zField": json.RawMessage(`{"b":2,"a":1}`),
			"aField": json.RawMessage(`"keep"`),
		},
	}
	right := &Definition{
		Version: "1",
		Name:    "acme",
		Resources: map[string]Resource{
			"main": {Type: ResourceTypeRepo, Path: "/tmp/repo"},
		},
		Additional: map[string]json.RawMessage{
			"aField": json.RawMessage(`"keep"`),
			"zField": json.RawMessage(`{"a":1,"b":2}`),
		},
	}

	rev1, src1, err := HashDefinition(left)
	require.NoError(t, err)
	rev2, src2, err := HashDefinition(right)
	require.NoError(t, err)
	assert.Equal(t, rev1, rev2)
	assert.Equal(t, src1, src2)
	assert.True(t, rev1.Valid())
	assert.True(t, strings.HasPrefix(string(rev1), "sha256:"))
	assert.Len(t, strings.TrimPrefix(string(rev1), "sha256:"), 64)
	assert.Equal(t, strings.ToLower(string(rev1)), string(rev1))
}

func TestRevision_NumericSpellingsCanonicalize(t *testing.T) {
	one, err := CanonicalJSON(map[string]any{"n": 1})
	require.NoError(t, err)
	onePointZero, err := CanonicalJSON(json.RawMessage(`{"n":1.0}`))
	require.NoError(t, err)
	assert.Equal(t, one, onePointZero)
}

func TestRevision_RejectsNonJCSValues(t *testing.T) {
	_, err := CanonicalJSON(map[string]any{"n": json.Number("NaN")})
	assert.Error(t, err)
}

func TestRevision_IgnoresProvenanceInHash(t *testing.T) {
	def := &Definition{Version: "1", Name: "acme", Resources: map[string]Resource{}}
	revA, srcA, err := HashDefinition(def)
	require.NoError(t, err)
	revB, srcB, err := HashDefinition(def)
	require.NoError(t, err)
	assert.Equal(t, revA, revB)
	assert.Equal(t, srcA, srcB)
}

func TestRevision_Parse(t *testing.T) {
	ok, err := ParseRevision(Revision("sha256:" + strings.Repeat("ab", 32)))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("ab", 32), ok.DigestHex())

	_, err = ParseRevision(Revision(strings.Repeat("ab", 32)))
	assert.Error(t, err)
	_, err = ParseRevision(Revision("sha256:xyz"))
	assert.Error(t, err)
}

func TestRawSourceRevision_ExactBytes(t *testing.T) {
	a := []byte(`{"version":1`)
	b := []byte(`{"version": 1`)
	assert.NotEqual(t, RawSourceRevision(a), RawSourceRevision(b))
	assert.True(t, strings.HasPrefix(string(RawSourceRevision(a)), "sha256:"))
}
