package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssembleManifest_Empty(t *testing.T) {
	def := &Definition{Version: "1", Name: "x"}
	assert.Empty(t, AssembleManifest(def, t.TempDir()))
}

func TestGuessMIME(t *testing.T) {
	assert.Equal(t, "text/markdown", GuessMIME("a.md"))
	assert.Equal(t, "application/json", GuessMIME("a.json"))
}
