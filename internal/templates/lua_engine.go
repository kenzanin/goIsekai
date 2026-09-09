package templates

import (
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
