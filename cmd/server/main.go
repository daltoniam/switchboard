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
	"github.com/daltoniam/switchboard/daemon"
	"github.com/daltoniam/switchboard/datadog"
	"github.com/daltoniam/switchboard/github"
	"github.com/daltoniam/switchboard/linear"
	"github.com/daltoniam/switchboard/metabase"
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
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		handleDaemon(os.Args[2:])
		return
	}

	stdioMode := flag.Bool("stdio", false, "Run MCP server over stdio transport (default is HTTP)")
	port := flag.Int("port", 3847, "Port for the HTTP server")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("switchboard %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	runServer(*stdioMode, *port)
}

func handleDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	port := fs.Int("port", 3847, "Port for the HTTP server")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: switchboard daemon <command> [options]

Commands:
  install     Install as a system service (launchd on macOS, systemd on Linux)
  uninstall   Remove the system service
  start       Start the daemon
  stop        Stop the daemon
  status      Show daemon status
  logs        Show log file path

Options:
`)
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	cmd := args[0]
	if cmd == "-h" || cmd == "-help" || cmd == "--help" {
		fs.Usage()
		os.Exit(0)
	}
	fs.Parse(args[1:])

	switch cmd {
	case "install":
		if err := daemon.Install(*port); err != nil {
			log.Fatalf("Install failed: %v", err)
		}
		fmt.Println("Service installed. Run 'switchboard daemon start' to start.")
	case "uninstall":
		if err := daemon.Uninstall(); err != nil {
			log.Fatalf("Uninstall failed: %v", err)
		}
	case "start":
		status, _ := daemon.GetStatus(*port)
		if status != nil && status.Running {
			fmt.Printf("Switchboard is already running (PID %d)\n", status.PID)
			os.Exit(0)
		}
		if err := daemon.Start(*port); err != nil {
			log.Fatalf("Start failed: %v", err)
		}
		time.Sleep(time.Second)
		status, _ = daemon.GetStatus(*port)
		if status != nil && status.Running {
			fmt.Printf("Switchboard started (PID %d) on port %d\n", status.PID, *port)
			if status.Healthy {
				fmt.Println("Health check: OK")
			}
		} else {
			logPath, _ := daemon.LogPath()
			fmt.Printf("Switchboard may have started — check %s for details\n", logPath)
		}
	case "stop":
		if err := daemon.Stop(); err != nil {
			log.Fatalf("Stop failed: %v", err)
		}
		fmt.Println("Switchboard stopped")
	case "status":
		status, err := daemon.GetStatus(*port)
		if err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
		if !status.Running {
			fmt.Println("Switchboard is not running")
			if daemon.IsServiceInstalled() {
				fmt.Println("Service is installed")
			}
			os.Exit(1)
		}
		fmt.Printf("Switchboard is running (PID %d)\n", status.PID)
		if status.Healthy {
			fmt.Printf("Health: OK (port %d)\n", *port)
		} else {
			fmt.Printf("Health: NOT OK (port %d)\n", *port)
		}
		if daemon.IsServiceInstalled() {
			fmt.Println("Service: installed")
		}
	case "logs":
		logPath, err := daemon.LogPath()
		if err != nil {
			log.Fatalf("Failed to get log path: %v", err)
		}
		fmt.Println(logPath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown daemon command: %s\n", cmd)
		fs.Usage()
		os.Exit(1)
	}
}

func runServer(stdioMode bool, port int) {
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

	if stdioMode {
		if err := srv.RunStdio(ctx); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
		return
	}

	if err := daemon.WritePID(os.Getpid()); err != nil {
		log.Printf("WARN: failed to write PID file: %v", err)
	}
	defer daemon.RemovePID()

	mux := http.NewServeMux()

	mux.Handle("/mcp", srv.Handler())

	ws := web.New(services, port)
	mux.Handle("/", ws.Handler())

	addr := fmt.Sprintf(":%d", port)
	fmt.Fprintf(os.Stderr, "Switchboard on http://localhost:%d\n", port)
	fmt.Fprintf(os.Stderr, "  Web UI:  http://localhost:%d/\n", port)
	fmt.Fprintf(os.Stderr, "  MCP:     http://localhost:%d/mcp\n", port)

	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}
