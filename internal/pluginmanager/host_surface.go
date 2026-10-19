package pluginmanager

import (
	"strings"

	"goisekai/internal/pluginutil"
)

// host_surface.go is the single source of truth for what a plugin may call.
//
// Every runtime (lua, js, wasm) used to hardcode its own list of host helpers,
// so a helper added to one silently stayed missing from the others — `text.trim`,
// `text.unescape` and `text.lua_escape` were reachable from Lua and wasm but not
// from JS for exactly that reason. The table below is the definition; the
// runtimes read it instead of restating it, and helper_parity_test fails when a
// runtime cannot serve a name listed here.
//
// kindPure entries carry a Go function and are wired automatically by every
// runtime. kindRuntime entries are names whose call shape cannot be shared (a
// Lua doc handle, a goja object, a wasm JSON envelope), so each runtime still
// wires them itself — but their NAMES and presence are still declared here, so
// the surface is visible in one place.

// helperKind is the call shape of a pure helper. Runtimes switch on it to pick
// the wrapper that matches their native calling convention.
type helperKind uint8

const (
	kindStr1    helperKind = iota // func(string) string
	kindStr1Err                   // func(string) (string, error)
	kindStr2                      // func(string, string) string
	kindStr2Err                   // func(string, string) (string, error)
	kindStr3Err                   // func(string, string, string) (string, error)
	kindStr4Err                   // func(string, string, string, string) (string, error)
	kindFloat1                    // func(string) float64
	kindRuntime                   // wired per runtime; fn is nil
)

// hostHelper is one entry of the plugin-facing host surface. name is the
// "<namespace>.<helper>" key the wasm dispatcher receives; Lua/JS split it into
// a table and a field.
type hostHelper struct {
	name string
	kind helperKind
	fn   any
	note string // why a kindRuntime helper cannot be table-driven
}

// hostSurface lists every helper a plugin may reach, grouped by namespace. The
// order inside a group is the order the runtimes register them, which keeps the
// generated Lua/JS tables stable across runs.
var hostSurface = []hostHelper{
	// ── host.text ──
	{"text.url_encode", kindStr1, pluginutil.URLEncode, ""},
	{"text.url_decode", kindStr1, pluginutil.URLDecode, ""},
	{"text.html_decode", kindStr1, pluginutil.HTMLDecode, ""},
	{"text.strip_html", kindStr1, pluginutil.StripHTML, ""},
	{"text.strip_markdown", kindStr1, pluginutil.StripMarkdown, ""},
	{"text.titlecase", kindStr1, pluginutil.Titlecase, ""},
	{"text.normalize_title", kindStr1, pluginutil.NormalizeTitle, ""},
	{"text.strip_link_blocks", kindStr1, pluginutil.StripLinkBlocks, ""},
	{"text.normalize_status", kindRuntime, nil, "one-arg and (map, raw) forms differ per runtime"},
	{"text.unescape", kindStr1, pluginutil.HTMLEntityUnescape, ""},
	{"text.trim", kindStr1, strings.TrimSpace, ""},
	{"text.lua_escape", kindStr1, pluginutil.LuaEscape, ""},
	{"text.chapter_num", kindFloat1, pluginutil.ChapterNum, ""},
	{"text.json_blob", kindStr2, pluginutil.JSONBlob, ""},
	{"text.date_to_iso", kindStr1, pluginutil.DateToISONow, ""},

	// ── host.codecs ──
	{"codecs.base64_encode", kindStr1, pluginutil.Base64Encode, ""},
	{"codecs.base64_decode", kindStr1Err, pluginutil.Base64Decode, ""},
	{"codecs.base64url_encode", kindStr1, pluginutil.Base64URLEncode, ""},
	{"codecs.base64url_decode", kindStr1Err, pluginutil.Base64URLDecode, ""},
	{"codecs.hex_encode", kindStr1, pluginutil.HexEncode, ""},
	{"codecs.hex_decode", kindStr1Err, pluginutil.HexDecode, ""},
	{"codecs.b64_decode_hex", kindStr1Err, pluginutil.B64DecodeHex, ""},
	{"codecs.b64url_encode_hex", kindStr1Err, pluginutil.B64URLEncodeHex, ""},
	{"codecs.b64url_decode_hex", kindStr1Err, pluginutil.B64URLDecodeHex, ""},

	// ── host.crypto ──
	{"crypto.sha256_hex", kindStr1, pluginutil.SHA256Hex, ""},
	{"crypto.md5_hex", kindStr1, pluginutil.MD5Hex, ""},
	{"crypto.hmac_sha256_hex", kindStr2, pluginutil.HMACSHA256Hex, ""},
	{"crypto.xor", kindStr2Err, pluginutil.XORHex, ""},
	{"crypto.utf8_hex", kindStr1, pluginutil.UTF8Hex, ""},
	{"crypto.vrf_sign", kindRuntime, nil, "takes native tables in Lua/JS, JSON in wasm"},
	{"crypto.aes_gcm_decrypt", kindStr4Err, pluginutil.AESGCMDecryptB64, ""},
	{"crypto.substitute_cipher", kindStr3Err, pluginutil.SubstituteCipher, ""},

	// ── host.json ──
	{"json.decode", kindRuntime, nil, "converts to the runtime's own value type"},
	{"json.encode", kindRuntime, nil, "converts from the runtime's own value type"},

	// ── host.html (stateless in wasm, doc-handle in Lua/JS) ──
	{"html.parse", kindRuntime, nil, "Lua/JS return an opaque document handle"},
	{"html.find_text", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.find_attr", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.find_list_text", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.find_list_attr", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.xpath_text", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.xpath_attr", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.xpath_list_text", kindRuntime, nil, "takes a doc handle in Lua/JS"},
	{"html.xpath_list_attr", kindRuntime, nil, "takes a doc handle in Lua/JS"},

	// ── host.regex ──
	{"regex.find", kindRuntime, nil, "multi-value returns per language"},
	{"regex.match", kindRuntime, nil, "multi-value returns per language"},
	{"regex.find_all", kindRuntime, nil, "multi-value returns per language"},
	{"regex.gmatch", kindRuntime, nil, "Lua iterator; no JSON equivalent"},
	{"regex.find_index", kindRuntime, nil, "multi-value returns per language"},
	{"regex.quote", kindStr1, pluginutil.RegexQuote, ""},
	{"regex.replace", kindRuntime, nil, "multi-value returns per language"},

	// ── host.http (needs the manager, plugin id and invoke context) ──
	{"http.get", kindRuntime, nil, "needs manager + plugin id + ctx"},
	{"http.post", kindRuntime, nil, "needs manager + plugin id + ctx"},
	{"http.get_body", kindRuntime, nil, "needs manager + plugin id + ctx"},
	{"http.post_body", kindRuntime, nil, "needs manager + plugin id + ctx"},

	// ── host.browser (CDP; same per-invoke dependencies as http) ──
	{"browser.fetch", kindRuntime, nil, "needs manager + plugin id"},
	{"browser.evaluate", kindRuntime, nil, "needs manager + plugin id"},
	{"browser.evaluate_with_init", kindRuntime, nil, "needs manager + plugin id"},

	// ── log (globals in Lua/JS, a host_call key in wasm) ──
	{"log.debug", kindRuntime, nil, "global in Lua/JS, host_call key in wasm"},
	{"log.info", kindRuntime, nil, "global in Lua/JS, host_call key in wasm"},
	{"log.warn", kindRuntime, nil, "global in Lua/JS, host_call key in wasm"},
	{"log.error", kindRuntime, nil, "global in Lua/JS, host_call key in wasm"},
}

