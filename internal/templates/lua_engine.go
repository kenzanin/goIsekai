package templates

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"

	lua "github.com/mmcdole/lunar"
)

// LuaEngine wraps lunar to render .lua template files.
// Each .lua file is a Lua module that returns function(data) → HTML string.
// Layouts (e.g. layouts/base.lua) return function(data, content) → full page HTML.
// Views (e.g. views/library.lua) return function(data) → <main> content only.
// Partials (e.g. partials/nav.lua) return function(data) → fragment HTML.
type LuaEngine struct {
	templatesFS fs.FS
	protos      map[string]*lua.Prototype // compiled cache keyed by name w/o extension
	devMode     bool
	mu          sync.RWMutex
}

// NewLuaEngine walks templatesFS, compiles all .lua files to bytecode, and
// prepares helpers. devMode re-reads from disk on every Render call.
func NewLuaEngine(templatesFS fs.FS, devMode bool) (*LuaEngine, error) {
	e := &LuaEngine{
		templatesFS: templatesFS,
		protos:      make(map[string]*lua.Prototype),
		devMode:     devMode,
	}
	if err := e.loadBytecodes(); err != nil {
		return nil, err
	}
	return e, nil
}

// loadBytecodes compiles every .lua file in templatesFS into the bytecode cache.
func (e *LuaEngine) loadBytecodes() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return fs.WalkDir(e.templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".lua") {
			return nil
		}
		src, err := fs.ReadFile(e.templatesFS, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		proto, err := lua.Compile(path, string(src))
		if err != nil {
			return fmt.Errorf("compile %s: %w", path, err)
		}
		// Key is path without extension (e.g. "views/library")
		name := strings.TrimSuffix(path, ".lua")
		e.protos[name] = proto
		return nil
	})
}

// newVM creates a fresh lunar VM with standard libraries, FS-based require,
// and all Go helpers registered.
func (e *LuaEngine) newVM() (*lua.State, error) {
	scriptLoader := lua.FSLoader(e.templatesFS).WithPackagePath("?.lua")
	S, err := lua.New(lua.Options{
		Libraries:    lua.FullLibraries(),
		ScriptLoader: scriptLoader,
	})
	if err != nil {
		return nil, fmt.Errorf("lunar.New: %w", err)
	}
	if err := registerHelpers(S); err != nil {
		_ = S.Close()
		return nil, err
	}
	return S, nil
}

// Has returns true if a Lua bytecode exists for the given name (without extension).
func (e *LuaEngine) Has(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.protos[name]
	return ok
}

// Render executes the named template and writes the result to w.
// name is the path without extension (e.g. "views/library").
func (e *LuaEngine) Render(w io.Writer, name string, data map[string]any) error {
	return e.render(w, name, data, false)
}

// RenderPartial executes the named template with _partial=true so layouts
// return content only, no wrapping.
func (e *LuaEngine) RenderPartial(w io.Writer, name string, data map[string]any) error {
	return e.render(w, name, data, true)
}

