package pluginmanager

import (
	"github.com/dop251/goja"

	"goisekai/internal/pluginutil"
)

// registerJSHostNatives installs the shared `host` object (text/codecs/crypto)
// on the goja VM. Mirrors the Lua runtime's surface.
func registerJSHostNatives(vm *goja.Runtime) error {
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
			"base64_encode":    jsStr1(vm, pluginutil.Base64Encode),
			"base64_decode":    jsStr1Err(vm, pluginutil.Base64Decode),
			"base64url_encode": jsStr1(vm, pluginutil.Base64URLEncode),
			"base64url_decode": jsStr1Err(vm, pluginutil.Base64URLDecode),
			"hex_encode":       jsStr1(vm, pluginutil.HexEncode),
			"hex_decode":       jsStr1Err(vm, pluginutil.HexDecode),
		}},
		{"crypto", map[string]any{
			"sha256_hex":      jsStr1(vm, pluginutil.SHA256Hex),
			"md5_hex":         jsStr1(vm, pluginutil.MD5Hex),
			"hmac_sha256_hex": jsStr2(vm, pluginutil.HMACSHA256Hex),
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
	return vm.Set("host", host)
}

func jsStr1(vm *goja.Runtime, fn func(string) string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fn(call.Argument(0).String()))
	}
}

func jsStr1Err(vm *goja.Runtime, fn func(string) (string, error)) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		out, err := fn(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(out)
	}
}

func jsStr2(vm *goja.Runtime, fn func(string, string) string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fn(call.Argument(0).String(), call.Argument(1).String()))
	}
}
