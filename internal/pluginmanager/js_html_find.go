package pluginmanager

import (
	"errors"

	"goisekai/internal/htmldoc"
	"modernc.org/quickjs"
)

// jsHTMLParse wraps host.html.parse for the quickjs runtime: a markup string
// becomes a numeric document handle. On failure it throws, the JS native
// convention.
func jsHTMLParse(vm *quickjs.VM, reg *jsHTMLHandleRegistry) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return nil, errors.New(htmlParseArgErr)
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, errors.New(htmlParseArgErr)
		}
		doc, err := htmldoc.Parse(s)
		if err != nil {
			return nil, err
		}
		return reg.newHandle(doc), nil
	}
}

// jsHTMLFindText wraps host.html.find_text for the quickjs runtime.
func jsHTMLFindText(vm *quickjs.VM, reg *jsHTMLHandleRegistry) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 2 {
			return nil, errors.New("host.html.find_text: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.find_text: argument 0 must be a document handle")
		}
		selector, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.find_text: argument 1 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return nil, err
		}
		text, err := doc.FindText(selector)
		if err != nil {
			return nil, err
		}
		return text, nil
	}
}

// jsHTMLFindAttr wraps host.html.find_attr for the quickjs runtime.
func jsHTMLFindAttr(vm *quickjs.VM, reg *jsHTMLHandleRegistry) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 3 {
			return nil, errors.New("host.html.find_attr: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.find_attr: argument 0 must be a document handle")
		}
		selector, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.find_attr: argument 1 must be a string")
		}
		attr, ok := args[2].(string)
		if !ok {
			return nil, errors.New("host.html.find_attr: argument 2 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return nil, err
		}
		val, err := doc.FindAttr(selector, attr)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// jsHTMLFindListText wraps host.html.find_list_text for the quickjs runtime.
func jsHTMLFindListText(vm *quickjs.VM, reg *jsHTMLHandleRegistry) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 2 {
			return nil, errors.New("host.html.find_list_text: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.find_list_text: argument 0 must be a document handle")
		}
		selector, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.find_list_text: argument 1 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return nil, err
		}
		texts, err := doc.FindListText(selector)
		if err != nil {
			return nil, err
		}
		return texts, nil
	}
}

// jsHTMLFindListAttr wraps host.html.find_list_attr for the quickjs runtime.
func jsHTMLFindListAttr(vm *quickjs.VM, reg *jsHTMLHandleRegistry) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 3 {
			return nil, errors.New("host.html.find_list_attr: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.find_list_attr: argument 0 must be a document handle")
		}
		selector, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.find_list_attr: argument 1 must be a string")
		}
		attr, ok := args[2].(string)
		if !ok {
			return nil, errors.New("host.html.find_list_attr: argument 2 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return nil, err
		}
		vals, err := doc.FindListAttr(selector, attr)
		if err != nil {
			return nil, err
		}
		return vals, nil
	}
}
