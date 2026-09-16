package templates

import (
	"fmt"
	"html"

	lua "github.com/mmcdole/lunar"
)

// luaHelpers returns all helper functions to register as Lua globals.
func luaHelpers(S *lua.State) map[string]lua.NativeFunc {
	return map[string]lua.NativeFunc{
		"h":                hHelper(S),
		"formatDate":       formatDateHelper(S),
		"formatChapterNum": formatChapterNumHelper(S),
		"getInitials":      getInitialsHelper(S),
		"formatBytes":      formatBytesHelper(S),
		"pageWindow":       pageWindowHelper(S),
		"pageURL":          pageURLHelper(S),
	}
}

// registerHelpers installs all Go helpers as Lua globals.
func registerHelpers(S *lua.State) error {
	for name, fn := range luaHelpers(S) {
		nf, err := S.NewNativeFunction(fn)
		if err != nil {
			return fmt.Errorf("create helper %s: %w", name, err)
		}
		if err := S.SetGlobal(name, nf.Value()); err != nil {
			return fmt.Errorf("register helper %s: %w", name, err)
		}
	}
	return nil
}

// hHelper returns a Lua native function that HTML-escapes its argument.
func hHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		arg, ok := frame.Argument(0)
		if !ok {
			return frame.ReturnString("")
		}
		s, err := frame.ToString(arg)
		if err != nil {
			s = arg.String()
		}
		return frame.ReturnString(html.EscapeString(s))
	}
}
