package pluginmanager

import (
	"fmt"
	"time"

	lua "github.com/mmcdole/lunar"
)

// createLuaState creates a sandboxed Lunar 5.4 VM with a restricted "os" table
// (time/date/clock only). Returns the ready state with no globals registered.
func createLuaState(id string) (*lua.State, error) {
	state, err := lua.New(lua.Options{
		Libraries: lua.LibrarySet{
			lua.BaseLibrary,
			lua.StringLibrary,
			lua.TableLibrary,
			lua.MathLibrary,
		},
		MaxHeapBytes: 64 << 20, // 64 MiB heap cap
	})
	if err != nil {
		return nil, fmt.Errorf("lua new state: %w", err)
	}

	// Register a hand-built "os" library with only time/date/clock.
	osTable, _ := state.NewTable()

	osTime, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnNumber(float64(time.Now().Unix()))
	})
	_ = osTable.RawSetString("time", osTime.Value())

	osDate, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnString(time.Now().Format(time.RFC3339))
	})
	_ = osTable.RawSetString("date", osDate.Value())

	osClock, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnNumber(float64(time.Now().UnixNano()) / 1e9)
	})
	_ = osTable.RawSetString("clock", osClock.Value())

	_ = state.RawSetGlobal("os", osTable.Value())

	return state, nil
}
