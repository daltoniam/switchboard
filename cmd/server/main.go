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
	"github.com/daltoniam/switchboard/browser"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/daemon"
	acpInt "github.com/daltoniam/switchboard/integrations/acp"
	"github.com/daltoniam/switchboard/integrations/amazon"
	awsInt "github.com/daltoniam/switchboard/integrations/aws"
	"github.com/daltoniam/switchboard/integrations/botidentity"
	"github.com/daltoniam/switchboard/integrations/clickhouse"
	"github.com/daltoniam/switchboard/integrations/cloudflare"
	"github.com/daltoniam/switchboard/integrations/confluence"
	"github.com/daltoniam/switchboard/integrations/datadog"
	"github.com/daltoniam/switchboard/integrations/digitalocean"
	"github.com/daltoniam/switchboard/integrations/elasticsearch"
	flyInt "github.com/daltoniam/switchboard/integrations/fly"
	gcpInt "github.com/daltoniam/switchboard/integrations/gcp"
	"github.com/daltoniam/switchboard/integrations/github"
	"github.com/daltoniam/switchboard/integrations/gmail"
	"github.com/daltoniam/switchboard/integrations/jira"
	"github.com/daltoniam/switchboard/integrations/linear"
	"github.com/daltoniam/switchboard/integrations/metabase"
	nomadInt "github.com/daltoniam/switchboard/integrations/nomad"
	notionInt "github.com/daltoniam/switchboard/integrations/notion"
	"github.com/daltoniam/switchboard/integrations/pganalyze"
	"github.com/daltoniam/switchboard/integrations/postgres"
	"github.com/daltoniam/switchboard/integrations/posthog"
	"github.com/daltoniam/switchboard/integrations/readarr"
	"github.com/daltoniam/switchboard/integrations/rwx"
	"github.com/daltoniam/switchboard/integrations/salesforce"
	"github.com/daltoniam/switchboard/integrations/sentry"
	slackInt "github.com/daltoniam/switchboard/integrations/slack"
	snowflakeInt "github.com/daltoniam/switchboard/integrations/snowflake"
	"github.com/daltoniam/switchboard/integrations/suno"
	webfetchInt "github.com/daltoniam/switchboard/integrations/webfetch"
	xInt "github.com/daltoniam/switchboard/integrations/x"
	"github.com/daltoniam/switchboard/integrations/ynab"
	"github.com/daltoniam/switchboard/marketplace"
	"github.com/daltoniam/switchboard/project"
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/server"
	"github.com/daltoniam/switchboard/version"
	wasmmod "github.com/daltoniam/switchboard/wasm"
	"github.com/daltoniam/switchboard/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		handleDaemon(os.Args[2:])
		return
	}

	stdioMode := flag.Bool("stdio", false, "Run MCP server over stdio transport (default is HTTP)")
	port := flag.Int("port", 3847, "Port for the HTTP server")
	discoverAll := flag.Bool("discover-all", false, "Search returns tools from all registered integrations, not just enabled ones")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("switchboard %s\n", version.Full())
		os.Exit(0)
	}

	runServer(*stdioMode, *port, *discoverAll)
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

	_ = fs.Parse(args)
	remaining := fs.Args()

	if len(remaining) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	cmd := remaining[0]

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

