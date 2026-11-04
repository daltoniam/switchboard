package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// Live acceptance tests exercise an integration's tools against a real
// instance of the product, configured through the real setup UI and called
// through the real MCP endpoint. Each integration registers a liveSpec from
// web/live_<name>_test.go. TestLiveSpecs_Coverage runs on every go test and
// keeps specs honest; TestLive only runs under make test-live.

// liveTier says where a spec's provider comes from, which bounds what it may run.
type liveTier string

const (
	// liveContainer providers are disposable containers started by the test,
	// so every tool, including writes, may run.
	liveContainer liveTier = "container"
	// liveSandbox providers are shared vendor sandbox accounts reached with
	// stored credentials, so only read tools may run.
	liveSandbox liveTier = "sandbox"
)

// liveCase is one MCP execute call and what its response must contain.
type liveCase struct {
	Tool   mcp.ToolName
	Args   map[string]any
	Expect []string
}

type liveSpec struct {
	Integration string
	Tier        liveTier
	// Adapter is a fresh instance from the integration constructor; its tool
	// list and compaction specs are what the spec must cover.
	Adapter mcp.Integration
	// Provision starts or reaches the provider, seeds known data, and returns
	// what the setup page and cases need.
	Provision func(t *testing.T) liveProvision
	Cases     []liveCase
	// Skips lists tools that cannot run live, with the reason.
	Skips map[mcp.ToolName]string
}

// liveProvision is what a provider hands back once it is ready.
type liveProvision struct {
	// Credentials are entered on the setup page, keyed by credential name
	// (the form field is #cred_<name>).
	Credentials map[string]string
	// Values fill ${name} placeholders in case Args and Expect, for data only
	// known after seeding, such as generated IDs or today's date.
	Values map[string]string
}

var liveSpecs = map[string]liveSpec{}

func registerLiveSpec(spec liveSpec) {
	if _, exists := liveSpecs[spec.Integration]; exists {
		panic("duplicate live spec for " + spec.Integration)
	}
	liveSpecs[spec.Integration] = spec
}

// liveSpecProblems reports every way a spec fails to prove its integration.
func liveSpecProblems(spec liveSpec) []string {
	var problems []string
	if spec.Integration == "" {
		problems = append(problems, "integration name is empty")
	}
	if spec.Tier != liveContainer && spec.Tier != liveSandbox {
		problems = append(problems, fmt.Sprintf("unknown tier %q", spec.Tier))
	}
	if spec.Provision == nil {
		problems = append(problems, "no Provision function")
	}
	var tools []mcp.ToolDefinition
	if spec.Adapter != nil {
		tools = spec.Adapter.Tools()
	}
	if len(tools) == 0 {
		problems = append(problems, "no tools")
	}
	if len(spec.Cases) == 0 {
		problems = append(problems, "no cases")
	}
	byName := make(map[mcp.ToolName]mcp.ToolDefinition, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
		if !tool.Access.Declared() {
			problems = append(problems, fmt.Sprintf("%s does not declare Access (read, write or destructive)", tool.Name))
		}
	}
	covered := map[mcp.ToolName]bool{}
	for _, c := range spec.Cases {
		tool, ok := byName[c.Tool]
		if !ok {
			problems = append(problems, fmt.Sprintf("case calls unknown tool %s", c.Tool))
			continue
		}
		covered[c.Tool] = true
		if len(c.Expect) == 0 {
			problems = append(problems, fmt.Sprintf("case for %s asserts nothing; add Expect values from seeded data", c.Tool))
		}
		if spec.Tier == liveSandbox && tool.Access != mcp.AccessRead {
			problems = append(problems, fmt.Sprintf("%s is %q but sandbox specs may only run read tools", c.Tool, tool.Access))
		}
	}
	for name, reason := range spec.Skips {
		if _, ok := byName[name]; !ok {
			problems = append(problems, fmt.Sprintf("skip names unknown tool %s", name))
		}
		if strings.TrimSpace(reason) == "" {
			problems = append(problems, fmt.Sprintf("skip for %s has no reason", name))
		}
		if covered[name] {
			problems = append(problems, fmt.Sprintf("%s is both tested and skipped", name))
		}
	}
	for _, tool := range tools {
		if _, skipped := spec.Skips[tool.Name]; !covered[tool.Name] && !skipped {
			problems = append(problems, fmt.Sprintf("%s has no live case and no skip reason", tool.Name))
		}
	}
	slices.Sort(problems)
	return problems
}

