package pluginmanager

import (
	"context"
	"strings"

	"github.com/goccy/go-json"
	"github.com/tetratelabs/wazero/api"

	"goisekai/internal/pluginutil"
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

// wasmHostStr1 maps every fixed one-string host helper straight to the Go
// implementation the Lua/JS natives call, keyed by "<namespace>.<name>".
var wasmHostStr1 = map[string]func(string) string{
	"text.url_encode":         pluginutil.URLEncode,
	"text.url_decode":         pluginutil.URLDecode,
	"text.html_decode":        pluginutil.HTMLDecode,
	"text.strip_html":         pluginutil.StripHTML,
	"text.strip_markdown":     pluginutil.StripMarkdown,
	"text.titlecase":          pluginutil.Titlecase,
	"text.normalize_title":    pluginutil.NormalizeTitle,
	"text.strip_link_blocks":  pluginutil.StripLinkBlocks,
	"text.unescape":           pluginutil.HTMLEntityUnescape,
	"text.trim":               strings.TrimSpace,
	"text.lua_escape":         pluginutil.LuaEscape,
	"text.date_to_iso":        pluginutil.DateToISONow,
	"codecs.base64_encode":    pluginutil.Base64Encode,
	"codecs.base64url_encode": pluginutil.Base64URLEncode,
	"codecs.hex_encode":       pluginutil.HexEncode,
	"crypto.sha256_hex":       pluginutil.SHA256Hex,
	"crypto.md5_hex":          pluginutil.MD5Hex,
	"crypto.utf8_hex":         pluginutil.UTF8Hex,
	"regex.quote":             pluginutil.RegexQuote,
}

// wasmHostStr1Err are the one-string helpers whose Go implementation can fail
// (a bad encoding, a bad hex string) instead of always returning a value.
var wasmHostStr1Err = map[string]func(string) (string, error){
	"codecs.base64_decode":     pluginutil.Base64Decode,
	"codecs.base64url_decode":  pluginutil.Base64URLDecode,
	"codecs.hex_decode":        pluginutil.HexDecode,
	"codecs.b64_decode_hex":    pluginutil.B64DecodeHex,
	"codecs.b64url_encode_hex": pluginutil.B64URLEncodeHex,
	"codecs.b64url_decode_hex": pluginutil.B64URLDecodeHex,
}

// wasmHostStr2 are the two-string helpers with no error return.
var wasmHostStr2 = map[string]func(string, string) string{
	"text.json_blob":         pluginutil.JSONBlob,
	"crypto.hmac_sha256_hex": pluginutil.HMACSHA256Hex,
}

// wasmHostStr2Err are the two-string helpers that can fail.
var wasmHostStr2Err = map[string]func(string, string) (string, error){
	"crypto.xor": pluginutil.XORHex,
}

// wasmArg reads one positional argument, defaulting to "" like the Lua/JS
// natives do for a missing one.
func wasmArg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}
