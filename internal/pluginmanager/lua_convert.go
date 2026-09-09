package pluginmanager

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mmcdole/lunar"
)

// lunarToGo converts a Lunar Value to a Go interface{} suitable for json.Marshal.
func lunarToGo(val lua.Value) (any, error) {
	switch val.Kind() {
	case lua.NilKind:
		return nil, nil
	case lua.BoolKind:
		b, _ := val.AsBool()
		return b, nil
	case lua.NumberKind:
		n, _ := val.AsNumber()
		return n, nil
	case lua.StringKind:
		s, _ := val.AsString()
		return s, nil
	case lua.TableKind:
		tbl, _ := val.AsTable()
		// Functions in tables are silently dropped (not JSON-serializable).
		goVal, err := tableToGoMap(tbl)
		if err != nil {
			return nil, err
		}
		return goVal, nil
	default:
		return nil, fmt.Errorf("unsupported Lua type: %s", val.Kind())
	}
}

// tableToGoMap iterates a Lunar table using tbl.Next and returns a map or slice.
func tableToGoMap(tbl *lua.Table) (any, error) {
	if tbl == nil {
		return nil, nil
	}
	// First pass: collect all key-value pairs.
	type kv struct {
		key   lua.Value
		value lua.Value
		isInt bool
		intK  int
	}
	var entries []kv
	cur := lua.Nil()
	for {
		k, v, ok, err := tbl.Next(cur)
		if err != nil {
			return nil, fmt.Errorf("table iteration: %w", err)
		}
		if !ok {
			break
		}
		entry := kv{key: k, value: v}
		if k.Kind() == lua.NumberKind {
			n, _ := k.AsNumber()
			if n == float64(int(n)) && n >= 1 {
				entry.isInt = true
				entry.intK = int(n)
			}
		}
		entries = append(entries, entry)
		cur = k
	}

	if len(entries) == 0 {
		// Empty Lua table is ambiguous; ABI results are arrays, so emit [].
		return []any{}, nil
	}

	// If all keys are sequential integers 1..n, return a slice.
	allInt := true
	maxInt := 0
	for _, e := range entries {
		if !e.isInt {
			allInt = false
			break
		}
		if e.intK > maxInt {
			maxInt = e.intK
		}
	}
	if allInt && maxInt == len(entries) {
		arr := make([]any, len(entries))
		for _, e := range entries {
			goVal, err := lunarToGo(e.value)
			if err != nil {
				return nil, err
			}
			arr[e.intK-1] = goVal
		}
		return arr, nil
	}

	// Map-like: string keys.
	m := make(map[string]any, len(entries))
	for _, e := range entries {
		if e.key.Kind() == lua.StringKind {
			k, _ := e.key.AsString()
			goVal, err := lunarToGo(e.value)
			if err != nil {
				return nil, err
			}
			m[k] = goVal
		}
	}
	return m, nil
}

// lunarTableToJSON marshals a Lunar table directly to JSON by walking it via tbl.Next.
func lunarTableToJSON(state *lua.State, tbl *lua.Table) ([]byte, error) {
	goVal, err := tableToGoMap(tbl)
	if err != nil {
		return nil, err
	}
	return json.Marshal(goVal)
}

// goLunarValue converts a Go interface{} (from json.Unmarshal) to a Lunar Value.
func goLunarValue(state *lua.State, v any) (lua.Value, error) {
	switch val := v.(type) {
	case nil:
		return lua.Nil(), nil
	case bool:
		return lua.Bool(val), nil
	case float64:
		return lua.Number(val), nil
	case string:
		return state.String(val), nil
	case []any:
		tbl, err := state.NewTable()
		if err != nil {
			return lua.Nil(), err
		}
		for i, item := range val {
			lv, err := goLunarValue(state, item)
			if err != nil {
				return lua.Nil(), err
			}
			_ = tbl.RawSetInt(i+1, lv) // Lua 1-indexed
		}
		return tbl.Value(), nil
	case map[string]any:
		tbl, err := state.NewTable()
		if err != nil {
			return lua.Nil(), err
		}
		for k, item := range val {
			lv, err := goLunarValue(state, item)
			if err != nil {
				return lua.Nil(), err
			}
			_ = tbl.RawSetString(k, lv)
		}
		return tbl.Value(), nil
	default:
		return state.String(fmt.Sprintf("%v", v)), nil
	}
}

// errorTable creates a Lua table {status=0, error=msg} for proxy error returns.
func errorTable(state *lua.State, msg string) lua.Value {
	tbl, _ := state.NewTable()
	_ = tbl.RawSetString("status", lua.Number(0))
	_ = tbl.RawSetString("error", state.String(msg))
	return tbl.Value()
}

// readFile reads a file into memory. Used for loading .lua files without
// ScriptLoader (state.Load takes an io.Reader).
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
