package pluginmanager

import (
	"encoding/json"
	"fmt"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
	"os"
	"path/filepath"
	"strings"
)

// infoPrefix namespaces enrichment-script ids inside the manager's plugin map.
// It keeps an enrichment script from colliding with a scraper plugin of the
// same folder name (both /plugins/mangadex and /info/mangadex can coexist);
// the declared enrichment provider id stays the bare folder name.
const infoPrefix = "info:"

// readPluginJSONMeta reads <dir>/plugin.json into plugin metadata. This is the
// single source of a plugin's metadata for every runtime: the host reads it at
// discovery, before instantiating anything, so the plugin list can show a
// display name and logo, and gated image CDNs get their Referer, without paying
// for a VM. Missing or malformed files yield the zero value, so a plugin that
// declares nothing still loads — it just has no custom name or logo.
func readPluginJSONMeta(dir string) types.PluginMeta {
	if dir == "" {
		return types.PluginMeta{}
	}
	data, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return types.PluginMeta{}
	}
	var meta types.PluginMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return types.PluginMeta{}
	}
	return meta
}

// registerFolderPlugin records a folder-backed plugin (lua, js, yaegi or wasm)
// without instantiating its runtime, seeding the metadata declared beside the
// entry point. Lazy plugins stay lazy: the metadata is on disk, not in the VM.
// It returns false without registering when the plugin declares a different ABI
// version, so an incompatible plugin is refused before any runtime is created.
func (m *Manager) registerFolderPlugin(id, dir, kind string) bool {
	meta := readPluginJSONMeta(dir)
	if meta.ContractVersion != 0 {
		if err := types.CheckVersion(meta.ContractVersion); err != nil {
			logger.Error("plugin ABI version rejected, skipping", "id", id, "kind", kind, "error", err)
			return false
		}
	}
	m.plugins[id] = &loadedPlugin{
		id:              id,
		wasmPath:        dir,
		kind:            kind,
		meta:            meta,
		contractVersion: meta.ContractVersion,
	}
	return true
}

// isInfoPlugin reports whether an id belongs to an enrichment script.
func isInfoPlugin(id string) bool {
	return strings.HasPrefix(id, infoPrefix)
}

// pluginMetaFor reads the metadata a plugin declares in plugin.json beside its
// entry point, rejecting a plugin built against a different ABI. Every loader
// goes through here, so plugin.json is the one place metadata lives — no
// runtime has to be told about it, and a lazy plugin never needs a VM to
// appear in the plugin list with its name and logo.
func pluginMetaFor(id, kind, dir string) (types.PluginMeta, error) {
	meta := readPluginJSONMeta(dir)
	if meta.ContractVersion != 0 {
		if err := types.CheckVersion(meta.ContractVersion); err != nil {
			return meta, fmt.Errorf("%s plugin %s: %w", kind, id, err)
		}
	}
	return meta, nil
}

// Discover scans pluginsDir and registers every folder containing main.lua,
// main.js, main.wasm or main.go, WITHOUT instantiating any runtime. Plugins are
// lazily instantiated on first use via ensureLoaded. A folder that collides with
// an already-registered id is logged and skipped rather than aborting discovery,
// so the order of the passes decides which runtime wins a folder claiming more
// than one entry point: wasm is scanned before yaegi because a wasm plugin
// folder also carries the main.go it was built from.
// It then scans infoDir for enrichment scripts, which are registered the same
// way but serve metadata rather than a manga source.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Enrichment scripts: one folder per source, main.lua entry, folder name is
	// both the declared provider id and the enrich source id used by the UI.
	if m.infoDir != "" {
		infoMatches, err := filepath.Glob(filepath.Join(m.infoDir, "*", "main.lua"))
		if err != nil {
			return err
		}
		for _, path := range infoMatches {
			id := filepath.Base(filepath.Dir(path))
			key := infoPrefix + id
			if _, dup := m.plugins[key]; dup {
				logger.Error("info script id collision, skipping", "id", id)
				continue
			}
			m.plugins[key] = &loadedPlugin{
				id:       key,
				wasmPath: filepath.Dir(path),
				kind:     "lua",
				infoOnly: true,
			}
			logger.Info("info script registered", "id", id, "path", path)
		}
	}

	// Lua plugins: one folder per plugin, main.lua entry, folder name = id.
	luaMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.lua"))
	if err != nil {
		return err
	}
	for _, path := range luaMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "lua")
			continue
		}
		if !m.registerFolderPlugin(id, filepath.Dir(path), "lua") {
			continue
		}
		logger.Info("lua plugin registered", "id", id, "path", path)
	}

	// JS plugins: one folder per plugin, main.js entry, folder name = id.
	jsMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.js"))
	if err != nil {
		return err
	}
	for _, path := range jsMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "js")
			continue
		}
		if !m.registerFolderPlugin(id, filepath.Dir(path), "js") {
			continue
		}
		logger.Info("js plugin registered", "id", id, "path", path)
	}

	// Wasm plugins: one folder with main.wasm, or a single *.wasm file.
	//
	// This runs BEFORE the yaegi glob below on purpose. A wasm plugin folder
	// ships both main.wasm and the main.go it was built from, so the more general
	// */main.go pattern would claim the id first and the plugin would silently run
	// under the Yaegi interpreter instead of wazero - which is how every wasm
	// plugin in examples/plugins/wasm was being loaded.
	if err := m.discoverWasm(); err != nil {
		return err
	}

	// Yaegi plugins: one folder per plugin, main.go entry, folder name = id.
	yaegiMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.go"))
	if err != nil {
		return err
	}
	for _, path := range yaegiMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "yaegi")
			continue
		}
		if !m.registerFolderPlugin(id, filepath.Dir(path), "yaegi") {
			continue
		}
		logger.Info("yaegi plugin registered", "id", id, "path", path)
	}

	// Go native plugins: one .so file per plugin, filename (minus .so) = id.
	if err := m.discoverGo(); err != nil {
		return err
	}
	return nil
}
