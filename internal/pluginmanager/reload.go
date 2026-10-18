package pluginmanager

import (
	"fmt"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
	"io"
	"os"
	"path/filepath"
)

// UnloadPlugin releases the runtime for a plugin, reverting it to the
// registered-only state. The plugin stays in the manager and database so
// it will be lazily reloaded on next use.
func (m *Manager) UnloadPlugin(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not loaded", id)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.loaded {
		return nil
	}
	if p.kind == "lua" && p.lunar != nil {
		_ = p.lunar.Close()
	}
	if p.kind == "js" && p.js != nil {
		p.js.Interrupt("unloading")
	}
	if p.kind == "wasm" && p.wasmMod != nil {
		_ = p.wasmMod.Close(m.ctx)
	}
	if p.kind == "go" && p.goLib != nil {
		_ = p.goLib.close()
	}
	p.lunar = nil
	p.js = nil
	p.goLib = nil
	p.goFns = nil
	p.yaegi = nil
	p.wasmMod = nil
	p.wasmFns = nil
	p.contractVersion = 0
	p.meta = types.PluginMeta{}
	p.loaded = false
	logger.Debug("plugin unloaded", "id", id)
	return nil
}

// ReloadPlugin picks up a plugin's current files on disk, keeping the same id.
//
// It is deliberately unload-then-load rather than a bespoke re-instantiation:
// UnloadPlugin already releases every runtime (dlclose for a native .so, Close
// for a wasm module, Close/Interrupt for the VMs), and ensureLoaded already
// re-instantiates each kind from p.wasmPath. Reusing both keeps the five kinds
// on one code path — the hand-rolled version this replaced mis-derived the path
// for .so and wasm plugins (whose wasmPath is an entry FILE, not a folder) and
// removed the plugin from the registry before loading, so a failed reload
// deleted the plugin until restart.
//
// The dlclose matters for correctness, not just hygiene: dlopen of an
// already-loaded path returns the cached handle, so a native plugin would keep
// executing its old code unless the old handle is released first.
//
// On failure the plugin stays registered but unloaded, so the next call
// lazy-loads it again — a bad edit is recoverable by fixing the file.
func (m *Manager) ReloadPlugin(id string) (string, error) {
	if err := m.UnloadPlugin(id); err != nil {
		return "", err
	}
	if err := m.ensureLoaded(id); err != nil {
		logger.Warn("plugin reload failed", "id", id, "error", err)
		return "", fmt.Errorf("reload %s: %w", id, err)
	}
	logger.Debug("plugin reloaded", "id", id)
	return id, nil
}

// copyDir recursively copies src dir to dst (lua plugin folders).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the runtime of every instantiated plugin. Registered-only
// (deferred) plugins have no runtime to release and are skipped.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plugins {
		if !p.loaded {
			continue
		}
		if p.kind == "lua" && p.lunar != nil {
			_ = p.lunar.Close()
		}
	}
	if m.runtime != nil {
		_ = m.runtime.Close(m.ctx)
		m.runtime = nil
	}
	return nil
}

// copyFile copies src to dst by writing a sibling temp file and renaming it
// into place, so dst is never truncated in place.
//
// That rename is what keeps a reinstall of a native .so from killing the host:
// os.Create(dst) opens with O_TRUNC on the SAME inode, and dlopen has that
// inode mmap'd, so truncating it zeroes the very pages the loader mapped. The
// next call into the plugin — or the dlclose during ReloadPlugin — then jumps
// into the hole and SIGSEGVs the process. Rename swaps in a new inode and
// leaves the old mapping intact until it is unmapped.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp is 0600; keep the source's mode so a copied .so stays loadable.
	if info, err := os.Stat(src); err == nil {
		_ = os.Chmod(tmpName, info.Mode().Perm())
	}
	return os.Rename(tmpName, dst)
}
