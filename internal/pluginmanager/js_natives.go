package pluginmanager

import (
	"github.com/dop251/goja"
)

// registerJSHostNatives installs the shared `host` object
// (text/codecs/crypto/json/http) on the goja VM.
//
// The list of helpers lives in hostSurface (host_surface.go); this function only
// supplies the JS calling convention. Before the table existed, JS restated the
// list by hand and quietly fell behind — host.text.trim, host.text.unescape and
// host.text.lua_escape were reachable from Lua and wasm but not from JS.
func registerJSHostNatives(vm *goja.Runtime, m *Manager, id string) error {
	host := vm.NewObject()
	groups := map[string]goja.Value{}
	groupOf := func(ns string) *goja.Object {
		if obj, ok := groups[ns].(*goja.Object); ok {
			return obj
		}
		obj := vm.NewObject()
		groups[ns] = obj
		return obj
	}

	// Pure helpers: the same Go function the Lua natives and the wasm
	// dispatcher use.
	for _, h := range hostSurface {
		if h.kind == kindRuntime {
			continue
		}
		ns, fn := splitHostName(h.name)
		if err := groupOf(ns).Set(fn, jsPure(vm, h)); err != nil {
			return err
		}
	}

	// Helpers whose call shape is JS-specific.
	// host.text.normalize_status also carries a .default property.
	normalize := jsNormalizeStatus(vm)
	if err := groupOf("text").Set("normalize_status", normalize); err != nil {
		return err
	}
	if err := groupOf("crypto").Set("vrf_sign", jsVrfSign(vm)); err != nil {
		return err
	}
	if err := groupOf("json").Set("decode", jsJSONDecode(vm)); err != nil {
		return err
	}
	if err := groupOf("json").Set("encode", jsJSONEncode(vm)); err != nil {
		return err
	}
	if err := groupOf("plugin").Set("param", jsPluginParam(vm, m, id)); err != nil {
		return err
	}

	// host.html — HTML parsing helpers with an opaque handle.
	htmlObj, err := htmlGroupJS(vm)
	if err != nil {
		return err
	}

	// host.regex — the same Go regex engine the Lua runtime gets.
	regexObj, err := regexGroupJS(vm)
	if err != nil {
		return err
	}
	options := map[string]goja.Value{"html": htmlObj, "regex": regexObj}
	for ns, obj := range options {
		groups[ns] = obj
	}

	// host.http — thin wrappers over the http_request proxy.
	httpObj := groupOf("http")
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

	// host.browser — fetch pages through a real browser (CDP) so
	// client-side JavaScript runs.
	browserObj := groupOf("browser")
	if err := browserObj.Set("fetch", jsBrowserFetch(vm, m, id)); err != nil {
		return err
	}
	if err := browserObj.Set("evaluate", jsBrowserEvaluate(vm, m, id)); err != nil {
		return err
	}
	if err := browserObj.Set("evaluate_with_init", jsBrowserEvaluateWithInit(vm, m, id)); err != nil {
		return err
	}

	for ns, obj := range groups {
		if err := host.Set(ns, obj); err != nil {
			return err
		}
	}
	return vm.Set("host", host)
}

// jsPure wraps one table entry as a goja native, picking the wrapper that
// matches the helper's kind.
func jsPure(vm *goja.Runtime, h hostHelper) any {
	switch h.kind {
	case kindStr1:
		return jsStr1(vm, h.fn.(func(string) string))
	case kindStr1Err:
		return jsStr1Err(vm, h.fn.(func(string) (string, error)))
	case kindStr2:
		return jsStr2(vm, h.fn.(func(string, string) string))
	case kindStr2Err:
		return jsStr2Err(vm, h.fn.(func(string, string) (string, error)))
	case kindStr3Err:
		return jsStr3Err(vm, h.fn.(func(string, string, string) (string, error)))
	case kindStr4Err:
		return jsStr4Err(vm, h.fn.(func(string, string, string, string) (string, error)))
	case kindFloat1:
		return jsFloat1(vm, h.fn.(func(string) float64))
	}
	panic("jsPure: unexpected helper kind for " + h.name)
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