// render is the shared implementation. partial=true sets data["_partial]=true.
func (e *LuaEngine) render(w io.Writer, name string, data map[string]any, partial bool) error {
	if data == nil {
		data = map[string]any{}
	}
	if _, exists := data["active"]; !exists {
		data["active"] = ""
	}
	if partial {
		data["_partial"] = true
	}

	// In devMode, recompile from disk every time.
	var proto *lua.Prototype
	if e.devMode {
		src, err := fs.ReadFile(e.templatesFS, name+".lua")
		if err != nil {
			return fmt.Errorf("read template %s: %w", name, err)
		}
		compiled, err := lua.Compile(name+".lua", string(src))
		if err != nil {
			return fmt.Errorf("compile template %s: %w", name, err)
		}
		proto = compiled
	} else {
		e.mu.RLock()
		proto = e.protos[name]
		e.mu.RUnlock()
		if proto == nil {
			return fmt.Errorf("template not found: %s", name)
		}
	}

	S, err := e.newVM()
	if err != nil {
		return err
	}
	defer func() { _ = S.Close() }()

	// Set data global
	if err := marshalDataToLua(S, data); err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	// Execute the module — it returns a function
	fn, err := S.LoadPrototype(proto)
	if err != nil {
		return fmt.Errorf("load template %s: %w", name, err)
	}
	results, err := S.Call(fn.Value())
	if err != nil {
		return fmt.Errorf("execute template %s: %w", name, err)
	}
	if len(results) == 0 || results[0].IsNil() {
		return fmt.Errorf("template %s returned nil", name)
	}
	viewFn := results[0]

	if partial {
		// Partial: call view directly, return body only.
		return e.callAndWrite(w, S, viewFn, data, "view "+name)
	}

	// Full page: call view to get body content.
	bodyResults, err := S.Call(viewFn, marshalGoToLua(S, data))
	if err != nil {
		return fmt.Errorf("call view %s: %w", name, err)
	}
	if len(bodyResults) == 0 {
		return fmt.Errorf("view %s returned nothing", name)
	}
	bodyStr, err := S.ToString(bodyResults[0])
	if err != nil {
		return fmt.Errorf("view %s did not return a string: %w", name, err)
	}

	// Wrap body with the layout. Views can override via _layout data key
	// (e.g. reader sets "_layout": "blank" for no-nav fullscreen).
	layoutName := "layouts/base"
	if ln, ok := data["_layout"]; ok {
		if s, ok := ln.(string); ok && s != "" {
			layoutName = "layouts/" + s
		}
	}
	var layoutProto *lua.Prototype
	if e.devMode {
		src, rerr := fs.ReadFile(e.templatesFS, layoutName+".lua")
		if rerr != nil {
			// No Lua layout — return body as-is.
			_, _ = io.Copy(w, bytes.NewBufferString(bodyStr))
			return nil
		}
		compiled, cerr := lua.Compile(layoutName+".lua", string(src))
		if cerr != nil {
			return fmt.Errorf("compile layout %s: %w", layoutName, cerr)
		}
		layoutProto = compiled
	} else {
		e.mu.RLock()
		layoutProto = e.protos[layoutName]
		e.mu.RUnlock()
	}
	if layoutProto == nil {
		// No Lua layout — return body as-is.
		_, _ = io.Copy(w, bytes.NewBufferString(bodyStr))
		return nil
	}

	layoutFn, err := S.LoadPrototype(layoutProto)
	if err != nil {
		return fmt.Errorf("load layout %s: %w", layoutName, err)
	}
	// Execute the layout module — it returns function(data, content) → string.
	layoutModResults, err := S.Call(layoutFn.Value())
	if err != nil {
		return fmt.Errorf("execute layout %s: %w", layoutName, err)
	}
	if len(layoutModResults) == 0 || layoutModResults[0].IsNil() {
		return fmt.Errorf("layout %s returned nil", layoutName)
	}
	// Call the layout function with data and content.
	layoutArgs := []lua.Value{marshalGoToLua(S, data), lua.String(bodyStr)}
	layoutResults, err := S.Call(layoutModResults[0], layoutArgs...)
	if err != nil {
		return fmt.Errorf("call layout %s: %w", layoutName, err)
	}
	if len(layoutResults) == 0 {
		return fmt.Errorf("layout %s returned nothing", layoutName)
	}
	result, err := S.ToString(layoutResults[0])
	if err != nil {
		return fmt.Errorf("layout %s did not return a string: %w", layoutName, err)
	}
	_, err = io.Copy(w, bytes.NewBufferString(result))
	return err
}

// callAndWrite executes a Lua function with data and writes the result.
func (e *LuaEngine) callAndWrite(w io.Writer, S *lua.State, fn lua.Value, data map[string]any, label string) error {
	results, err := S.Call(fn, marshalGoToLua(S, data))
	if err != nil {
		return fmt.Errorf("call %s: %w", label, err)
	}
	if len(results) == 0 {
		return fmt.Errorf("%s returned nothing", label)
	}
	result, err := S.ToString(results[0])
	if err != nil {
		return fmt.Errorf("%s did not return a string: %w", label, err)
	}
	_, err = io.Copy(w, bytes.NewBufferString(result))
	return err
}
