package pluginmanager

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/tetratelabs/wazero/api"
)

// The wasm runtime exposes the Lua/JS `host` object through ONE generic import,
// env.host_call, rather than one import per helper. The guest passes
// {"fn":"<namespace>.<name>","args":[...]} and gets the JSON result back; every
// case below calls the same Go implementation the Lua/JS natives call, so the
// three runtimes cannot drift. See docs/plugin-wasm-helpers.md.

// wasmHostCallRequest is the envelope env.host_call decodes.
type wasmHostCallRequest struct {
	Fn   string   `json:"fn"`
	Args []string `json:"args"`
}

// hostCall is the env.host_call host import. The guest passes (ptr, len) of a
// JSON request and receives a packed i64 (ptr|len<<32) buffer holding the JSON
// result, or {"error":"..."} on failure. A zero return means the request could
// not be read or the response could not be allocated.
func (m *Manager) hostCall(ctx context.Context, mod api.Module, stack []uint64) {
	ptr, length := uint32(stack[0]), uint32(stack[1])
	reqBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		stack[0] = pack(0, 0)
		return
	}
	var req wasmHostCallRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		stack[0] = writeWasmResult(ctx, mod, wasmHostError("host_call: "+err.Error()))
		return
	}
	value, err := m.dispatchHostCall(ctx, mod.Name(), req.Fn, req.Args)
	if err != nil {
		stack[0] = writeWasmResult(ctx, mod, wasmHostError(err.Error()))
		return
	}
	out, err := json.Marshal(value)
	if err != nil {
		stack[0] = writeWasmResult(ctx, mod, wasmHostError("host_call "+req.Fn+": "+err.Error()))
		return
	}
	stack[0] = writeWasmResult(ctx, mod, out)
}

// writeWasmResult allocates the response in the guest's memory and returns the
// packed i64, or a zero pointer when allocation or the write failed.
func writeWasmResult(ctx context.Context, mod api.Module, out []byte) uint64 {
	respPtr, ok := wasmAlloc(ctx, mod, uint32(len(out)))
	if !ok || !mod.Memory().Write(respPtr, out) {
		return pack(0, 0)
	}
	return pack(respPtr, uint32(len(out)))
}

// wasmHostError is the JSON error envelope env.host_call returns.
func wasmHostError(msg string) []byte {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return b
}

// wasmArg reads one positional argument, defaulting to "" like the Lua/JS
// natives do for a missing one.
func wasmArg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}
