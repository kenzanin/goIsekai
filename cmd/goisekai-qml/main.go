//go:build cgo

// Command goisekai-qml is the Qt Quick desktop client for goIsekai.
//
// It is a thin HTTP client over a running goisekai server: it reads the same
// /api JSON the browser UI uses, so no business logic lives here. The library
// page is the first screen; clicking a card still opens the web reader.
//
// This package is the only place in the repo that needs cgo. The build tag
// above keeps it out of every CGO_ENABLED=0 invocation - `just build`,
// `just test`, `just lint` and any `go build ./...` - because Go would
// otherwise try to resolve the cgo-only miqt import and fail with "build
// constraints exclude all Go files".
//
// Build with `just build-qml` (CGO_ENABLED=1, needs Qt 6 dev + kimageformats
// for AVIF covers).
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/qml"

	"goisekai/cmd/goisekai-qml/client"
)

// qmlFS holds the QML tree. It is unpacked to a temp directory at startup
// because relative imports (`import "components"`, `import ".."` for the Theme
// singleton) are resolved against the loaded file's URL, and LoadData has no
// base URL to resolve them against.
//
//go:embed qml
var qmlFS embed.FS

// unpackQML writes the embedded tree to a fresh temp dir and returns its path.
func unpackQML() (string, error) {
	root, err := os.MkdirTemp("", "goisekai-qml-*")
	if err != nil {
		return "", err
	}
	err = fs.WalkDir(qmlFS, "qml", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("qml", path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := qmlFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return "", err
	}
	return root, nil
}

func main() {
	base := flag.String("server", "http://127.0.0.1:8080", "goisekai server base URL")
	apiKey := flag.String("apiKey", "", "X-API-Key, when the server sets api_key")
	flag.Parse()

	// An unreachable server is not fatal: the grid renders empty with the
	// reason in the header, which beats refusing to start.
	items, err := client.FetchLibrary(*base, *apiKey)
	status := fmt.Sprintf("%d manga", len(items))
	if err != nil {
		status = "offline — " + err.Error()
		fmt.Fprintf(os.Stderr, "goisekai-qml: %v\n", err)
	}
	facets, ferr := client.FetchFacets(*base, *apiKey)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "goisekai-qml: facets: %v\n", ferr)
	}

	libraryJSON, err := json.Marshal(items)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goisekai-qml: encode library: %v\n", err)
		os.Exit(1)
	}
	facetsJSON, err := json.Marshal(facets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goisekai-qml: encode facets: %v\n", err)
		os.Exit(1)
	}

	qmlRoot, err := unpackQML()
	if err != nil {
		fmt.Fprintf(os.Stderr, "goisekai-qml: unpack qml: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(qmlRoot) }()

	qt.NewQApplication(os.Args)
	engine := qml.NewQQmlApplicationEngine()

	// A QML error must be loud: a window that opens blank is otherwise
	// indistinguishable from a bug in the data.
	engine.OnObjectCreationFailed(func(_ *qt.QUrl) {
		fmt.Fprintln(os.Stderr, "goisekai-qml: failed to create the QML root object")
		os.Exit(1)
	})

	engine.RootContext().SetContextProperty2("libraryJson", qt.NewQVariant14(string(libraryJSON)))
	engine.RootContext().SetContextProperty2("facetsJson", qt.NewQVariant14(string(facetsJSON)))
	engine.RootContext().SetContextProperty2("statusText", qt.NewQVariant14(status))
	engine.RootContext().SetContextProperty2("serverBase", qt.NewQVariant14(*base))

	engine.Load(qt.QUrl_FromLocalFile(filepath.Join(qmlRoot, "Main.qml")))
	qt.QApplication_Exec()
}
