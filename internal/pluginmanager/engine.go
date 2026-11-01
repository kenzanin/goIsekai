package pluginmanager

import (
	"errors"
	"fmt"
)

// engine is one plugin runtime kind. Every runtime the host can run is reached
// through this interface, so adding or replacing one is a single entry in
// engines rather than an edit at each dispatch site (invoke, load, teardown).
type engine interface {
	load(m *Manager, id, path string) (*loadedPlugin, error)
	call(m *Manager, p *loadedPlugin, fnName, input string) (string, error)
	close(m *Manager, p *loadedPlugin) error
}

// ErrUnsupportedKind reports a plugin whose declared kind has no registered
// runtime.
var ErrUnsupportedKind = errors.New("unsupported plugin kind")

// engines maps the kind a plugin declares in plugin.json to its runtime.
var engines = map[string]engine{
	"lua":   luaEngine{},
	"js":    jsEngine{},
	"go":    goEngine{},
	"yaegi": yaegiEngine{},
	"wasm":  wasmEngine{},
}

// engineFor returns the runtime registered for a plugin kind.
func engineFor(kind string) (engine, error) {
	e, ok := engines[kind]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnsupportedKind, kind)
	}
	return e, nil
}

type luaEngine struct{}

func (luaEngine) load(m *Manager, id, path string) (*loadedPlugin, error) {
	return m.loadLua(id, path)
}

func (luaEngine) call(_ *Manager, p *loadedPlugin, fnName, input string) (string, error) {
	return callLua(p, fnName, input)
}

func (luaEngine) close(_ *Manager, p *loadedPlugin) error {
	if p.lunar == nil {
		return nil
	}
	return p.lunar.Close()
}

type jsEngine struct{}

func (jsEngine) load(m *Manager, id, path string) (*loadedPlugin, error) {
	return m.loadJS(id, path)
}

func (jsEngine) call(_ *Manager, p *loadedPlugin, fnName, input string) (string, error) {
	return callJS(p, fnName, input)
}

// close releases the html registry before the VM, then clears p.js so a second
// close is a no-op (quickjs VM.Close must not run twice).
func (jsEngine) close(_ *Manager, p *loadedPlugin) error {
	if p.js == nil {
		return nil
	}
	vm := p.js
	releaseHTMLRegistry(vm)
	p.js = nil
	return vm.Close()
}

type goEngine struct{}

func (goEngine) load(m *Manager, id, path string) (*loadedPlugin, error) {
	return m.loadGo(id, path)
}

func (goEngine) call(_ *Manager, p *loadedPlugin, fnName, input string) (string, error) {
	return callGo(p, fnName, input)
}

func (goEngine) close(_ *Manager, p *loadedPlugin) error {
	if p.goLib == nil {
		return nil
	}
	return p.goLib.close()
}

type yaegiEngine struct{}

func (yaegiEngine) load(m *Manager, id, path string) (*loadedPlugin, error) {
	return m.loadYaegi(id, path)
}

func (yaegiEngine) call(m *Manager, p *loadedPlugin, fnName, input string) (string, error) {
	return callYaegi(m, p, fnName, input)
}

// yaegi holds no closable runtime resource: unloading only drops the field.
func (yaegiEngine) close(*Manager, *loadedPlugin) error { return nil }

type wasmEngine struct{}

func (wasmEngine) load(m *Manager, id, path string) (*loadedPlugin, error) {
	return m.loadWasm(id, path)
}

func (wasmEngine) call(m *Manager, p *loadedPlugin, fnName, input string) (string, error) {
	return m.callWasm(p, fnName, input)
}

func (wasmEngine) close(m *Manager, p *loadedPlugin) error {
	if p.wasmMod == nil {
		return nil
	}
	return p.wasmMod.Close(m.ctx)
}
