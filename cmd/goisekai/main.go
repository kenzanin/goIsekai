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
	flag.Parse()

	// Config file: goisekai.ini in the working directory, overridable via
	// GOISEKAI_CONFIG.
	cfgPath := os.Getenv("GOISEKAI_CONFIG")
	if cfgPath == "" {
		cfgPath = "goisekai.ini"
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
	logger.Info("starting goIsekai", "level", level, "data_dir", cfg.DataDir)

	// Data directory holds the SQLite file and the plugins/ wasm directory.
	dataDir := cfg.DataDir
	pluginsDir := filepath.Join(dataDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		log.Fatalf("mkdir plugins dir: %v", err)
	}

	db, err := database.Open(filepath.Join(dataDir, "goisekai.db"))
	if err != nil {
		log.Fatalf("open database: %v", err)
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
		log.Fatalf("discover plugins: %v", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

	svc := bridge.NewAppService(db, mgr, proxy)

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
		log.Fatal(err)
	}
}
