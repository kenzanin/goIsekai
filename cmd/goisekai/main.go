package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"goisekai/internal/bridge"
	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	logLevel := flag.String("logLevel", "", "log level: debug|info|warning (overrides goisekai.ini log_level)")
	genIni := flag.Bool("genini", false, "generate a default goisekai.ini and exit")
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

	// Logger level: flag overrides config, config overrides the "info" default.
	level := cfg.LogLevel
	if *logLevel != "" {
		level = *logLevel
	}
	if err := logger.Init(level); err != nil {
		log.Fatalf("init logger: %v", err)
	}
	logger.Info("starting goIsekai", "log_level", level, "data_dir", cfg.DataDir)

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
			ID:       p.ID,
			Name:     p.ID,
			Version:  p.Version,
			WasmPath: p.WasmPath,
			IsActive: true,
		}); err != nil {
			logger.Error("register discovered plugin", "id", p.ID, "error", err)
		}
	}

	svc := bridge.NewAppService(db, mgr, proxy, cfgPath, cacheDir)

	app := application.New(application.Options{
		Name: "goIsekai",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(svc),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: cfg.Title, Width: cfg.Width, Height: cfg.Height,
	})
	if err := app.Run(); err != nil {
		logger.Fatal("run app", "error", err)
	}
}
