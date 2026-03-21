package portutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    int
		wantWrn bool
	}{
		{"unset", "", DefaultPort, false},
		{"valid", "8080", 8080, false},
		{"max valid", "65535", 65535, false},
		{"zero", "0", DefaultPort, true},
		{"exceeds max", "65536", DefaultPort, true},
		{"non-numeric", "abc", DefaultPort, true},
		{"negative", "-1", DefaultPort, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SWITCHBOARD_PORT", tt.env)
			port, warn := FromEnv()
			assert.Equal(t, tt.want, port)
			if tt.wantWrn {
				assert.NotEmpty(t, warn)
			} else {
				assert.Empty(t, warn)
			}
		})
	}
}
