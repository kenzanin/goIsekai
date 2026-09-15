package pluginmanager

import (
	"errors"

	"github.com/dop251/goja"
)

// jsJSONDecode wraps host.json.decode for the goja runtime: a JSON string
// becomes a JS object, array or primitive. Failures throw, which is how the
// other JS natives report errors.
func jsJSONDecode(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		s, ok := call.Argument(0).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New(jsonDecodeArgErr)))
		}
		decoded, err := jsonDecodeString(s)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(decoded)
	}
}

// jsJSONEncode wraps host.json.encode for the goja runtime: a JS value becomes a
// JSON string. Values with no JSON form (functions) throw instead of producing a
// partial document.
func jsJSONEncode(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		out, err := jsonEncodeValue(call.Argument(0).Export())
		if err != nil {
			panic(vm.NewGoError(errors.New(jsonEncodeErr)))
		}
		return vm.ToValue(out)
	}
}
