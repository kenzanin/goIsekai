package pluginmanager

import (
	"fmt"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// yaegiABIFns are the ABI entry points the host can invoke on a plugin.
// Wrappers are generated at load time for every function the plugin defines.
var yaegiABIFns = []string{
	types.SearchFunc,
	types.GetMangaDetailFunc,
	types.GetChapterListFunc,
	types.GetPageListFunc,
	types.GetGenresFunc,
}

// yaegiPlugin wraps a Yaegi interpreter instance for a single plugin.
type yaegiPlugin struct {
	i      *interp.Interpreter
	proxy  *hostnet.Proxy
	plugin string // plugin ID
}

// loadYaegi loads a Go plugin from source using the Yaegi interpreter.
// The interpreter runs untrusted Go code in a sandbox: only stdlib + hostnet
// are exposed. Third-party imports fail the import-check before evaluation.
func (m *Manager) loadYaegi(id, dir string) (*loadedPlugin, error) {
	src, err := readFile(dir + "/main.go")
	if err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: read main.go: %w", id, err)
	}

	// Check required functions exist in source.
	srcStr := string(src)
	for _, fn := range yaegiRequired {
		if !strings.Contains(srcStr, "func "+fn+"(") {
			return nil, fmt.Errorf("yaegi plugin %s: missing required function %s", id, fn)
		}
	}

	// Verify: reject third-party imports.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: parse failed: %w", id, err)
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		// Allow "hostnet" — the synthetic host-provided package.
		if path == "hostnet" {
			continue
		}
		if !isGoStdlib(path) {
			return nil, fmt.Errorf("yaegi plugin %s: imports disallowed package %q", id, path)
		}
	}

	// Build Yaegi interpreter. stdlib.Symbols exposes the whole standard
	// library in binary form; yaegi can parse stdlib source but not
	// bogdanfinn/fhttp, so hostnet functions are exposed through
	// stdlib-only wrappers below.
	i := interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: stdlib: %w", id, err)
	}

	// Expose hostnet package with all helper functions. The HTML helpers live in
	// the same synthetic package as Get/Post: the bridge's shape is plain
	// functions in one package, and a plugin's only permitted non-stdlib import
	// is `hostnet`.
	hp := yaegiHostPkg{proxy: m.proxy, id: id}
	if err := i.Use(interp.Exports{
		"hostnet/hostnet": map[string]reflect.Value{
			"Get":           reflect.ValueOf(hp.Get),
			"Post":          reflect.ValueOf(hp.Post),
			"Parse":         reflect.ValueOf(hp.Parse),
			"FindText":      reflect.ValueOf(hp.FindText),
			"FindAttr":      reflect.ValueOf(hp.FindAttr),
			"FindListText":  reflect.ValueOf(hp.FindListText),
			"FindListAttr":  reflect.ValueOf(hp.FindListAttr),
			"XPathText":     reflect.ValueOf(hp.XPathText),
			"XPathAttr":     reflect.ValueOf(hp.XPathAttr),
			"XPathListText": reflect.ValueOf(hp.XPathListText),
			"XPathListAttr": reflect.ValueOf(hp.XPathListAttr),
		},
	}); err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: host exports: %w", id, err)
	}

	// Evaluate plugin source.
	if _, err := i.Eval(srcStr); err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: compile failed: %w", id, err)
	}

	// Register a per-function wrapper shim (same scope as the plugin source)
	// that collapses the (string, error) ABI into a single string, panicking
	// on error so it surfaces as an Eval error. Calling a multi-return ABI
	// function directly via Eval would silently drop the error value.
	// ponytail: static wrappers per defined fn; dynamic dispatch by name isn't
	// possible in interpreted Go; one wrapper per ABI fn, generated at load.
	var wb strings.Builder
	wb.WriteString("package main\n")
	for _, fn := range yaegiABIFns {
		if !strings.Contains(srcStr, "func "+fn+"(") {
			continue
		}
		fmt.Fprintf(&wb,
			"func gskCall_%s(arg string) string { s, e := %s(arg); if e != nil { panic(\"plugin error: \" + e.Error()) }; return s }\n",
			fn, fn)
	}
	if _, err := i.Eval(wb.String()); err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: wrapper compile failed: %w", id, err)
	}

	yp := &yaegiPlugin{
		i:      i,
		proxy:  m.proxy,
		plugin: id,
	}

	// Metadata lives in plugin.json beside the entry point, never in the VM:
	// the host already read it at discovery, so a plugin never has to execute
	// just to be listed with its name and logo.
	meta, err := pluginMetaFor(id, "yaegi", dir)
	if err != nil {
		return nil, err
	}
	contractVer := meta.ContractVersion
	if contractVer == 0 {
		contractVer = types.ContractVersion
	}

	p := &loadedPlugin{
		id:              id,
		wasmPath:        dir,
		kind:            "yaegi",
		loaded:          true,
		contractVersion: contractVer,
		yaegi:           yp,
		meta:            meta,
	}

	logger.Info("yaegi plugin loaded", "id", id)
	return p, nil
}
