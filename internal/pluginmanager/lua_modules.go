package pluginmanager

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	lua "github.com/mmcdole/lunar"
)

// setupRequire creates a sandboxed require(name) global: loads sibling .lua
// modules from the plugin folder only — no package lib, no path search, no
// cpath, no ".." escapes. Also pre-execs every sibling module except
// main.lua (alphabetical order) so require() inside main is a cache hit.
//
// ponytail: Lunar forbids re-entering state.Call from a native callback,
// so modules are pre-executed in Go before main.lua (alphabetical order);
// a module requiring a later-alphabetical sibling fails. Add a topo-sort
// here if a plugin ever needs that.
func setupRequire(state *lua.State, id, dir string) error {
	moduleCache := map[string]lua.Value{}

	preload := func(name string) error {
		if _, exists := moduleCache[name]; exists {
			return nil
		}
		data, err := readFile(filepath.Join(dir, name+".lua"))
		if err != nil {
			return err
		}
		loaded, err := state.Load(name+".lua", bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("load: %w", err)
		}
		results, err := state.Call(loaded.Value())
		if err != nil {
			return err
		}
		ret := lua.Nil()
		if len(results) > 0 {
			ret = results[0]
		}
		moduleCache[name] = ret
		return nil
	}

	// Pre-execute every sibling module except main.lua before main runs.
	if entries, err := os.ReadDir(dir); err == nil {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && strings.HasSuffix(n, ".lua") && n != "main.lua" {
				names = append(names, strings.TrimSuffix(n, ".lua"))
			}
		}
		sort.Strings(names)
		for _, n := range names {
			if err := preload(n); err != nil {
				_ = state.Close()
				return fmt.Errorf("lua plugin %s: preload %s: %w", id, n, err)
			}
		}
	}

	reqFn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		name, ok := frame.String(0)
		if !ok {
			frame.ThrowError(fmt.Errorf("require: argument must be a string"))
			return frame.ReturnNil()
		}
		if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
			frame.ThrowError(fmt.Errorf("require: %q is not a plain module name (plugin folder only)", name))
			return frame.ReturnNil()
		}
		if v, exists := moduleCache[name]; exists {
			return frame.ReturnValue(v)
		}
		// Not preloaded: either missing or required lazily from a callback,
		// which Lunar forbids (no re-entry). Try an idle-time load anyway:
		// reachable only if main.lua is somehow re-executing top-level code.
		if err := preload(name); err != nil {
			frame.ThrowError(fmt.Errorf("require %s: %v", name, err))
			return frame.ReturnNil()
		}
		return frame.ReturnValue(moduleCache[name])
	})
	_ = state.RawSetGlobal("require", reqFn.Value())

	// Harden base: strip file-reading entry points.
	_ = state.RawSetGlobal("dofile", lua.Nil())
	_ = state.RawSetGlobal("loadfile", lua.Nil())

	return nil
}
