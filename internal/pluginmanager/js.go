package pluginmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/logger"
	"goisekai/pkg/types"

	"modernc.org/quickjs"
)

// jsFnNames maps host ABI function names to the JS global names a JS plugin
// defines. The ABI is normalised: the JS name is the host name in camelCase, so
// Search is `search` and GetMangaDetail is `getMangaDetail`. One map drives both
// the load-time verification and the call-time dispatch.
var jsFnNames = map[string]string{
	types.SearchFunc:         "search",
	types.GetMangaDetailFunc: "getMangaDetail",
	types.GetChapterListFunc: "getChapterList",
	types.GetPageListFunc:    "getPageList",
	types.GetGenresFunc:      "getGenres",
}

// jsLegacyNames are the globals shipped before the ABI names were unified. A
// plugin that defines one still loads and runs, with a deprecation warning, so
// publishing the normalised ABI breaks nothing. Drop an entry once no shipped
// plugin uses it.
var jsLegacyNames = map[string]string{
	types.SearchFunc: "searchManga",
}

// resolveJSFn returns the first name for fnName that is a function on the VM,
// preferring the canonical name.
func resolveJSFn(vm *quickjs.VM, id, fnName string) (string, bool) {
	candidates := []string{}
	if n, ok := jsFnNames[fnName]; ok {
		candidates = append(candidates, n)
	}
	if n, ok := jsLegacyNames[fnName]; ok {
		candidates = append(candidates, n)
	}
	for _, name := range candidates {
		val, err := vm.Eval("typeof "+name+" === 'function' ? 1 : 0", quickjs.EvalGlobal)
		if err != nil {
			continue
		}
		if n, ok := val.(int); ok && n == 1 {
			if name != jsFnNames[fnName] {
				logger.Warn("plugin uses a legacy ABI name", "plugin", id, "fn", fnName, "legacy", name, "use", jsFnNames[fnName])
			}
			return name, true
		}
	}
	return "", false
}

