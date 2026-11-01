package pluginmanager

import (
	"fmt"

	"modernc.org/quickjs"
)

// registerJSHostNatives installs the shared `host` object
// (text/codecs/crypto/json/http) on the quickjs VM.
//
// The list of helpers lives in hostSurface (host_surface.go); this function only
// supplies the JS calling convention. Before the table existed, JS restated the
// list by hand and quietly fell behind — host.text.trim, host.text.unescape and
// host.text.lua_escape were reachable from Lua and wasm but not from JS.
func registerJSHostNatives(vm *quickjs.VM, m *Manager, id string) error {
	hostObj, err := vm.NewObjectValue()
	if err != nil {
		return fmt.Errorf("new host object: %w", err)
	}
	var scratch []string
	groups := map[string]quickjs.Value{}
	groupOf := func(ns string) (quickjs.Value, error) {
		if obj, ok := groups[ns]; ok {
			return obj, nil
		}
		obj, err := vm.NewObjectValue()
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		groups[ns] = obj
		return obj, nil
	}

	setFn := func(obj quickjs.Value, name string, fn quickjs.HostFunc) error {
		s := jsScratchName("host", name)
		if err := jsSetHostFunc(vm, obj, s, name, fn); err != nil {
			return err
		}
		scratch = append(scratch, s)
		return nil
	}

	// Pure helpers: the same Go function the Lua natives and the wasm
	// dispatcher use.
	for _, h := range hostSurface {
		if h.kind == kindRuntime {
			continue
		}
		ns, fn := splitHostName(h.name)
		obj, err := groupOf(ns)
		if err != nil {
			return err
		}
		if err := setFn(obj, fn, jsPure(h)); err != nil {
			return err
		}
	}

	// Helpers whose call shape is JS-specific.
	if obj, err := groupOf("text"); err == nil {
		if err := setFn(obj, "normalize_status", jsNormalizeStatus()); err != nil {
			return err
		}
	}
	if obj, err := groupOf("crypto"); err == nil {
		if err := setFn(obj, "vrf_sign", jsVrfSign(vm)); err != nil {
			return err
		}
	}
	if obj, err := groupOf("json"); err == nil {
		if err := setFn(obj, "decode", jsJSONDecode(vm)); err != nil {
			return err
		}
		if err := setFn(obj, "encode", jsJSONEncode(vm)); err != nil {
			return err
		}
	}
	if obj, err := groupOf("plugin"); err == nil {
		if err := setFn(obj, "param", jsPluginParam(vm, m, id)); err != nil {
			return err
		}
	}

	// host.html — HTML parsing helpers with an opaque handle.
	htmlObj, err := htmlGroupJS(vm)
	if err != nil {
		return err
	}
	groups["html"] = htmlObj

	// host.regex — the same Go regex engine the Lua runtime gets.
	regexObj, err := regexGroupJS(vm)
	if err != nil {
		return err
	}
	groups["regex"] = regexObj

	// host.http — thin wrappers over the http_request proxy.
	if httpObj, err := groupOf("http"); err == nil {
		if err := setFn(httpObj, "get", jsHttpGet(vm, m, id)); err != nil {
			return err
		}
		if err := setFn(httpObj, "post", jsHttpPost(vm, m, id)); err != nil {
			return err
		}
		if err := setFn(httpObj, "get_body", jsHTTPGetBody(vm, m, id)); err != nil {
			return err
		}
		if err := setFn(httpObj, "post_body", jsHTTPPostBody(vm, m, id)); err != nil {
			return err
		}
	}

	// host.browser — fetch pages through a real browser (CDP) so
	// client-side JavaScript runs.
	if browserObj, err := groupOf("browser"); err == nil {
		if err := setFn(browserObj, "fetch", jsBrowserFetch(vm, m, id)); err != nil {
			return err
		}
		if err := setFn(browserObj, "evaluate", jsBrowserEvaluate(vm, m, id)); err != nil {
			return err
		}
		if err := setFn(browserObj, "evaluate_with_init", jsBrowserEvaluateWithInit(vm, m, id)); err != nil {
			return err
		}
	}

	if err := jsDropScratch(vm, scratch...); err != nil {
		return err
	}

	for ns, obj := range groups {
		atom, err := vm.NewAtom(ns)
		if err != nil {
			return fmt.Errorf("new atom %s: %w", ns, err)
		}
		if err := vm.SetProperty(hostObj, atom, obj); err != nil {
			return err
		}
	}
	hostAtom, err := vm.NewAtom("host")
	if err != nil {
		return fmt.Errorf("new atom host: %w", err)
	}
	return vm.SetProperty(vm.GlobalObject(), hostAtom, hostObj)
}

// jsPure wraps one table entry as a quickjs host function, picking the wrapper
// that matches the helper's kind.
func jsPure(h hostHelper) quickjs.HostFunc {
	switch h.kind {
	case kindStr1:
		return jsStr1(h.fn.(func(string) string))
	case kindStr1Err:
		return jsStr1Err(h.fn.(func(string) (string, error)))
	case kindStr2:
		return jsStr2(h.fn.(func(string, string) string))
	case kindStr2Err:
		return jsStr2Err(h.fn.(func(string, string) (string, error)))
	case kindStr3Err:
		return jsStr3Err(h.fn.(func(string, string, string) (string, error)))
	case kindStr4Err:
		return jsStr4Err(h.fn.(func(string, string, string, string) (string, error)))
	case kindFloat1:
		return jsFloat1(h.fn.(func(string) float64))
	}
	panic("jsPure: unexpected helper kind for " + h.name)
}
