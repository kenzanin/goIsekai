package pluginmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/open2b/scriggo"
	"github.com/open2b/scriggo/native"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// scriggoHost holds per-invocation state shared between the host shim and
// interpreted code. The shim's native closures close over exactly one of
// these, allocated when the plugin is loaded.
type scriggoHost struct {
	fn     string
	arg    string
	out    string
	errMsg string
}

// scriggoPlugin holds the compiled Scriggo program and its host bridge.
type scriggoPlugin struct {
	prog *scriggo.Program
	host *scriggoHost
}

// scriggoIDRe matches characters not allowed in a Go module path component.
var scriggoIDRe = regexp.MustCompile(`[^A-Za-z0-9]`)

// scriggoRequired are the ABI function names every scriggo plugin MUST define.
var scriggoRequired = []string{
	types.SearchFunc,
	types.GetMangaDetailFunc,
	types.GetChapterListFunc,
	types.GetPageListFunc,
}

// loadScriggo builds and loads a Scriggo plugin from <dir>/main.go. It
// constructs a virtual module FS, registers native host packages, compiles
// the program, and optionally runs Init to populate metadata.
func (m *Manager) loadScriggo(id, dir string) (*loadedPlugin, error) {
	srcPath := dir + "/main.go"
	srcBytes, err := readFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("scriggo plugin %s: read main.go: %w", id, err)
	}
	src := string(srcBytes)

	// Check required functions exist in source.
	for _, fn := range scriggoRequired {
		if !containsFuncDef(src, fn) {
			return nil, fmt.Errorf("scriggo plugin %s: missing required function %s", id, fn)
		}
	}

	hasAltTitles := containsFuncDef(src, types.GetAltTitlesFunc)
	hasInit := containsFuncDef(src, types.InitFunc)

	safeID := scriggoIDRe.ReplaceAllString(id, "")
	modPath := "goisekai.scriggo." + safeID

	pluginSrc := rewritePackageMain(src)
	shimSrc := buildScriggoShim(modPath, hasAltTitles, hasInit)

	fsys := scriggo.Files{
		"go.mod":           []byte("module " + modPath + "\n"),
		"plugin/plugin.go": []byte(pluginSrc),
		"main.go":          []byte(shimSrc),
	}

	host := &scriggoHost{}

	pkgImporter := native.Packages{
		"hostapi": hostapiPackage(host),
		"hostnet": hostnetPackage(id, m.proxy),
		"fmt":     scriggoFmtPackage(),
	}

	prog, err := scriggo.Build(fsys, &scriggo.BuildOptions{
		Packages: pkgImporter,
	})
	if err != nil {
		return nil, fmt.Errorf("scriggo plugin %s: build: %w", id, err)
	}

	p := &loadedPlugin{
		id:              id,
		wasmPath:        dir,
		kind:            "scriggo",
		loaded:          true,
		contractVersion: 1,
		scriggo: &scriggoPlugin{
			prog: prog,
			host: host,
		},
	}

	// Run Init if present to populate metadata.
	if hasInit {
		host.fn = types.InitFunc
		host.arg = ""
		host.out = ""
		host.errMsg = ""
		ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
		defer cancel()
		runErr := prog.Run(&scriggo.RunOptions{Context: ctx})
		if runErr != nil {
			return nil, fmt.Errorf("scriggo plugin %s: Init: %w", id, runErr)
		}
		if host.errMsg != "" {
			return nil, fmt.Errorf("scriggo plugin %s: Init: %s", id, host.errMsg)
		}
		if host.out != "" {
			var meta types.PluginMeta
			if err := json.Unmarshal([]byte(host.out), &meta); err != nil {
				return nil, fmt.Errorf("scriggo plugin %s: Init returned invalid JSON: %w", id, err)
			}
			p.meta = meta
		}
	}

	logger.Info("scriggo plugin loaded", "id", id)
	return p, nil
}

// callScriggo invokes an ABI function on a Scriggo plugin.
func callScriggo(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	host := p.scriggo.host
	host.fn = fnName
	host.arg = inputJSON
	host.out = ""
	host.errMsg = ""

	ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
	defer cancel()

	runErr := p.scriggo.prog.Run(&scriggo.RunOptions{Context: ctx})

	if runErr != nil {
		if panicErr, ok := errors.AsType[*scriggo.PanicError](runErr); ok {
			return "", fmt.Errorf("scriggo plugin %s %s: panic: %v", p.id, fnName, panicErr.Message())
		}
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("scriggo plugin %s %s: timed out", p.id, fnName)
		}
		return "", fmt.Errorf("scriggo plugin %s %s: %w", p.id, fnName, runErr)
	}

	if host.errMsg != "" {
		return "", fmt.Errorf("scriggo plugin %s %s: %s", p.id, fnName, host.errMsg)
	}

	out := strings.TrimSpace(host.out)
	if out == "" {
		return "", fmt.Errorf("scriggo plugin %s %s: empty result", p.id, fnName)
	}
	var probe any
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		return "", fmt.Errorf("scriggo plugin %s %s: result is not JSON: %w", p.id, fnName, err)
	}
	return out, nil
}

// containsFuncDef returns true if src contains a top-level function definition
// matching "func <name>(".
func containsFuncDef(src, name string) bool {
	return strings.Contains(src, "func "+name+"(")
}

// rewritePackageMain replaces the first "package main" clause with "package plugin".
// Line-anchored and limited to one replacement so package mentions inside
// comments or strings are never touched.
func rewritePackageMain(src string) string {
	re := regexp.MustCompile(`(?m)^package\s+main\b`)
	first := true
	return re.ReplaceAllStringFunc(src, func(m string) string {
		if !first {
			return m
		}
		first = false
		return "package plugin"
	})
}
