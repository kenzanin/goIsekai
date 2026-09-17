package pluginmanager

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/pluginutil"
)

// registerHostNatives installs the shared `host` table
// (text/codecs/crypto/json/http)
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
	_ = text.RawSetString("normalize_status", luaNormalizeStatus(state))
	_ = text.RawSetString("unescape", luaStr1(state, pluginutil.HTMLEntityUnescape))
	_ = text.RawSetString("trim", luaStr1(state, strings.TrimSpace))
	_ = text.RawSetString("lua_escape", luaStr1(state, pluginutil.LuaEscape))
	_ = text.RawSetString("chapter_num", luaFloat1(state, pluginutil.ChapterNum))
	_ = text.RawSetString("json_blob", luaStr2(state, pluginutil.JSONBlob))
	_ = text.RawSetString("date_to_iso", luaStr1(state, pluginutil.DateToISONow))

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

	jsonTbl, _ := state.NewTable()
	_ = jsonTbl.RawSetString("decode", luaJSONDecode(state))
	_ = jsonTbl.RawSetString("encode", luaJSONEncode(state))

	htmlTbl, err := htmlGroup(state)
	if err != nil {
		panic(err)
	}

	regexTbl := regexGroup(state)

	host, _ := state.NewTable()
	_ = host.RawSetString("text", text.Value())
	_ = host.RawSetString("codecs", codecs.Value())
	_ = host.RawSetString("crypto", crypto.Value())
	_ = host.RawSetString("json", jsonTbl.Value())
	_ = host.RawSetString("html", htmlTbl.Value())
	_ = host.RawSetString("regex", regexTbl.Value())

	// host.http — thin wrappers over http_request proxy.
	http, _ := state.NewTable()
	_ = http.RawSetString("get", luaHttpGet(state, m, id))
	_ = http.RawSetString("post", luaHttpPost(state, m, id))
	_ = http.RawSetString("get_body", luaHTTPGetBody(state, m, id))
	_ = http.RawSetString("post_body", luaHTTPPostBody(state, m, id))

	_ = host.RawSetString("http", http.Value())
	_ = state.RawSetGlobal("host", host.Value())
}

// luaHTTPArg reads an http native's optional trailing table argument.
func luaHTTPArg(frame lua.Frame, index int) any {
	if frame.ArgumentCount() <= index {
		return nil
	}
	arg, ok := frame.Argument(index)
	if !ok || frame.Kind(index) == lua.NilKind {
		return nil
	}
	v, _ := lunarToGo(arg)
	return v
}

// luaHTTPRequest runs one proxied request and decodes the response object.
func luaHTTPRequest(m *Manager, id, method, url, body string, headers any) (map[string]any, error) {
	req := map[string]any{"url": url, "method": method}
	if body != "" {
		req["body"] = body
	}
	if headers != nil {
		req["headers"] = headers
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("http.%s marshal: %w", strings.ToLower(method), err)
	}
	respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}

// responseBody reports the body of a 200 response.
func responseBody(resp map[string]any) (string, bool) {
	status, _ := resp["status"].(float64)
	if status != 200 {
		return "", false
	}
	text, _ := resp["body"].(string)
	return text, true
}

// luaHTTPFn builds one host.http native. The _body variants hand back only the
// response body, or nil when the request failed or the server did not answer
// 200: the host logs the reason, and every plugin otherwise repeats that same
// status guard at each call site.
func luaHTTPFn(state *lua.State, m *Manager, id, method string, bodyOnly bool) lua.Value {
	post := method == "POST"
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.CoerceString(0)
		var body string
		headersIndex := 1
		if post {
			body, _ = frame.CoerceString(1)
			headersIndex = 2
		}
		resp, err := luaHTTPRequest(m, id, method, url, body, luaHTTPArg(frame, headersIndex))
		if bodyOnly {
			// Nil covers both a transport failure and a non-200 answer, so a
			// plugin can treat one nil check as "the fetch failed".
			if err != nil {
				return frame.ReturnValue(lua.Nil())
			}
			if text, ok := responseBody(resp); ok {
				return frame.ReturnValue(lua.String(text))
			}
			return frame.ReturnValue(lua.Nil())
		}
		if err != nil {
			return frame.ReturnValue(errorTable(state, err.Error()))
		}
		luaval, err := goLunarValue(state, resp)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "convert response: "+err.Error()))
		}
		return frame.ReturnValue(luaval)
	})
	return fn.Value()
}

// luaHttpGet wraps host.http.get(url, headers?) → {status, headers, body}.
func luaHttpGet(state *lua.State, m *Manager, id string) lua.Value {
	return luaHTTPFn(state, m, id, "GET", false)
}

// luaHttpPost wraps host.http.post(url, body, headers?) → {status, headers, body}.
func luaHttpPost(state *lua.State, m *Manager, id string) lua.Value {
	return luaHTTPFn(state, m, id, "POST", false)
}

// luaHTTPGetBody wraps host.http.get_body(url, headers?) → body, or nil on
// a failed request or a non-200 response.
func luaHTTPGetBody(state *lua.State, m *Manager, id string) lua.Value {
	return luaHTTPFn(state, m, id, "GET", true)
}

// luaHTTPPostBody wraps host.http.post_body(url, body, headers?) → body, or nil
// on a failed request or a non-200 response.
func luaHTTPPostBody(state *lua.State, m *Manager, id string) lua.Value {
	return luaHTTPFn(state, m, id, "POST", true)
}
