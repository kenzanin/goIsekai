package pluginmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dop251/goja"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
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
		if abi == types.GetAltTitlesFunc {
			continue
		}
		val := vm.Get(jsName)
		if val == nil || goja.IsUndefined(val) {
			return nil, fmt.Errorf("js plugin %s: missing function %s (abi: %s)", id, jsName, abi)
		}
	}

	logger.Info("js plugin loaded", "id", id, "name", meta.VerifyURL)

	return &loadedPlugin{
		id:              id,
		wasmPath:        filepath.Join(dir, "main.js"),
		kind:            "js",
		loaded:          true,
		js:              vm,
		contractVersion: contractVer,
		meta:            meta,
	}, nil
}

// callJS invokes a JS function on a goja-based plugin. The ABI contract is
// identical to Lua: the function receives a single JSON string argument and
// must return a JSON string (or an object that goja will serialize).
func callJS(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	jsName, ok := jsFnNames[fnName]
	if !ok {
		return "", fmt.Errorf("js plugin %s: no js mapping for %s", p.id, fnName)
	}
	fn := p.js.Get(jsName)
	if fn == nil || goja.IsUndefined(fn) {
		return "", fmt.Errorf("js plugin %s: %s is not a function", p.id, jsName)
	}

	// Timeout via Interrupt. A stale interrupt on an idle VM persists into the
	// next call, so always ClearInterrupt after the call completes.
	stop := make(chan struct{})
	go func() {
		select {
		case <-time.After(invokeTimeout):
			p.js.Interrupt("timeout")
		case <-stop:
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	callable, ok := goja.AssertFunction(fn)
	if !ok {
		close(stop)
		return "", fmt.Errorf("js plugin %s %s: not callable", p.id, jsName)
	}

	v, err := callable(goja.Undefined(), p.js.ToValue(inputJSON))
	close(stop)
	p.js.ClearInterrupt()
	if err != nil {
		return "", fmt.Errorf("js plugin %s %s: %w", p.id, jsName, err)
	}
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return "", fmt.Errorf("js plugin %s %s: returned nil", p.id, jsName)
	}
	// If it's already a string, return directly.
	if s, ok := v.Export().(string); ok {
		return s, nil
	}
	// Otherwise marshal to JSON.
	raw, err := json.Marshal(v.Export())
	if err != nil {
		return "", fmt.Errorf("js plugin %s %s: marshal result: %w", p.id, jsName, err)
	}
	return string(raw), nil
}

// jsConsole wraps fmt.Printf-style console output for JS plugins.
type jsConsole struct{ id string }

func newJSConsole(id string) *jsConsole { return &jsConsole{id: id} }

func (c *jsConsole) Log(args ...goja.Value)   { c.log("info", args...) }
func (c *jsConsole) Info(args ...goja.Value)  { c.log("info", args...) }
func (c *jsConsole) Warn(args ...goja.Value)  { c.log("warn", args...) }
func (c *jsConsole) Error(args ...goja.Value) { c.log("error", args...) }
func (c *jsConsole) Debug(args ...goja.Value) { c.log("debug", args...) }

func (c *jsConsole) log(level string, args ...goja.Value) {
	parts := make([]any, len(args))
	for i, a := range args {
		parts[i] = a.Export()
	}
	switch level {
	case "debug":
		logger.Debug("plugin="+c.id, "msg", fmt.Sprint(parts...))
	case "info":
		logger.Info("plugin="+c.id, "msg", fmt.Sprint(parts...))
	case "warn":
		logger.Warn("plugin="+c.id, "msg", fmt.Sprint(parts...))
	case "error":
		logger.Error("plugin="+c.id, "msg", fmt.Sprint(parts...))
	}
}
