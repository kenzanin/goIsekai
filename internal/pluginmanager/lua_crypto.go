package pluginmanager

import (
	"fmt"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/pluginutil"
)

// luaNormalizeStatus wraps host.text.normalize_status(raw) → canonical status,
// and the host.text.normalize_status(map, raw) form for plugins that extend the
// default vocabulary. An empty map means "use the defaults".
func luaNormalizeStatus(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		var m map[string]string
		var raw string
		if frame.ArgumentCount() > 1 {
			arg, ok := frame.Argument(0)
			if ok && frame.Kind(0) != lua.NilKind {
				gv, err := lunarToGo(arg)
				if err != nil {
					return frame.ReturnValues(lua.Nil(), lua.String("normalize_status: "+err.Error()))
				}
				if mv, ok := gv.(map[string]any); ok && len(mv) > 0 {
					m = make(map[string]string, len(mv))
					for k, v := range mv {
						switch val := v.(type) {
						case string:
							m[k] = val
						case int64:
							m[k] = fmt.Sprint(val)
						case float64:
							m[k] = fmt.Sprint(val)
						default:
							return frame.ReturnValues(lua.Nil(), lua.String("normalize_status param "+k+": unsupported type"))
						}
					}
				}
			}
			raw, _ = frame.String(1)
		} else {
			raw, _ = frame.String(0)
		}
		return frame.ReturnValue(lua.String(pluginutil.NormalizeStatus(m, raw)))
	})
	return v.Value()
}

// luaVrfSign wraps host.crypto.vrf_sign(apiPath, params, stages) — VRF
// signer algorithm in Go, tables supplied by plugin data ({iv,key,tbl}
// base64), so rotations plugin-side constant edits, no host rebuild.
func luaVrfSign(state *lua.State) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		apiPath, _ := frame.String(0)
		params := map[string]string{}
		if frame.ArgumentCount() > 1 {
			arg, ok := frame.Argument(1)
			if ok && frame.Kind(1) != lua.NilKind {
				gv, err := lunarToGo(arg)
				if err != nil {
					return frame.ReturnValues(lua.Nil(), lua.String("vrf_sign params: "+err.Error()))
				}
				if m, ok := gv.(map[string]any); ok {
					for k, v := range m {
						switch val := v.(type) {
						case string:
							params[k] = val
						case int64:
							params[k] = fmt.Sprint(val)
						case float64:
							params[k] = fmt.Sprint(val)
						default:
							return frame.ReturnValues(lua.Nil(), lua.String("vrf_sign param "+k+": unsupported type"))
						}
					}
				}
			}
		}
		var stages []pluginutil.VRFStageB64
		if frame.ArgumentCount() > 2 {
			arg, ok := frame.Argument(2)
			if ok && frame.Kind(2) != lua.NilKind {
				gv, err := lunarToGo(arg)
				if err != nil {
					return frame.ReturnValues(lua.Nil(), lua.String("vrf_sign stages: "+err.Error()))
				}
				if arr, ok := gv.([]any); ok {
					for i, item := range arr {
						m, ok := item.(map[string]any)
						if !ok {
							return frame.ReturnValues(lua.Nil(), lua.String(fmt.Sprintf("vrf_sign stage %d: not a table", i)))
						}
						s, err := pluginutil.VRFStageFromMap(m)
						if err != nil {
							return frame.ReturnValues(lua.Nil(), lua.String(fmt.Sprintf("vrf_sign stage %d: %v", i, err)))
						}
						stages = append(stages, s)
					}
				}
			}
		}
		decoded, err := pluginutil.VRFStagesB64(stages)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "vrf_sign stages: "+err.Error()))
		}
		result := pluginutil.VRFSign(apiPath, params, decoded)
		return frame.ReturnValue(lua.String(result))
	})
	return fn.Value()
}