func runServer(stdioMode bool, port int, discoverAll bool) {
	cfgMgr, err := config.NewManager()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	browserDone := make(chan struct{})
	var browserSvc mcp.BrowserService
	go func() {
		defer close(browserDone)
		svc, err := browser.New(true /* headless */)
		if err != nil {
			log.Printf("browser service unavailable (%v) — browser-based integrations disabled", err)
			return
		}
		browserSvc = svc
	}()

	gmailIntegration := gmail.New()
	amazonIntegration := amazon.New()
	reg := registry.New()
	for _, i := range []mcp.Integration{
		github.New(),
		datadog.New(),
		linear.New("https://mcp.linear.app"),
		sentry.New(),
		slackInt.New(),
		metabase.New(),
		awsInt.New(),
		posthog.New(),
		postgres.New(),
		clickhouse.New(),
		elasticsearch.New(),
		pganalyze.New(),
		rwx.New(),
		ynab.New(),
		amazonIntegration,
		gmailIntegration,
		jira.New(),
		confluence.New(),
		notionInt.New(),
		gcpInt.New(),
		suno.New(),
		readarr.New(),
		salesforce.New(),
		cloudflare.New(),
		digitalocean.New(),
		flyInt.New(),
		snowflakeInt.New(),
		acpInt.New(),
		webfetchInt.New(),
		nomadInt.New(),
		botidentity.New(),
		xInt.New(),
	} {
		if err := reg.Register(i); err != nil {
			log.Fatalf("Failed to register integration: %v", err)
		}
	}

	services := &mcp.Services{
		Config:   cfgMgr,
		Registry: reg,
		Metrics:  mcp.NewMetrics(),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create WASM runtime and loader (always — needed for live-reload from web UI).
	cfg := cfgMgr.Get()
	wasmCtx := context.Background()
	wasmRT, err := wasmmod.NewRuntime(wasmCtx)
	if err != nil {
		log.Fatalf("Failed to create WASM runtime: %v", err)
	}
	defer wasmRT.Close(ctx) //nolint:errcheck
	wasmLoader := wasmmod.NewLoader(wasmRT, reg, cfgMgr)

	// Load WASM modules from config + marketplace installed plugins.
	allWasmModules := make([]mcp.WasmModuleConfig, len(cfg.WasmModules))
	copy(allWasmModules, cfg.WasmModules)
	if cfg.Marketplace != nil {
		seen := make(map[string]bool)
		for _, wm := range allWasmModules {
			seen[wm.Path] = true
		}
		for _, ip := range cfg.Marketplace.InstalledPlugins {
			if ip.Path != "" && !seen[ip.Path] {
				allWasmModules = append(allWasmModules, mcp.WasmModuleConfig{Path: ip.Path})
				seen[ip.Path] = true
			}
		}
	}
	for _, wc := range allWasmModules {
		if err := wasmLoader.LoadPlugin(wasmCtx, wc.Path, wc.Name); err != nil {
			log.Printf("WARN: %v", err)
		}
	}

	gmail.SetConfigService(gmailIntegration, cfgMgr)
	go func() {
		<-browserDone
		if browserSvc != nil {
			amazon.SetBrowserService(amazonIntegration, browserSvc)
			log.Println("browser service ready — Amazon browser features enabled")
		}
	}()

	var serverOpts []server.Option
	if discoverAll {
		serverOpts = append(serverOpts, server.WithDiscoverAll(true))
	}
	if cfg.SessionStore == "file" {
		serverOpts = append(serverOpts, server.WithSessionStore(
			server.NewFileSessionStore(server.DefaultSessionDir(), server.DefaultSessionTTL),
		))
	}
	srv := server.New(services, serverOpts...)

	if stdioMode {
		if err := srv.RunStdio(ctx); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
		return
	}

	if os.Getenv("SWITCHBOARD_DAEMON") == "1" {
		if err := daemon.WritePID(os.Getpid()); err != nil {
			log.Printf("WARN: failed to write PID file: %v", err)
		}
		defer func() { _ = daemon.RemovePID() }()
	}

	projectStore := project.NewStore(project.DefaultConfigDir())
	if err := projectStore.Load(); err != nil {
		log.Printf("WARN: failed to load project definitions: %v", err)
	}
	if names := projectStore.Names(); len(names) > 0 {
		log.Printf("Loaded %d project(s): %v", len(names), names)
	}

	projectRouter := server.NewProjectRouter(services, projectStore, "", srv.SearchIndex())

	mux := http.NewServeMux()

	mux.Handle("/mcp", srv.Handler())
	mux.Handle("/mcp/{project}", projectRouter.Handler())

	// Initialize plugin marketplace.
	var mpCfg marketplace.Config
	if cfg.Marketplace != nil {
		mpCfg = marketplace.Config{
			AutoUpdate:    cfg.Marketplace.AutoUpdate,
			CheckInterval: cfg.Marketplace.CheckInterval,
			PluginDir:     cfg.Marketplace.PluginDir,
			LastCheck:     cfg.Marketplace.LastCheck,
		}
		for _, src := range cfg.Marketplace.ManifestSources {
			mpCfg.ManifestSources = append(mpCfg.ManifestSources, marketplace.ManifestSource{
				URL:     src.URL,
				Name:    src.Name,
				Enabled: src.Enabled,
			})
		}
		for _, ip := range cfg.Marketplace.InstalledPlugins {
			mpCfg.InstalledPlugins = append(mpCfg.InstalledPlugins, marketplace.InstalledPlugin{
				Name:          ip.Name,
				Version:       ip.Version,
				ManifestURL:   ip.ManifestURL,
				InstalledAt:   ip.InstalledAt,
				Path:          ip.Path,
				SHA256:        ip.SHA256,
				AutoUpdate:    ip.AutoUpdate,
				LatestVersion: ip.LatestVersion,
			})
		}
	}
	mp := marketplace.NewManager(mpCfg, "", func(c marketplace.Config) error {
		mc := &mcp.MarketplaceConfig{
			AutoUpdate:    c.AutoUpdate,
			CheckInterval: c.CheckInterval,
			PluginDir:     c.PluginDir,
			LastCheck:     c.LastCheck,
		}
		for _, src := range c.ManifestSources {
			mc.ManifestSources = append(mc.ManifestSources, mcp.MarketplaceManifestSource{
				URL:     src.URL,
				Name:    src.Name,
				Enabled: src.Enabled,
			})
		}
		for _, ip := range c.InstalledPlugins {
			mc.InstalledPlugins = append(mc.InstalledPlugins, mcp.MarketplaceInstalledPlugin{
				Name:          ip.Name,
				Version:       ip.Version,
				ManifestURL:   ip.ManifestURL,
				InstalledAt:   ip.InstalledAt,
				Path:          ip.Path,
				SHA256:        ip.SHA256,
				AutoUpdate:    ip.AutoUpdate,
				LatestVersion: ip.LatestVersion,
			})
		}
		cfgNow := cfgMgr.Get()
		cfgNow.Marketplace = mc
		return cfgMgr.Update(cfgNow)
	})

	cancelAutoUpdate := mp.StartAutoUpdateLoop(ctx)
	defer cancelAutoUpdate()

	ws := web.New(services, port, mp, wasmLoader)
	mux.Handle("/", ws.Handler())

	addr := fmt.Sprintf(":%d", port)
	fmt.Fprintf(os.Stderr, "Switchboard %s on http://localhost:%d\n", version.String(), port)
	fmt.Fprintf(os.Stderr, "  Web UI:  http://localhost:%d/\n", port)
	fmt.Fprintf(os.Stderr, "  MCP:     http://localhost:%d/mcp\n", port)
	fmt.Fprintf(os.Stderr, "  Project: http://localhost:%d/mcp/{project}\n", port)

	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	<-browserDone
	if browserSvc != nil {
		browserSvc.Close() //nolint:errcheck
	}
}
