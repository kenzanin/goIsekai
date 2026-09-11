package pluginmanager

import (
	"encoding/json"
	"fmt"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/pluginutil"
)

// registerHostNatives installs the shared `host` table (text/codecs/crypto/http)
// on the Lua state. Every plugin runtime exposes the same surface so helpers are
// written once in Go instead of per plugin per language.
func registerHostNatives(state *lua.State, m *Manager, id string) {
	text, _ := state.NewTable()
	_ = text.RawSetString("url_encode", luaStr1(state, pluginutil.URLEncode))
	_ = text.RawSetString("url_decode", luaStr1(state, pluginutil.URLDecode))
	_ = text.RawSetString("html_decode", luaStr1(state, pluginutil.HTMLDecode))
	_ = text.RawSetString("strip_html", luaStr1(state, pluginutil.StripHTML))
	_ = text.RawSetString("strip_markdown", luaStr1(state, pluginutil.StripMarkdown))
	_ = text.RawSetString("titlecase", luaStr1(state, pluginutil.Titlecase))

	codecs, _ := state.NewTable()
	_ = codecs.RawSetString("base64_encode", luaStr1(state, pluginutil.Base64Encode))
	_ = codecs.RawSetString("base64_decode", luaStr1Err(state, pluginutil.Base64Decode))
	_ = codecs.RawSetString("base64url_encode", luaStr1(state, pluginutil.Base64URLEncode))
	_ = codecs.RawSetString("base64url_decode", luaStr1Err(state, pluginutil.Base64URLDecode))
	_ = codecs.RawSetString("hex_encode", luaStr1(state, pluginutil.HexEncode))
	_ = codecs.RawSetString("hex_decode", luaStr1Err(state, pluginutil.HexDecode))
	_ = codecs.RawSetString("b64_decode_hex", luaStr1Err(state, pluginutil.B64DecodeHex))
	_ = codecs.RawSetString("b64url_encode_hex", luaStr1Err(state, pluginutil.B64URLEncodeHex))
	_ = codecs.RawSetString("b64url_decode_hex", luaStr1Err(state, pluginutil.B64URLDecodeHex))

	crypto, _ := state.NewTable()
	_ = crypto.RawSetString("sha256_hex", luaStr1(state, pluginutil.SHA256Hex))
	_ = crypto.RawSetString("md5_hex", luaStr1(state, pluginutil.MD5Hex))
	_ = crypto.RawSetString("hmac_sha256_hex", luaStr2(state, pluginutil.HMACSHA256Hex))
	_ = crypto.RawSetString("xor", luaStr2Err(state, pluginutil.XORHex))
	_ = crypto.RawSetString("utf8_hex", luaStr1(state, pluginutil.UTF8Hex))
	_ = crypto.RawSetString("vrf_sign", luaVrfSign(state))

	host, _ := state.NewTable()
	_ = host.RawSetString("text", text.Value())
	_ = host.RawSetString("codecs", codecs.Value())
	_ = host.RawSetString("crypto", crypto.Value())

	// host.http — thin wrappers over http_request proxy.
	http, _ := state.NewTable()
	_ = http.RawSetString("get", luaHttpGet(state, m, id))
	_ = http.RawSetString("post", luaHttpPost(state, m, id))

	_ = host.RawSetString("http", http.Value())
	_ = state.RawSetGlobal("host", host.Value())
}

// luaVrfSign wraps host.crypto.vrf_sign(apiPath, params, stages) — VRF
// signer algorithm in Go, tables supplied by the plugin as data ({iv,key,tbl}
// base64), so rotations are plugin-side constant edits, no host rebuild.
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
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(pluginutil.VRFSign(apiPath, params, decoded)))
	})
	return fn.Value()
}

// luaHttpGet wraps host.http.get(url, headers?) → {status, headers, body}.
func luaHttpGet(state *lua.State, m *Manager, id string) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.String(0)
		var headers any
		if frame.ArgumentCount() > 1 {
			arg, ok := frame.Argument(1)
			if ok && frame.Kind(1) != lua.NilKind {
				headers, _ = lunarToGo(arg)
			}
		}
		req := map[string]any{"url": url, "method": "GET"}
		if headers != nil {
			req["headers"] = headers
		}
		reqJSON, err := json.Marshal(req)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "http.get marshal: "+err.Error()))
		}
		respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
		if err != nil {
			return frame.ReturnValue(errorTable(state, err.Error()))
		}
		var respVal any
		if err := json.Unmarshal([]byte(respJSON), &respVal); err != nil {
			return frame.ReturnValue(errorTable(state, "decode response: "+err.Error()))
		}
		luaval, err := goLunarValue(state, respVal)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "convert response: "+err.Error()))
		}
		return frame.ReturnValue(luaval)
	})
	return fn.Value()
}

// luaHttpPost wraps host.http.post(url, body, headers?) → {status, headers, body}.
func luaHttpPost(state *lua.State, m *Manager, id string) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.String(0)
		body, _ := frame.String(1)
		var headers any
		if frame.ArgumentCount() > 2 {
			arg, ok := frame.Argument(2)
			if ok && frame.Kind(2) != lua.NilKind {
				headers, _ = lunarToGo(arg)
			}
		}
		req := map[string]any{"url": url, "method": "POST", "body": body}
		if headers != nil {
			req["headers"] = headers
		}
		reqJSON, err := json.Marshal(req)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "http.post marshal: "+err.Error()))
		}
		respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
		if err != nil {
			return frame.ReturnValue(errorTable(state, err.Error()))
		}
		var respVal any
		if err := json.Unmarshal([]byte(respJSON), &respVal); err != nil {
			return frame.ReturnValue(errorTable(state, "decode response: "+err.Error()))
		}
		luaval, err := goLunarValue(state, respVal)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "convert response: "+err.Error()))
		}
		return frame.ReturnValue(luaval)
	})
	return fn.Value()
}

// luaStr1 wraps a string->string helper as a Lua native.
func luaStr1(state *lua.State, fn func(string) string) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.String(0)
		return frame.ReturnValue(lua.String(fn(s)))
	})
	return v.Value()
}

// luaStr1Err wraps a string->(string,error) helper, returning nil+message on error.
func luaStr1Err(state *lua.State, fn func(string) (string, error)) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.String(0)
		out, err := fn(s)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}

// luaStr2 wraps a (string,string)->string helper as a Lua native.
func luaStr2(state *lua.State, fn func(string, string) string) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		a, _ := frame.String(0)
		b, _ := frame.String(1)
		return frame.ReturnValue(lua.String(fn(a, b)))
	})
	return v.Value()
}

// luaStr2Err wraps a (string,string)->(string,error) helper.
func luaStr2Err(state *lua.State, fn func(string, string) (string, error)) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		a, _ := frame.String(0)
		b, _ := frame.String(1)
		out, err := fn(a, b)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}
