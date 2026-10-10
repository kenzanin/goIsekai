package pluginmanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/goccy/go-json"
)

// wasmHTTP runs host.http.* over the same hostnet proxy the other runtimes use.
// GET takes [url, headers?]; POST takes [url, body, headers?]. headers is a JSON
// object. The plain forms return the response object, with a {status:0,body:err}
// object on a transport failure; the _body forms return the response body, or
// null when the request failed or did not answer 200.
func (m *Manager) wasmHTTP(ctx context.Context, id, method string, args []string, bodyOnly bool) (any, error) {
	name := strings.ToLower(method)
	url := wasmArg(args, 0)
	body, headersJSON := "", wasmArg(args, 1)
	if method == "POST" {
		body, headersJSON = wasmArg(args, 1), wasmArg(args, 2)
	}
	req := map[string]any{"url": url, "method": method}
	if body != "" {
		req["body"] = body
	}
	if strings.TrimSpace(headersJSON) != "" {
		var headers any
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			return nil, fmt.Errorf("http.%s headers: %w", name, err)
		}
		req["headers"] = headers
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("http.%s marshal: %w", name, err)
	}
	respJSON, err := m.proxy.HandleRequestContext(ctx, id, string(reqJSON))
	if bodyOnly {
		if err != nil {
			return nil, nil
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
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
	var resp map[string]any
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		return nil, fmt.Errorf("http.%s decode response: %w", name, err)
	}
	return resp, nil
}
