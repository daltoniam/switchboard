package project

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevision_Deterministic(t *testing.T) {
	left := &Definition{Version: "1", Name: "acme", Description: "d"}
	right := &Definition{Version: "1", Name: "acme", Description: "d"}
	rev1, src1, err := HashDefinition(left)
	require.NoError(t, err)
	rev2, src2, err := HashDefinition(right)
	require.NoError(t, err)
	assert.Equal(t, rev1, rev2)
	assert.Equal(t, src1, src2)
	assert.True(t, rev1.Valid())
	assert.True(t, strings.HasPrefix(string(rev1), "sha256:"))
}

func TestRevision_Parse(t *testing.T) {
	ok, err := ParseRevision(Revision("sha256:" + strings.Repeat("ab", 32)))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("ab", 32), ok.DigestHex())
	_, err = ParseRevision(Revision("bad"))
	assert.Error(t, err)
}
