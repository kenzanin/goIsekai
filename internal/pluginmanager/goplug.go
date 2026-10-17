package pluginmanager

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/goccy/go-json"

	"goisekai/pkg/types"
)

// Native .so plugin support, loaded through purego's platform loader rather
// than the stdlib `plugin` package. That matters because `plugin` requires cgo
// and an identical toolchain, while the host builds CGO_ENABLED=0 — so no .so
// could ever load. purego only needs dlopen, and works in a cgo-free binary.
//
// ABI: each entry point takes a NUL-terminated JSON string and returns a
// NUL-terminated JSON string, both `char *`:
//
//	char *Search(const char *json);
//	void  plugin_free(char *p);
//
// The returned buffer belongs to the plugin: the host copies it out and then
// calls `plugin_free` on exactly that pointer, which keeps ownership
// unambiguous for C, C++ and Rust authors alike. Failure is reported through
// the same `{"error":"code"}` envelope the other runtimes use, so PluginError
// renders it.
//
// Exported symbols: Search, GetMangaDetail, GetChapterList, GetPageList
// (plus optional GetGenres, and the required plugin_free). Metadata is read
// from plugin.json beside the plugin, exactly like every other runtime.

// goFnNames maps host ABI function names to the exported symbols a native
// plugin provides.
var goFnNames = map[string]string{
	types.SearchFunc:         "Search",
	types.GetMangaDetailFunc: "GetMangaDetail",
	types.GetChapterListFunc: "GetChapterList",
	types.GetPageListFunc:    "GetPageList",
	types.GetGenresFunc:      "GetGenres",
}

// goFn is one resolved entry point plus the free function that releases the
// buffer it returns. Both come from the same library, so they travel together.
type goFn struct {
	call func(string) uintptr
	free func(uintptr)
}

// cString reads a NUL-terminated C string at ptr. purego keeps its own
// CString/GoString helpers internal, so the host walks the bytes itself; JSON
// payloads never contain a NUL, which is what makes the terminator safe.
//
// The uintptr→unsafe.Pointer conversion cannot be avoided: dlopen hands symbols
// back as uintptr, so vet cannot trace it. It is safe by contract — the pointer
// is a live C string owned by the plugin for the duration of the call.
func cString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	base := unsafe.Pointer(ptr) //nolint:govet // see above: purego returns C pointers as uintptr
	var b []byte
	for i := 0; ; i++ {
		c := *(*byte)(unsafe.Add(base, i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

// loadGo opens a native .so plugin and validates its ABI symbols.
func (m *Manager) loadGo(id, path string) (*loadedPlugin, error) {
	lib, err := openNativeLib(path)
	if err != nil {
		return nil, fmt.Errorf("go plugin %s: open %s: %w", id, path, err)
	}

	// Metadata lives in plugin.json beside the plugin, never in the .so: the
	// host already read it at discovery, so a plugin never has to be opened
	// just to be listed with its name and logo.
	meta, err := pluginMetaFor(id, "go", filepath.Dir(path))
	if err != nil {
		_ = lib.close()
		return nil, err
	}
	contractVer := meta.ContractVersion
	if contractVer == 0 {
		contractVer = types.ContractVersion
	}

	// plugin_free is required: without it the host cannot hand a result back,
	// so a plugin missing it would leak on every call.
	var free func(uintptr)
	if err := lib.bind("plugin_free", &free); err != nil {
		_ = lib.close()
		return nil, fmt.Errorf("go plugin %s: symbol plugin_free missing: %w", id, err)
	}

	fns := map[string]any{}
	for hostName, symName := range goFnNames {
		var call func(string) uintptr
		if err := lib.bind(symName, &call); err != nil {
			// GetGenres is optional; every other mapped export is required.
			if hostName == types.GetGenresFunc {
				continue
			}
			_ = lib.close()
			return nil, fmt.Errorf("go plugin %s: symbol %s missing: %w", id, symName, err)
		}
		fns[hostName] = &goFn{call: call, free: free}
	}

	return &loadedPlugin{
		id:              id,
		wasmPath:        path,
		kind:            "go",
		loaded:          true,
		contractVersion: contractVer,
		meta:            meta,
		goLib:           lib,
		goFns:           fns,
	}, nil
}

// callGo invokes an ABI function on a native plugin. The call itself is a
// direct C call the host cannot interrupt, so no deadline is applied here —
// unlike the script runtimes, whose VMs support interruption.
func callGo(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sym, ok := p.goFns[fnName]
	if !ok {
		return "", fmt.Errorf("go plugin %s: no symbol for %s", p.id, fnName)
	}
	fn, ok := sym.(*goFn)
	if !ok {
		return "", fmt.Errorf("go plugin %s %s: bad symbol type", p.id, fnName)
	}

	// purego converts the Go string to a NUL-terminated C string for the call
	// and keeps it alive across it, so the input needs no manual buffer.
	out := fn.call(inputJSON)
	if out == 0 {
		return "", fmt.Errorf("go plugin %s %s: returned a null result", p.id, fnName)
	}
	res := cString(out)
	fn.free(out)

	if strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("go plugin %s %s: empty result", p.id, fnName)
	}
	var probe any
	if err := json.Unmarshal([]byte(res), &probe); err != nil {
		return "", fmt.Errorf("go plugin %s %s: result is not JSON: %w", p.id, fnName, err)
	}
	// A plugin reports failure through the shared error envelope.
	if obj, ok := probe.(map[string]any); ok {
		if msg, ok := obj["error"].(string); ok && len(obj) == 1 {
			if reason := PluginError(msg); reason != "" {
				return "", fmt.Errorf("go plugin %s %s: %s", p.id, fnName, reason)
			}
			return "", fmt.Errorf("go plugin %s %s: plugin reported an error", p.id, fnName)
		}
	}
	return res, nil
}

// discoverGo registers *.so files as native plugins, in the same two shapes
// every other runtime uses: a folder plugin (<dir>/<id>/main.so, so plugin.json
// sits beside it) and a bare <dir>/<id>.so.
func (m *Manager) discoverGo() error {
	globs := []struct {
		pattern string
		folder  bool
	}{
		{filepath.Join(m.pluginsDir, "*", "main.so"), true},
		{filepath.Join(m.pluginsDir, "*.so"), false},
	}
	for _, g := range globs {
		matches, err := filepath.Glob(g.pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			id := strings.TrimSuffix(filepath.Base(path), ".so")
			dir := m.pluginsDir
			if g.folder {
				id = filepath.Base(filepath.Dir(path))
				dir = filepath.Dir(path)
			}
			if _, dup := m.plugins[id]; dup {
				continue
			}
			if !m.registerFolderPlugin(id, dir, "go") {
				continue
			}
			m.plugins[id].wasmPath = path
		}
	}
	return nil
}
