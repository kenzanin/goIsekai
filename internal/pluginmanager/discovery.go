package pluginmanager

import (
	"goisekai/internal/logger"
	"path/filepath"
)

// infoPrefix namespaces enrichment-script ids inside the manager's plugin map.
// It keeps an enrichment script from colliding with a scraper plugin of the
// same folder name (both /plugins/mangadex and /info/mangadex can coexist);
// the declared enrichment provider id stays the bare folder name.
const infoPrefix = "info:"

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

	// Enrichment scripts: one folder per source, folder name is both the
	// declared provider id and the enrich source id used by the UI. A folder
	// may ship either main.lua or main.wasm; wasm wins when both are present,
	// matching plugin discovery (a built wasm folder also carries its source).
	if m.infoDir != "" {
		dirs, err := filepath.Glob(filepath.Join(m.infoDir, "*"))
		if err != nil {
			return err
		}
		for _, dir := range dirs {
			// wasmPath means different things per runtime: loadLua/loadJS/loadYaegi
			// want the FOLDER (they append the entry filename themselves), while
			// loadWasm wants the .wasm FILE. Getting this wrong yields
			// "main.lua/main.lua".
			entry, kind, wasmPath := filepath.Join(dir, "main.lua"), "lua", dir
			if file := filepath.Join(dir, "main.wasm"); fileExists(file) {
				entry, kind, wasmPath = file, "wasm", file
			} else if !fileExists(entry) {
				continue // neither entry point
			}
			id := filepath.Base(dir)
			key := infoPrefix + id
			if _, dup := m.plugins[key]; dup {
				logger.Error("info script id collision, skipping", "id", id)
				continue
			}
			m.plugins[key] = &loadedPlugin{
				id:       key,
				wasmPath: wasmPath,
				kind:     kind,
				infoOnly: true,
				// Seed metadata so the declared providers are visible without
				// instantiating the runtime (enrichment scripts stay lazy).
				meta: readPluginJSONMeta(dir),
			}
			logger.Debug("info script registered", "id", id, "kind", kind, "path", entry)
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
		logger.Debug("lua plugin registered", "id", id, "path", path)
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
		logger.Debug("js plugin registered", "id", id, "path", path)
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
		logger.Debug("yaegi plugin registered", "id", id, "path", path)
	}

	// Go native plugins: one .so file per plugin, filename (minus .so) = id.
	if err := m.discoverGo(); err != nil {
		return err
	}
	return nil
}
