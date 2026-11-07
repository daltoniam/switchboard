package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/sourcebot"
	"github.com/stretchr/testify/require"
)

const liveSourcebotImage = "ghcr.io/sourcebot-dev/sourcebot:v4.17.4@sha256:228dee568f4f4e71d56f839066e1edc5c258cc4d596a970408d7ef8bea98be41"

const liveSourcebotEncryptionKey = "live-encryption-key-0123456789ab"

// liveSourcebotSeed creates a small git repository and the declarative
// config that indexes it, then starts a stub OpenAI-compatible model on
// loopback so Ask Sourcebot can answer without a real language model.
const liveSourcebotSeed = `set -e
mkdir -p /repos/api/cmd/server /repos/api/internal/billing
cd /repos/api
git init -q -b main
git config user.name "Ada Lovelace"
git config user.email "ada@example.com"
printf '# API\n\nInventory and billing service.\n' > README.md
git add README.md
GIT_AUTHOR_DATE=2026-01-05T10:00:00Z GIT_COMMITTER_DATE=2026-01-05T10:00:00Z git commit -q -m "Start the API service"
cat > internal/billing/invoice.go <<'GO'
package billing

// ComputeInvoiceTotal sums line items.
func ComputeInvoiceTotal(items []int) int {
	total := 0
	for _, item := range items {
		total += item
	}
	return total
}
GO
cat > cmd/server/main.go <<'GO'
package main

import "github.com/acme/api/internal/billing"

func main() {
	_ = billing.ComputeInvoiceTotal([]int{1, 2, 3})
}
GO
git add .
GIT_AUTHOR_DATE=2026-01-06T10:00:00Z GIT_COMMITTER_DATE=2026-01-06T10:00:00Z git commit -q -m "Add invoice totals for billing" -m "Totals include every line item."
git remote add origin https://github.com/acme/api.git
cat > /data/config.json <<'JSON'
{"connections":{"local":{"type":"git","url":"file:///repos/api"}},"models":[{"provider":"openai-compatible","model":"live-model","displayName":"Live Model","baseUrl":"http://127.0.0.1:4010/v1"}]}
JSON
cat > /tmp/stub-model.js <<'JS'
const http = require('http');
const answer = 'ComputeInvoiceTotal in internal/billing/invoice.go sums the invoice line items.';
http.createServer((req, res) => {
  let body = '';
  req.on('data', chunk => { body += chunk; });
  req.on('end', () => {
    const request = JSON.parse(body || '{}');
    const base = { id: 'live', created: 0, model: request.model };
    if (!request.stream) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ...base, object: 'chat.completion', choices: [{ index: 0, message: { role: 'assistant', content: 'Invoice totals' }, finish_reason: 'stop' }], usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 } }));
      return;
    }
    res.writeHead(200, { 'Content-Type': 'text/event-stream' });
    const send = (delta, finish) => res.write('data: ' + JSON.stringify({ ...base, object: 'chat.completion.chunk', choices: [{ index: 0, delta, finish_reason: finish }] }) + '\n\n');
    send({ role: 'assistant', content: answer }, null);
    send({}, 'stop');
    res.end('data: [DONE]\n\n');
  });
}).listen(4010, '127.0.0.1');
JS
node /tmp/stub-model.js &
cd /app
exec /sbin/tini -- ./entrypoint.sh
`

func init() {
	repo := "github.com/acme/api"
	registerLiveSpec(liveSpec{
		Integration: "sourcebot",
		Tier:        liveContainer,
		Adapter:     sourcebot.New(),
		Provision:   liveSourcebotProvision,
		Cases: []liveCase{
			{Tool: "sourcebot_search_code", Args: map[string]any{"query": "ComputeInvoiceTotal", "repos": []any{repo}, "languages": []any{"Go"}}, Expect: []string{"internal/billing/invoice.go", "cmd/server/main.go", "https://github.com/acme/api/blob/main/internal/billing/invoice.go", "func ComputeInvoiceTotal"}},
			{Tool: "sourcebot_list_repos", Args: map[string]any{"query": "acme"}, Expect: []string{repo, "https://github.com/acme/api", "main"}},
			{Tool: "sourcebot_read_file", Args: map[string]any{"repo": repo, "path": "internal/billing/invoice.go", "start_line": 3, "end_line": 4}, Expect: []string{"func ComputeInvoiceTotal(items []int) int {", "ComputeInvoiceTotal sums line items", "totalLines"}},
			{Tool: "sourcebot_list_tree", Args: map[string]any{"repo": repo, "depth": 3}, Expect: []string{"internal/billing/invoice.go", "cmd/server/main.go", "README.md"}},
			{Tool: "sourcebot_list_commits", Args: map[string]any{"repo": repo}, Expect: []string{"Add invoice totals for billing", "Totals include every line item.", "Start the API service", "Ada Lovelace", "ada@example.com"}},
			{Tool: "sourcebot_find_references", Args: map[string]any{"symbol": "ComputeInvoiceTotal", "language": "Go"}, Expect: []string{"cmd/server/main.go", "internal/billing/invoice.go", "billing.ComputeInvoiceTotal"}},
			{Tool: "sourcebot_find_definitions", Args: map[string]any{"symbol": "ComputeInvoiceTotal", "language": "Go", "repo": repo}, Expect: []string{"internal/billing/invoice.go", "func ComputeInvoiceTotal(items []int) int {"}},
			{Tool: "sourcebot_list_language_models", Expect: []string{"openai-compatible", "live-model", "Live Model"}},
			{Tool: "sourcebot_ask_codebase", Args: map[string]any{"query": "How are invoice totals computed?", "repos": []any{repo}}, Expect: []string{"sums the invoice line items", "/chat/", "live-model"}},
		},
	})
}

func liveSourcebotProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveSourcebotImage,
		Env: map[string]string{
			"AUTH_SECRET":                  "live-auth-secret-0123456789abcdef0123456789",
			"AUTH_URL":                     "https://sourcebot.example.com",
			"CONFIG_PATH":                  "/data/config.json",
			"SOURCEBOT_ENCRYPTION_KEY":     liveSourcebotEncryptionKey,
			"SOURCEBOT_TELEMETRY_DISABLED": "true",
		},
		Ports:      []string{"3000/tcp"},
		Entrypoint: "/bin/sh",
		Cmd:        []string{"-c", liveSourcebotSeed},
	})
	host, port := container.Address(t, "3000/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 10 * time.Second}

	liveEventually(t, 3*time.Minute, func() error {
		out, err := liveSourcebotSQL(t, container, `SELECT id FROM "Org" WHERE id = 1`)
		if err == nil && strings.TrimSpace(out) == "" {
			err = fmt.Errorf("organization not created yet")
		}
		return err
	})

	// Least privilege: the key belongs to a MEMBER of the organization, not
	// the owner. Sourcebot stores HMAC-SHA256(SOURCEBOT_ENCRYPTION_KEY, secret)
	// and accepts the key as sourcebot-<secret>.
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	require.NoError(t, err)
	secretHex := hex.EncodeToString(secret)
	mac := hmac.New(sha256.New, []byte(liveSourcebotEncryptionKey))
	mac.Write([]byte(secretHex))
	apiKey := "sourcebot-" + secretHex
	_, err = liveSourcebotSQL(t, container, fmt.Sprintf(`
INSERT INTO "User" (id, name, email, "updatedAt") VALUES ('live-reader', 'Live Reader', 'reader@example.com', now());
INSERT INTO "UserToOrg" ("orgId", "userId", role) VALUES (1, 'live-reader', 'MEMBER');
INSERT INTO "ApiKey" (name, hash, "orgId", "createdById") VALUES ('switchboard-live', '%s', 1, 'live-reader');
`, hex.EncodeToString(mac.Sum(nil))))
	require.NoError(t, err)

	var repoID string
	liveEventually(t, 3*time.Minute, func() error {
		out, err := liveSourcebotSQL(t, container, `SELECT id FROM "Repo" WHERE name = 'github.com/acme/api' AND "indexedAt" IS NOT NULL`)
		if err != nil {
			return err
		}
		if repoID = strings.TrimSpace(out); repoID == "" {
			return fmt.Errorf("repository not indexed yet")
		}
		return nil
	})

	// Sourcebot only links back to the code host (externalWebUrl) for hosted
	// repositories, and indexing one would need network access. Present the
	// local checkout as a GitHub repository, as a GitHub connection would
	// record it, so the live run proves those links reach the agent.
	_, err = liveSourcebotSQL(t, container, fmt.Sprintf(`UPDATE "Repo" SET "webUrl" = 'https://github.com/acme/api', "external_codeHostType" = 'github' WHERE id = %s`, repoID))
	require.NoError(t, err)
	container.Exec(t, "", "sh", "-c", fmt.Sprintf("mkdir -p /data/.sourcebot/repos && ln -sfn /repos/api /data/.sourcebot/repos/%s", repoID))

	liveEventually(t, 2*time.Minute, func() error {
		return liveSourcebotSearchReady(client, baseURL, apiKey)
	})

	return liveProvision{Credentials: map[string]string{
		"base_url": baseURL,
		"api_key":  apiKey,
	}}
}

// liveSourcebotSQL runs SQL against Sourcebot's embedded Postgres and
// returns unaligned tuples. It returns an error instead of failing so it can
// poll while the database starts.
func liveSourcebotSQL(t *testing.T, container *liveDockerContainer, sql string) (string, error) {
	cmd := exec.CommandContext(t.Context(), "docker", "exec", "--interactive", container.id, // #nosec G204 -- fixed test commands
		"psql", "--no-psqlrc", "--tuples-only", "--no-align", "-v", "ON_ERROR_STOP=1", "postgresql://postgres@localhost:5432/sourcebot")
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("psql: %w: %s", err, out)
	}
	return string(out), nil
}

func liveSourcebotSearchReady(client *http.Client, baseURL, apiKey string) error {
	body := strings.NewReader(`{"query":"\"ComputeInvoiceTotal\"","matches":10}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/search", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sourcebot-Api-Key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		Files []struct {
			ExternalWebURL string `json:"externalWebUrl"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("search: status %d: %w", resp.StatusCode, err)
	}
	if len(result.Files) == 0 || result.Files[0].ExternalWebURL == "" {
		return fmt.Errorf("search index not ready")
	}
	return nil
}
