package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"goisekai/internal/bridge"
	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/httpserver"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
	"goisekai/internal/version"
)

func main() {
	// Handle "stop" subcommand before flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "stop" {
		runStop()
		return
	}

	logLevel := flag.String("logLevel", "", "log level: debug|info|warning (overrides goisekai.ini log_level)")
	host := flag.String("host", "", "HTTP server bind address (overrides goisekai.ini host)")
	port := flag.Int("port", 0, "HTTP server port (overrides goisekai.ini port)")
	open := flag.Bool("open", false, "open the default browser on startup")
	genIni := flag.Bool("genini", false, "generate a default goisekai.ini and exit")
	cdpEngine := flag.String("cdpEngine", "", "CDP browser engine for anti-bot solving: off|lightpanda|obscura|chrome (overrides goisekai.ini cdp_engine)")
	cdpPath := flag.String("cdpPath", "", "browser binary path (chrome) or CDP ws:// URL (lightpanda/obscura) (overrides goisekai.ini cdp_path)")
	apiKey := flag.String("apiKey", "", "API key for /api/* endpoints (overrides goisekai.ini api_key)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	// Config file: goisekai.ini in the working directory, overridable via
	// GOISEKAI_CONFIG.
	cfgPath := os.Getenv("GOISEKAI_CONFIG")
	if cfgPath == "" {
		cfgPath = "goisekai.ini"
	}

	if *genIni {
		if err := config.Default().Save(cfgPath); err != nil {
			log.Fatalf("generate config: %v", err)
		}
		log.Printf("wrote default config to %s", cfgPath)
		return
	}

	// Auto-generate a default config on first run so there is a file to edit.
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := config.Default().Save(cfgPath); err != nil {
			log.Fatalf("generate config: %v", err)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// CLI flags override config values.
	if *host != "" {
		cfg.Host = *host
	}
	if *port != 0 {
		cfg.Port = *port
	}
	if *cdpEngine != "" {
		cfg.CDPEngine = *cdpEngine
	}
	if *cdpPath != "" {
		cfg.CDPPath = *cdpPath
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}

	// Logger level: flag overrides config, config overrides the "info" default.
	level := cfg.LogLevel
	if *logLevel != "" {
		level = *logLevel
	}
	if err := logger.Init(level); err != nil {
		log.Fatalf("init logger: %v", err)
	}
	logger.Info("starting goIsekai", "log_level", level, "data_dir", cfg.DataDir, "addr", cfg.Host, "port", cfg.Port)

	dataDir, pluginsDir, infoDir, cacheDir := setupDirs(cfg)

	// PID file — written after dataDir exists, removed on shutdown.
	removePID := writePIDFile(dataDir)
	defer removePID()

	db, err := database.Open(filepath.Join(dataDir, "goisekai.db"))
	if err != nil {
		logger.Fatal("open database", "error", err)
	}

	// Wire the database to the plugin manager with cache TTLs from config.
	detailTTL := time.Duration(cfg.CacheTTLHours) * time.Hour
	chapterTTL := time.Duration(cfg.ChapterCacheTTLHours) * time.Hour

	proxy := setupProxy(cfg, db)

	// Hot-reload the safe config subset (log level, user-agent, referer,
	// max_cache_gb). maxCacheGB carries the image-cache cap atomically so the
	// maintenance goroutine never races with the watcher.
	maxCacheGB := &atomic.Int64{}
	maxCacheGB.Store(int64(math.Float64bits(cfg.MaxCacheGB)))
	startConfigWatch(cfgPath, proxy, maxCacheGB)

	mgr := pluginmanager.NewManager(proxy, pluginsDir)
	mgr.SetInfoDir(infoDir)
	mgr.SetDB(db, detailTTL)
	mgr.SetChapterCacheTTL(chapterTTL)
	if err := mgr.Discover(); err != nil {
		logger.Fatal("discover plugins", "error", err)
	}

	// Track known hosts and trigger preconnect for all discovered plugins.
	mgr.TrackKnownHosts()

	// Preconnect all known hosts on startup (warm connection pool).
	go preconnectHosts(mgr)

	// Register plugins loaded from the plugins dir so they appear in
	// ListPlugins (Discover only loads them into memory).
	registerLoadedPlugins(db, mgr)

	// Build the enrichment registry. It starts empty: every provider is
	// plugin-declared and registers itself when its plugin first loads, so
	// changing what a source returns is a plugin edit, not a host rebuild.
	enrichReg := enrich.NewRegistry()

	svc := bridge.NewAppService(db, mgr, proxy, cfgPath, cacheDir, enrichReg)
	svc.LogEnhanceStatus()
	svc.SeedVerifyCookies() // re-seed saved human-verify cookies into the proxy
	mgr.SetOnLoad(svc.SyncPluginMeta)
	mgr.SetEnrichRegistry(enrichReg)

	// Signal handling: wait for SIGTERM/SIGINT, then shut down gracefully.
	mainCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Maintenance: prune orphaned rows at startup, then back up + re-prune
	// on the configured interval onto the maintenance lane.
	maintenanceStop := startMaintenance(mainCtx, db, cfg, dataDir, maxCacheGB, svc.GetPool())
	defer close(maintenanceStop)

	// Hourly background refresh of stale library manga (update_stale_days).
	schedulerStop := make(chan struct{})
	bridge.StartLibraryScheduler(svc, schedulerStop)
	defer close(schedulerStop)

	// cfg.HotReload re-reads + recompiles changed Lua templates per render, so
	// a .lua edit takes effect on refresh with no rebuild or restart. Off by
	// default: renders then execute the startup bytecode cache and never touch
	// the template tree.
	eng, err := templates.New(os.DirFS(cfg.TemplatesDir), cfg.HotReload)
	if err != nil {
		log.Fatalf("init templates: %v", err)
	}

	srv := httpserver.New(cfg.Host, cfg.Port, cfg.APIKey, os.DirFS(cfg.FrontendDir), svc, nil, slog.Default(), eng)
	if *open {
		srv.OpenBrowser()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-mainCtx.Done():
		logger.Info("received shutdown signal", "signal", mainCtx.Err())
	case err := <-errCh:
		// ErrServerClosed means the listener was closed on purpose — by the
		// POST /api/shutdown endpoint — and falls through to the ordered
		// shutdown below (logger.Fatal would os.Exit and skip it). Anything
		// else (port bind failure, etc.) is fatal.
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("http server", "error", err)
		}
		logger.Info("http server closed by shutdown request")
	}

	// Ordered shutdown: HTTP → worker pool → plugins → DB → logs → PID file (PID is deferred).
	svc.Shutdown()
	shutdown(srv, mgr, db)
}
