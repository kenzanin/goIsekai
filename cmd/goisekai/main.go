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
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	// Data directory: configurable for local dev; defaults to a git-ignored
	// ./app_data tree. Holds the SQLite file and the plugins/ wasm directory.
	dataDir := os.Getenv("GOISEKAI_DATA_DIR")
	if dataDir == "" {
		dataDir = "app_data"
	}
	pluginsDir := filepath.Join(dataDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		log.Fatalf("mkdir plugins dir: %v", err)
	}

	db, err := database.Open(filepath.Join(dataDir, "goisekai.db"))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	proxy := hostnet.NewProxy()

	mgr := pluginmanager.NewManager(proxy, pluginsDir)
	if err := mgr.Discover(); err != nil {
		log.Fatalf("discover plugins: %v", err)
	}
	defer mgr.Close()

	svc := bridge.NewAppService(db, mgr, proxy)

	err = wails.Run(&options.App{
		Title:  "goIsekai",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{svc},
	})
	if err != nil {
		log.Fatal(err)
	}
}
