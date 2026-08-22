package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/awm"
	"github.com/daltoniam/switchboard/awmgrpc"
	"github.com/daltoniam/switchboard/browser"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/daemon"
	acpInt "github.com/daltoniam/switchboard/integrations/acp"
	agentsInt "github.com/daltoniam/switchboard/integrations/agents"
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
	"github.com/daltoniam/switchboard/integrations/gcal"
	"github.com/daltoniam/switchboard/integrations/gchat"
	gcpInt "github.com/daltoniam/switchboard/integrations/gcp"
	"github.com/daltoniam/switchboard/integrations/gdocs"
	"github.com/daltoniam/switchboard/integrations/gdrive"
	"github.com/daltoniam/switchboard/integrations/gforms"
	"github.com/daltoniam/switchboard/integrations/github"
	"github.com/daltoniam/switchboard/integrations/gmail"
	"github.com/daltoniam/switchboard/integrations/gmeet"
	"github.com/daltoniam/switchboard/integrations/gong"
	"github.com/daltoniam/switchboard/integrations/gpeople"
	"github.com/daltoniam/switchboard/integrations/gsheets"
	"github.com/daltoniam/switchboard/integrations/gslides"
	"github.com/daltoniam/switchboard/integrations/gtasks"
	"github.com/daltoniam/switchboard/integrations/jira"
	"github.com/daltoniam/switchboard/integrations/kubernetes"
	"github.com/daltoniam/switchboard/integrations/linear"
	"github.com/daltoniam/switchboard/integrations/metabase"
	"github.com/daltoniam/switchboard/integrations/netsuite"
	nomadInt "github.com/daltoniam/switchboard/integrations/nomad"
	notionInt "github.com/daltoniam/switchboard/integrations/notion"
	"github.com/daltoniam/switchboard/integrations/ollama"
	"github.com/daltoniam/switchboard/integrations/pganalyze"
	"github.com/daltoniam/switchboard/integrations/postgres"
	"github.com/daltoniam/switchboard/integrations/posthog"
	"github.com/daltoniam/switchboard/integrations/projectinterop"
	"github.com/daltoniam/switchboard/integrations/ramp"
	"github.com/daltoniam/switchboard/integrations/rwx"
	"github.com/daltoniam/switchboard/integrations/salesforce"
	"github.com/daltoniam/switchboard/integrations/sentry"
	signozInt "github.com/daltoniam/switchboard/integrations/signoz"
	slackInt "github.com/daltoniam/switchboard/integrations/slack"
	"github.com/daltoniam/switchboard/integrations/slackmcp"
	snowflakeInt "github.com/daltoniam/switchboard/integrations/snowflake"
	"github.com/daltoniam/switchboard/integrations/stripe"
	"github.com/daltoniam/switchboard/integrations/suno"
	switchboardInt "github.com/daltoniam/switchboard/integrations/switchboard"
	"github.com/daltoniam/switchboard/integrations/vercel"
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
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		handleDaemon(os.Args[2:])
		return
	}

	stdioMode := flag.Bool("stdio", false, "Run MCP server over stdio transport (default is HTTP)")
	port := flag.Int("port", 3847, "Port for the HTTP/MCP server and shared h2c gRPC listener")
	listenHost := flag.String("listen-host", "127.0.0.1", "TCP listen host for HTTP/MCP and shared h2c gRPC (default loopback; set 0.0.0.0 to expose)")
	grpcSocket := flag.String("grpc-socket", "", "Optional Unix-domain socket for native AWM gRPC only (does not serve HTTP/MCP)")
	discoverAll := flag.Bool("discover-all", false, "Search returns tools from all registered integrations, not just enabled ones")
	verbose := flag.Bool("verbose", false, "Enable debug logging (compaction savings, request processing)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("switchboard %s\n", version.Full())
		os.Exit(0)
	}

	configureLogging(*verbose)
	runServer(*stdioMode, *port, *listenHost, *grpcSocket, *discoverAll)
}

func configureLogging(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if verbose {
		slog.Debug("verbose logging enabled")
	}
}

