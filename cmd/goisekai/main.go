package main

import (
	"embed"
	"flag"
	"log"
	"log/slog"
	"os"
	"path/filepath"
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
	logLevel := flag.String("logLevel", "", "log level: debug|info|warning (overrides goisekai.ini log_level)")
	host := flag.String("host", "", "HTTP server bind address (overrides goisekai.ini host)")
	port := flag.Int("port", 0, "HTTP server port (overrides goisekai.ini port)")
	open := flag.Bool("open", false, "open the default browser on startup")
	genIni := flag.Bool("genini", false, "generate a default goisekai.ini and exit")
	cdpEngine := flag.String("cdpEngine", "", "CDP browser engine for anti-bot solving: off|lightpanda|chrome (overrides goisekai.ini cdp_engine)")
	cdpPath := flag.String("cdpPath", "", "browser binary path (chrome) or CDP ws:// URL (lightpanda) (overrides goisekai.ini cdp_path)")
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

	db, err := database.Open(filepath.Join(dataDir, "goisekai.db"))
	if err != nil {
		logger.Fatal("open database", "error", err)
	}
	defer func() {
		_ = db.Close()
	}()

	proxy := hostnet.NewProxy()
	proxy.SetDefaultHeader("User-Agent", cfg.UserAgent)
	proxy.SetDefaultHeader("Accept-Language", cfg.AcceptLanguage)
	proxy.SetDefaultHeader("Referer", cfg.Referer)
	proxy.ConfigureCDP(hostnet.CDPConfig{
		Engine:  cfg.CDPEngine,
		Path:    cfg.CDPPath,
		Timeout: time.Duration(cfg.CDPSolveTimeout) * time.Second,
	})

	mgr := pluginmanager.NewManager(proxy, pluginsDir)
	if err := mgr.Discover(); err != nil {
		logger.Fatal("discover plugins", "error", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

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

	srv := httpserver.New(cfg.Host, cfg.Port, assets, svc, slog.Default(), eng)
	if *open {
		srv.OpenBrowser()
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal("http server", "error", err)
	}
}
