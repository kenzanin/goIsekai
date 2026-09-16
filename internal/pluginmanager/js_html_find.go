package pluginmanager

import (
	"errors"

	"github.com/dop251/goja"
	"goisekai/internal/htmldoc"
)

// jsHTMLParse wraps host.html.parse for the goja runtime: a markup string
// becomes a document handle. On failure it throws, the JS native convention.
func jsHTMLParse(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		s, ok := call.Argument(0).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New(htmlParseArgErr)))
		}
		doc, err := htmldoc.Parse(s)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		handle := &jsHTMLOpaqueHandle{doc: doc}
		return vm.ToValue(handle)
	}
}

// jsHTMLFindText wraps host.html.find_text for the goja runtime.
func jsHTMLFindText(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_text: argument 0 must be a document handle")))
		}
		selector, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_text: argument 1 must be a string")))
		}
		text, err := handle.doc.FindText(selector)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(text)
	}
}

// jsHTMLFindAttr wraps host.html.find_attr for the goja runtime.
func jsHTMLFindAttr(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_attr: argument 0 must be a document handle")))
		}
		selector, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_attr: argument 1 must be a string")))
		}
		attr, ok := call.Argument(2).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_attr: argument 2 must be a string")))
		}
		val, err := handle.doc.FindAttr(selector, attr)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(val)
	}
}

// jsHTMLFindListText wraps host.html.find_list_text for the goja runtime.
func jsHTMLFindListText(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_list_text: argument 0 must be a document handle")))
		}
		selector, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_list_text: argument 1 must be a string")))
		}
		texts, err := handle.doc.FindListText(selector)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(texts)
	}
}

// jsHTMLFindListAttr wraps host.html.find_list_attr for the goja runtime.
func jsHTMLFindListAttr(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_list_attr: argument 0 must be a document handle")))
		}
		selector, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_list_attr: argument 1 must be a string")))
		}
		attr, ok := call.Argument(2).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.find_list_attr: argument 2 must be a string")))
		}
		vals, err := handle.doc.FindListAttr(selector, attr)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(vals)
	}
}
