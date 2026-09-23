# WASM host helpers

WASM plugins (TinyGo `wasip1`) reach the same helper library the Lua and JS
runtimes expose as the `host` object. Instead of one wasm import per helper,
there is **one** generic import — `env.host_call` — that dispatches by name.
The plugin side is a three-line wrapper around JSON.

`env.host_http_request` remains available for plugins that already use it, and
is the low-level request primitive `host.http.*` is built on; new plugins
should prefer `host_call`.

## ABI

```
env.host_call(ptr i32, len i32) -> i64
```

* **In**: `(ptr, len)` of a UTF-8 JSON request in the guest's linear memory:
  `{"fn":"<namespace>.<name>","args":["...", ...]}`.
* **Out**: a packed `i64` — low 32 bits are the pointer, high 32 bits the
  length — of the JSON result buffer, allocated with the guest's exported
  `malloc`. Free it with the guest's `free` when done.
* **Errors**: the buffer holds `{"error":"..."}`. A zero return means the
  request could not be read or the response could not be allocated.
* All arguments are strings; structured values (objects, arrays, numbers) are
  passed and returned as JSON text.

`env.host_http_request(ptr i32, len i32) -> i64` keeps its original
`HTTPRequest`/`HTTPResponse` JSON ABI.

## Functions

`args` are positional and missing ones default to `""`. Returns are JSON.

### `host.text.*`

| fn | args | returns |
| --- | --- | --- |
| `text.url_encode` | `s` | string |
| `text.url_decode` | `s` | string |
| `text.html_decode` | `s` | string |
| `text.strip_html` | `s` | string |
| `text.strip_markdown` | `s` | string |
| `text.titlecase` | `s` | string |
| `text.normalize_title` | `s` | string |
| `text.strip_link_blocks` | `s` | string |
| `text.unescape` | `s` | HTML entity decoded string |
| `text.trim` | `s` | string |
| `text.lua_escape` | `s` | string |
| `text.date_to_iso` | `s` | ISO-8601 date string |
| `text.chapter_num` | `s` | number (`0` = none) |
| `text.json_blob` | `text`, `marker` | first balanced JSON blob (string) |
| `text.normalize_status` | `raw` **or** `mapJSON`, `raw` | canonical status string |

`normalize_status` with one argument uses the default status vocabulary; with
two, the first is a JSON object of lowercase source keys to canonical values
(an empty object `{}` means the default vocabulary).

### `host.codecs.*`

| fn | args | returns |
| --- | --- | --- |
| `codecs.base64_encode` | `s` | string |
| `codecs.base64_decode` | `s` | string |
| `codecs.base64url_encode` | `s` | string |
| `codecs.base64url_decode` | `s` | string |
| `codecs.hex_encode` | `s` | string |
| `codecs.hex_decode` | `s` | string |
| `codecs.b64_decode_hex` | `s` | string |
| `codecs.b64url_encode_hex` | `s` | string |
| `codecs.b64url_decode_hex` | `s` | string |

A malformed input for a `*_decode` returns an error envelope.

### `host.crypto.*`

| fn | args | returns |
| --- | --- | --- |
| `crypto.sha256_hex` | `s` | hex string |
| `crypto.md5_hex` | `s` | hex string |
| `crypto.hmac_sha256_hex` | `key`, `msg` | hex string |
| `crypto.xor` | `s`, `key` | string (hex-XOR keystream) |
| `crypto.utf8_hex` | `s` | hex string |
| `crypto.vrf_sign` | `apiPath`, `paramsJSON`, `stagesJSON` | base64 signature string |

`paramsJSON` is a JSON object of string values. `stagesJSON` is a JSON array of
`{"iv":"<b64>","key":"<b64>","tbl":"<b64>"}` objects.

### `host.json.*`

| fn | args | returns |
| --- | --- | --- |
| `json.decode` | `jsonText` | the decoded value |
| `json.encode` | `jsonText` | the compact JSON string |

`json.encode` validates and compacts; the value is already JSON text at the
wasm boundary, so it re-serialises that value.

### `host.regex.*`

