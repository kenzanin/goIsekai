package templates

import (
	"fmt"
	"reflect"

	lua "github.com/mmcdole/lunar"
)

// marshalGoToLua converts a Go value to a lunar Value.
func marshalGoToLua(S *lua.State, v any) lua.Value {
	if v == nil {
		return lua.Nil()
	}

	switch val := v.(type) {
	case bool:
		return lua.Bool(val)
	case int:
		return lua.Number(float64(val))
	case int8:
		return lua.Number(float64(val))
	case int16:
		return lua.Number(float64(val))
	case int32:
		return lua.Number(float64(val))
	case int64:
		return lua.Number(float64(val))
	case uint:
		return lua.Number(float64(val))
	case uint8:
		return lua.Number(float64(val))
	case uint16:
		return lua.Number(float64(val))
	case uint32:
		return lua.Number(float64(val))
	case uint64:
		return lua.Number(float64(val))
	case float32:
		return lua.Number(float64(val))
	case float64:
		return lua.Number(val)
	case string:
		return lua.String(val)
	case []any:
		return marshalSliceToLua(S, val)
	case map[string]any:
		return marshalMapToLua(S, val)
	default:
		// Try reflection for slices, structs, and other types.
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return lua.Nil()
			}
			rv = rv.Elem()
		}
		switch rv.Kind() {
		case reflect.Slice:
			return marshalReflectSliceToLua(S, rv)
		case reflect.Struct:
			return marshalStructToLua(S, rv)
		}
		return lua.String(fmt.Sprint(val))
	}
}

// marshalSliceToLua converts a Go slice to a Lua table (1-indexed).
// Empty slices produce an empty table (the "[]" JSON array convention).
func marshalSliceToLua(S *lua.State, s []any) lua.Value {
	t, err := S.NewTableWithCapacity(len(s), 0)
	if err != nil {
		return lua.Nil()
	}
	for i, item := range s {
		if err := t.RawSetInt(i+1, marshalGoToLua(S, item)); err != nil {
			return lua.Nil()
		}
	}
	return t.Value()
}

// marshalMapToLua converts a Go map to a Lua table. Empty maps produce
// an empty table.
func marshalMapToLua(S *lua.State, m map[string]any) lua.Value {
	t, err := S.NewTableWithCapacity(0, len(m))
	if err != nil {
		return lua.Nil()
	}
	for k, v := range m {
		if err := t.RawSetString(k, marshalGoToLua(S, v)); err != nil {
			return lua.Nil()
		}
	}
	return t.Value()
}

// marshalStructToLua converts a struct to a Lua table via exported fields.
func marshalStructToLua(S *lua.State, rv reflect.Value) lua.Value {
	rt := rv.Type()
	n := rv.NumField()
	t, err := S.NewTableWithCapacity(0, n)
	if err != nil {
		return lua.Nil()
	}
	for i := range n {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		if err := t.RawSetString(field.Name, marshalGoToLua(S, rv.Field(i).Interface())); err != nil {
			return lua.Nil()
		}
	}
	return t.Value()
}

// marshalReflectSliceToLua converts a reflect.Value slice to a Lua table.
func marshalReflectSliceToLua(S *lua.State, rv reflect.Value) lua.Value {
	len := rv.Len()
	t, err := S.NewTableWithCapacity(len, 0)
	if err != nil {
		return lua.Nil()
	}
	for i := range len {
		if err := t.RawSetInt(i+1, marshalGoToLua(S, rv.Index(i).Interface())); err != nil {
			return lua.Nil()
		}
	}
	return t.Value()
}

// marshalDataToLua sets a "data" global in the Lua VM from a Go map.
func marshalDataToLua(S *lua.State, data map[string]any) error {
	return S.SetGlobal("data", marshalGoToLua(S, data))
}