func TestLiveSpecProblems(t *testing.T) {
	provision := func(*testing.T) liveProvision { return liveProvision{} }
	read := mcp.ToolDefinition{Name: "demo_list", Access: mcp.AccessRead}
	write := mcp.ToolDefinition{Name: "demo_create", Access: mcp.AccessWrite}
	valid := func() liveSpec {
		return liveSpec{
			Integration: "demo",
			Tier:        liveContainer,
			Adapter:     &mockIntegration{name: "demo", tools: []mcp.ToolDefinition{read, write}},
			Provision:   provision,
			Cases:       []liveCase{{Tool: "demo_list", Expect: []string{"seeded"}}},
			Skips:       map[mcp.ToolName]string{"demo_create": "needs a paid plan"},
		}
	}
	tests := []struct {
		name   string
		mutate func(*liveSpec)
		want   string
	}{
		{"valid", func(*liveSpec) {}, ""},
		{"unknown tier", func(s *liveSpec) { s.Tier = "staging" }, `unknown tier "staging"`},
		{"no provision", func(s *liveSpec) { s.Provision = nil }, "no Provision function"},
		{"no cases", func(s *liveSpec) { s.Cases = nil; s.Skips = nil }, "no cases"},
		{"undeclared access", func(s *liveSpec) { s.Adapter.(*mockIntegration).tools[0].Access = "" }, "demo_list does not declare Access"},
		{"uncovered tool", func(s *liveSpec) { s.Skips = nil }, "demo_create has no live case and no skip reason"},
		{"empty skip reason", func(s *liveSpec) { s.Skips["demo_create"] = " " }, "skip for demo_create has no reason"},
		{"unknown case tool", func(s *liveSpec) { s.Cases = append(s.Cases, liveCase{Tool: "demo_nope", Expect: []string{"x"}}) }, "case calls unknown tool demo_nope"},
		{"unknown skip tool", func(s *liveSpec) { s.Skips["demo_nope"] = "gone" }, "skip names unknown tool demo_nope"},
		{"tested and skipped", func(s *liveSpec) { s.Skips["demo_list"] = "flaky" }, "demo_list is both tested and skipped"},
		{"no expectations", func(s *liveSpec) { s.Cases[0].Expect = nil }, "case for demo_list asserts nothing"},
		{"write in sandbox", func(s *liveSpec) {
			s.Tier = liveSandbox
			s.Skips = nil
			s.Cases = append(s.Cases, liveCase{Tool: "demo_create", Expect: []string{"id"}})
		}, `demo_create is "write" but sandbox specs may only run read tools`},
		{"write in container", func(s *liveSpec) {
			s.Skips = nil
			s.Cases = append(s.Cases, liveCase{Tool: "demo_create", Expect: []string{"id"}})
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := valid()
			tt.mutate(&spec)
			problems := liveSpecProblems(spec)
			if tt.want == "" {
				require.Empty(t, problems)
				return
			}
			require.NotEmpty(t, problems)
			require.Contains(t, strings.Join(problems, "\n"), tt.want)
		})
	}
}

func TestLiveFieldPresent(t *testing.T) {
	raw := []any{
		map[string]any{"index_name": "orders_pkey", "tablespace": nil, "owner": map[string]any{"login": "ada"}},
		map[string]any{"index_name": "orders_customer_idx", "labels": []any{map[string]any{"name": "hot"}}},
	}
	tests := []struct {
		spec string
		want bool
	}{
		{"index_name", true},
		{"tablespace", true},
		{"indexname", false},
		{"index_name:name", true},
		{"owner.login", true},
		{"owner.email", false},
		{"labels[].name", true},
		{"labels[].color", false},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			fields, err := mcp.ParseCompactSpecs([]string{tt.spec})
			require.NoError(t, err)
			require.Equal(t, tt.want, liveFieldPresent(raw, fields[0]))
		})
	}
}

