package teams

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSearchTerm(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "alice", "alice"},
		{"strips_double_quotes", `foo" OR "mail:secret`, "foo OR mail:secret"},
		{"strips_backslash", `bob\admin`, "bobadmin"},
		{"trims_whitespace", "  carol  ", "carol"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeSearchTerm(tc.in)
			assert.Equal(t, tc.want, got)
			// Result must never contain a bare double-quote, else the OData
			// expression we interpolate it into would break.
			assert.False(t, strings.Contains(got, `"`), "result contains a double-quote")
		})
	}
}
