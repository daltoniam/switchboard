package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcp "github.com/daltoniam/switchboard"
	awsInt "github.com/daltoniam/switchboard/aws"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/datadog"
	"github.com/daltoniam/switchboard/github"
	"github.com/daltoniam/switchboard/linear"
	"github.com/daltoniam/switchboard/metabase"
	"github.com/daltoniam/switchboard/posthog"
	"github.com/daltoniam/switchboard/postgres"
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/sentry"
	"github.com/daltoniam/switchboard/server"
	slackInt "github.com/daltoniam/switchboard/slack"
	"github.com/daltoniam/switchboard/web"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	stdioMode := flag.Bool("stdio", false, "Run MCP server over stdio transport (default is HTTP)")
	port := flag.Int("port", 3847, "Port for the HTTP server")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("switchboard %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	cfgMgr, err := config.NewManager()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	reg := registry.New()
	for _, i := range []mcp.Integration{
		github.New(),
		datadog.New(),
		linear.New(),
		sentry.New(),
		slackInt.New(),
		metabase.New(),
		awsInt.New(),
		posthog.New(),
		postgres.New(),
	} {
		if err := reg.Register(i); err != nil {
			log.Fatalf("Failed to register integration: %v", err)
		}
	}

	services := &mcp.Services{
		Config:   cfgMgr,
		Registry: reg,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := server.New(services)

	if *stdioMode {
		if err := srv.RunStdio(ctx); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
		return
	}

	mux := http.NewServeMux()

	// Mount MCP streamable HTTP handler at /mcp
	mux.Handle("/mcp", srv.Handler())

	// Mount web UI handler for everything else
	ws := web.New(services, *port)
	mux.Handle("/", ws.Handler())

	addr := fmt.Sprintf(":%d", *port)
	fmt.Fprintf(os.Stderr, "Switchboard on http://localhost:%d\n", *port)
	fmt.Fprintf(os.Stderr, "  Web UI:  http://localhost:%d/\n", *port)
	fmt.Fprintf(os.Stderr, "  MCP:     http://localhost:%d/mcp\n", *port)

	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}
