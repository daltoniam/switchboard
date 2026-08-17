package snowflake

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey generates a throwaway 2048-bit RSA key for tests.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func pemEncodePKCS1(key *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

func pemEncodePKCS8(key *rsa.PrivateKey) string {
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}))
}

func TestParsePrivateKey(t *testing.T) {
	key := testKey(t)

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"PKCS1", pemEncodePKCS1(key), ""},
		{"PKCS8", pemEncodePKCS8(key), ""},
		{"empty", "", "failed to decode PEM"},
		{"garbage", "not-a-pem", "failed to decode PEM"},
		{"wrong block type", string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("x")})), "unsupported PEM block type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parsePrivateKey(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, parsed)
			assert.Equal(t, key.D.Cmp(parsed.D), 0, "parsed key should match original")
		})
	}
}

func TestParsePrivateKey_FlattenedPEM(t *testing.T) {
	key := testKey(t)
	// Simulate what an HTML <input> does: strip all newlines.
	normalPEM := pemEncodePKCS8(key)
	flatPEM := strings.ReplaceAll(normalPEM, "\n", "")

	parsed, err := parsePrivateKey(flatPEM)
	require.NoError(t, err)
	assert.Equal(t, 0, key.D.Cmp(parsed.D), "parsed key should match original")
}

func TestParsePrivateKey_EscapedNewlines(t *testing.T) {
	key := testKey(t)
	// Simulate literal \n in JSON or env var.
	normalPEM := pemEncodePKCS8(key)
	escapedPEM := strings.ReplaceAll(normalPEM, "\n", `\n`)

	parsed, err := parsePrivateKey(escapedPEM)
	require.NoError(t, err)
	assert.Equal(t, 0, key.D.Cmp(parsed.D), "parsed key should match original")
}

func TestPublicKeyFingerprint(t *testing.T) {
	key := testKey(t)

	fp, err := publicKeyFingerprint(key)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(fp, "SHA256:"), "fingerprint should start with SHA256:")

	// Verify it's deterministic.
	fp2, err := publicKeyFingerprint(key)
	require.NoError(t, err)
	assert.Equal(t, fp, fp2)

	// Verify the hash matches manual computation.
	pubDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	hash := sha256.Sum256(pubDER)
	want := "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
	assert.Equal(t, want, fp)
}

func TestAccountLocator(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"xy12345.us-east-1", "XY12345"},
		{"xy12345.us-east-1.aws", "XY12345"},
		{"XY12345", "XY12345"},
		{"myorg-myaccount", "MYORG-MYACCOUNT"},
		{"lower", "LOWER"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, accountLocator(tt.input))
		})
	}
}

func TestGenerateSnowflakeJWT(t *testing.T) {
	key := testKey(t)
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	token, err := generateSnowflakeJWT(key, "xy12345.us-east-1", "MYUSER", now)
	require.NoError(t, err)

	parts := strings.SplitN(token, ".", 3)
	require.Len(t, parts, 3, "JWT should have 3 parts")

	// Verify header.
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var header map[string]string
	require.NoError(t, json.Unmarshal(headerJSON, &header))
	assert.Equal(t, "RS256", header["alg"])
	assert.Equal(t, "JWT", header["typ"])

	// Verify claims.
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(claimsJSON, &claims))

	fp, _ := publicKeyFingerprint(key)
	assert.Equal(t, "XY12345.MYUSER."+fp, claims["iss"])
	assert.Equal(t, "XY12345.MYUSER", claims["sub"])
	assert.Equal(t, float64(now.Unix()), claims["iat"])
	assert.Equal(t, float64(now.Add(time.Hour).Unix()), claims["exp"])

	// Verify signature is valid.
	unsigned := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	hash := sha256.Sum256([]byte(unsigned))
	err = rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sigBytes)
	require.NoError(t, err, "JWT signature should verify")
}

func TestJWTCache_Caching(t *testing.T) {
	key := testKey(t)
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	cache := &jwtCache{nowFunc: func() time.Time { return now }}

	tok1, err := cache.getOrGenerate(key, "acct", "user")
	require.NoError(t, err)

	tok2, err := cache.getOrGenerate(key, "acct", "user")
	require.NoError(t, err)

	assert.Equal(t, tok1, tok2, "same token should be returned within TTL")
}

func TestJWTCache_Refresh(t *testing.T) {
	key := testKey(t)
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	cache := &jwtCache{nowFunc: func() time.Time { return now }}

	tok1, err := cache.getOrGenerate(key, "acct", "user")
	require.NoError(t, err)

	// Advance past the refresh buffer (55 min).
	now = now.Add(56 * time.Minute)
	cache.nowFunc = func() time.Time { return now }

	tok2, err := cache.getOrGenerate(key, "acct", "user")
	require.NoError(t, err)

	assert.NotEqual(t, tok1, tok2, "token should be regenerated after refresh buffer")
}