func TestLiveExpandCases(t *testing.T) {
	cases := []liveCase{{
		Tool:   "demo_get",
		Args:   map[string]any{"id": "${user_id}", "max": 5, "from": "${today}"},
		Expect: []string{"${user_id}", "ada"},
	}}
	got := liveExpandCases(t, cases, map[string]string{"user_id": "u-42", "today": "2026-11-04"})
	require.Equal(t, map[string]any{"id": "u-42", "max": 5, "from": "2026-11-04"}, got[0].Args)
	require.Equal(t, []string{"u-42", "ada"}, got[0].Expect)
	require.Equal(t, "${user_id}", cases[0].Args["id"], "expansion must not mutate the registered spec")
}

func TestLiveDecodeScript(t *testing.T) {
	value := liveDecodeScript(t, "demo_list", `"[{\"id\":1}]"pinned as $3`)
	require.Equal(t, []any{map[string]any{"id": float64(1)}}, value)
	value = liveDecodeScript(t, "demo_list", `{"id":2}`)
	require.Equal(t, map[string]any{"id": float64(2)}, value)
}

// TestLiveSpecs_Coverage keeps every registered spec complete without
// needing Docker or credentials, so gaps fail ordinary CI.
func TestLiveSpecs_Coverage(t *testing.T) {
	require.NotEmpty(t, liveSpecs)
	for name, spec := range liveSpecs {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, name, spec.Integration)
			require.Empty(t, liveSpecProblems(spec))
		})
	}
}

// TestLive runs each spec end to end. make test-live INTEGRATION=<name>
// selects one with -run '^TestLive$/^<name>$'.
func TestLive(t *testing.T) {
	if os.Getenv("SWITCHBOARD_LIVE") != "1" {
		t.Skip("run make test-live to exercise integrations against real providers")
	}
	require.NotEmpty(t, os.Getenv("SWITCHBOARD_UI_BINARY"), "make test-live builds the development binary")
	for name, spec := range liveSpecs {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, liveSpecProblems(spec))
			runLiveSpec(t, spec)
		})
	}
}

type liveEvidence struct {
	Integration             string             `json:"integration"`
	Tier                    liveTier           `json:"tier"`
	Cases                   []liveCaseEvidence `json:"cases"`
	Skips                   map[string]string  `json:"skips,omitempty"`
	Script                  bool               `json:"script_verified"`
	CompactionFieldsChecked int                `json:"compaction_fields_checked"`
	Persisted               bool               `json:"persisted_across_restart"`
	Screenshots             []string           `json:"screenshots"`
}

// liveCaseEvidence records outcomes and sizes only; response bodies can hold
// provider data and never leave the runner.
type liveCaseEvidence struct {
	Tool   mcp.ToolName   `json:"tool"`
	Access mcp.ToolAccess `json:"access"`
	Passed bool           `json:"passed"`
	Bytes  int            `json:"bytes"`
}