// hostSurfaceIndex resolves a "<namespace>.<helper>" key to its table row. The
// wasm dispatcher looks helpers up here instead of in per-kind maps.
var hostSurfaceIndex = func() map[string]hostHelper {
	m := make(map[string]hostHelper, len(hostSurface))
	for _, h := range hostSurface {
		m[h.name] = h
	}
	return m
}()

// splitHostName splits "<namespace>.<helper>".
func splitHostName(name string) (ns, fn string) {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return name[:i], name[i+1:]
		}
	}
	return name, ""
}

// hostSurfaceNames returns every name in the table, in declaration order.
func hostSurfaceNames() []string {
	out := make([]string, 0, len(hostSurface))
	for _, h := range hostSurface {
		out = append(out, h.name)
	}
	return out
}

// hostSurfaceByNamespace groups the pure helpers by namespace, so a runtime can
// build its table without walking the flat list.
func hostSurfaceByNamespace() map[string][]hostHelper {
	out := map[string][]hostHelper{}
	for _, h := range hostSurface {
		ns, _ := splitHostName(h.name)
		out[ns] = append(out[ns], h)
	}
	return out
}

// invoke runs one kindPure helper with positional string arguments and returns
// its value and an error. ok is false for kindRuntime entries, telling the
// caller to use its own wiring.
func (h hostHelper) invoke(args []string) (out any, err error, ok bool) {
	arg := func(i int) string { return wasmArg(args, i) }
	switch h.kind {
	case kindStr1:
		return h.fn.(func(string) string)(arg(0)), nil, true
	case kindStr1Err:
		v, e := h.fn.(func(string) (string, error))(arg(0))
		return v, e, true
	case kindStr2:
		return h.fn.(func(string, string) string)(arg(0), arg(1)), nil, true
	case kindStr2Err:
		v, e := h.fn.(func(string, string) (string, error))(arg(0), arg(1))
		return v, e, true
	case kindStr3Err:
		v, e := h.fn.(func(string, string, string) (string, error))(arg(0), arg(1), arg(2))
		return v, e, true
	case kindStr4Err:
		v, e := h.fn.(func(string, string, string, string) (string, error))(arg(0), arg(1), arg(2), arg(3))
		return v, e, true
	case kindFloat1:
		return h.fn.(func(string) float64)(arg(0)), nil, true
	}
	return nil, nil, false
}
