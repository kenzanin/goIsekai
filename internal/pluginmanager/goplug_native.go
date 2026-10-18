//go:build linux || darwin || freebsd || netbsd || openbsd

package pluginmanager

import (
	"io"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

// nativeLib is a dlopen'd shared object (a C/C++ .so plugin). purego opens it
// through the platform loader directly, so the host keeps building
// CGO_ENABLED=0 — which stdlib `plugin` could never do.
type nativeLib struct{ handle uintptr }

// openNativeLib maps the plugin from a PRIVATE copy of its file, not the file
// itself.
//
// dlopen keeps that file mmap'd for as long as the library is loaded, so
// writing the original in place later — an installer's os.Create, a plain
// `cp`, anything that opens with O_TRUNC — truncates the SAME inode and zeroes
// the pages the loader mapped. The next call into the plugin, or the dlclose
// during ReloadPlugin, then jumps into the hole and takes the whole host down
// with a SIGSEGV. Mapping a copy makes the live library immune to whatever
// happens to the file on disk.
//
// The copy sits beside the original (so a plugin that resolves sibling files
// relative to its own path still finds them) and is unlinked right after
// dlopen: on these platforms the inode survives while it stays mapped, so
// nothing is left behind even if the process dies later.
func openNativeLib(path string) (*nativeLib, error) {
	src, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".loaded-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename below runs

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	h, err := purego.Dlopen(tmpName, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(tmpName)
	return &nativeLib{handle: h}, nil
}

// bind resolves name and wires it to fptr, which must be a pointer to a
// function variable. Resolving first keeps an absent optional symbol (GetGenres)
// an ordinary error instead of the panic RegisterLibFunc raises on a miss.
func (l *nativeLib) bind(name string, fptr any) error {
	if _, err := purego.Dlsym(l.handle, name); err != nil {
		return err
	}
	purego.RegisterLibFunc(fptr, l.handle, name)
	return nil
}

// wireHostCall installs the host_call bridge, if the plugin exports plugin_init.
// The export is optional: a plugin that omits it still loads and simply has no
// host helpers, which is what every native plugin did before this existed.
//
// The callback address outlives this call — purego keeps every registered
// callback alive in its own table — so the plugin may store and use it later.
//
// ponytail: one callback slot per load, and purego has no way to release one
// (its table is append-only, maxCB = 2000), so repeated ReloadPlugin calls
// would eventually exhaust it. A cache keyed by plugin id would bound that to
// the number of distinct plugins; add it if reloads ever become frequent.
func (l *nativeLib) wireHostCall(m *Manager, p *loadedPlugin) {
	var initFn func(uintptr)
	if err := l.bind("plugin_init", &initFn); err != nil {
		return
	}
	initFn(purego.NewCallback(hostCallCallback(m, p)))
}

func (l *nativeLib) close() error { return purego.Dlclose(l.handle) }
