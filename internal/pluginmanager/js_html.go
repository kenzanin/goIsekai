package pluginmanager

import (
	"errors"
	"fmt"
	"sync"

	"goisekai/internal/htmldoc"
	"modernc.org/quickjs"
)

// jsHTMLHandleRegistry maps numeric handles to htmldoc.Document per VM.
// quickjs JSON-marshals Go values, so a struct pointer becomes {} — a per-VM
// int->*Document map is the minimal fix.
type jsHTMLHandleRegistry struct {
	mu     sync.Mutex
	docs   map[int]*htmldoc.Document
	nextID int
}

// newHandle registers a document and returns its numeric id.
func (r *jsHTMLHandleRegistry) newHandle(doc *htmldoc.Document) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID
	r.nextID++
	r.docs[id] = doc
	return id
}

// getHandle returns the document for a handle id.
func (r *jsHTMLHandleRegistry) getHandle(id int) (*htmldoc.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	doc, ok := r.docs[id]
	if !ok {
		return nil, errors.New("unknown document handle")
	}
	return doc, nil
}

// close drops all handles. Safe to call more than once.
func (r *jsHTMLHandleRegistry) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.docs = map[int]*htmldoc.Document{}
}

// htmlRegistries maps each VM to its html handle registry.
var (
	htmlRegistries   = map[*quickjs.VM]*jsHTMLHandleRegistry{}
	htmlRegistriesMu sync.Mutex
)

// htmlRegistryFor returns (creating on first use) the registry for vm.
func htmlRegistryFor(vm *quickjs.VM) *jsHTMLHandleRegistry {
	htmlRegistriesMu.Lock()
	defer htmlRegistriesMu.Unlock()
	if reg, ok := htmlRegistries[vm]; ok {
		return reg
	}
	reg := &jsHTMLHandleRegistry{docs: map[int]*htmldoc.Document{}}
	htmlRegistries[vm] = reg
	return reg
}

// releaseHTMLRegistry removes vm's registry and drops its handles. It is a
// no-op for a VM that never created a registry or was already released. Call
// it before closing the VM.
func releaseHTMLRegistry(vm *quickjs.VM) {
	htmlRegistriesMu.Lock()
	reg, ok := htmlRegistries[vm]
	delete(htmlRegistries, vm)
	htmlRegistriesMu.Unlock()
	if ok {
		reg.close()
	}
}

// jsHTMLXPathText wraps host.html.xpath_text for the quickjs runtime.
func jsHTMLXPathText(vm *quickjs.VM) quickjs.HostFunc {
	reg := htmlRegistryFor(vm)
	return func(args []any) (any, error) {
		if len(args) < 2 {
			return nil, errors.New("host.html.xpath_text: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.xpath_text: argument 0 must be a document handle")
		}
		expr, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.xpath_text: argument 1 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		text, err := doc.XPathText(expr)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		return text, nil
	}
}

// jsHTMLXPathAttr wraps host.html.xpath_attr for the quickjs runtime.
func jsHTMLXPathAttr(vm *quickjs.VM) quickjs.HostFunc {
	reg := htmlRegistryFor(vm)
	return func(args []any) (any, error) {
		if len(args) < 3 {
			return nil, errors.New("host.html.xpath_attr: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.xpath_attr: argument 0 must be a document handle")
		}
		expr, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.xpath_attr: argument 1 must be a string")
		}
		attr, ok := args[2].(string)
		if !ok {
			return nil, errors.New("host.html.xpath_attr: argument 2 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		val, err := doc.XPathAttr(expr, attr)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		return val, nil
	}
}

// jsHTMLXPathListText wraps host.html.xpath_list_text for the quickjs runtime.
func jsHTMLXPathListText(vm *quickjs.VM) quickjs.HostFunc {
	reg := htmlRegistryFor(vm)
	return func(args []any) (any, error) {
		if len(args) < 2 {
			return nil, errors.New("host.html.xpath_list_text: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.xpath_list_text: argument 0 must be a document handle")
		}
		expr, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.xpath_list_text: argument 1 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		texts, err := doc.XPathListText(expr)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		return texts, nil
	}
}

// jsHTMLXPathListAttr wraps host.html.xpath_list_attr for the quickjs runtime.
func jsHTMLXPathListAttr(vm *quickjs.VM) quickjs.HostFunc {
	reg := htmlRegistryFor(vm)
	return func(args []any) (any, error) {
		if len(args) < 3 {
			return nil, errors.New("host.html.xpath_list_attr: missing arguments")
		}
		handleID, ok := args[0].(int)
		if !ok {
			return nil, errors.New("host.html.xpath_list_attr: argument 0 must be a document handle")
		}
		expr, ok := args[1].(string)
		if !ok {
			return nil, errors.New("host.html.xpath_list_attr: argument 1 must be a string")
		}
		attr, ok := args[2].(string)
		if !ok {
			return nil, errors.New("host.html.xpath_list_attr: argument 2 must be a string")
		}
		doc, err := reg.getHandle(handleID)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		vals, err := doc.XPathListAttr(expr, attr)
		if err != nil {
			return quickjs.UndefinedValue, err
		}
		return vals, nil
	}
}

// htmlGroupJS creates the host.html object with all nine functions for JS.
func htmlGroupJS(vm *quickjs.VM) (quickjs.Value, error) {
	reg := htmlRegistryFor(vm)
	obj, err := vm.NewObjectValue()
	if err != nil {
		return quickjs.UndefinedValue, err
	}
	var scratch []string
	setFn := func(name string, fn quickjs.HostFunc) error {
		s := jsScratchName("html", name)
		if err := jsSetHostFunc(vm, obj, s, name, fn); err != nil {
			return err
		}
		scratch = append(scratch, s)
		return nil
	}
	if err := setFn("parse", jsHTMLParse(vm, reg)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set parse: %w", err)
	}
	if err := setFn("find_text", jsHTMLFindText(vm, reg)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set find_text: %w", err)
	}
	if err := setFn("find_attr", jsHTMLFindAttr(vm, reg)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set find_attr: %w", err)
	}
	if err := setFn("find_list_text", jsHTMLFindListText(vm, reg)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set find_list_text: %w", err)
	}
	if err := setFn("find_list_attr", jsHTMLFindListAttr(vm, reg)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set find_list_attr: %w", err)
	}
	if err := setFn("xpath_text", jsHTMLXPathText(vm)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set xpath_text: %w", err)
	}
	if err := setFn("xpath_attr", jsHTMLXPathAttr(vm)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set xpath_attr: %w", err)
	}
	if err := setFn("xpath_list_text", jsHTMLXPathListText(vm)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set xpath_list_text: %w", err)
	}
	if err := setFn("xpath_list_attr", jsHTMLXPathListAttr(vm)); err != nil {
		return quickjs.UndefinedValue, fmt.Errorf("set xpath_list_attr: %w", err)
	}
	if err := jsDropScratch(vm, scratch...); err != nil {
		return quickjs.UndefinedValue, err
	}
	return obj, nil
}
