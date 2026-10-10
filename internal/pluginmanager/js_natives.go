package pluginmanager

import (
	"github.com/dop251/goja"

	"goisekai/internal/pluginutil"
)

// registerJSHostNatives installs the shared `host` object
// (text/codecs/crypto/json/http) on the goja VM. Mirrors the Lua runtime's surface.
func registerJSHostNatives(vm *goja.Runtime, m *Manager, id string) error {
	groups := []struct {
		name string
		fns  map[string]any
	}{
		{"text", map[string]any{
			"url_encode":        jsStr1(vm, pluginutil.URLEncode),
			"url_decode":        jsStr1(vm, pluginutil.URLDecode),
			"html_decode":       jsStr1(vm, pluginutil.HTMLDecode),
			"strip_html":        jsStr1(vm, pluginutil.StripHTML),
			"strip_markdown":    jsStr1(vm, pluginutil.StripMarkdown),
			"titlecase":         jsStr1(vm, pluginutil.Titlecase),
			"normalize_title":   jsStr1(vm, pluginutil.NormalizeTitle),
			"strip_link_blocks": jsStr1(vm, pluginutil.StripLinkBlocks),
			"chapter_num":       jsFloat1(vm, pluginutil.ChapterNum),
			"json_blob":         jsStr2(vm, pluginutil.JSONBlob),
			"date_to_iso":       jsStr1(vm, pluginutil.DateToISONow),
		}},
		{"codecs", map[string]any{
			"base64_encode":     jsStr1(vm, pluginutil.Base64Encode),
			"base64_decode":     jsStr1Err(vm, pluginutil.Base64Decode),
			"base64url_encode":  jsStr1(vm, pluginutil.Base64URLEncode),
			"base64url_decode":  jsStr1Err(vm, pluginutil.Base64URLDecode),
			"hex_encode":        jsStr1(vm, pluginutil.HexEncode),
			"hex_decode":        jsStr1Err(vm, pluginutil.HexDecode),
			"b64_decode_hex":    jsStr1Err(vm, pluginutil.B64DecodeHex),
			"b64url_encode_hex": jsStr1Err(vm, pluginutil.B64URLEncodeHex),
			"b64url_decode_hex": jsStr1Err(vm, pluginutil.B64URLDecodeHex),
		}},
		{"crypto", map[string]any{
			"sha256_hex":        jsStr1(vm, pluginutil.SHA256Hex),
			"md5_hex":           jsStr1(vm, pluginutil.MD5Hex),
			"hmac_sha256_hex":   jsStr2(vm, pluginutil.HMACSHA256Hex),
			"xor":               jsStr2Err(vm, pluginutil.XORHex),
			"utf8_hex":          jsStr1(vm, pluginutil.UTF8Hex),
			"vrf_sign":          jsVrfSign(vm),
			"aes_gcm_decrypt":   jsStr4Err(vm, pluginutil.AESGCMDecryptB64),
			"substitute_cipher": jsStr3Err(vm, pluginutil.SubstituteCipher),
		}},
		{"json", map[string]any{
			"decode": jsJSONDecode(vm),
			"encode": jsJSONEncode(vm),
		}},
	}

	host := vm.NewObject()
	for _, g := range groups {
		obj := vm.NewObject()
		for name, fn := range g.fns {
			if err := obj.Set(name, fn); err != nil {
				return err
			}
		}
		// Special case: host.text.normalize_status needs a .default property.
		if g.name == "text" {
			_ = obj.Set("normalize_status", jsNormalizeStatus(vm))
		}
		if err := host.Set(g.name, obj); err != nil {
			return err
		}
	}

	// host.html — HTML parsing helpers with opaque handle.
	htmlObj, err := htmlGroupJS(vm)
	if err != nil {
		return err
	}
	if err := host.Set("html", htmlObj); err != nil {
		return err
	}

	// host.regex — the same Go regex engine the Lua runtime gets.
	regexObj, err := regexGroupJS(vm)
	if err != nil {
		return err
	}
	if err := host.Set("regex", regexObj); err != nil {
		return err
	}

	// host.http — thin wrappers over http_request proxy.
	httpObj := vm.NewObject()
	if err := httpObj.Set("get", jsHttpGet(vm, m, id)); err != nil {
		return err
	}
	if err := httpObj.Set("post", jsHttpPost(vm, m, id)); err != nil {
		return err
	}
	if err := httpObj.Set("get_body", jsHTTPGetBody(vm, m, id)); err != nil {
		return err
	}
	if err := httpObj.Set("post_body", jsHTTPPostBody(vm, m, id)); err != nil {
		return err
	}
	if err := host.Set("http", httpObj); err != nil {
		return err
	}

	// host.browser — fetch pages through a real browser (CDP) so
	// client-side JavaScript runs.
	browserObj := vm.NewObject()
	if err := browserObj.Set("fetch", jsBrowserFetch(vm, m, id)); err != nil {
		return err
	}
	if err := browserObj.Set("evaluate", jsBrowserEvaluate(vm, m, id)); err != nil {
		return err
	}
	if err := browserObj.Set("evaluate_with_init", jsBrowserEvaluateWithInit(vm, m, id)); err != nil {
		return err
	}
	if err := host.Set("browser", browserObj); err != nil {
		return err
	}

	return vm.Set("host", host)
}

// jsHTTPHeaders reads an http native's optional trailing object argument.
func jsHTTPHeaders(vm *goja.Runtime, call goja.FunctionCall, index int) any {
	if len(call.Arguments) <= index {
		return nil
	}
	arg := call.Arguments[index]
	if arg.SameAs(goja.Undefined()) || arg.SameAs(goja.Null()) {
		return nil
	}
	obj := arg.ToObject(vm)
	m := make(map[string]any, len(obj.Keys()))
	for _, key := range obj.Keys() {
		m[key] = obj.Get(key).Export()
	}
	return m
}
