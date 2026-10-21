package pluginmanager

import (
	"github.com/dop251/goja"

	lua "github.com/mmcdole/lunar"
)

// params_native.go wires host.plugin.param into the script runtimes.
//
// The helper reads one value the plugin declared in the "params" object of its
// own plugin.json. It is kindRuntime rather than a pure row because the value
// belongs to ONE plugin: the same helper name must answer differently for two
// plugins, so the function needs the manager and the plugin id.

// luaPluginParam wraps host.plugin.param(name) for the Lua runtime.
func luaPluginParam(state *lua.State, m *Manager, id string) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		name, _ := frame.CoerceString(0)
		return frame.ReturnValue(lua.String(m.pluginParam(id, name)))
	})
	return v.Value()
}

// jsPluginParam wraps host.plugin.param(name) for the JS runtime.
func jsPluginParam(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(m.pluginParam(id, call.Arguments[0].String()))
	}
}
