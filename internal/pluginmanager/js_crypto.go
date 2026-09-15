package pluginmanager

import (
	"github.com/dop251/goja"

	"goisekai/internal/pluginutil"
)

// jsVrfSign wraps host.crypto.vrf_sign(apiPath, params, stages) — the VRF
// signer algorithm in Go, with the current table set supplied by the plugin
// (so rotations are a plugin-side constant edit, no host rebuild).
// stages: [{iv:int, key:<b64>, tbl:<b64>}...]; returns the base64url signature.
func jsVrfSign(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		apiPath := call.Arguments[0].String()

		params := map[string]string{}
		if len(call.Arguments) > 1 {
			arg := call.Arguments[1]
			if !arg.SameAs(goja.Undefined()) && !arg.SameAs(goja.Null()) {
				obj := arg.ToObject(vm)
				for _, k := range obj.Keys() {
					params[k] = obj.Get(k).String()
				}
			}
		}

		var stages []pluginutil.VRFStageB64
		if len(call.Arguments) > 2 {
			arg := call.Arguments[2]
			if !arg.SameAs(goja.Undefined()) && !arg.SameAs(goja.Null()) {
				stages, _ = jsVRFStages(arg)
			}
		}

		decoded, err := pluginutil.VRFStagesB64(stages)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(pluginutil.VRFSign(apiPath, params, decoded))
	}
}

// jsVRFStages converts a goja array/object of stages to wire form.
func jsVRFStages(v goja.Value) ([]pluginutil.VRFStageB64, error) {
	var stages []pluginutil.VRFStageB64
	if obj, ok := v.Export().([]any); ok {
		for _, item := range obj {
			if m, ok := item.(map[string]any); ok {
				s, err := pluginutil.VRFStageFromMap(m)
				if err != nil {
					return nil, err
				}
				stages = append(stages, s)
			}
		}
	}
	return stages, nil
}

// jsNormalizeStatus creates a callable function with a .default property.
// Usage: host.text.normalize_status(raw) for the default vocabulary, or
// host.text.normalize_status(map, raw) to extend it. An empty map means
// "use the defaults".
func jsNormalizeStatus(vm *goja.Runtime) *goja.Object {
	fn := func(call goja.FunctionCall) goja.Value {
		var m map[string]string
		var raw string
		if len(call.Arguments) > 1 {
			if arg := call.Arguments[0]; !arg.SameAs(goja.Undefined()) && !arg.SameAs(goja.Null()) {
				if obj, ok := arg.(*goja.Object); ok {
					keys := obj.Keys()
					if len(keys) > 0 {
						m = make(map[string]string, len(keys))
						for _, k := range keys {
							if v := obj.Get(k); v != nil {
								m[k] = v.String()
							}
						}
					}
				}
			}
			raw = call.Arguments[1].String()
		} else if len(call.Arguments) > 0 {
			raw = call.Arguments[0].String()
		}
		return vm.ToValue(pluginutil.NormalizeStatus(m, raw))
	}
	fnVal := vm.ToValue(fn)
	fnObj := fnVal.ToObject(vm)
	_ = fnObj.Set("default", vm.ToValue(pluginutil.DefaultStatusMap()))
	return fnObj
}
