package pluginmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"reflect"
	"runtime/debug"
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
	types.GetAltTitlesFunc,
	types.InitFunc,
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

	hasInit := strings.Contains(srcStr, "func "+types.InitFunc+"(")

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

	// Expose hostnet.Get/Post as a synthetic "hostnet" package. The export
	// key must be import-path/package-name, so bare `import "hostnet"`
	// resolves against "hostnet/hostnet".
	hp := yaegiHostPkg{proxy: m.proxy, id: id}
	if err := i.Use(interp.Exports{
		"hostnet/hostnet": map[string]reflect.Value{
			"Get":  reflect.ValueOf(hp.Get),
			"Post": reflect.ValueOf(hp.Post),
		},
	}); err != nil {
		return nil, fmt.Errorf("yaegi plugin %s: hostnet export: %w", id, err)
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
		if fn == types.InitFunc {
			if w := initWrapperSrc(srcStr); w != "" {
				wb.WriteString(w + "\n")
			}
			continue
		}
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

	p := &loadedPlugin{
		id:              id,
		wasmPath:        dir,
		kind:            "yaegi",
		loaded:          true,
		contractVersion: 1,
		yaegi:           yp,
	}

	// Run Init if present to populate metadata.
	if hasInit {
		out, err := yp.callWithTimeout(context.Background(), types.InitFunc, "")
		if err != nil {
			return nil, fmt.Errorf("yaegi plugin %s: Init: %w", id, err)
		}
		if out != "" {
			var meta types.PluginMeta
			if err := json.Unmarshal([]byte(out), &meta); err != nil {
				return nil, fmt.Errorf("yaegi plugin %s: Init returned invalid JSON: %w", id, err)
			}
			p.meta = meta
		}
	}

	logger.Info("yaegi plugin loaded", "id", id)
	return p, nil
}

// callYaegi invokes an ABI function on a Yaegi plugin.
func callYaegi(m *Manager, p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	yp := p.yaegi
	if yp == nil {
		return "", fmt.Errorf("yaegi plugin %s: not loaded", p.id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
	defer cancel()

	out, err := yp.callWithTimeout(ctx, fnName, inputJSON)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("yaegi plugin %s %s: timed out", p.id, fnName)
		}
		return "", fmt.Errorf("yaegi plugin %s %s: %w", p.id, fnName, err)
	}

	if out == "" {
		return "", fmt.Errorf("yaegi plugin %s %s: empty result", p.id, fnName)
	}

	var probe any
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		return "", fmt.Errorf("yaegi plugin %s %s: result is not JSON: %w", p.id, fnName, err)
	}

	return out, nil
}

// closeYaegi shuts down a Yaegi plugin interpreter.
func closeYaegi(p *loadedPlugin) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.yaegi = nil
}

// callWithTimeout invokes the load-time wrapper for fnName in the Yaegi
// interpreter. The wrapper returns a single string; plugin errors (and
// timeouts, via EvalWithContext) surface as Eval errors.
func (yp *yaegiPlugin) callWithTimeout(ctx context.Context, fnName, arg string) (out string, err error) {
	// Recover from panics in interpreted code.
	defer func() {
		if r := recover(); r != nil {
			out = ""
			err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()

	result, err := yp.i.EvalWithContext(ctx, fmt.Sprintf("gskCall_%s(%q)", fnName, arg))
	if err != nil {
		return "", err
	}

	val := result.Interface()
	switch t := val.(type) {
	case string:
		return t, nil
	case []any:
		// Defensive: a multi-value surface would mean the wrapper is missing.
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s, nil
			}
		}
		return "", fmt.Errorf("yaegi %s: unexpected return %T", fnName, val)
	default:
		return "", fmt.Errorf("yaegi %s: unexpected return type %T", fnName, val)
	}
}

// yaegiRequired are the ABI function names every yaegi plugin MUST define.
var yaegiRequired = []string{
	types.SearchFunc,
	types.GetMangaDetailFunc,
	types.GetChapterListFunc,
	types.GetPageListFunc,
}

// initWrapperSrc returns the gskCall_Init wrapper matching the plugin's
// Init signature, or "" when the plugin has no Init. Init is optional and
// its signature varies: no arg or one arg, returning string or (string, error).
func initWrapperSrc(srcStr string) string {
	if !strings.Contains(srcStr, "func "+types.InitFunc+"(") {
		return ""
	}
	// Locate the param list between "func Init(" and its matching ")".
	o := strings.Index(srcStr, "func "+types.InitFunc+"(")
	o += len("func " + types.InitFunc + "(")
	depth := 1
	j := o
	for j < len(srcStr) && depth > 0 {
		switch srcStr[j] {
		case '(':
			depth++
		case ')':
			depth--
		}
		j++
	}
	params := strings.TrimSpace(srcStr[o : j-1])
	rest := strings.TrimSpace(srcStr[j:])
	hasErr := strings.HasPrefix(rest, "(string, error)")
	call := "Init()"
	if params != "" {
		call = "Init(arg)"
	}
	if hasErr {
		return "func gskCall_Init(arg string) string { s, e := " + call + "; if e != nil { panic(\"plugin error: \" + e.Error()) }; return s }"
	}
	return "func gskCall_Init(arg string) string { return " + call + " }"
}

// isGoStdlib checks if the import path is a Go standard library package.
func isGoStdlib(path string) bool {
	if strings.HasPrefix(path, "github.com/") ||
		strings.HasPrefix(path, "golang.org/x/") ||
		strings.HasPrefix(path, "gopkg.in/") ||
		strings.HasPrefix(path, "gitea.com/") ||
		strings.HasPrefix(path, "bitbucket.org/") {
		return false
	}
	// Also block goisekai internal paths that aren't the hostnet bridge.
	if strings.HasPrefix(path, "goisekai/") {
		return false
	}
	return true
}

// yaegiHostPkg is a Yaegi-compatible host bridge.
// It only exposes functions that use stdlib-compatible types, because yaegi
// cannot parse bogdanfinn/fhttp source that hostnet uses internally.
type yaegiHostPkg struct {
	proxy *hostnet.Proxy
	id    string
}

// Get is available as hostnet.Get in interpreted plugins.
func (y yaegiHostPkg) Get(url string) (string, error) {
	resp, err := y.proxy.Request(y.id, types.HTTPRequest{
		Method: "GET",
		URL:    url,
	})
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}

// Post is available as hostnet.Post in interpreted plugins.
func (y yaegiHostPkg) Post(url, body string) (string, error) {
	resp, err := y.proxy.Request(y.id, types.HTTPRequest{
		Method: "POST",
		URL:    url,
		Body:   body,
	})
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}