func runLiveSpec(t *testing.T, spec liveSpec) {
	provision := spec.Provision(t)
	require.NotEmpty(t, provision.Credentials, "Provision must return setup credentials")
	cases := liveExpandCases(t, spec.Cases, provision.Values)
	evidence := liveEvidence{Integration: spec.Integration, Tier: spec.Tier, Skips: map[string]string{}}
	for name, reason := range spec.Skips {
		evidence.Skips[string(name)] = reason
	}
	t.Cleanup(func() { liveWriteEvidence(t, evidence) })

	app := newUIApp(t)
	if spec.Tier == liveContainer {
		// Container credentials are disposable, so the server log is safe to
		// show; sandbox runs never print it.
		t.Cleanup(func() {
			if t.Failed() {
				liveServerLog(t, app)
			}
		})
	}
	page := uiPage(t)
	liveConfigure(t, page, app, spec.Integration, provision.Credentials)
	for _, shot := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 900}, {"mobile", 390, 844}} {
		require.NoError(t, page.SetViewportSize(shot.width, shot.height))
		name := fmt.Sprintf("live-%s-%s", spec.Integration, shot.name)
		uiCapture(t, page, name)
		evidence.Screenshots = append(evidence.Screenshots, name+".png")
	}
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/"+spec.Integration)
	uiText(t, page.Locator(".badge").First(), "Connected")
	evidence.Persisted = true

	session := liveSession(t, app)
	tools := spec.Adapter.Tools()
	access := make(map[mcp.ToolName]mcp.ToolAccess, len(tools))
	for _, tool := range tools {
		access[tool.Name] = tool.Access
	}
	discovered := map[mcp.ToolName]bool{}
	for _, c := range cases {
		if !discovered[c.Tool] {
			names := liveSearch(t, session, spec.Integration, string(c.Tool))
			require.Contains(t, names, string(c.Tool), "search must find %s once the integration is configured", c.Tool)
			discovered[c.Tool] = true
		}
		record := liveCaseEvidence{Tool: c.Tool, Access: access[c.Tool]}
		text, isError := liveCall(t, session, "execute", map[string]any{"tool_name": string(c.Tool), "arguments": liveArgs(c.Args)})
		record.Bytes = len(text)
		record.Passed = !isError && liveContainsAll(text, c.Expect)
		evidence.Cases = append(evidence.Cases, record)
		if !record.Passed {
			t.Errorf("%s: error=%v, expected %q in response:\n%s", c.Tool, isError, c.Expect, liveTruncate(text))
		}
	}

	// Scripts return raw tool output without compaction. Each read case runs
	// again as a script, which proves the raw path and lets every compaction
	// field be checked against what the provider really returns; a field
	// that matches nothing silently strips data from every LLM response.
	compactor, _ := spec.Adapter.(mcp.FieldCompactionIntegration)
	for _, c := range cases {
		if access[c.Tool] != mcp.AccessRead {
			continue
		}
		args, err := json.Marshal(liveArgs(c.Args))
		require.NoError(t, err)
		// JSON.stringify keeps the script engine from columnarizing the result.
		script := fmt.Sprintf("JSON.stringify(api.call(%q, %s))", c.Tool, args)
		text, isError := liveCall(t, session, "execute", map[string]any{"script": script})
		if isError {
			t.Errorf("script call to %s failed:\n%s", c.Tool, liveTruncate(text))
			continue
		}
		raw := liveDecodeScript(t, c.Tool, text)
		rawText, err := json.Marshal(raw)
		require.NoError(t, err)
		if !liveContainsAll(string(rawText), c.Expect) {
			t.Errorf("script call to %s: expected %q in raw response:\n%s", c.Tool, c.Expect, liveTruncate(string(rawText)))
			continue
		}
		evidence.Script = true
		if compactor == nil {
			continue
		}
		fields, ok := compactor.CompactSpec(c.Tool)
		if !ok {
			continue
		}
		for _, field := range fields {
			if field.Exclusion() {
				continue
			}
			if !liveFieldPresent(raw, field) {
				t.Errorf("compaction field %q for %s matches nothing in the live response; raw keys: %v", field.Spec(), c.Tool, liveKeys(raw))
				continue
			}
			evidence.CompactionFieldsChecked++
		}
	}
	require.True(t, evidence.Script, "at least one read case must succeed through a script")
}

// liveExpandCases fills ${name} placeholders in string args and expectations.
func liveExpandCases(t *testing.T, cases []liveCase, values map[string]string) []liveCase {
	t.Helper()
	pairs := make([]string, 0, 2*len(values))
	for key, value := range values {
		pairs = append(pairs, "${"+key+"}", value)
	}
	replacer := strings.NewReplacer(pairs...)
	expand := func(text string) string {
		expanded := replacer.Replace(text)
		require.NotContains(t, expanded, "${", "unresolved placeholder in %q; add it to Provision Values", text)
		return expanded
	}
	expanded := make([]liveCase, 0, len(cases))
	for _, c := range cases {
		next := liveCase{Tool: c.Tool, Args: make(map[string]any, len(c.Args))}
		for key, value := range c.Args {
			if text, ok := value.(string); ok {
				value = expand(text)
			}
			next.Args[key] = value
		}
		for _, want := range c.Expect {
			next.Expect = append(next.Expect, expand(want))
		}
		expanded = append(expanded, next)
	}
	return expanded
}

