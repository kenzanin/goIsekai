# goisekai-wails

Wails v2 webview wrapper around a running goIsekai server. This is a **thin
launcher**: it opens a desktop window whose webview loads the server's existing
web UI over HTTP. No UI is re-implemented here — the library grid, manga detail,
reader, plugins, settings, everything is served by `cmd/goisekai` exactly as in a
browser.

This is its own Go module (`go.mod` here), so the heavy Wails dependency tree
never touches the main server's `go.mod`. Go's `./cmd/...` patterns skip nested
modules automatically, so `just build` / `just test` are unaffected.

## Why a webview wrapper

The Qt6/QML client (`goisekai-qt6/`) re-implements every page in QML and stalls
whenever the web UI gains a feature. A Wails wrapper gets every page for free by
loading the real web UI, and picks up web-side fixes (like the adaptive 5×5
library grid) without any client change.

## Requirements

- Go 1.27+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Linux: webkit2gtk-4.1 dev packages (Wails system deps)

## Build

```sh
just build-wails        # -> bin/goisekai-wails
# or manually:
cd cmd/goisekai-wails && go build -o ../../bin/goisekai-wails .
```

## Run

Start the server first, then the wrapper:

```sh
./bin/goisekai                              # defaults to http://127.0.0.1:8080
./bin/goisekai-wails -server http://192.168.1.5:8080 -width 1440 -height 900
```

Flags:

| flag     | default               | meaning                       |
|----------|-----------------------|-------------------------------|
| `-server`| `http://127.0.0.1:8080` | goIsekai server base URL    |
| `-title` | `goIsekai`            | window title                  |
| `-width` | `1280`                | window width                  |
| `-height`| `860`                 | window height                 |

## How it works

`main.go` embeds `frontend/dist/index.html` and binds an `App` method
`GetServerURL()`. On startup the launcher page calls that bound method and
`window.location.href`-redirects the webview to the server URL. After that the
webview is just browsing the server's web UI.

## Scope

- Wrapper only: library, manga detail, reader, plugins, settings, logs — all via
  the server's web UI. Nothing is duplicated.
- The server must already be running; the wrapper does not start or embed it.
