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
	"fmt"
	"strings"
	"sync"
	"time"
)

// parsePrivateKey decodes a PEM-encoded RSA private key.
// Supports both PKCS#1 (RSA PRIVATE KEY) and PKCS#8 (PRIVATE KEY) formats.
// Handles PEM data where newlines have been stripped (e.g. from a single-line
// HTML input field) by restoring them before decoding.
func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	pemData = normalizePEM(pemData)
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("snowflake: failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("snowflake: PKCS#8 key is not RSA")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("snowflake: unsupported PEM block type %q", block.Type)
	}
}

// publicKeyFingerprint returns the SHA256 fingerprint of the RSA public key
// in the format Snowflake expects: "SHA256:<base64>".
func publicKeyFingerprint(key *rsa.PrivateKey) (string, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", fmt.Errorf("snowflake: marshal public key: %w", err)
	}
	hash := sha256.Sum256(pubDER)
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:]), nil
}

// accountLocator extracts the account locator from a full Snowflake account
// identifier. For "xy12345.us-east-1" it returns "XY12345". If no dots are
// present the full identifier is returned uppercased.
func accountLocator(account string) string {
	if idx := strings.IndexByte(account, '.'); idx >= 0 {
		return strings.ToUpper(account[:idx])
	}
	return strings.ToUpper(account)
}

// generateSnowflakeJWT creates an RS256 JWT for Snowflake key-pair auth.
func generateSnowflakeJWT(key *rsa.PrivateKey, account, user string, now time.Time) (string, error) {
	fp, err := publicKeyFingerprint(key)
	if err != nil {
		return "", err
	}

	locator := accountLocator(account)
	upperUser := strings.ToUpper(user)

	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}

	claims, err := json.Marshal(map[string]any{
		"iss": locator + "." + upperUser + "." + fp,
		"sub": locator + "." + upperUser,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}

	unsigned := base64url(header) + "." + base64url(claims)

	hash := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("snowflake: sign JWT: %w", err)
	}

	return unsigned + "." + base64url(sig), nil
}

// normalizePEM restores newlines in a PEM block that may have been flattened
// into a single line (e.g. by an HTML <input> field stripping newlines on paste).
func normalizePEM(s string) string {
	// If the PEM already contains newlines it's fine as-is.
	if strings.Contains(s, "\n") {
		return s
	}
	// Handle literal \n escape sequences (e.g. from JSON or env vars).
	if strings.Contains(s, `\n`) {
		return strings.ReplaceAll(s, `\n`, "\n")
	}
	// Restore structure: header + base64 body in 64-char lines + footer.
	for _, kind := range []string{"RSA PRIVATE KEY", "PRIVATE KEY"} {
		header := "-----BEGIN " + kind + "-----"
		footer := "-----END " + kind + "-----"
		if strings.HasPrefix(s, header) && strings.HasSuffix(s, footer) {
			body := strings.TrimSpace(s[len(header) : len(s)-len(footer)])
			var lines []string
			lines = append(lines, header)
			for len(body) > 64 {
				lines = append(lines, body[:64])
				body = body[64:]
			}
			if len(body) > 0 {
				lines = append(lines, body)
			}
			lines = append(lines, footer)
			return strings.Join(lines, "\n")
		}
	}
	return s
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// jwtCache caches a generated JWT and regenerates it when close to expiry.
type jwtCache struct {
	mu      sync.Mutex
	token   string
	expires time.Time
	nowFunc func() time.Time // for testing; nil means time.Now
}

const jwtRefreshBuffer = 5 * time.Minute

func (c *jwtCache) now() time.Time {
	if c.nowFunc != nil {
		return c.nowFunc()
	}
	return time.Now()
}

func (c *jwtCache) getOrGenerate(key *rsa.PrivateKey, account, user string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if c.token != "" && now.Before(c.expires.Add(-jwtRefreshBuffer)) {
		return c.token, nil
	}

	token, err := generateSnowflakeJWT(key, account, user, now)
	if err != nil {
		return "", err
	}
	c.token = token
	c.expires = now.Add(time.Hour)
	return token, nil
}
