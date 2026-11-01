package pluginmanager

import (
	"encoding/json"
	"errors"

	"modernc.org/quickjs"
)

// jsJSONDecode wraps host.json.decode for the quickjs runtime: a JSON string
// becomes a JS object, array or primitive. Failures throw, which is how the
// other JS natives report errors.
func jsJSONDecode(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return nil, errors.New(jsonDecodeArgErr)
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, errors.New(jsonDecodeArgErr)
		}
		if _, err := jsonDecodeString(s); err != nil {
			return nil, err
		}
		// Round-trip through JSON.parse so the JS object/array structure is
		// preserved, exactly as goja's ToValue(decoded) did.
		literal, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		out, err := vm.Eval("JSON.parse("+string(literal)+")", quickjs.EvalGlobal)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
}

// jsJSONEncode wraps host.json.encode for the quickjs runtime: a JS value
// becomes a JSON string. Values with no JSON form (functions) throw instead
// of producing a partial document.
func jsJSONEncode(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return "", errors.New(jsonEncodeErr)
		}
		v := args[0]
		// A JS object arrives as *quickjs.Object, which keeps JS insertion
		// order. Decode it into a Go map so the encoder sorts keys, as the
		// Lua runtime's encoder does.
		if obj, ok := v.(*quickjs.Object); ok {
			var g any
			if err := obj.Into(&g); err != nil {
				return "", errors.New(jsonEncodeErr)
			}
			v = g
		}
		out, err := jsonEncodeValue(v)
		if err != nil {
			return "", errors.New(jsonEncodeErr)
		}
		return out, nil
	}
}
