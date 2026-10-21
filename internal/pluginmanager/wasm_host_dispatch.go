package pluginmanager

import (
	"context"
	"fmt"
	"strconv"

	"github.com/goccy/go-json"

	"goisekai/internal/htmldoc"
	"goisekai/internal/pluginutil"
)

// dispatchHostCall resolves one host helper by its "<namespace>.<name>" key.
// id is the plugin id, used to attribute proxied HTTP requests; ctx bounds the
// invocations that reach the proxy (an HTTP fetch, a CDP solve) so they die
// with the invoke deadline instead of outliving it.
func (m *Manager) dispatchHostCall(ctx context.Context, id, fn string, args []string) (any, error) {
	// Pure helpers come straight from the shared surface table, so the three
	// runtimes cannot drift on which helpers exist.
	if h, ok := hostSurfaceIndex[fn]; ok {
		if v, err, pure := h.invoke(args); pure {
			return v, err
		}
	}
	switch fn {
	case "text.normalize_status":
		return wasmNormalizeStatus(args)
	case "json.decode":
		var v any
		if err := json.Unmarshal([]byte(wasmArg(args, 0)), &v); err != nil {
			return nil, fmt.Errorf("json.decode: %w", err)
		}
		return v, nil
	case "json.encode":
		var v any
		if err := json.Unmarshal([]byte(wasmArg(args, 0)), &v); err != nil {
			return nil, fmt.Errorf("json.encode: %w", err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("json.encode: %w", err)
		}
		return string(b), nil
	case "regex.find":
		found, err := pluginutil.RegexFind(wasmArg(args, 0), wasmArg(args, 1))
		if err != nil {
			return nil, err
		}
		return wasmRegexRow(found), nil
	case "regex.match":
		return pluginutil.RegexMatch(wasmArg(args, 0), wasmArg(args, 1))
	case "regex.find_all":
		rows, err := pluginutil.RegexFindAll(wasmArg(args, 0), wasmArg(args, 1))
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, wasmRegexRow(row))
		}
		return out, nil
	case "regex.find_index":
		init := 1
		if n, err := strconv.Atoi(wasmArg(args, 2)); err == nil {
			init = n
		}
		start, end, found, err := pluginutil.RegexFindIndex(wasmArg(args, 0), wasmArg(args, 1), init)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		return []int{start, end}, nil
	case "regex.replace":
		return pluginutil.RegexReplace(wasmArg(args, 0), wasmArg(args, 1), wasmArg(args, 2))
	case "crypto.vrf_sign":
		return wasmVRFSign(wasmArg(args, 0), wasmArg(args, 1), wasmArg(args, 2))
	case "html.parse":
		if _, err := htmldoc.Parse(wasmArg(args, 0)); err != nil {
			return nil, err
		}
		return wasmArg(args, 0), nil
	case "html.find_text":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.FindText(wasmArg(args, 1)) })
	case "html.find_attr":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.FindAttr(wasmArg(args, 1), wasmArg(args, 2)) })
	case "html.find_list_text":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.FindListText(wasmArg(args, 1)) })
	case "html.find_list_attr":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.FindListAttr(wasmArg(args, 1), wasmArg(args, 2)) })
	case "html.xpath_text":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.XPathText(wasmArg(args, 1)) })
	case "html.xpath_attr":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.XPathAttr(wasmArg(args, 1), wasmArg(args, 2)) })
	case "html.xpath_list_text":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.XPathListText(wasmArg(args, 1)) })
	case "html.xpath_list_attr":
		return wasmHTML(args, func(d *htmldoc.Document) (any, error) { return d.XPathListAttr(wasmArg(args, 1), wasmArg(args, 2)) })
	case "http.get":
		return m.wasmHTTP(ctx, id, "GET", args, false)
	case "http.post":
		return m.wasmHTTP(ctx, id, "POST", args, false)
	case "http.get_body":
		return m.wasmHTTP(ctx, id, "GET", args, true)
	case "http.post_body":
		return m.wasmHTTP(ctx, id, "POST", args, true)
	case "browser.fetch":
		return m.wasmBrowserFetch(id, wasmArg(args, 0))
	case "browser.evaluate":
		return m.wasmBrowserEvaluate(id, wasmArg(args, 0), wasmArg(args, 1))
	case "browser.evaluate_with_init":
		return m.wasmBrowserEvaluateWithInit(id, wasmArg(args, 0), wasmArg(args, 1), wasmArg(args, 2))
	case "log.debug", "log.info", "log.warn", "log.error":
		wasmLog(id, fn, wasmArg(args, 0))
		return nil, nil
	}
	return nil, fmt.Errorf("unknown host function %q", fn)
}
