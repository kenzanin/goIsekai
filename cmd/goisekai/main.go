package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goisekai/internal/bridge"
	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/httpserver"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
)

//go:embed all:frontend
var assets embed.FS

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
	flag.Parse()

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

	// Data directory holds the SQLite file and the plugins/ wasm directory.
	dataDir := cfg.DataDir
	pluginsDir := filepath.Join(dataDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		logger.Fatal("mkdir plugins dir", "error", err)
	}

	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(dataDir, "cache")
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "images"), 0o755); err != nil {
		logger.Fatal("mkdir cache dir", "error", err)
	}

	// PID file — written after dataDir exists, removed on shutdown.
	pidPath := filepath.Join(dataDir, "goisekai.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		logger.Warn("write PID file", "error", err)
	} else {
		defer func() {
			logger.Info("removing PID file", "path", pidPath)
			os.Remove(pidPath)
		}()
	}

	db, err := database.Open(filepath.Join(dataDir, "goisekai.db"))
	if err != nil {
		logger.Fatal("open database", "error", err)
	}

	proxy := hostnet.NewProxy()
	proxy.SetDefaultHeader("User-Agent", cfg.UserAgent)
	proxy.SetDefaultHeader("Accept-Language", cfg.AcceptLanguage)
	proxy.SetDefaultHeader("Referer", cfg.Referer)
	proxy.ConfigureCDP(hostnet.CDPConfig{
		Engine:  cfg.CDPEngine,
		Path:    cfg.CDPPath,
		Timeout: time.Duration(cfg.CDPSolveTimeout) * time.Second,
	})

	// Hot-reload: poll goisekai.ini every 5s and apply the safe subset
	// (log level, user-agent, referer) live. Unsafe fields like host/port/
	// cdp_engine need a restart, so they are deliberately not applied here.
	_ = config.Watch(cfgPath, 5*time.Second, func(updated *config.Config) {
		if err := logger.Init(updated.LogLevel); err == nil {
			logger.Info("config reloaded", "log_level", updated.LogLevel)
		}
		proxy.SetDefaultHeader("User-Agent", updated.UserAgent)
		proxy.SetDefaultHeader("Referer", updated.Referer)
	})

	mgr := pluginmanager.NewManager(proxy, pluginsDir)
	if err := mgr.Discover(); err != nil {
		logger.Fatal("discover plugins", "error", err)
	}

	// Register plugins loaded from the plugins dir so they appear in
	// ListPlugins (Discover only loads them into memory).
	for _, p := range mgr.LoadedPlugins() {
		if err := db.RegisterPlugin(database.Plugin{
			ID:         p.ID,
			Name:       p.ID,
			Version:    p.Version,
			WasmPath:   p.WasmPath,
			IsActive:   true,
			ThumbRatio: p.ThumbRatio,
		}); err != nil {
			logger.Error("register discovered plugin", "id", p.ID, "error", err)
		}
	}

	svc := bridge.NewAppService(db, mgr, proxy, cfgPath, cacheDir)

	eng, err := templates.New(false)
	if err != nil {
		log.Fatalf("init templates: %v", err)
	}

	srv := httpserver.New(cfg.Host, cfg.Port, cfg.APIKey, assets, svc, slog.Default(), eng)
	if *open {
		srv.OpenBrowser()
	}

	// Signal handling: wait for SIGTERM/SIGINT, then shut down gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("received shutdown signal", "signal", ctx.Err())
	case err := <-errCh:
		// Server exited on its own (port bind failure, etc.).
		logger.Fatal("http server", "error", err)
	}

	// Ordered shutdown: HTTP → plugins → DB → logs → PID file (PID is deferred).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown", "error", err)
	}

	logger.Info("closing plugins")
	if err := mgr.Close(); err != nil {
		logger.Error("plugin manager close", "error", err)
	}

	logger.Info("closing database")
	if err := db.Close(); err != nil {
		logger.Error("database close", "error", err)
	}

	logger.Info("shutdown complete")
}

// runStop reads the PID file from the data directory and sends SIGTERM.
func runStop() {
	cfgPath := os.Getenv("GOISEKAI_CONFIG")
	if cfgPath == "" {
		cfgPath = "goisekai.ini"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pidPath := filepath.Join(cfg.DataDir, "goisekai.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		log.Fatalf("read PID file: %v", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Fatalf("invalid PID %q: %v", pidStr, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("find process %d: %v", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		log.Fatalf("send SIGTERM to %d: %v", pid, err)
	}

	fmt.Printf("sent SIGTERM to process %d\n", pid)
}
