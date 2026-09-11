package pluginmanager

import (
	lua "github.com/mmcdole/lunar"

	"goisekai/internal/pluginutil"
)

// registerHostNatives installs the shared `host` table (text/codecs/crypto) on
// the Lua state. Every plugin runtime exposes the same surface so helpers are
// written once in Go instead of per plugin per language.
func registerHostNatives(state *lua.State) {
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

	crypto, _ := state.NewTable()
	_ = crypto.RawSetString("sha256_hex", luaStr1(state, pluginutil.SHA256Hex))
	_ = crypto.RawSetString("md5_hex", luaStr1(state, pluginutil.MD5Hex))
	_ = crypto.RawSetString("hmac_sha256_hex", luaStr2(state, pluginutil.HMACSHA256Hex))

	host, _ := state.NewTable()
	_ = host.RawSetString("text", text.Value())
	_ = host.RawSetString("codecs", codecs.Value())
	_ = host.RawSetString("crypto", crypto.Value())
	_ = state.RawSetGlobal("host", host.Value())
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
