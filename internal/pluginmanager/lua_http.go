package pluginmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/hostnet"
)

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
// ctx bounds the call: a challenge-solve cascade is capped by the caller's
// deadline (the plugin invoke ctx from frame.Context).
func luaHTTPRequest(ctx context.Context, m *Manager, id, method, url, body string, headers any) (map[string]any, error) {
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
	respJSON, err := m.proxy.HandleRequestContext(ctx, id, string(reqJSON))
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
		resp, err := luaHTTPRequest(frame.Context(), m, id, method, url, body, luaHTTPArg(frame, headersIndex))
		if err != nil && errors.Is(err, hostnet.ErrChallenge) {
			// A dead anti-bot session must ABORT the plugin call: the bodyOnly
			// contract (nil on failure) would swallow the challenge into an empty
			// result and the human-verify wizard could never re-open. callLua
			// re-wraps this marker into a typed ChallengeError.
			frame.ThrowError(fmt.Errorf("%s: %s", hostnet.ErrChallenge.Error(), url))
			return frame.ReturnValue(lua.Nil()) // unreachable: ThrowError never returns
		}
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