// liveDecodeScript parses a script result that returned JSON.stringify output.
// The execute response may append a pin note after the JSON value.
func liveDecodeScript(t *testing.T, tool mcp.ToolName, text string) any {
	t.Helper()
	var value any
	require.NoError(t, json.NewDecoder(strings.NewReader(text)).Decode(&value), "script output for %s", tool)
	if encoded, ok := value.(string); ok {
		require.NoError(t, json.Unmarshal([]byte(encoded), &value), "stringified script output for %s", tool)
	}
	return value
}

// liveFieldPresent reports whether a compaction field matches the raw
// response. Plain top-level keys count when present even if null, since
// compaction drops nulls and fresh fixtures often have them; paths fall back
// to running the field through the compactor.
func liveFieldPresent(raw any, field mcp.CompactField) bool {
	source, _, _ := strings.Cut(field.Spec(), ":")
	if !strings.ContainsAny(source, ".[*") {
		return slices.Contains(liveKeys(raw), source)
	}
	return !liveEmpty(mcp.CompactAny(raw, []mcp.CompactField{field}))
}

// liveEmpty reports whether compaction kept no values, only empty containers.
func liveEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, item := range typed {
			if !liveEmpty(item) {
				return false
			}
		}
		return true
	case []any:
		for _, item := range typed {
			if !liveEmpty(item) {
				return false
			}
		}
		return true
	}
	return false
}

