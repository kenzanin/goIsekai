package pluginmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/logger"
	"goisekai/pkg/types"

	"github.com/dop251/goja"
)

// jsFnNames maps host ABI function names to the JS global names a JS plugin
// defines (camelCase). One map drives both the load-time verification and the
// call-time dispatch.
var jsFnNames = map[string]string{
	types.SearchFunc:         "searchManga",
	types.GetMangaDetailFunc: "getMangaDetail",
	types.GetChapterListFunc: "getChapterList",
	types.GetPageListFunc:    "getPageList",
	types.GetAltTitlesFunc:   "getAltTitles",
	types.GetAltSummaryFunc:  "getAltSummary",
}

// loadJS creates a sandboxed JavaScript VM via goja, loads <dir>/main.js,
// reads the PLUGIN metadata, and returns a ready-to-call loadedPlugin.
func (m *Manager) loadJS(id, dir string) (*loadedPlugin, error) {
	vm := goja.New()

	// Sandboxing: no fs, os, net, http, process, require, setTimeout globals.
	// Only expose: console (debug/info/warn/error), JSON, and the log table.
	_ = vm.Set("console", newJSConsole(id))

	// Register the log table (same shape as Lua's log global).
	logObj := vm.NewObject()
	if err := logObj.Set("debug", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			logger.Debug("plugin="+id, "msg", fmt.Sprintf("%v", call.Arguments[0]))
		}
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("js plugin %s: set log.debug: %w", id, err)
	}
	if err := logObj.Set("info", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			logger.Info("plugin="+id, "msg", fmt.Sprintf("%v", call.Arguments[0]))
		}
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("js plugin %s: set log.info: %w", id, err)
	}
	if err := logObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			logger.Warn("plugin="+id, "msg", fmt.Sprintf("%v", call.Arguments[0]))
		}
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("js plugin %s: set log.warn: %w", id, err)
	}
	if err := logObj.Set("error", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			logger.Error("plugin="+id, "msg", fmt.Sprintf("%v", call.Arguments[0]))
		}
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("js plugin %s: set log.error: %w", id, err)
	}
	if err := vm.Set("log", logObj); err != nil {
		return nil, fmt.Errorf("js plugin %s: set log global: %w", id, err)
	}

	// Register http_request — delegates to the hostnet proxy (same path as
	// WASM and Lua). Reads a JSON arg {"url","method","headers","body"},
	// returns JSON {"status","headers","body"}.
	if err := vm.Set("http_request", func(call goja.FunctionCall) goja.Value {
		input := call.Arguments[0].String()
		result, err := m.proxy.HandleRequest(id, input)
		if err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		return vm.ToValue(result)
	}); err != nil {
		return nil, fmt.Errorf("js plugin %s: set http_request: %w", id, err)
	}

	// Register a sandboxed require() that only loads sibling .js files from
	// the plugin folder. No node_modules, no parent traversal.
	if err := vm.Set("require", func(call goja.FunctionCall) goja.Value {
		modulePath := call.Arguments[0].String()
		if strings.Contains(modulePath, "..") || strings.HasPrefix(modulePath, "/") {
			panic(vm.NewGoError(fmt.Errorf("require: path %q not allowed", modulePath)))
		}
		// Resolve relative to plugin dir.
		resolved := filepath.Join(dir, modulePath)
		if !strings.HasSuffix(resolved, ".js") {
			resolved += ".js"
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("require %q: %w", modulePath, err)))
		}
		v, err := vm.RunString(string(data))
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("require %q: %w", modulePath, err)))
		}
		return v
	}); err != nil {
		return nil, fmt.Errorf("js plugin %s: set require: %w", id, err)
	}

	// Load main.js.
	src, err := os.ReadFile(filepath.Join(dir, "main.js"))
	if err != nil {
		return nil, fmt.Errorf("js plugin %s: read main.js: %w", id, err)
	}
	if _, err := vm.RunString(string(src)); err != nil {
		return nil, fmt.Errorf("js plugin %s: run main.js: %w", id, err)
	}

	// Read PLUGIN metadata.
	var meta types.PluginMeta
	pluginVal := vm.Get("PLUGIN")
	contractVer := int32(1)
	if pluginVal == nil || goja.IsUndefined(pluginVal) {
		logger.Warn("plugin="+id, "msg", "no PLUGIN object; using zero metadata")
	} else {
		raw, err := json.Marshal(pluginVal.Export())
		if err != nil {
			return nil, fmt.Errorf("js plugin %s: marshal PLUGIN: %w", id, err)
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, fmt.Errorf("js plugin %s: parse PLUGIN: %w", id, err)
		}
		if err := types.CheckVersion(contractVer); err != nil {
			return nil, fmt.Errorf("js plugin %s: %w", id, err)
		}
	}

	// Verify that all required ABI functions exist. GetAltTitles is OPTIONAL
	// (enricher capability) — its absence is not an error.
	for abi, jsName := range jsFnNames {
		if abi == types.GetAltTitlesFunc || abi == types.GetAltSummaryFunc {
			continue
		}
		val := vm.Get(jsName)
		if val == nil || goja.IsUndefined(val) {
			return nil, fmt.Errorf("js plugin %s: missing function %s (abi: %s)", id, jsName, abi)
		}
	}

	logger.Info("js plugin loaded", "id", id, "name", meta.Name)

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
