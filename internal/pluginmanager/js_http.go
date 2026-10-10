package pluginmanager

import (
	"github.com/goccy/go-json"

	"github.com/dop251/goja"
)

// jsHTTPRequest runs one proxied request and decodes the response object.
func jsHTTPRequest(m *Manager, id, method, url, body string, headers any) (map[string]any, error) {
	req := map[string]any{"url": url, "method": method}
	if body != "" {
		req["body"] = body
	}
	if headers != nil {
		req["headers"] = headers
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	respJSON, err := m.proxy.HandleRequestContext(m.invokeCtx(id), id, string(reqJSON))
	if err != nil {
		return nil, err
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// jsHTTPFn builds one host.http native, mirroring the Lua runtime. The _body
// variants hand back only the response body, or null when the request failed or
// the server did not answer 200.
func jsHTTPFn(vm *goja.Runtime, m *Manager, id, method string, bodyOnly bool) func(goja.FunctionCall) goja.Value {
	post := method == "POST"
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		body := ""
		headersIndex := 1
		if post {
			headersIndex = 2
			if len(call.Arguments) > 1 {
				body = call.Arguments[1].String()
			}
		}
		resp, err := jsHTTPRequest(m, id, method, url, body, jsHTTPHeaders(vm, call, headersIndex))
		if bodyOnly {
			// Null covers both a transport failure and a non-200 answer, so a
			// plugin can treat one nil check as "the fetch failed".
			if err != nil {
				return goja.Null()
			}
			if text, ok := responseBody(resp); ok {
				return vm.ToValue(text)
			}
			return goja.Null()
		}
		if err != nil {
			return vm.ToValue(map[string]any{"status": 0, "body": err.Error()})
		}
		return vm.ToValue(resp)
	}
}

// jsHttpGet wraps host.http.get(url, headers?) → {status, headers, body}.
func jsHttpGet(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "GET", false)
}

// jsHttpPost wraps host.http.post(url, body, headers?) → {status, headers, body}.
func jsHttpPost(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "POST", false)
}

// jsHTTPGetBody wraps host.http.get_body(url, headers?) → body, or null on a
// failed request or a non-200 response.
func jsHTTPGetBody(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "GET", true)
}

// jsHTTPPostBody wraps host.http.post_body(url, body, headers?) → body, or null
// on a failed request or a non-200 response.
func jsHTTPPostBody(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return jsHTTPFn(vm, m, id, "POST", true)
}
