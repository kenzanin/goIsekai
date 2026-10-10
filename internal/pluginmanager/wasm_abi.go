package pluginmanager

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/tetratelabs/wazero/api"

	"goisekai/internal/logger"
)

// hostHTTPRequest is the env.host_http_request host import. The guest passes
// (ptr, len) of an HTTPRequest JSON and receives a packed i64 (ptr|len<<32)
// response buffer, allocated from the guest's own malloc.
func (m *Manager) hostHTTPRequest(ctx context.Context, mod api.Module, stack []uint64) {
	ptr, length := uint32(stack[0]), uint32(stack[1])
	reqBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		stack[0] = pack(0, 0)
		return
	}
	respJSON, err := m.proxy.HandleRequestContext(ctx, mod.Name(), string(reqBytes))
	if err != nil {
		logger.Warn("wasm host_http_request failed", "plugin", mod.Name(), "error", err)
		stack[0] = pack(0, 0)
		return
	}
	resp := []byte(respJSON)
	respPtr, ok := wasmAlloc(ctx, mod, uint32(len(resp)))
	if !ok || !mod.Memory().Write(respPtr, resp) {
		stack[0] = pack(0, 0)
		return
	}
	stack[0] = pack(respPtr, uint32(len(resp)))
}

// wasmAlloc allocates size bytes in the module's linear memory via its
// exported malloc, or ok=false when the module lacks one or allocation fails.
func wasmAlloc(ctx context.Context, mod api.Module, size uint32) (uint32, bool) {
	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		return 0, false
	}
	res, err := malloc.Call(ctx, uint64(size))
	if err != nil || len(res) == 0 {
		return 0, false
	}
	ptr := uint32(res[0])
	if ptr == 0 {
		return 0, false
	}
	return ptr, true
}

// wasmFree releases a buffer by calling the module's exported free, if any.
func wasmFree(ctx context.Context, mod api.Module, ptr uint32) {
	if free := mod.ExportedFunction("free"); free != nil {
		_, _ = free.Call(ctx, uint64(ptr))
	}
}

// pack combines pointer and length into one i64 ABI return
// (low 32 bits = pointer, high 32 bits = length).
func pack(ptr, length uint32) uint64 { return uint64(length)<<32 | uint64(ptr) }

// unpack splits a packed i64 back into (pointer, length).
func unpack(v uint64) (ptr, length uint32) { return uint32(v), uint32(v >> 32) }

// discoverWasm registers wasm-kind plugins without instantiating them:
// <pluginsDir>/*/main.wasm (folder plugin) and <pluginsDir>/*.wasm (single file).
func (m *Manager) discoverWasm() error {
	globs := []struct {
		pattern string
		folder  bool
	}{
		{filepath.Join(m.pluginsDir, "*", "main.wasm"), true},
		{filepath.Join(m.pluginsDir, "*.wasm"), false},
	}
	for _, g := range globs {
		matches, err := filepath.Glob(g.pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			id := strings.TrimSuffix(filepath.Base(path), ".wasm")
			if g.folder {
				id = filepath.Base(filepath.Dir(path))
			}
			if _, dup := m.plugins[id]; dup {
				logger.Error("plugin id collision, skipping", "id", id, "kind", "wasm")
				continue
			}
			lp := &loadedPlugin{id: id, wasmPath: path, kind: "wasm"}
			if g.folder {
				// Seed site_url before first load so gated image CDNs get a
				// Referer without a plugin invoke (lazy plugins stay lazy).
				lp.meta = readPluginJSONMeta(filepath.Dir(path))
			}
			m.plugins[id] = lp
			logger.Info("wasm plugin registered", "id", id, "path", path)
		}
	}
	return nil
}
