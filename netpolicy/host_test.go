package netpolicy

import (
	"context"
	"net"
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
	assert.NoError(t, ValidateUserHTTPURL("https://1.1.1.1/v1"))
	assert.Error(t, ValidateUserHTTPURL("https://127.0.0.1/pets"))
	assert.Error(t, ValidateUserHTTPURL("https://[::1%lo]/pets"))
	assert.Error(t, ValidateUserHTTPURL("ftp://example.com"))
	assert.Error(t, ValidateUserHTTPURL("https://metadata.google.internal/computeMetadata/v1/"))
}

func TestValidateUserHTTPURL_RejectsDisallowedResolution(t *testing.T) {
	orig := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = orig })
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	assert.Error(t, ValidateUserHTTPURL("https://rebind.example.test/"))
}

func TestAllowedIPAddrs_RejectsMixedPublicAndPrivate(t *testing.T) {
	orig := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = orig })
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("8.8.8.8")},
			{IP: net.ParseIP("127.0.0.1")},
		}, nil
	}
	_, err := allowedIPAddrs(context.Background(), "mixed.example.test")
	assert.ErrorIs(t, err, errDisallowedHost)
}
