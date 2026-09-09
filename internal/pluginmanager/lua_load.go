package pluginmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	lua "github.com/mmcdole/lunar"

	"goisekai/pkg/types"
)

// loadLua creates a sandboxed Lunar 5.4 VM, loads <dir>/main.lua, reads the
// PLUGIN metadata table, and returns a ready-to-call loadedPlugin.
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

	// Read the PLUGIN metadata table.
	pluginVal, err := state.RawGlobal("PLUGIN")
	if err != nil || pluginVal.IsNil() || pluginVal.Kind() != lua.TableKind {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN global is not a table", id)
	}
	pluginTbl, _ := pluginVal.AsTable()

	metaJSON, err := lunarTableToJSON(state, pluginTbl)
	if err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: encode PLUGIN: %w", id, err)
	}
	var meta types.PluginMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: decode PLUGIN metadata: %w", id, err)
	}

	// Resolve contract_version from the PLUGIN table.
	verVal := pluginTbl.RawGetString("contract_version")
	if verVal.IsNil() {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version missing", id)
	}
	verNum, ok := verVal.AsNumber()
	if !ok {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version not a number", id)
	}
	contractVer := int32(verNum)
	if err := types.CheckVersion(contractVer); err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: %w", id, err)
	}

	// Verify all ABI globals are functions (snake_case Lua names).
	// GetAltTitles is OPTIONAL (enricher capability) — mirror js.go.
	for abi, name := range luaFnNames {
		if abi == types.GetAltTitlesFunc || abi == types.GetAltSummaryFunc {
			continue
		}
		fn, err := state.RawGlobal(name)
		if err != nil || fn.Kind() != lua.FunctionKind {
			_ = state.Close()
			return nil, fmt.Errorf("lua plugin %s: global %q is not a function", id, name)
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
