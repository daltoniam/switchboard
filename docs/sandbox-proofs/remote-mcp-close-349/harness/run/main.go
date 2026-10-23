// Adapter-level E2E harness for PR #349. Lives outside the PR tree; builds against
// either the PR worktree (go.mod) or main (go.main.mod) via -modfile. Talks to the
// stand-in MCP server over real HTTP; the adapters use the real remotemcp + MCP Go SDK client.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/figma"
	"github.com/daltoniam/switchboard/integrations/linear"
	"github.com/daltoniam/switchboard/integrations/slackmcp"
)

type Ev struct {
	Seq        int    `json:"seq"`
	Server     string `json:"server"`
	Method     string `json:"http_method"`
	SessionIn  string `json:"session_in"`
	SessionOut string `json:"session_out"`
	RPC        string `json:"rpc"`
	Tool       string `json:"tool"`
	Status     int    `json:"status"`
	Kind       string `json:"kind"`
	Probe      bool   `json:"probe"`
}

type Check struct {
	Scenario string `json:"scenario"`
	Step     string `json:"step"`
	Result   string `json:"result"`
	Evidence string `json:"evidence"`
	Events   []Ev   `json:"server_events,omitempty"`
}

var (
	base   string
	label  string
	checks []Check
	mark   int
	ctx    = context.Background()
)

func events() []Ev {
	resp, err := http.Get(base + "/_events")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var e []Ev
	json.NewDecoder(resp.Body).Decode(&e)
	return e
}

// since returns non-probe events after the last mark and moves the mark.
func since() []Ev {
	all := events()
	var out []Ev
	for _, e := range all {
		if e.Seq > mark {
			out = append(out, e)
		}
	}
	if len(all) > 0 {
		mark = all[len(all)-1].Seq
	}
	return out
}

func count(evs []Ev, kind, server string) (n int, sids []string) {
	for _, e := range evs {
		if e.Kind == kind && (server == "" || e.Server == server) && !e.Probe {
			n++
			if e.SessionOut != "" {
				sids = append(sids, e.SessionOut)
			} else {
				sids = append(sids, e.SessionIn)
			}
		}
	}
	return
}

