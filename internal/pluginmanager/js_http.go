package pluginmanager

import (
	"encoding/json"
	"errors"

	"modernc.org/quickjs"
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

// jsHTTPHeaders reads an http native's optional trailing object argument.
// The object arrives as a *quickjs.Object; keys are enumerated in the VM.
func jsHTTPHeaders(vm *quickjs.VM, args []any, index int) any {
	if len(args) <= index {
		return nil
	}
	arg := args[index]
	switch obj := arg.(type) {
	case nil, quickjs.Undefined:
		return nil
	case *quickjs.Object:
		// Convert the object to a Go map via its JSON form.
		var m map[string]any
		if err := obj.Into(&m); err != nil {
			return nil
		}
		return m
	}
	return nil
}

// jsHTTPFn builds one host.http native, mirroring the Lua runtime. The _body
// variants hand back only the response body, or null when the request failed or
// the server did not answer 200.
func jsHTTPFn(vm *quickjs.VM, m *Manager, id, method string, bodyOnly bool) quickjs.HostFunc {
	post := method == "POST"
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return nil, errors.New("http: missing url")
		}
		url, _ := args[0].(string)
		body := ""
		headersIndex := 1
		if post {
			headersIndex = 2
			if len(args) > 1 {
				body, _ = args[1].(string)
			}
		}
		headers := jsHTTPHeaders(vm, args, headersIndex)
		resp, err := jsHTTPRequest(m, id, method, url, body, headers)
		if bodyOnly {
			if err != nil {
				return nil, nil
			}
			if text, ok := responseBody(resp); ok {
				return text, nil
			}
			return nil, nil
		}
		if err != nil {
			return map[string]any{"status": 0, "body": err.Error()}, nil
		}
		return resp, nil
	}
}

// jsHttpGet wraps host.http.get(url, headers?) → {status, headers, body}.
func jsHttpGet(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return jsHTTPFn(vm, m, id, "GET", false)
}

// jsHttpPost wraps host.http.post(url, body, headers?) → {status, headers, body}.
func jsHttpPost(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return jsHTTPFn(vm, m, id, "POST", false)
}

// jsHTTPGetBody wraps host.http.get_body(url, headers?) → body, or null on a
// failed request or a non-200 response.
func jsHTTPGetBody(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return jsHTTPFn(vm, m, id, "GET", true)
}

// jsHTTPPostBody wraps host.http.post_body(url, body, headers?) → body, or null
// on a failed request or a non-200 response.
func jsHTTPPostBody(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return jsHTTPFn(vm, m, id, "POST", true)
}
