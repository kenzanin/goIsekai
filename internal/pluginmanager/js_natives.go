package pluginmanager

import (
	"github.com/goccy/go-json"

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
			"url_encode":     jsStr1(vm, pluginutil.URLEncode),
			"url_decode":     jsStr1(vm, pluginutil.URLDecode),
			"html_decode":    jsStr1(vm, pluginutil.HTMLDecode),
			"strip_html":     jsStr1(vm, pluginutil.StripHTML),
			"strip_markdown": jsStr1(vm, pluginutil.StripMarkdown),
			"titlecase":      jsStr1(vm, pluginutil.Titlecase),
			"chapter_num":    jsFloat1(vm, pluginutil.ChapterNum),
			"json_blob":      jsStr2(vm, pluginutil.JSONBlob),
			"date_to_iso":    jsStr1(vm, pluginutil.DateToISONow),
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
			"sha256_hex":      jsStr1(vm, pluginutil.SHA256Hex),
			"md5_hex":         jsStr1(vm, pluginutil.MD5Hex),
			"hmac_sha256_hex": jsStr2(vm, pluginutil.HMACSHA256Hex),
			"xor":             jsStr2Err(vm, pluginutil.XORHex),
			"utf8_hex":        jsStr1(vm, pluginutil.UTF8Hex),
			"vrf_sign":        jsVrfSign(vm),
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

// jsHTTPRequest runs one proxied request and decodes the response object.
func jsHTTPRequest(m *Manager, id, method, url, body string, headers any) (map[string]any, error) {
	req := map[string]any{"url": url, "method": method}
	if body != "" {
		req["body"] = body
	}
	if headers != nil {
		req["headers"] = headers
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// jsHTTPFn builds one host.http native, mirroring the Lua runtime. The _body
// variants hand back only the response body, or null when the request failed or
// the server did not answer 200.
func jsHTTPFn(vm *goja.Runtime, m *Manager, id, method string, bodyOnly bool) func(goja.FunctionCall) goja.Value {
	post := method == "POST"
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		body := ""
		headersIndex := 1
		if post {
			headersIndex = 2
			if len(call.Arguments) > 1 {
				body = call.Arguments[1].String()
			}
		}
		resp, err := jsHTTPRequest(m, id, method, url, body, jsHTTPHeaders(vm, call, headersIndex))
		if bodyOnly {
			// Null covers both a transport failure and a non-200 answer, so a
			// plugin can treat one nil check as "the fetch failed".
			if err != nil {
				return goja.Null()
			}
			if text, ok := responseBody(resp); ok {
				return vm.ToValue(text)
			}
			return goja.Null()
		}
		if err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		return vm.ToValue(resp)
	}
}

// jsHttpGet wraps host.http.get(url, headers?) → {status, headers, body}.
func jsHttpGet(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "GET", false)
}

// jsHttpPost wraps host.http.post(url, body, headers?) → {status, headers, body}.
func jsHttpPost(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "POST", false)
}

// jsHTTPGetBody wraps host.http.get_body(url, headers?) → body, or null on a
// failed request or a non-200 response.
func jsHTTPGetBody(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "GET", true)
}

// jsHTTPPostBody wraps host.http.post_body(url, body, headers?) → body, or null
// on a failed request or a non-200 response.
func jsHTTPPostBody(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "POST", true)
}
