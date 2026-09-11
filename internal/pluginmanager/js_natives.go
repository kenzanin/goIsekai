package pluginmanager

import (
	"encoding/json"

	"github.com/dop251/goja"

	"goisekai/internal/pluginutil"
)

// registerJSHostNatives installs the shared `host` object (text/codecs/crypto/http)
// on the goja VM. Mirrors the Lua runtime's surface.
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
		if err := host.Set(g.name, obj); err != nil {
			return err
		}
	}

	// host.http — thin wrappers over http_request proxy.
	httpObj := vm.NewObject()
	if err := httpObj.Set("get", jsHttpGet(vm, m, id)); err != nil {
		return err
	}
	if err := httpObj.Set("post", jsHttpPost(vm, m, id)); err != nil {
		return err
	}
	if err := host.Set("http", httpObj); err != nil {
		return err
	}

	return vm.Set("host", host)
}

// jsHttpGet wraps host.http.get(url, headers?) → {status, headers, body}.
func jsHttpGet(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		var headers any
		if len(call.Arguments) > 1 {
			arg := call.Arguments[1]
			if !arg.SameAs(goja.Undefined()) && !arg.SameAs(goja.Null()) {
				obj := arg.ToObject(vm)
				m := make(map[string]any)
				for _, key := range obj.Keys() {
					m[key] = obj.Get(key).Export()
				}
				headers = m
			}
		}
		req := map[string]any{"url": url, "method": "GET"}
		if headers != nil {
			req["headers"] = headers
		}
		reqJSON, err := json.Marshal(req)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
		if err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		var respVal any
		if err := json.Unmarshal([]byte(respJSON), &respVal); err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		return vm.ToValue(respVal)
	}
}

// jsHttpPost wraps host.http.post(url, body, headers?) → {status, headers, body}.
func jsHttpPost(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		body := call.Arguments[1].String()
		var headers any
		if len(call.Arguments) > 2 {
			arg := call.Arguments[2]
			if !arg.SameAs(goja.Undefined()) && !arg.SameAs(goja.Null()) {
				obj := arg.ToObject(vm)
				m := make(map[string]any)
				for _, key := range obj.Keys() {
					m[key] = obj.Get(key).Export()
				}
				headers = m
			}
		}
		req := map[string]any{"url": url, "method": "POST", "body": body}
		if headers != nil {
			req["headers"] = headers
		}
		reqJSON, err := json.Marshal(req)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
		if err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		var respVal any
		if err := json.Unmarshal([]byte(respJSON), &respVal); err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		return vm.ToValue(respVal)
	}
}

func jsStr1(vm *goja.Runtime, fn func(string) string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fn(call.Arguments[0].String()))
	}
}

func jsStr1Err(vm *goja.Runtime, fn func(string) (string, error)) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		out, err := fn(call.Arguments[0].String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(out)
	}
}

func jsStr2(vm *goja.Runtime, fn func(string, string) string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fn(call.Arguments[0].String(), call.Arguments[1].String()))
	}
}

func jsStr2Err(vm *goja.Runtime, fn func(string, string) (string, error)) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		out, err := fn(call.Arguments[0].String(), call.Arguments[1].String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(out)
	}
}
