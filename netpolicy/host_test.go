package netpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostDisallowed(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		{"10.0.0.1", true},
		{"192.168.0.1", true},
		{"172.16.0.1", true},
		{"169.254.169.254", true},
		{"::1", true},
		{"[::1]:443", true},
		{"metadata.google.internal", true},
		{"metadata.goog", true},
		{"api.example.com", false},
		{"8.8.8.8", false},
		{"0.0.0.0", true},
		{"0.0.0.1", true},
		{"100.64.0.1", true},
		{"::", true},
		{"fe80::1%eth0", true},
		{"::1%lo", true},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			assert.Equal(t, tc.want, HostDisallowed(tc.host))
		})
	}
}

func TestValidateUserHTTPURL(t *testing.T) {
	assert.NoError(t, ValidateUserHTTPURL("https://api.example.com/v1"))
	assert.Error(t, ValidateUserHTTPURL("https://127.0.0.1/pets"))
	assert.Error(t, ValidateUserHTTPURL("ftp://example.com"))
	assert.Error(t, ValidateUserHTTPURL("https://metadata.google.internal/computeMetadata/v1/"))
}