| fn | args | returns |
| --- | --- | --- |
| `regex.find` | `subject`, `pattern` | first match's captures (string if one, array if several), or `null` |
| `regex.match` | `subject`, `pattern` | boolean |
| `regex.find_all` | `subject`, `pattern` | array of shaped rows |
| `regex.find_index` | `subject`, `pattern`, `init` | `[start, end]` 1-based byte offsets, or `null` |
| `regex.quote` | `s` | escaped literal string |
| `regex.replace` | `subject`, `pattern`, `repl` | string (`$1` capture refs) |

Lua's `regex.gmatch` is a Lua iterator and has no JSON equivalent, so it is not
exposed here.

### `host.html.*`

Every call is stateless: the markup is the first argument.

| fn | args | returns |
| --- | --- | --- |
| `html.parse` | `markup` | the markup, after validating it parses |
| `html.find_text` | `markup`, `css` | first match's trimmed text, or `null` |
| `html.find_attr` | `markup`, `css`, `attr` | first match's attribute, or `null` |
| `html.find_list_text` | `markup`, `css` | array of texts |
| `html.find_list_attr` | `markup`, `css`, `attr` | array of attribute values |
| `html.xpath_text` | `markup`, `xpath` | first match's trimmed text, or `null` |
| `html.xpath_attr` | `markup`, `xpath`, `attr` | first match's attribute, or `null` |
| `html.xpath_list_text` | `markup`, `xpath` | array of texts |
| `html.xpath_list_attr` | `markup`, `xpath`, `attr` | array of attribute values |

An invalid CSS selector or XPath expression returns an error envelope.

### `host.http.*`

Requests go through the same `hostnet` proxy as Lua/JS (TLS fingerprinting, CDP
challenge solving, default headers).

| fn | args | returns |
| --- | --- | --- |
| `http.get` | `url`, `headersJSON?` | `{status, headers, body}` |
| `http.post` | `url`, `body`, `headersJSON?` | `{status, headers, body}` |
| `http.get_body` | `url`, `headersJSON?` | body on 200, else `null` |
| `http.post_body` | `url`, `body`, `headersJSON?` | body on 200, else `null` |

`headersJSON` is a JSON object of header name to value. On a transport failure
the plain forms return `{"status":0,"body":"<reason>"}` and the `_body` forms
return `null`.

## Example plugin

A full TinyGo plugin that calls `host.text.titlecase` and `host.http.get_body`.
The `hostCall` wrapper is the only piece a plugin needs to copy.

```go
//go:build tinygo.wasm

package main

import (
	"encoding/json"
	"unsafe"
)

// Contract version the host checks at load time.
//
//go:wasmexport contract_version
func contractVersion() int32 { return 1 }

type hostRequest struct {
	Fn   string   `json:"fn"`
	Args []string `json:"args"`
}

//go:wasmimport env host_call
func hostCallRaw(ptr, length uint32) uint64

//go:wasmimport env host_http_request
func hostHTTPRequestRaw(ptr, length uint32) uint64

//go:wasmexport search
func search(ptr, length uint32) uint64 {
	// json.RawMessage here is the Search input; echo it back for this example.
	input := goBytes(ptr, length)
	return stringResult(string(input))
}

// hostCall runs one host helper and returns its JSON result.
func hostCall(fn string, args ...string) string {
	req, _ := json.Marshal(hostRequest{Fn: fn, Args: args})
	out := hostCallRaw(ptrOf(req), uint32(len(req)))
	rptr, rlen := uint32(out), uint32(out>>32)
	if rptr == 0 {
		return ""
	}
	return string(goBytes(rptr, rlen))
}

func main() {}

// --- helpers: memory + packed i64 ---

func ptrOf(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0])))
}

func goBytes(ptr, length uint32) []byte {
	if ptr == 0 || length == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

func stringResult(s string) uint64 {
	b := append([]byte(s), 0)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	return uint64(len(b))<<32 | uint64(ptr)
}
```

Build:

```sh
tinygo build -o main.wasm -target wasm ./main.go
```

Place the result at `<pluginsDir>/<id>/main.wasm` (folder plugin) or
`<pluginsDir>/<id>.wasm` (single file). The module must export
`contract_version`, `Search`, `GetMangaDetail`, `GetChapterList` and
`GetPageList`, and a `malloc`/`free` pair for host responses.

> The example skips input ownership: a real plugin decodes the JSON input and
> returns its own JSON, and frees host response buffers with `free()`. This
> snippet only shows the `host_call` boundary.
