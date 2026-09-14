package pluginmanager

import (
	"github.com/dop251/goja"
)

// jsStr1 wraps a string->string helper as a Goja native function.
func jsStr1(vm *goja.Runtime, fn func(string) string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fn(call.Arguments[0].String()))
	}
}

// jsStr1Err wraps a string->(string,error) helper, panicking on error.
func jsStr1Err(vm *goja.Runtime, fn func(string) (string, error)) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		out, err := fn(call.Arguments[0].String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(out)
	}
}

// jsStr2 wraps a (string,string)->string helper as a Goja native function.
func jsStr2(vm *goja.Runtime, fn func(string, string) string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fn(call.Arguments[0].String(), call.Arguments[1].String()))
	}
}

// jsStr2Err wraps a (string,string)->(string,error) helper, panicking on error.
func jsStr2Err(vm *goja.Runtime, fn func(string, string) (string, error)) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		out, err := fn(call.Arguments[0].String(), call.Arguments[1].String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(out)
	}
}
