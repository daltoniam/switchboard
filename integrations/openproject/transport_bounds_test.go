package openproject

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigureRejectsMalformedPEMBeforeValidCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	valid := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	for _, malformed := range []string{
		"-----BEGIN CERTIFICATE-----\ninvalid\n-----END CERTIFICATE-----\n",
		"-----BEGIN CERTIFICATE-----\ninvalid\n",
		"-----BEGIN PRIVATE KEY-----\ninvalid\n-----END PRIVATE KEY-----\n",
	} {
		t.Run(malformed, func(t *testing.T) {
			creds := credentials(server.URL)
			creds["ca_cert"] = malformed + valid
			if err := New().Configure(context.Background(), creds); err == nil {
				t.Fatal("malformed PEM content before a valid certificate was skipped")
			}
		})
	}
}

func TestConfigureRejectsUnbracketedIPv6(t *testing.T) {
	for _, base := range []string{"https://::1:443", "https://2001:db8::1:443", "https://[example.com]:443"} {
		t.Run(base, func(t *testing.T) {
			if err := New().Configure(context.Background(), credentials(base)); err == nil {
				t.Fatal("ambiguous or invalid bracketed hostname accepted")
			}
		})
	}
}

func TestHTTPRejectsInvalidUTF8WithoutReplacement(t *testing.T) {
	invalid := strings.Replace(workPackageFixture, "Fix search bug", "invalid\xfftitle", 1)
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, invalid)
	})
	result := execute(t, integration, "openproject_get_work_package", map[string]any{"id": 42})
	if !result.IsError {
		t.Fatal("invalid UTF-8 source data was silently replaced")
	}
	assertSafe(t, result.Data)
}
