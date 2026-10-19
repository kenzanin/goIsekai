// wasm_abi.odin — reusable goIsekai WASM plugin ABI glue for Odin (wasip1).
//
// Copy this whole `abi/` directory into a new plugin directory as-is; the plugin
// then does `import "abi"`. It provides:
//   • the env.host_call import (JSON dispatch — see docs/plugin-wasm-helpers.md)
//   • malloc / free / contract_version exports
//   • at / read_string / pack / unpack / reply_json
//   • host_args / host_str / get_body / http_body / http_post_body / to_json
//
// The plugin's own main.odin must export, with @(export):
//   Search, GetMangaDetail, GetChapterList, GetPageList  (+ optional GetGenres)
// returning reply_json(...) each time, and must NOT re-export malloc/free/
// contract_version — this file already does.
//
// Build: odin build . -target:wasi_wasm32 -no-entry-point -out:main.wasm
//
// Compared with the C++ wasm_abi.h this needs no libc++ and no
// -Wl,--export=malloc/free flags: Odin emits the exports itself, and allocation
// goes through core:mem's wasm allocator rather than a libc malloc the host
// does not provide.
package abi

import "base:runtime"
import "core:encoding/json"
import "core:mem"
import "core:strings"

// env.host_call(ptr, length) -> packed i64 (low 32 = pointer, high 32 = length).
foreign import env "env"

@(default_calling_convention="c")
foreign env {
	host_call :: proc(ptr: u32, length: u32) -> u64 ---
}

// ─── exports the host resolves ────────────────────────────────────────────────

@(export)
contract_version :: proc "c" () -> i32 {
	return 1
}

// malloc/free are the host's window into this module's linear memory: it
// allocates the input buffer and frees our reply with them, so both must come
// from one allocator. Note this is core:mem, not libc — a `foreign import
// libc "system:c"` here would emit a `system:c.malloc` import the host does not
// provide and the module would fail to instantiate.
@(export)
malloc :: proc "c" (n: u32) -> rawptr {
	context = runtime.default_context()
	buf, err := mem.alloc(int(max(n, 1)), 8)
	if err != .None {
		return nil
	}
	return buf
}

@(export)
free :: proc "c" (p: rawptr) {
	if p == nil {
		return
	}
	context = runtime.default_context()
	mem.free(p)
}

// ─── memory ──────────────────────────────────────────────────────────────────

// at reinterprets a linear-memory address as a byte slice. contextless so the
// packing helpers below stay callable before any context exists.
at :: proc "contextless" (ptr, length: u32) -> []u8 {
	if ptr == 0 || length == 0 {
		return nil
	}
	return (cast([^]u8)(uintptr(ptr)))[:length]
}

// read_string copies an ABI argument out of linear memory.
read_string :: proc "contextless" (ptr, length: u32) -> string {
	return string(at(ptr, length))
}

pack :: proc "contextless" (ptr, length: u32) -> u64 {
	return u64(ptr) | u64(length) << 32
}

unpack :: proc "contextless" (packed: u64) -> (ptr, length: u32) {
	return u32(packed & 0xFFFF_FFFF), u32(packed >> 32)
}

// reply_json hands an answer back in a buffer allocated through OUR malloc, so
// the host frees it through our own free: ownership never crosses allocators.
reply_json :: proc (s: string) -> u64 {
	n := len(s)
	p := malloc(u32(max(n, 1)))
	if p == nil {
		return 0
	}
	copy(at(u32(uintptr(p)), u32(n)), transmute([]u8)s)
	return pack(u32(uintptr(p)), u32(n))
}

// ─── host helpers ────────────────────────────────────────────────────────────

// Envelope is the same {"fn":..,"args":[..]} request the C++ and wasm guests
// send; the host dispatches it against every Lua/JS helper.
Envelope :: struct {
	fn:   string   `json:"fn"`,
	args: []string `json:"args"`,
}

// host_args sends one helper call and returns the host's raw JSON answer, or ""
// when the bridge is absent or the call could not be marshalled.
host_args :: proc (fn: string, args: []string) -> string {
	payload, err := json.marshal(Envelope{fn = fn, args = args})
	if err != nil {
		return ""
	}
	p := malloc(u32(len(payload)))
	if p == nil {
		return ""
	}
	copy(at(u32(uintptr(p)), u32(len(payload))), payload)
	packed := host_call(u32(uintptr(p)), u32(len(payload)))
	free(p)

	ptr, length := unpack(packed)
	if ptr == 0 || length == 0 {
		return ""
	}
	return string(at(ptr, length))
}

// host_str calls a one-string helper and returns its string result. A helper
// failure arrives as {"error":"..."}, which fails the decode and yields "".
host_str :: proc (fn: string, arg: string) -> string {
	raw := host_args(fn, {arg})
	if raw == "" {
		return ""
	}
	out: string
	if json.unmarshal(transmute([]u8)raw, &out) != nil {
		return ""
	}
	return out
}

// http_body fetches url through the host proxy (which applies the plugin's
// headers, rate limiting and any CDP solve). ok is false on non-200 or on any
// error envelope, mirroring the C++ helper.
http_body :: proc (url: string, headers_json: string) -> (body: string, ok: bool) {
	raw := host_args("http.get_body", {url, headers_json})
	if raw == "" || raw == "null" {
		return "", false
	}
	if json.unmarshal(transmute([]u8)raw, &body) != nil {
		return "", false
	}
	return body, body != ""
}

get_body :: proc (url: string) -> (string, bool) {
	return http_body(url, "")
}

// http_post_body POSTs url; args are (url, body, headers json).
http_post_body :: proc (url, body, headers_json: string) -> (string, bool) {
	raw := host_args("http.post_body", {url, body, headers_json})
	if raw == "" || raw == "null" {
		return "", false
	}
	out: string
	if json.unmarshal(transmute([]u8)raw, &out) != nil {
		return "", false
	}
	return out, out != ""
}

