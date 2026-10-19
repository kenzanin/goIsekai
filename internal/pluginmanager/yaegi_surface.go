package pluginmanager

import (
	"reflect"

	"goisekai/internal/pluginutil"
)

// yaegi_surface.go is the yaegi counterpart of host_surface.go.
//
// yaegi needed its own table rather than a rename layer over hostSurface for a
// hard reason: their call shapes differ. A script runtime asks for
// "text.url_encode" and gets a dynamic value back; an interpreted Go plugin
// calls hostnet.URLEncode(s) and needs a statically typed Go function. The same
// helper therefore has to be published twice, under two spellings.
//
// What must NOT differ is behaviour. Every entry that has a hostSurface
// counterpart points at that row's function, and
// TestYaegiSurfaceMatchesHostSurface asserts identity (not mere equality of
// output), so the two surfaces cannot drift apart. Entries with an empty helper
// key are the cases hostSurface marks kindRuntime only because Lua and JS
// disagree on return shape — in Go a multi-value return is unambiguous, so the
// raw function is exactly right.
//
// Before this table, yaegi published 11 hand-written methods and nothing else:
// no codecs, no crypto, no regex, no text sanitising. A yaegi plugin had to
// hand-roll all of it.

// yaegiHelper is one function the synthetic `hostnet` package exposes.
type yaegiHelper struct {
	goName string // hostnet.<goName>
	fn     any
	helper string // hostSurface key when one exists; "" when yaegi uses the raw fn
	note   string // why helper is "" for an entry that has a hostSurface row
}

// yaegiHelpers is the yaegi-facing surface, in the order it is published.
var yaegiHelpers = []yaegiHelper{
	// ── text ──
	{goName: "URLEncode", helper: "text.url_encode"}, {goName: "URLDecode", helper: "text.url_decode"},
	{goName: "HTMLDecode", helper: "text.html_decode"},
	{goName: "StripHTML", helper: "text.strip_html"},
	{goName: "StripMarkdown", helper: "text.strip_markdown"},
	{goName: "Titlecase", helper: "text.titlecase"},
	{goName: "NormalizeTitle", helper: "text.normalize_title"},
	{goName: "StripLinkBlocks", helper: "text.strip_link_blocks"},
	{goName: "NormalizeStatus", fn: pluginutil.NormalizeStatus, helper: "text.normalize_status",
		note: "Go passes the override map directly; Lua/JS wrap it differently"},
	{goName: "HTMLEntityUnescape", helper: "text.unescape"},
	{goName: "TrimSpace", helper: "text.trim"},
	{goName: "LuaEscape", helper: "text.lua_escape"},
	{goName: "ChapterNum", helper: "text.chapter_num"},
	{goName: "JSONBlob", helper: "text.json_blob"},
	{goName: "DateToISO", helper: "text.date_to_iso"},

	// ── codecs ──
	{goName: "Base64Encode", helper: "codecs.base64_encode"},
	{goName: "Base64Decode", helper: "codecs.base64_decode"},
	{goName: "Base64URLEncode", helper: "codecs.base64url_encode"},
	{goName: "Base64URLDecode", helper: "codecs.base64url_decode"},
	{goName: "HexEncode", helper: "codecs.hex_encode"},
	{goName: "HexDecode", helper: "codecs.hex_decode"},
	{goName: "B64DecodeHex", helper: "codecs.b64_decode_hex"},
	{goName: "B64URLEncodeHex", helper: "codecs.b64url_encode_hex"},
	{goName: "B64URLDecodeHex", helper: "codecs.b64url_decode_hex"},

	// ── crypto ──
	{goName: "SHA256Hex", helper: "crypto.sha256_hex"},
	{goName: "MD5Hex", helper: "crypto.md5_hex"},
	{goName: "HMACSHA256Hex", helper: "crypto.hmac_sha256_hex"},
	{goName: "XORHex", helper: "crypto.xor"},
	{goName: "UTF8Hex", helper: "crypto.utf8_hex"},
	{goName: "VRFSign", fn: pluginutil.VRFSign, helper: "crypto.vrf_sign",
		note: "Go takes the stage slice directly; Lua/JS marshal JSON"},
	{goName: "AESGCMDecryptB64", helper: "crypto.aes_gcm_decrypt"},
	{goName: "SubstituteCipher", helper: "crypto.substitute_cipher"},

	// ── regex ──
	// hostSurface calls these kindRuntime because Lua returns captures as
	// separate values and JS as arrays. Go has neither problem: the function
	// signature already says what comes back, so the raw function is published.
	{goName: "RegexQuote", helper: "regex.quote"},
	{goName: "RegexFind", fn: pluginutil.RegexFind, helper: "regex.find",
		note: "multi-value return is unambiguous in Go"},
	{goName: "RegexMatch", fn: pluginutil.RegexMatch, helper: "regex.match",
		note: "multi-value return is unambiguous in Go"},
	{goName: "RegexFindAll", fn: pluginutil.RegexFindAll, helper: "regex.find_all",
		note: "multi-value return is unambiguous in Go"},
	{goName: "RegexFindIndex", fn: pluginutil.RegexFindIndex, helper: "regex.find_index",
		note: "multi-value return is unambiguous in Go"},
	{goName: "RegexReplace", fn: pluginutil.RegexReplace, helper: "regex.replace",
		note: "Go passes the replacement as a string; per-match callbacks are a Lua/JS idiom"},
}

// yaegiExports builds the reflect.Value map the interpreter imports as
// `hostnet`. Pure entries take their function from hostSurface so the two
// tables cannot disagree about behaviour.
func yaegiExports() map[string]reflect.Value {
	out := make(map[string]reflect.Value, len(yaegiHelpers))
	for _, h := range yaegiHelpers {
		fn := h.fn
		if fn == nil {
			row, ok := hostSurfaceIndex[h.helper]
			if !ok || row.fn == nil {
				// A test asserts this cannot happen; skipping keeps a bad row
				// from taking the whole plugin load down.
				continue
			}
			fn = row.fn
		}
		out[h.goName] = reflect.ValueOf(fn)
	}
	return out
}
