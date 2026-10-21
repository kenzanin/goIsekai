package pluginmanager

import (
	"context"
	"fmt"
	"path/filepath"
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
//
// Host helpers: a .so has no wasm import table, so the host instead installs
// the host_call bridge through an OPTIONAL export:
//
//	void plugin_init(host_call_fn fn);
//	typedef int32_t (*host_call_fn)(const char *req, char *out, int32_t cap);
//
// The plugin calls fn with the same {"fn":"ns.name","args":[...]} envelope the
// wasm import takes and reads a JSON answer back from its own buffer. That one
// bridge reaches every helper, because dispatchHostCall resolves them all.
// A plugin without plugin_init still loads and just lacks the helpers.

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
//
// The pointers are unsafe.Pointer rather than uintptr: purego maps
// unsafe.Pointer <-> void* directly, so the C buffer never has to be laundered
// through an integer type (which is what made go vet flag a "possible misuse of
// unsafe.Pointer"). A real pointer is also the correct holder for C memory — a
// uintptr carries no liveness information at all.
type goFn struct {
	call func(string) unsafe.Pointer
	free func(unsafe.Pointer)
}

// hostCallCallback is the bridge that gives a native plugin the helpers wasm
// plugins reach through the env.host_call import. A .so has no import table, so
// instead the host installs this function pointer via the plugin's optional
// plugin_init export, and the plugin calls it with
// {"fn":"text.url_encode","args":[...]} — the exact envelope wasm uses. Every
// helper is therefore reachable through one bridge: dispatchHostCall is already
// the single place they are resolved.
//
// The host writes its answer into a buffer the PLUGIN owns (out/outCap), so no
// memory ever crosses the two allocators: nothing malloc'd by Go is freed by
// the plugin, and vice versa. Returns the length written, or -1 when the
// response does not fit.
//
// Runs on the same goroutine that made the C call, while callGo holds p.mu, so
// it reads p.curCtx directly — taking p.mu here would deadlock.
func hostCallCallback(m *Manager, p *loadedPlugin) func(*byte, *byte, int32) int32 {
	return func(req *byte, out *byte, outCap int32) int32 {
		var r wasmHostCallRequest
		if err := json.Unmarshal([]byte(cStringFrom(req)), &r); err != nil {
			return writeCString(out, outCap, string(wasmHostError("host_call: "+err.Error())))
		}

		// A nil curCtx means the callback fired outside an invoke; the helpers
		// that reach the network then fall back to an unbounded context.
		ctx := p.curCtx
		if ctx == nil {
			ctx = context.Background()
		}
		value, err := m.dispatchHostCall(ctx, p.id, r.Fn, r.Args)
		if err != nil {
			return writeCString(out, outCap, string(wasmHostError(err.Error())))
		}
		b, err := json.Marshal(value)
		if err != nil {
			return writeCString(out, outCap, string(wasmHostError("host_call "+r.Fn+": "+err.Error())))
		}
		return writeCString(out, outCap, string(b))
	}
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
	var free func(unsafe.Pointer)
	if err := lib.bind("plugin_free", &free); err != nil {
		_ = lib.close()
		return nil, fmt.Errorf("go plugin %s: symbol plugin_free missing: %w", id, err)
	}

	fns := map[string]any{}
	for hostName, symName := range goFnNames {
		var call func(string) unsafe.Pointer
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

	p := &loadedPlugin{
		id:              id,
		wasmPath:        path,
		kind:            "go",
		loaded:          true,
		contractVersion: contractVer,
		meta:            meta,
		goLib:           lib,
		goFns:           fns,
	}
	// Optional: only a plugin exporting plugin_init gets the host helpers.
	lib.wireHostCall(m, p)
	return p, nil
}
