// Stand-in streamable-HTTP MCP server for the PR #349 proof. NOT Linear/Figma/Slack.
// Built on the official MCP Go SDK (v1.7.0, same version Switchboard's remotemcp uses).
// Mounts one independent stateful server per path prefix (/linear, /figma-a, /figma-b, /slack),
// logs every HTTP request (method, path, Mcp-Session-Id in/out, JSON-RPC method, status) as JSONL,
// and exposes GET /_events and /_sessions for the harness.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Event struct {
	Seq       int    `json:"seq"`
	Time      string `json:"time"`
	Server    string `json:"server"`
	Method    string `json:"http_method"`
	Path      string `json:"path"`
	SessionIn string `json:"session_in,omitempty"`
	SessionOut string `json:"session_out,omitempty"`
	RPC       string `json:"rpc,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Status    int    `json:"status"`
	TokenFP   string `json:"token_fp,omitempty"`
	Kind      string `json:"kind"` // session_open | request | session_delete | rejected_deleted_session | rejected
	Probe     bool   `json:"probe,omitempty"`
}

var (
	mu     sync.Mutex
	events []Event
	logf   *os.File
	live   = map[string]map[string]bool{} // server -> sid -> alive
	dead   = map[string]map[string]bool{}
)

type recW struct {
	http.ResponseWriter
	status int
}

func (r *recW) WriteHeader(c int) { r.status = c; r.ResponseWriter.WriteHeader(c) }
func (r *recW) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func fp(auth string) string {
	t := strings.TrimPrefix(auth, "Bearer ")
	if t == "" {
		return ""
	}
	h := sha256.Sum256([]byte(t))
	return "sha256:" + hex.EncodeToString(h[:])[:10]
}

func newServer(name string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "standin-" + name, Version: "0.1"}, nil)
	type In struct {
		Query string `json:"query,omitempty" jsonschema:"optional query"`
	}
	add := func(tool, desc string) {
		mcp.AddTool(s, &mcp.Tool{Name: tool, Description: desc}, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			sid := ""
			if req.Session != nil {
				sid = req.Session.ID()
			}
			out := map[string]any{"standin": name, "tool": tool, "session": sid, "query": in.Query}
			b, _ := json.Marshal(out)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
		})
	}
	switch {
	case name == "linear":
		add("list_issues", "stand-in list issues")
		add("get_user", "stand-in viewer")
	case strings.HasPrefix(name, "figma"):
		add("whoami", "stand-in whoami")
		add("get_figjam", "stand-in figjam")
	case name == "slack":
		add("slack_search_channels", "stand-in channel search")
		add("slack_read_channel", "stand-in channel read")
	}
	return s
}

func wrap(name string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		rpc, tool := "", ""
		var m struct {
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if json.Unmarshal(body, &m) == nil {
			rpc, tool = m.Method, m.Params.Name
		}
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing bearer", 401)
			record(Event{Server: name, Method: r.Method, Path: r.URL.Path, RPC: rpc, Status: 401, Kind: "rejected"})
			return
		}
		sidIn := r.Header.Get("Mcp-Session-Id")
		rw := &recW{ResponseWriter: w, status: 200}
		h.ServeHTTP(rw, r)
		sidOut := rw.Header().Get("Mcp-Session-Id")
		ev := Event{Server: name, Method: r.Method, Path: r.URL.Path, SessionIn: sidIn, RPC: rpc, Tool: tool, Status: rw.status, TokenFP: fp(r.Header.Get("Authorization")), Kind: "request", Probe: r.Header.Get("X-Standin-Probe") == "1"}
		mu.Lock()
		if live[name] == nil {
			live[name], dead[name] = map[string]bool{}, map[string]bool{}
		}
		switch {
		case r.Method == http.MethodPost && rpc == "initialize" && sidIn == "" && sidOut != "":
			ev.Kind, ev.SessionOut = "session_open", sidOut
			live[name][sidOut] = true
		case r.Method == http.MethodDelete:
			ev.Kind = "session_delete"
			if live[name][sidIn] && rw.status < 300 {
				delete(live[name], sidIn)
				dead[name][sidIn] = true
			}
		case rw.status == 404 && dead[name][sidIn]:
			ev.Kind = "rejected_deleted_session"
		}
		mu.Unlock()
		record(ev)
	})
}

func record(ev Event) {
	mu.Lock()
	defer mu.Unlock()
	ev.Seq = len(events) + 1
	ev.Time = time.Now().Format(time.RFC3339Nano)
	events = append(events, ev)
	b, _ := json.Marshal(ev)
	fmt.Fprintln(logf, string(b))
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18349", "")
	out := flag.String("log", "standin-events.jsonl", "")
	flag.Parse()
	var err error
	if logf, err = os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, name := range []string{"linear", "figma-a", "figma-b", "slack"} {
		srv := newServer(name)
		h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		mux.Handle("/"+name+"/mcp", wrap(name, h))
	}
	mux.HandleFunc("/_events", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(events)
	})
	mux.HandleFunc("/_sessions", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"live": live, "deleted": dead})
	})
	log.Printf("stand-in MCP listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
