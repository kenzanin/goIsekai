package pluginmanager

import (
	"modernc.org/quickjs"

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

// jsPluginParam wraps host.plugin.param(name) for the quickjs runtime.
func jsPluginParam(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return "", nil
		}
		n, _ := args[0].(string)
		return m.pluginParam(id, n), nil
	}
}
