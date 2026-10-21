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

// fileExists reports whether path names an existing regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

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
