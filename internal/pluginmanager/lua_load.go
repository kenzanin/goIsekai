package pluginmanager

import (
	"bytes"
	"fmt"
	"path/filepath"

	"goisekai/pkg/types"
)

// loadLua creates a sandboxed Lunar 5.4 VM, loads <dir>/main.lua, verifies the
// source ABI globals, and returns a ready-to-call loadedPlugin. Metadata comes
// from plugin.json, not from the VM.
func (m *Manager) loadLua(id, dir string) (*loadedPlugin, error) {
	state, err := createLuaState(id)
	if err != nil {
		return nil, err
	}

	if err := setupRequire(state, id, dir); err != nil {
		return nil, err
	}

	if err := m.setupGlobals(state, id); err != nil {
		_ = state.Close()
		return nil, err
	}

	// Load main.lua via state.Load (reader-backed, no ScriptLoader needed).
	mainPath := filepath.Join(dir, "main.lua")
	mainData, err := readFile(mainPath)
	if err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua read %s: %w", mainPath, err)
	}
	loaded, err := state.Load("main.lua", bytes.NewReader(mainData))
	if err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua load %s: %w", mainPath, err)
	}
	if _, err := state.Call(loaded.Value()); err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua exec %s: %w", mainPath, err)
	}

	// Metadata lives in plugin.json beside the entry point, never in the VM:
	// the host already read it at discovery, so a plugin never has to execute
	// just to be listed with its name and logo.
	meta, err := pluginMetaFor(id, "lua", dir)
	if err != nil {
		_ = state.Close()
		return nil, err
	}
	contractVer := meta.ContractVersion
	if contractVer == 0 {
		contractVer = types.ContractVersion
	}

	// Verify the source ABI globals are functions (snake_case Lua names).
	// luaFnNames holds the source ABI only, so an enrichment script's
	// get_enrichment is never required of a source plugin.
	if !isInfoPlugin(id) {
		for abi := range luaFnNames {
			// GetGenres is the one optional export among them.
			if abi == types.GetGenresFunc {
				continue
			}
			if _, ok := resolveLuaFn(id, state, abi); !ok {
				_ = state.Close()
				return nil, fmt.Errorf("lua plugin %s: global %q is not a function", id, luaFnNames[abi])
			}
		}
	}

	return &loadedPlugin{
		id: id,
		// Return the plugin FOLDER, not the entry file: ensureLoaded passes this
		// path back to loadLua on lazy reload, and loadLua appends "main.lua"
		// itself — a file path here would produce "main.lua/main.lua".
		wasmPath:        dir,
		kind:            "lua",
		loaded:          true,
		lunar:           state,
		contractVersion: contractVer,
		meta:            meta,
	}, nil
}
