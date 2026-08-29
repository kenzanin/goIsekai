package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"goisekai/internal/bridge"
	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

//go:embed all:frontend
var assets embed.FS

func main() {
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

	err = wails.Run(&options.App{
		Title:  cfg.Title,
		Width:  cfg.Width,
		Height: cfg.Height,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []any{svc},
	})
	if err != nil {
		log.Fatal(err)
	}
}
