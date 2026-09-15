package pluginmanager

import (
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"goisekai/internal/htmldoc"
)

// jsHTMLOpaqueHandle wraps a *htmldoc.Document in a JS object the plugin cannot
// inspect, only pass back to the host functions.
type jsHTMLOpaqueHandle struct {
	doc *htmldoc.Document
}

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

// jsHTMLXPathText wraps host.html.xpath_text for the goja runtime.
func jsHTMLXPathText(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_text: argument 0 must be a document handle")))
		}
		expr, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_text: argument 1 must be a string")))
		}
		text, err := handle.doc.XPathText(expr)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(text)
	}
}

// jsHTMLXPathAttr wraps host.html.xpath_attr for the goja runtime.
func jsHTMLXPathAttr(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_attr: argument 0 must be a document handle")))
		}
		expr, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_attr: argument 1 must be a string")))
		}
		attr, ok := call.Argument(2).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_attr: argument 2 must be a string")))
		}
		val, err := handle.doc.XPathAttr(expr, attr)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(val)
	}
}

// jsHTMLXPathListText wraps host.html.xpath_list_text for the goja runtime.
func jsHTMLXPathListText(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_list_text: argument 0 must be a document handle")))
		}
		expr, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_list_text: argument 1 must be a string")))
		}
		texts, err := handle.doc.XPathListText(expr)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(texts)
	}
}

// jsHTMLXPathListAttr wraps host.html.xpath_list_attr for the goja runtime.
func jsHTMLXPathListAttr(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		handle, ok := call.Argument(0).Export().(*jsHTMLOpaqueHandle)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_list_attr: argument 0 must be a document handle")))
		}
		expr, ok := call.Argument(1).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_list_attr: argument 1 must be a string")))
		}
		attr, ok := call.Argument(2).Export().(string)
		if !ok {
			panic(vm.NewGoError(errors.New("host.html.xpath_list_attr: argument 2 must be a string")))
		}
		vals, err := handle.doc.XPathListAttr(expr, attr)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(vals)
	}
}

// htmlGroupJS creates the host.html object with all nine functions for JS.
func htmlGroupJS(vm *goja.Runtime) (goja.Value, error) {
	obj := vm.NewObject()
	if err := obj.Set("parse", jsHTMLParse(vm)); err != nil {
		return nil, fmt.Errorf("set parse: %w", err)
	}
	if err := obj.Set("find_text", jsHTMLFindText(vm)); err != nil {
		return nil, fmt.Errorf("set find_text: %w", err)
	}
	if err := obj.Set("find_attr", jsHTMLFindAttr(vm)); err != nil {
		return nil, fmt.Errorf("set find_attr: %w", err)
	}
	if err := obj.Set("find_list_text", jsHTMLFindListText(vm)); err != nil {
		return nil, fmt.Errorf("set find_list_text: %w", err)
	}
	if err := obj.Set("find_list_attr", jsHTMLFindListAttr(vm)); err != nil {
		return nil, fmt.Errorf("set find_list_attr: %w", err)
	}
	if err := obj.Set("xpath_text", jsHTMLXPathText(vm)); err != nil {
		return nil, fmt.Errorf("set xpath_text: %w", err)
	}
	if err := obj.Set("xpath_attr", jsHTMLXPathAttr(vm)); err != nil {
		return nil, fmt.Errorf("set xpath_attr: %w", err)
	}
	if err := obj.Set("xpath_list_text", jsHTMLXPathListText(vm)); err != nil {
		return nil, fmt.Errorf("set xpath_list_text: %w", err)
	}
	if err := obj.Set("xpath_list_attr", jsHTMLXPathListAttr(vm)); err != nil {
		return nil, fmt.Errorf("set xpath_list_attr: %w", err)
	}
	return obj, nil
}