func liveKeys(value any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key := range typed {
				seen[key] = true
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// liveConfigure enters credentials on the integration's setup page, enables
// it, saves, and requires the health check against the real provider to pass.
func liveConfigure(t *testing.T, page playwright.Page, app *uiApp, integration string, credentials map[string]string) {
	t.Helper()
	uiNavigate(t, page, app.URL+"/integrations/"+integration)
	keys := make([]string, 0, len(credentials))
	for key := range credentials {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		field := page.Locator("#cred_" + key)
		count, err := field.Count()
		require.NoError(t, err)
		require.Equal(t, 1, count, "setup page must render exactly one #cred_%s field", key)
		require.NoError(t, field.Fill(credentials[key]))
	}
	enabled := page.Locator(`input[type="checkbox"][name="enabled"]`)
	checked, err := enabled.IsChecked()
	require.NoError(t, err)
	if !checked {
		require.NoError(t, page.Locator(".toggle").Filter(playwright.LocatorFilterOptions{Has: enabled}).Click())
	}
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
}

func liveSession(t *testing.T, app *uiApp) *mcpsdk.ClientSession {
	t.Helper()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "live-acceptance", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func liveCall(t *testing.T, session *mcpsdk.ClientSession, tool string, args map[string]any) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err, "%s %v", tool, args["tool_name"])
	var text strings.Builder
	for _, content := range result.Content {
		if part, ok := content.(*mcpsdk.TextContent); ok {
			text.WriteString(part.Text)
		}
	}
	return text.String(), result.IsError
}

func liveSearch(t *testing.T, session *mcpsdk.ClientSession, integration, query string) []string {
	t.Helper()
	text, isError := liveCall(t, session, "search", map[string]any{"query": query, "integration": integration, "limit": 10})
	require.False(t, isError, text)
	return uiToolNames(t, text)
}

func liveArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

func liveContainsAll(text string, expected []string) bool {
	for _, want := range expected {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}

func liveTruncate(text string) string {
	const limit = 2000
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

func liveServerLog(t *testing.T, app *uiApp) {
	logs, _ := filepath.Glob(filepath.Join(app.home, "server-*.log"))
	for _, path := range logs {
		data, err := os.ReadFile(path) // #nosec G304 -- test temp dir
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > 40 {
			lines = lines[len(lines)-40:]
		}
		t.Logf("switchboard server log %s:\n%s", filepath.Base(path), strings.Join(lines, "\n"))
	}
}

func liveWriteEvidence(t *testing.T, evidence liveEvidence) {
	root := os.Getenv("SWITCHBOARD_UI_ARTIFACTS")
	if root == "" {
		return
	}
	require.NoError(t, os.MkdirAll(root, 0o700))
	data, err := json.MarshalIndent(evidence, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "live-"+evidence.Integration+".json"), data, 0o600))
}

// liveDockerContainer is a disposable provider container bound to loopback.
type liveDockerContainer struct {
	id string
}

type liveDockerOptions struct {
	// Image must be pinned by digest so runs are reproducible.
	Image string
	Env   map[string]string
	// Ports are container ports such as "5432/tcp", published on a random
	// 127.0.0.1 port.
	Ports []string
	Cmd   []string
}

func liveStartContainer(t *testing.T, options liveDockerOptions) *liveDockerContainer {
	t.Helper()
	require.Contains(t, options.Image, "@sha256:", "pin live provider images by digest")
	args := []string{"run", "--detach", "--rm"}
	for _, port := range options.Ports {
		args = append(args, "--publish", "127.0.0.1::"+port)
	}
	keys := make([]string, 0, len(options.Env))
	for key := range options.Env {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		args = append(args, "--env", key+"="+options.Env[key])
	}
	args = append(args, options.Image)
	args = append(args, options.Cmd...)
	id := strings.TrimSpace(liveDocker(t, args...))
	require.NotEmpty(t, id)
	container := &liveDockerContainer{id: id}
	t.Cleanup(func() {
		if t.Failed() {
			logs, _ := exec.Command("docker", "logs", "--tail", "50", id).CombinedOutput() // #nosec G204 -- id comes from docker run
			t.Logf("provider container logs:\n%s", logs)
		}
		_ = exec.Command("docker", "rm", "--force", id).Run() // #nosec G204 -- id comes from docker run
	})
	return container
}

// Address returns the 127.0.0.1 host and port published for a container port.
func (c *liveDockerContainer) Address(t *testing.T, port string) (string, string) {
	t.Helper()
	output := strings.TrimSpace(liveDocker(t, "port", c.id, port))
	first, _, _ := strings.Cut(output, "\n")
	host, published, ok := strings.Cut(first, ":")
	require.True(t, ok, "unexpected docker port output %q", output)
	return host, published
}

// Exec runs a command inside the container, for seeding data.
func (c *liveDockerContainer) Exec(t *testing.T, stdin string, command ...string) string {
	t.Helper()
	args := append([]string{"exec", "--interactive", c.id}, command...)
	cmd := exec.CommandContext(t.Context(), "docker", args...) // #nosec G204 -- fixed test commands
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	require.NoError(t, err, "docker exec %v: %s", command, stderr.String())
	return string(output)
}

// ExecDetached starts a command inside the container without waiting, for
// seeding long-running activity such as held locks.
func (c *liveDockerContainer) ExecDetached(t *testing.T, command ...string) {
	t.Helper()
	liveDocker(t, append([]string{"exec", "--detach", c.id}, command...)...)
}

func liveDocker(t *testing.T, args ...string) string {
	t.Helper()
	_, err := exec.LookPath("docker")
	require.NoError(t, err, "live container tests need the docker CLI")
	cmd := exec.CommandContext(t.Context(), "docker", args...) // #nosec G204 -- fixed test commands
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	require.NoError(t, err, "docker %s: %s", args[0], stderr.String())
	return string(output)
}

// liveEventually retries check until it succeeds, reporting the last error.
func liveEventually(t *testing.T, timeout time.Duration, check func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		err := check()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			require.NoError(t, err, "provider not ready after %s", timeout)
		}
		select {
		case <-t.Context().Done():
			require.NoError(t, t.Context().Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}
