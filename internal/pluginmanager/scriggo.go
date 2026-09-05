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

	"goisekai/internal/hostnet"
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

// scriggoOptional are ABI function names that MAY appear in a plugin.
var scriggoOptional = []string{
	types.GetAltTitlesFunc,
	types.InitFunc,
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
		var panicErr *scriggo.PanicError
		if errors.As(runErr, &panicErr) {
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

// buildScriggoShim generates the main.go shim that dispatches ABI calls.
// The shim does NOT import "fmt" — it uses native err.Error() and string
// concatenation to avoid requiring any stdlib registration for the shim itself.
func buildScriggoShim(modPath string, hasAltTitles, hasInit bool) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n")
	b.WriteString(fmt.Sprintf("\tp %q\n", modPath+"/plugin"))
	b.WriteString("\t\"hostapi\"\n")
	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	b.WriteString("\tfn := hostapi.Fn()\n")
	b.WriteString("\targ := hostapi.Arg()\n")
	b.WriteString("\tvar out string\n")
	b.WriteString("\tvar err error\n")
	b.WriteString("\tswitch fn {\n")
	for _, name := range scriggoRequired {
		b.WriteString(fmt.Sprintf("\tcase %q:\n\t\tout, err = p.%s(arg)\n", name, name))
	}
	if hasAltTitles {
		b.WriteString(fmt.Sprintf("\tcase %q:\n\t\tout, err = p.%s(arg)\n", types.GetAltTitlesFunc, types.GetAltTitlesFunc))
	}
	if hasInit {
		b.WriteString(fmt.Sprintf("\tcase %q:\n\t\tout = p.%s()\n", types.InitFunc, types.InitFunc))
	}
	b.WriteString("\tdefault:\n\t\thostapi.Report(out, \"unknown ABI function: \"+fn)\n\t\treturn\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\thostapi.Report(out, err.Error())\n")
	b.WriteString("\t} else {\n")
	b.WriteString("\t\thostapi.Report(out, \"\")\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

// hostapiPackage returns a native.Package providing the host API bridge.
func hostapiPackage(host *scriggoHost) native.Package {
	return native.Package{
		Name: "hostapi",
		Declarations: native.Declarations{
			"Fn":     func() string { return host.fn },
			"Arg":    func() string { return host.arg },
			"Report": func(out, errMsg string) { host.out, host.errMsg = out, errMsg },
		},
	}
}

// hostnetPackage returns a native.Package providing network access through the
// hostnet proxy.
func hostnetPackage(pluginID string, proxy *hostnet.Proxy) native.Package {
	get := func(url string) (string, error) {
		reqJSON, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: url})
		if err != nil {
			return "", fmt.Errorf("hostnet.Get: marshal request: %w", err)
		}
		respJSON, err := proxy.HandleRequest(pluginID, string(reqJSON))
		if err != nil {
			return "", fmt.Errorf("hostnet.Get: %w", err)
		}
		var resp types.HTTPResponse
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
			return "", fmt.Errorf("hostnet.Get: unmarshal response: %w", err)
		}
		if resp.Status == 0 {
			return "", fmt.Errorf("hostnet.Get: request to %s failed (status 0)", url)
		}
		if resp.Status >= 400 {
			return "", fmt.Errorf("hostnet.Get: %s returned status %d", url, resp.Status)
		}
		return resp.Body, nil
	}

	post := func(url, body string) (string, error) {
		reqJSON, err := json.Marshal(types.HTTPRequest{Method: "POST", URL: url, Body: body})
		if err != nil {
			return "", fmt.Errorf("hostnet.Post: marshal request: %w", err)
		}
		respJSON, err := proxy.HandleRequest(pluginID, string(reqJSON))
		if err != nil {
			return "", fmt.Errorf("hostnet.Post: %w", err)
		}
		var resp types.HTTPResponse
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
			return "", fmt.Errorf("hostnet.Post: unmarshal response: %w", err)
		}
		if resp.Status == 0 {
			return "", fmt.Errorf("hostnet.Post: request to %s failed (status 0)", url)
		}
		if resp.Status >= 400 {
			return "", fmt.Errorf("hostnet.Post: %s returned status %d", url, resp.Status)
		}
		return resp.Body, nil
	}

	return native.Package{
		Name: "hostnet",
		Declarations: native.Declarations{
			"Get":  get,
			"Post": post,
		},
	}
}

// scriggoFmtPackage returns a native.Package wrapping a useful subset of real
// fmt functions. Plugins may import "fmt" to use these.
func scriggoFmtPackage() native.Package {
	return native.Package{
		Name: "fmt",
		Declarations: native.Declarations{
			"Println": fmt.Println,
			"Printf":  fmt.Printf,
			"Sprintf": fmt.Sprintf,
			"Sprint":  fmt.Sprint,
			"Errorf":  fmt.Errorf,
		},
	}
}
