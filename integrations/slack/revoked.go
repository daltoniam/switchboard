package slack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// errRevokedCredential is returned instead of sending a request whose
// credential Slack already rejected. See revoked_test.go for why.
var errRevokedCredential = errors.New("slack: Slack rejected these credentials — replace them on the Slack setup page; not resending")

// revokedCredentials records credentials (see credentialKey) that Slack
// answered with a terminal auth error (see terminalAuthErrors).
// Only SHA-256 hashes are kept, persisted at path so the record survives
// restarts and integration reconfigures.
type revokedCredentials struct {
	mu     sync.Mutex
	path   string
	hashes map[string]bool
}

// revokedByPath holds one revokedCredentials per record path. The running
// integration and the setup page's token check must see each other's marks
// at once; separate copies let the running client resend a token the setup
// page just saw rejected.
var (
	revokedByPathMu sync.Mutex
	revokedByPath   = map[string]*revokedCredentials{}
)

// newRevokedCredentials returns the shared record for path, merging in any
// hashes another process wrote to disk since it was last read.
func newRevokedCredentials(path string) (*revokedCredentials, error) {
	var list []string
	data, err := os.ReadFile(path) // #nosec G304 -- path is derived from the user's home dir, not input
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("slack: cannot read revoked-credential record %s: %w", path, err)
	default:
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("slack: revoked-credential record %s is corrupt (%v) — fix or delete it; Slack stays off until then so dead credentials are not resent", path, err)
		}
	}

	revokedByPathMu.Lock()
	r, ok := revokedByPath[path]
	if !ok {
		r = &revokedCredentials{path: path, hashes: map[string]bool{}}
		revokedByPath[path] = r
	}
	revokedByPathMu.Unlock()

	r.mu.Lock()
	for _, h := range list {
		r.hashes[h] = true
	}
	r.mu.Unlock()
	return r, nil
}

func hashCredential(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (r *revokedCredentials) isRevoked(key string) bool {
	if r == nil || key == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hashes[hashCredential(key)]
}

// revokedFileMu serializes read-merge-write of the record across every
// revokedCredentials instance in this process (server and setup page each build one).
var revokedFileMu sync.Mutex

func (r *revokedCredentials) mark(key string) {
	if r == nil || key == "" {
		return
	}
	// r.mu guards only the in-memory map, so isRevoked never waits on disk I/O.
	r.mu.Lock()
	r.hashes[hashCredential(key)] = true
	r.mu.Unlock()

	revokedFileMu.Lock()
	defer revokedFileMu.Unlock()
	var onDisk []string
	if data, err := os.ReadFile(r.path); err == nil { // #nosec G304 G703 -- path is fixed at Configure from the home dir
		if err := json.Unmarshal(data, &onDisk); err != nil {
			// Startup refuses a corrupt record; overwriting it here would
			// silently drop every hash it held.
			log.Printf("slack: revoked-credential record %s is corrupt (%v) — not overwriting; fix or delete it", r.path, err)
			return
		}
	}
	r.mu.Lock()
	for _, h := range onDisk {
		r.hashes[h] = true
	}
	list := make([]string, 0, len(r.hashes))
	for k := range r.hashes {
		list = append(list, k)
	}
	r.mu.Unlock()
	sort.Strings(list)
	data, _ := json.Marshal(list)
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil { // #nosec G703 -- path is fixed at Configure from the home dir; response data only selects the hash
		log.Printf("slack: could not persist revoked credential record: %v", err)
		return
	}
	if err := os.Rename(tmp, r.path); err != nil { // #nosec G703 -- same fixed path as above
		log.Printf("slack: could not persist revoked credential record: %v", err)
	}
}

// terminalAuthErrors are Slack error codes meaning the credential is dead.
var terminalAuthErrors = map[string]bool{
	"invalid_auth":     true,
	"token_revoked":    true,
	"token_expired":    true,
	"account_inactive": true,
	"not_authed":       true,
}

// revokedFilePath is the one home of the revoked-credential record's location.
func revokedFilePath() string {
	return filepath.Join(filepath.Dir(tokenFilePath()), ".slack-mcp-revoked.json")
}

// UserTokenRevoked reports whether a token is already on the revoked record,
// so the web UI can refuse it without sending it to Slack.
func UserTokenRevoked(token string) (bool, error) {
	r, err := newRevokedCredentials(revokedFilePath())
	if err != nil {
		return false, err
	}
	return r.isRevoked(token), nil
}

// markIfInvalidAuth inspects a Slack API JSON response and records the
// credential when Slack reports a terminal auth error. The body is restored for the caller.
func (r *revokedCredentials) markIfInvalidAuth(key string, resp *http.Response) {
	if r == nil || key == "" || resp == nil || resp.Body == nil {
		return
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return
	}
	var envelope struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && terminalAuthErrors[envelope.Error] {
		log.Printf("slack: %s — marking credential revoked; it will not be sent again", envelope.Error)
		r.mark(key)
	}
}