// to_json marshals a value to a compact JSON string, "" on failure.
to_json :: proc (v: any) -> string {
	data, err := json.marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// ─── shared host-helper wrappers ─────────────────────────────────────────────
//
// Every Odin plugin used to re-declare these in its own main.odin. They are
// generic (no site knowledge) and were byte-identical wherever duplicated, so
// they live here once. Call them qualified: `abi.find_all(...)`, the same style
// as abi.host_str / abi.http_body above.
//
// `using import "abi"` is NOT allowed by Odin, so an unqualified name cannot be
// imported into a plugin's scope — qualification is the only option.
//
// Site-specific helpers are deliberately NOT here: `decode_id` (4 shapes),
// `normalize_status` (4 shapes, one reimplements the vocabulary) and
// `parse_time` (2 shapes) genuinely differ per plugin and stay local.

// url_encode percent-encodes through the host (text.url_encode).
url_encode :: proc (s: string) -> string {
	return host_str("text.url_encode", s)
}

// find_one returns the first capture of the first match, trimmed; "" on no
// match. Mirrors host.regex.find.
find_one :: proc (subject, pattern: string) -> string {
	raw := host_args("regex.find", {subject, pattern})
	if raw == "" || raw == "null" {
		return ""
	}
	v: string
	if json.unmarshal_string(raw, &v) != nil {
		return ""
	}
	return strings.trim_space(v)
}

// find_all returns the first capture of every match, in document order.
//
// The host answers a ONE-capture pattern as a flat array and a multi-capture
// pattern as an array of arrays (see wasmRegexRow host-side). Overloading one
// name on that shape is the documented wasm substitute for Lua's `gmatch`
// iterator: `for row in abi.find_all(html, pat)` replaces
// `for x in host.regex.gmatch(html, pat)`.
//
// Unmarshalling a multi-capture answer here is a HARD error, not a silent zero:
// Odin rejects `[...]` into `[]string` with Unsupported_Type_Error. Pick
// find_all vs find_all_pairs to match the pattern's capture count.
find_all :: proc (subject, pattern: string) -> []string {
	raw := host_args("regex.find_all", {subject, pattern})
	if raw == "" || raw == "null" {
		return nil
	}
	out: []string
	if json.unmarshal_string(raw, &out) != nil {
		return nil
	}
	return out
}

// find_all_pairs returns every match of a TWO-or-more capture pattern as one
// row per match. See find_all for why the two shapes are separate functions.
find_all_pairs :: proc (subject, pattern: string) -> [][]string {
	raw := host_args("regex.find_all", {subject, pattern})
	if raw == "" || raw == "null" {
		return nil
	}
	out: [][]string
	if json.unmarshal_string(raw, &out) != nil {
		return nil
	}
	return out
}

// sub replaces every match of pattern, returning the subject unchanged when the
// helper is unavailable. Mirrors host.regex.replace.
sub :: proc (subject, pattern, repl: string) -> string {
	raw := host_args("regex.replace", {subject, pattern, repl})
	if raw == "" || raw == "null" {
		return subject
	}
	v: string
	if json.unmarshal_string(raw, &v) != nil {
		return subject
	}
	return v
}

// ─── html.* (markup in, no document handle) ─────────────────────────────────
// In wasm the html helpers are stateless: html.parse only validates the markup
// and hands the same string back, so each find_* takes the markup as its first
// argument and no document is carried around.

find_text :: proc (markup, css: string) -> string {
	raw := host_args("html.find_text", {markup, css})
	if raw == "" || raw == "null" {
		return ""
	}
	v: string
	if json.unmarshal_string(raw, &v) != nil {
		return ""
	}
	return strings.trim_space(v)
}

find_attr :: proc (markup, css, attr: string) -> string {
	raw := host_args("html.find_attr", {markup, css, attr})
	if raw == "" || raw == "null" {
		return ""
	}
	v: string
	if json.unmarshal_string(raw, &v) != nil {
		return ""
	}
	return strings.trim_space(v)
}

find_list_text :: proc (markup, css: string) -> []string {
	raw := host_args("html.find_list_text", {markup, css})
	if raw == "" || raw == "null" {
		return nil
	}
	out: []string
	if json.unmarshal_string(raw, &out) != nil {
		return nil
	}
	return out
}

find_list_attr :: proc (markup, css, attr: string) -> []string {
	raw := host_args("html.find_list_attr", {markup, css, attr})
	if raw == "" || raw == "null" {
		return nil
	}
	out: []string
	if json.unmarshal_string(raw, &out) != nil {
		return nil
	}
	return out
}

// ─── text.* / numeric answers ───────────────────────────────────────────────

// date_to_iso converts a unix-seconds string to RFC3339 via the host; "" stays
// "" so a missing date never becomes the 1970 timestamp.
date_to_iso :: proc (s: string) -> string {
	if s == "" {
		return ""
	}
	return host_str("text.date_to_iso", s)
}

// host_num reads a numeric answer from any one-argument host helper (0 when the
// call fails or the answer is not a number).
host_num :: proc (fn: string, arg: string) -> f64 {
	raw := host_args(fn, {arg})
	if raw == "" || raw == "null" {
		return 0
	}
	v: f64
	if json.unmarshal_string(raw, &v) != nil {
		return 0
	}
	return v
}

// chapter_num extracts a chapter number from a title/slug (0 when absent).
chapter_num :: proc (title: string) -> f64 {
	return host_num("text.chapter_num", title)
}

// sanitize_title unescapes entities then trims.
sanitize_title :: proc (s: string) -> string {
	return strings.trim_space(host_str("text.unescape", s))
}
