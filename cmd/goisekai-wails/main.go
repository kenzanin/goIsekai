package main

import (
	"embed"
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// App exposes the server URL to the embedded launcher page, which then
// redirects the webview to the running goIsekai server. This keeps the
// wrapper thin: no UI re-implementation, the web UI is served by the
// server itself.
type App struct {
	serverURL string
}

// GetServerURL returns the goIsekai server base URL the wrapper should load.
func (a *App) GetServerURL() string { return a.serverURL }

func main() {
	server := flag.String("server", "http://127.0.0.1:8080", "goIsekai server base URL")
	title := flag.String("title", "goIsekai", "window title")
	width := flag.Int("width", 1280, "window width")
	height := flag.Int("height", 860, "window height")
	flag.Parse()

	app := &App{serverURL: *server}

	err := wails.Run(&options.App{
		Title:  *title,
		Width:  *width,
		Height: *height,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