func handleDaemon(args []string) {
	opts, err := parseDaemonArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	port := opts.port
	verbose := opts.verbose
	cmd := opts.cmd

	switch cmd {
	case "install":
		if err := daemon.Install(port, verbose); err != nil {
			log.Fatalf("Install failed: %v", err)
		}
		fmt.Println("Service installed. Run 'switchboard daemon start' to start.")
	case "uninstall":
		if err := daemon.Uninstall(); err != nil {
			log.Fatalf("Uninstall failed: %v", err)
		}
	case "start":
		status, _ := daemon.GetStatus(port)
		if status != nil && status.Running {
			fmt.Printf("Switchboard is already running (PID %d)\n", status.PID)
			os.Exit(0)
		}
		if err := daemon.Start(port, verbose); err != nil {
			log.Fatalf("Start failed: %v", err)
		}
		time.Sleep(time.Second)
		status, _ = daemon.GetStatus(port)
		if status != nil && status.Running {
			fmt.Printf("Switchboard started (PID %d) on port %d\n", status.PID, port)
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
		status, err := daemon.GetStatus(port)
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
			fmt.Printf("Health: OK (port %d)\n", port)
		} else {
			fmt.Printf("Health: NOT OK (port %d)\n", port)
		}
		if daemon.IsServiceInstalled() {
			fmt.Println("Service: installed")
		}
	case "logs":
		if daemon.IsSystemdInstalled() {
			fmt.Println("journalctl --user -u switchboard -f")
			return
		}
		logPath, err := daemon.LogPath()
		if err != nil {
			log.Fatalf("Failed to get log path: %v", err)
		}
		fmt.Println(logPath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown daemon command: %s\n", cmd)
		os.Exit(1)
	}
}

type daemonOpts struct {
	cmd     string
	port    int
	verbose bool
}

func parseDaemonArgs(args []string) (daemonOpts, error) {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", 3847, "Port for the HTTP server")
	verbose := fs.Bool("verbose", false, "Enable debug logging in the installed/started daemon")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: switchboard daemon <command> [options]

Commands:
  install     Install as a system service (launchd on macOS, systemd on Linux)
  uninstall   Remove the system service
  start       Start the daemon
  stop        Stop the daemon
  status      Show daemon status
  logs        Show how to follow daemon logs

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return daemonOpts{}, err
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		fs.Usage()
		return daemonOpts{}, fmt.Errorf("daemon command required")
	}

	cmd := remaining[0]
	if len(remaining) > 1 {
		if err := fs.Parse(remaining[1:]); err != nil {
			return daemonOpts{}, err
		}
	}

	return daemonOpts{cmd: cmd, port: *port, verbose: *verbose}, nil
}