// loadJS creates a sandboxed JavaScript VM via quickjs, loads <dir>/main.js,
// verifies the ABI exports, and returns a ready-to-call loadedPlugin. Metadata
// comes from plugin.json, not from the VM.
func (m *Manager) loadJS(id, dir string) (*loadedPlugin, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, fmt.Errorf("js plugin %s: new vm: %w", id, err)
	}

	// The plugin's own params must be readable BEFORE its code runs: a plugin
	// that builds a URL at load time reads them at the top level.
	m.loadParams(id, dir)

	// Sandboxing: no fs, os, net, http, process, require, setTimeout globals.
	// quickjs ships none of them; only what we register is reachable.
	if err := vm.StdAddHelpers(); err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: std helpers: %w", id, err)
	}

	// Register the log table (same shape as Lua's log global).
	if err := registerJSLog(vm, id); err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: set log global: %w", id, err)
	}

	// Register http_request — delegates to the hostnet proxy (same path as
	// WASM and Lua). Reads a JSON string arg, returns a JSON string result.
	if err := vm.RegisterHostFunc("http_request", func(args []any) (any, error) {
		input, _ := args[0].(string)
		result, err := m.proxy.HandleRequestContext(m.invokeCtx(id), id, input)
		if err != nil {
			return fmt.Sprintf(`{"status":0,"body":%q}`, err.Error()), nil
		}
		return result, nil
	}); err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: register http_request: %w", id, err)
	}

	// Register host.* helpers (text/codecs/crypto/http) — same surface as the Lua runtime.
	if err := registerJSHostNatives(vm, m, id); err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: set host helpers: %w", id, err)
	}

	// Register a sandboxed require() that only loads sibling .js files from
	// the plugin folder. No node_modules, no parent traversal.
	if err := vm.RegisterHostFunc("require", func(args []any) (any, error) {
		modulePath, _ := args[0].(string)
		if strings.Contains(modulePath, "..") || strings.HasPrefix(modulePath, "/") {
			return nil, fmt.Errorf("require: path %q not allowed", modulePath)
		}
		resolved := filepath.Join(dir, modulePath)
		if !strings.HasSuffix(resolved, ".js") {
			resolved += ".js"
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("require %q: %w", modulePath, err)
		}
		out, err := vm.Eval(string(data), quickjs.EvalGlobal)
		if err != nil {
			return nil, fmt.Errorf("require %q: %w", modulePath, err)
		}
		return out, nil
	}); err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: register require: %w", id, err)
	}

	// Load main.js.
	src, err := os.ReadFile(filepath.Join(dir, "main.js"))
	if err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: read main.js: %w", id, err)
	}
	if _, err := vm.Eval(string(src), quickjs.EvalGlobal); err != nil {
		_ = vm.Close()
		return nil, fmt.Errorf("js plugin %s: run main.js: %w", id, err)
	}

	// Metadata lives in plugin.json beside the entry point, never in the VM:
	// the host already read it at discovery, so a plugin never has to execute
	// just to be listed with its name and logo.
	meta, err := pluginMetaFor(id, "js", dir)
	if err != nil {
		_ = vm.Close()
		return nil, err
	}
	contractVer := meta.ContractVersion
	if contractVer == 0 {
		contractVer = types.ContractVersion
	}

	// Verify that all required ABI functions exist.
	for abi := range jsFnNames {
		// GetGenres is optional; every other mapped export is required.
		if abi == types.GetGenresFunc {
			continue
		}
		if _, ok := resolveJSFn(vm, id, abi); !ok {
			_ = vm.Close()
			return nil, fmt.Errorf("js plugin %s: missing function %s (abi: %s)", id, jsFnNames[abi], abi)
		}
	}

	logger.Debug("js plugin loaded", "id", id, "name", meta.Name)
	return &loadedPlugin{
		id: id,
		// Folder, not entry file — see the loadLua comment on wasmPath.
		wasmPath:        dir,
		kind:            "js",
		loaded:          true,
		js:              vm,
		contractVersion: contractVer,
		meta:            meta,
	}, nil
}

// registerJSLog registers the log global (same shape as Lua's log table).
func registerJSLog(vm *quickjs.VM, id string) error {
	logFn := func(level string) quickjs.HostFunc {
		return func(args []any) (any, error) {
			if len(args) > 0 {
				msg := fmt.Sprintf("%v", args[0])
				switch level {
				case "debug":
					logger.Debug("plugin="+id, "msg", msg)
				case "info":
					logger.Info("plugin="+id, "msg", msg)
				case "warn":
					logger.Warn("plugin="+id, "msg", msg)
				case "error":
					logger.Error("plugin="+id, "msg", msg)
				}
			}
			return quickjs.Undefined{}, nil
		}
	}

	// RegisterHostFunc can only define a function on the global object, and
	// SetProperty's `any` path marshals its value, which a function cannot
	// survive. So each level lands on a temporary global, and the log table is
	// assembled and the temporaries dropped in one JS snippet.
	for _, lv := range []string{"debug", "info", "warn", "error"} {
		if err := vm.RegisterHostFunc(tmpLogFnName(lv), logFn(lv)); err != nil {
			return fmt.Errorf("register log.%s: %w", lv, err)
		}
	}
	_, err := vm.Eval(`
		globalThis.log = {
			debug: globalThis.__pm_log_debug,
			info: globalThis.__pm_log_info,
			warn: globalThis.__pm_log_warn,
			error: globalThis.__pm_log_error,
		};
		delete globalThis.__pm_log_debug;
		delete globalThis.__pm_log_info;
		delete globalThis.__pm_log_warn;
		delete globalThis.__pm_log_error;
	`, quickjs.EvalGlobal)
	if err != nil {
		return fmt.Errorf("build log global: %w", err)
	}
	return nil
}

// tmpLogFnName is the scratch global registerJSLog uses before folding the
// function into the log table.
func tmpLogFnName(level string) string { return "__pm_log_" + level }