// probe sends tools/list on an existing session id directly (not via the adapter).
func probe(server, sid string) int {
	body := `{"jsonrpc":"2.0","id":99,"method":"tools/list","params":{}}`
	req, _ := http.NewRequest("POST", base+"/"+server+"/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer probe")
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	req.Header.Set("X-Standin-Probe", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

func add(sc, step string, ok bool, ev string, evs []Ev) {
	r := "PASS"
	if !ok {
		r = "FAIL"
	}
	checks = append(checks, Check{sc, step, r, ev, evs})
	fmt.Printf("[%s] %-4s %s | %s | %s\n", label, r, sc, step, ev)
}
func info(sc, step, ev string, evs []Ev) {
	checks = append(checks, Check{sc, step, "INFO", ev, evs})
	fmt.Printf("[%s] INFO %s | %s | %s\n", label, sc, step, ev)
}

func closer(i mcp.Integration) (func() error, bool) {
	c, ok := i.(io.Closer)
	if !ok {
		return nil, false
	}
	return c.Close, true
}

func short(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func exec(i mcp.Integration, tool string, args map[string]any) (*mcp.ToolResult, error) {
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return i.Execute(c, mcp.ToolName(tool), args)
}

// lifecycle runs Configure → open → call → Close → probe 404 → Close again → re-Configure → fresh session.
func lifecycle(sc, server string, i mcp.Integration, configure func() error, tool string, args map[string]any) {
	since()
	if err := configure(); err != nil {
		add(sc, "Configure", false, err.Error(), nil)
		return
	}
	tools := i.Tools()
	res, err := exec(i, tool, args)
	evs := since()
	n, sids := count(evs, "session_open", server)
	ok := err == nil && res != nil && !res.IsError && strings.Contains(res.Data, `"standin"`) && n == 1
	data := ""
	if res != nil {
		data = res.Data
	}
	add(sc, "Configure → session opened → tool call works", ok, fmt.Sprintf("tools=%d, session_open=%d %v, %s → %s", len(tools), n, sids, tool, short(data, 140)), evs)
	if n == 0 {
		return
	}
	sid := sids[0]
	cl, has := closer(i)
	if !has {
		add(sc, "adapter implements io.Closer", false, "type does not implement io.Closer; host cannot end the session", nil)
		st := probe(server, sid)
		info(sc, "session after host drops instance (no Close possible)", fmt.Sprintf("probe tools/list on %s → HTTP %d (session still alive = leaked)", sid, st), nil)
		return
	}
	err = cl()
	evs = since()
	nd, dsids := count(evs, "session_delete", server)
	st := probe(server, sid)
	pe := since()
	add(sc, "Close() → DELETE sent, session invalid server-side", err == nil && nd == 1 && len(dsids) == 1 && dsids[0] == sid && st == 404,
		fmt.Sprintf("Close err=%v; DELETE count=%d sid=%v; probe tools/list on old sid → HTTP %d", err, nd, dsids, st), append(evs, pe...))
	err = cl()
	evs = since()
	nd2, _ := count(evs, "session_delete", "")
	add(sc, "second Close() is a no-op", err == nil && len(evs) == 0, fmt.Sprintf("Close err=%v; new server events=%d (DELETE=%d)", err, len(evs), nd2), evs)
	if r2, e2 := exec(i, tool, args); r2 != nil {
		info(sc, "Execute after Close (before re-Configure)", fmt.Sprintf("IsError=%v err=%v data=%s", r2.IsError, e2, short(r2.Data, 160)), since())
	}
	if err := configure(); err != nil {
		add(sc, "re-Configure", false, err.Error(), nil)
		return
	}
	res, err = exec(i, tool, args)
	evs = since()
	n, sids2 := count(evs, "session_open", server)
	ok = err == nil && res != nil && !res.IsError && n == 1 && sids2[0] != sid && strings.Contains(res.Data, sids2[0])
	data = ""
	if res != nil {
		data = res.Data
	}
	add(sc, "re-Configure → fresh session id → tool call works", ok, fmt.Sprintf("session_open=%d new=%v old=%s; %s", n, sids2, sid, short(data, 140)), evs)
	if has {
		cl()
		since()
	}
}

func main() {
	flag.StringVar(&base, "base", "http://127.0.0.1:18349", "")
	flag.StringVar(&label, "label", "after", "")
	out := flag.String("out", "harness.json", "")
	flag.Parse()
	since()

	// ---- Scenario 1: per-adapter lifecycle ----
	lin := linear.New(base + "/linear")
	lifecycle("1 Linear (mcp_access_token)", "linear", lin, func() error {
		return lin.Configure(ctx, mcp.Credentials{"mcp_access_token": "standin-linear-token"})
	}, "linear_list_issues", map[string]any{"query": "proof"})

	fig := figma.New()
	lifecycle("1 Figma", "figma-a", fig, func() error {
		return fig.Configure(ctx, mcp.Credentials{"mcp_access_token": "standin-figma-token", "base_url": base + "/figma-a"})
	}, "figma_whoami", nil)

	sl := slackmcp.New()
	mi := sl.(mcp.MultiIdentityIntegration)
	lifecycle("1 Slack MCP (1 identity)", "slack", sl, func() error {
		if err := sl.Configure(ctx, mcp.Credentials{"base_url": base + "/slack"}); err != nil {
			return err
		}
		return mi.ConfigureIdentities(ctx, map[string]mcp.IntegrationIdentity{"work": {Credentials: mcp.Credentials{"access_token": "standin-slack-work"}}})
	}, "slackmcp_search_channels", map[string]any{"identity_id": "work", "query": "general"})

	// ---- Scenario 2: Linear mcp_access_token → api_key ----
	sc := "2 Linear mcp → api_key switch"
	lin2 := linear.New(base + "/linear")
	since()
	lin2.Configure(ctx, mcp.Credentials{"mcp_access_token": "standin-linear-token"})
	r, _ := exec(lin2, "linear_list_issues", nil)
	evs := since()
	_, sids := count(evs, "session_open", "linear")
	add(sc, "remote mode session opened + call works", len(sids) == 1 && r != nil && !r.IsError && linear.IsRemoteMCP(lin2), fmt.Sprintf("sid=%v IsRemoteMCP=%v", sids, linear.IsRemoteMCP(lin2)), evs)
	if len(sids) == 1 {
		err := lin2.Configure(ctx, mcp.Credentials{"api_key": "lin_api_sandboxFAKEkey0000000000000000000000"})
		evs = since()
		nd, dsids := count(evs, "session_delete", "linear")
		st := probe("linear", sids[0])
		since()
		add(sc, "Configure(api_key) ends remote session", err == nil && nd == 1 && st == 404 && !linear.IsRemoteMCP(lin2),
			fmt.Sprintf("err=%v DELETE=%d %v; probe old sid → HTTP %d; IsRemoteMCP=%v", err, nd, dsids, st, linear.IsRemoteMCP(lin2)), evs)
		if label == "before" && nd == 0 {
			info(sc, "before: session left open after switch", fmt.Sprintf("probe old sid → HTTP %d (200 = leaked)", st), nil)
		}
		r, err = exec(lin2, "linear_viewer", nil)
		evs = since()
		d := ""
		if r != nil {
			d = r.Data
		}
		ok := err == nil && r != nil && r.IsError && (strings.Contains(d, "401") || strings.Contains(strings.ToLower(d), "authentication")) && len(evs) == 0
		add(sc, "api_key native REST path wired (fake key → api.linear.app)", ok, fmt.Sprintf("linear_viewer → IsError=%v %s; stand-in events=%d", r != nil && r.IsError, short(d, 200), len(evs)), nil)
	}

	// ---- Scenario 3: Figma base_url change ----
	sc = "3 Figma base_url change"
	fig2 := figma.New()
	since()
	fig2.Configure(ctx, mcp.Credentials{"mcp_access_token": "standin-figma-token", "base_url": base + "/figma-a"})
	r, _ = exec(fig2, "figma_whoami", nil)
	evs = since()
	_, sa := count(evs, "session_open", "figma-a")
	add(sc, "session on server A + call works", len(sa) == 1 && r != nil && !r.IsError, fmt.Sprintf("figma-a sid=%v", sa), evs)
	if len(sa) == 1 {
		fig2.Configure(ctx, mcp.Credentials{"mcp_access_token": "standin-figma-token", "base_url": base + "/figma-a"})
		r, _ = exec(fig2, "figma_whoami", nil)
		evs = since()
		no, _ := count(evs, "session_open", "")
		nd, _ := count(evs, "session_delete", "")
		info(sc, "same base_url + same token re-Configure reuses session", fmt.Sprintf("session_open=%d DELETE=%d", no, nd), evs)
		err := fig2.Configure(ctx, mcp.Credentials{"mcp_access_token": "standin-figma-token", "base_url": base + "/figma-b"})
		evs = since()
		nd, dsids := count(evs, "session_delete", "figma-a")
		st := probe("figma-a", sa[0])
		since()
		add(sc, "Configure(base_url=B) ends session on A", err == nil && nd == 1 && st == 404, fmt.Sprintf("err=%v DELETE on figma-a=%d %v; probe old A sid → HTTP %d", err, nd, dsids, st), evs)
		r, _ = exec(fig2, "figma_whoami", nil)
		evs = since()
		_, sb := count(evs, "session_open", "figma-b")
		d := ""
		if r != nil {
			d = r.Data
		}
		add(sc, "new session opened on B + call works", len(sb) == 1 && strings.Contains(d, `"standin":"figma-b"`), fmt.Sprintf("figma-b sid=%v → %s", sb, short(d, 140)), evs)
		if c, ok := closer(fig2); ok {
			c()
		}
		since()
	}

	// ---- Scenario 4: Slack MCP two identities ----
	sc = "4 Slack MCP two identities"
	sl2 := slackmcp.New()
	mi2 := sl2.(mcp.MultiIdentityIntegration)
	since()
	sl2.Configure(ctx, mcp.Credentials{"base_url": base + "/slack"})
	err := mi2.ConfigureIdentities(ctx, map[string]mcp.IntegrationIdentity{
		"work":     {Credentials: mcp.Credentials{"access_token": "standin-slack-work"}},
		"personal": {Credentials: mcp.Credentials{"access_token": "standin-slack-personal"}},
	})
	rw, _ := exec(sl2, "slackmcp_read_channel", map[string]any{"identity_id": "work"})
	rp, _ := exec(sl2, "slackmcp_read_channel", map[string]any{"identity_id": "personal"})
	evs = since()
	no, ss := count(evs, "session_open", "slack")
	add(sc, "two identities → two sessions, calls work on each", err == nil && no == 2 && rw != nil && !rw.IsError && rp != nil && !rp.IsError, fmt.Sprintf("err=%v sessions=%v work→%s personal→%s", err, ss, short(rw.Data, 90), short(rp.Data, 90)), evs)
	if c, ok := closer(sl2); ok {
		err = c()
		evs = since()
		nd, dsids := count(evs, "session_delete", "slack")
		st0, st1 := -1, -1
		if len(ss) == 2 {
			st0, st1 = probe("slack", ss[0]), probe("slack", ss[1])
		}
		since()
		add(sc, "Close() ends both sessions", err == nil && nd == 2 && st0 == 404 && st1 == 404, fmt.Sprintf("err=%v DELETE=%d %v; probes → %d, %d", err, nd, dsids, st0, st1), evs)
		err = c()
		evs = since()
		add(sc, "second Close() no-op", err == nil && len(evs) == 0, fmt.Sprintf("err=%v events=%d", err, len(evs)), evs)
		add(sc, "Healthy false after Close", !sl2.Healthy(ctx), "", nil)
	} else {
		add(sc, "adapter implements io.Closer", false, "no Close on main", nil)
		if len(ss) == 2 {
			info(sc, "sessions after host drops instance", fmt.Sprintf("probes → %d, %d (200 = leaked)", probe("slack", ss[0]), probe("slack", ss[1])), nil)
		}
	}

	resp, _ := http.Get(base + "/_sessions")
	var sess any
	json.NewDecoder(resp.Body).Decode(&sess)
	b, _ := json.MarshalIndent(map[string]any{"label": label, "checks": checks, "final_session_table": sess}, "", " ")
	os.WriteFile(*out, b, 0o644)
	p, f := 0, 0
	for _, c := range checks {
		if c.Result == "PASS" {
			p++
		} else if c.Result == "FAIL" {
			f++
		}
	}
	fmt.Printf("[%s] PASS=%d FAIL=%d\n", label, p, f)
}