func runServer(stdioMode bool, port int, listenHost, grpcSocket string, discoverAll bool) {
	cfgMgr, err := config.NewManager()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	browserSvc := newLazyBrowserService(func(ctx context.Context) (mcp.BrowserService, error) {
		svc, err := browser.New(true /* headless */)
		if err != nil {
			return nil, fmt.Errorf("browser service unavailable: %w", err)
		}
		log.Println("browser service ready — Amazon browser features enabled")
		return svc, nil
	})
	defer func() {
		if err := browserSvc.Close(); err != nil {
			log.Printf("browser service close failed: %v", err)
		}
	}()

	gmailIntegration := gmail.New()
	gcalIntegration := gcal.New()
	gdriveIntegration := gdrive.New()
	gdocsIntegration := gdocs.New()
	gsheetsIntegration := gsheets.New()
	gslidesIntegration := gslides.New()
	gformsIntegration := gforms.New()
	gtasksIntegration := gtasks.New()
	gchatIntegration := gchat.New()
	gpeopleIntegration := gpeople.New()
	gmeetIntegration := gmeet.New()
	amazonIntegration := amazon.New()

	// One process-wide filesystem catalog. ProjectInterop and the
	// project-scoped router receive this same object; neither allocates
	// another store.
	projectIntegrationConfig, _ := cfgMgr.GetIntegration("projectinterop")
	projectStore := project.NewStore(projectConfigRoot(projectIntegrationConfig))
	if err := projectStore.Load(); err != nil {
		log.Fatalf("Failed to load project catalog: %v", err)
	}
	if names := projectStore.Names(); len(names) > 0 {
		log.Printf("Loaded %d project(s): %v", len(names), names)
	}

	reg := registry.New()
	for _, i := range []mcp.Integration{
		github.New(),
		datadog.New(),
		linear.New("https://mcp.linear.app"),
		sentry.New(),
		slackInt.New(),
		slackmcp.New(),
		metabase.New(),
		awsInt.New(),
		posthog.New(),
		postgres.New(),
		clickhouse.New(),
		elasticsearch.New(),
		pganalyze.New(),
		rwx.New(),
		projectinterop.NewWithCatalog(projectStore),
		ramp.New(),
		ynab.New(),
		stripe.New(),
		amazonIntegration,
		gmailIntegration,
		gong.New(),
		gcalIntegration,
		gdriveIntegration,
		gdocsIntegration,
		gsheetsIntegration,
		gslidesIntegration,
		gformsIntegration,
		gtasksIntegration,
		gchatIntegration,
		gpeopleIntegration,
		gmeetIntegration,
		jira.New(),
		confluence.New(),
		notionInt.New(),
		ollama.New(),
		gcpInt.New(),
		suno.New(),
		salesforce.New(),
		netsuite.New(),
		cloudflare.New(),
		digitalocean.New(),
		flyInt.New(),
		kubernetes.New(),
		vercel.New(),
		snowflakeInt.New(),
		acpInt.New(),
		agentsInt.New(),
		signozInt.New(),
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
		Metrics:  mcp.NewMetrics().WithPersistence(metricsPath()),
	}

	switchboardIntegration := switchboardInt.New(services)
	if err := reg.Register(switchboardIntegration); err != nil {
		log.Fatalf("Failed to register switchboard integration: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() { _ = projectStore.Watch(ctx) }()

	// Periodic metrics flush. Flush is a no-op when not dirty, so this
	// produces zero disk traffic during idle periods.
	startMetricsFlusher(ctx, services.Metrics, 5*time.Minute)
	defer func() {
		if err := services.Metrics.Flush(); err != nil {
			log.Printf("metrics: final flush failed: %v", err)
		}
	}()

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
	gcal.SetConfigService(gcalIntegration, cfgMgr)
	gdrive.SetConfigService(gdriveIntegration, cfgMgr)
	gdocs.SetConfigService(gdocsIntegration, cfgMgr)
	gsheets.SetConfigService(gsheetsIntegration, cfgMgr)
	gslides.SetConfigService(gslidesIntegration, cfgMgr)
	gforms.SetConfigService(gformsIntegration, cfgMgr)
	gtasks.SetConfigService(gtasksIntegration, cfgMgr)
	gchat.SetConfigService(gchatIntegration, cfgMgr)
	gpeople.SetConfigService(gpeopleIntegration, cfgMgr)
	gmeet.SetConfigService(gmeetIntegration, cfgMgr)
	amazon.SetBrowserService(amazonIntegration, browserSvc)

	var serverOpts []server.Option
	if discoverAll {
		serverOpts = append(serverOpts, server.WithDiscoverAll(true))
	}
	if cfg.SessionStore == "file" {
		serverOpts = append(serverOpts, server.WithSessionStore(
			server.NewFileSessionStore(server.DefaultSessionDir(), server.DefaultSessionTTL),
		))
	}
	// Work profiles/sessions/agent profiles live under the Switchboard config root.
	workStore := awm.NewStore(projectStore.ConfigDir())
	workStore.SetCatalog(projectStore)
	projectStore.SetResourcePresence(workStore)
	if _, err := workStore.EnsureDefaultWorkProfile(ctx); err != nil {
		log.Printf("WARN: ensure default work profile: %v", err)
	}
	catalogWrites := mcp.ProjectCatalogWritesEnabled(cfg.ProjectCatalog)
	serverOpts = append(serverOpts, server.WithProjectWorkModel(workStore, catalogWrites))
	log.Printf("Project work-model store: %s (writes_enabled=%v)", workStore.Root(), catalogWrites)
	// Shared integrity rule for canonical delete and projectinterop DeleteCompatibility.
	projectStore.SetDeleteGuard(workStore)

	if mcp.ProjectCatalogEnabled(cfg.ProjectCatalog) {
		bus := project.NewEventBus()
		projectStore.SetEventBus(bus)
		catalogSrv := server.NewProjectCatalogServer(projectStore, projectStore, projectStore, projectStore, server.ProjectCatalogOptions{
			WritesEnabled: catalogWrites,
		})
		catalogSrv.SetWorkGuard(workStore)
		catalogSrv.StartEventBridge(bus)
		serverOpts = append(serverOpts, server.WithProjectCatalog(catalogSrv))
		log.Printf("Project Catalog on /mcp (writes_enabled=%v)", catalogWrites)
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

	projectRouter := server.NewProjectRouter(services, projectStore, "", srv.SearchIndex())

	mux := server.BuildHTTPMux(server.HTTPMuxConfig{
		MCP:     srv.StatelessHandler(),
		Project: projectRouter.Handler(),
	})

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
	}, marketplace.WithTokenFunc(marketplace.GitHubTokenFunc(func() string {
		ic, ok := cfgMgr.GetIntegration("github")
		if !ok || ic == nil {
			return ""
		}
		return ic.Credentials["token"]
	})))

	switchboardInt.SetMarketplace(switchboardIntegration, mp)

	cancelAutoUpdate := mp.StartAutoUpdateLoop(ctx)
	defer cancelAutoUpdate()

	ws := web.New(services, port, mp, wasmLoader,
		web.WithConfigChangeHook(srv.RefreshSearchIndex),
		web.WithProjectCatalog(projectStore),
		web.WithAWMStore(workStore),
	)
	mux.Handle("/", ws.Handler())

	// Native gRPC shares the HTTP port over h2c and uses the exact same
	// catalog/work stores as MCP. No generic JSON dispatch exists on this path.
	grpcOpts := awmgrpc.Options{
		CatalogEnabled: mcp.ProjectCatalogEnabled(cfg.ProjectCatalog),
		WritesEnabled:  catalogWrites,
	}
	grpcServer := awmgrpc.NewServer(projectStore, projectStore, projectStore, workStore, grpcOpts)
	defer grpcServer.GracefulStop()
	protocolHandler := awmgrpc.MultiplexHTTPAndGRPC(grpcServer, mux)

	addr, err := awmgrpc.TCPListenAddr(listenHost, port)
	if err != nil {
		log.Fatalf("Invalid listen address: %v", err)
	}
	displayHost := listenHost
	if displayHost == "" {
		displayHost = "127.0.0.1"
	}
	fmt.Fprintf(os.Stderr, "Switchboard %s on http://%s:%d\n", version.String(), displayHost, port)
	fmt.Fprintf(os.Stderr, "  Web UI:  http://%s:%d/\n", displayHost, port)
	fmt.Fprintf(os.Stderr, "  MCP:     http://%s:%d/mcp\n", displayHost, port)
	fmt.Fprintf(os.Stderr, "  Project: http://%s:%d/mcp/{project}\n", displayHost, port)
	fmt.Fprintf(os.Stderr, "  AWM gRPC (h2c): %s\n", addr)

	var udsListener net.Listener
	if grpcSocket != "" {
		uds, err := awmgrpc.ListenUnix(grpcSocket)
		if err != nil {
			log.Fatalf("AWM gRPC unix socket: %v", err)
		}
		udsListener = uds
		fmt.Fprintf(os.Stderr, "  AWM gRPC (UDS): unix://%s (native gRPC only; HTTP/MCP stay on TCP)\n", grpcSocket)
		go func() {
			if err := grpcServer.Serve(uds); err != nil && err != grpc.ErrServerStopped {
				log.Printf("AWM gRPC unix socket error: %v", err)
			}
		}()
	}

	httpServer := &http.Server{Addr: addr, Handler: protocolHandler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = httpServer.Close()
		if udsListener != nil {
			_ = udsListener.Close()
		}
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

}

type lazyBrowserService struct {
	mu      sync.Mutex
	new     func(ctx context.Context) (mcp.BrowserService, error)
	service mcp.BrowserService
	closed  bool
}

func newLazyBrowserService(newFn func(ctx context.Context) (mcp.BrowserService, error)) *lazyBrowserService {
	return &lazyBrowserService{new: newFn}
}

func (s *lazyBrowserService) NewSession(ctx context.Context) (mcp.BrowserSession, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("browser service closed")
	}
	if s.service == nil {
		svc, err := s.new(ctx)
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.service = svc
	}
	svc := s.service
	s.mu.Unlock()
	return svc.NewSession(ctx)
}

func (s *lazyBrowserService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.service == nil {
		return nil
	}
	return s.service.Close()
}

// metricsPath returns the on-disk path for persisted lifetime metrics.
// Falls back to an empty string (persistence disabled) if the home dir cannot
// be resolved, which only happens in unusual environments where we'd rather
// run without persistence than crash on startup.
func projectConfigRoot(integrationConfig *mcp.IntegrationConfig) string {
	if integrationConfig != nil {
		if root := integrationConfig.Credentials["config_root"]; root != "" {
			return project.ExpandHome(root)
		}
	}
	return project.DefaultConfigDir()
}

func metricsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "switchboard", "metrics.json")
}

// startMetricsFlusher launches a goroutine that flushes lifetime metric
// counters to disk every interval. Flush is a no-op when the dirty flag is
// clear, so this does not produce a steady stream of writes when the server
// is idle. The goroutine exits when ctx is cancelled.
func startMetricsFlusher(ctx context.Context, metrics *mcp.Metrics, interval time.Duration) {
	if metrics == nil || interval <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := metrics.Flush(); err != nil {
					log.Printf("metrics: periodic flush failed: %v", err)
				}
			}
		}
	}()
}
