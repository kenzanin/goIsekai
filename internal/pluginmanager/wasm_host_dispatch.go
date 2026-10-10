package pluginmanager

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-json"

	"goisekai/internal/htmldoc"
	"goisekai/internal/pluginutil"
)

// dispatchHostCall resolves one host helper by its "<namespace>.<name>" key.
// id is the plugin id, used to attribute proxied HTTP requests; ctx bounds the
// invocations that reach the proxy (an HTTP fetch, a CDP solve) so they die
// with the invoke deadline instead of outliving it.
func (m *Manager) dispatchHostCall(ctx context.Context, id, fn string, args []string) (any, error) {
	if f, ok := wasmHostStr1[fn]; ok {
		return f(wasmArg(args, 0)), nil
	}
	if f, ok := wasmHostStr1Err[fn]; ok {
		return f(wasmArg(args, 0))
	}
	if f, ok := wasmHostStr2[fn]; ok {
		return f(wasmArg(args, 0), wasmArg(args, 1)), nil
	}
	if f, ok := wasmHostStr2Err[fn]; ok {
		return f(wasmArg(args, 0), wasmArg(args, 1))
	}
	switch fn {
	case "text.chapter_num":
		return pluginutil.ChapterNum(wasmArg(args, 0)), nil
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
	}
	return nil, fmt.Errorf("unknown host function %q", fn)
}

// wasmRegexRow shapes one match's captures the way the JS find/find_all do: a
// single capture is the value itself, several are the capture array, and no
// match is null.
func wasmRegexRow(row []string) any {
	switch len(row) {
	case 0:
		return nil
	case 1:
		return row[0]
	default:
		return row
	}
}

// wasmNormalizeStatus is host.text.normalize_status in its two call shapes:
// (raw) with the default vocabulary, or (mapJSON, raw) with a plugin-supplied
// map of lowercased keys to canonical values.
func wasmNormalizeStatus(args []string) (any, error) {
	if len(args) < 2 {
		return pluginutil.NormalizeStatus(nil, wasmArg(args, 0)), nil
	}
	m := map[string]string{}
	if err := json.Unmarshal([]byte(args[0]), &m); err != nil {
		return nil, fmt.Errorf("normalize_status: %w", err)
	}
	if len(m) == 0 {
		m = nil
	}
	return pluginutil.NormalizeStatus(m, args[1]), nil
}

// wasmHTML parses the markup and runs one lookup. It is the stateless
// equivalent of the Lua/JS document handle: the markup is the first argument on
// every call, since a wasm guest cannot hold a Go handle between calls.
func wasmHTML(args []string, lookup func(*htmldoc.Document) (any, error)) (any, error) {
	doc, err := htmldoc.Parse(wasmArg(args, 0))
	if err != nil {
		return nil, err
	}
	return lookup(doc)
}

// wasmVRFSign rebuilds host.crypto.vrf_sign(apiPath, params, stages) from JSON:
// params is a JSON object of string values, stages a JSON array of the
// {iv,key,tbl} base64 objects the signer consumes.
func wasmVRFSign(apiPath, paramsJSON, stagesJSON string) (any, error) {
	params := map[string]string{}
	if strings.TrimSpace(paramsJSON) != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			return nil, fmt.Errorf("vrf_sign params: %w", err)
		}
	}
	var rawStages []map[string]any
	if strings.TrimSpace(stagesJSON) != "" {
		if err := json.Unmarshal([]byte(stagesJSON), &rawStages); err != nil {
			return nil, fmt.Errorf("vrf_sign stages: %w", err)
		}
	}
	stages := make([]pluginutil.VRFStageB64, 0, len(rawStages))
	for i, raw := range rawStages {
		stage, err := pluginutil.VRFStageFromMap(raw)
		if err != nil {
			return nil, fmt.Errorf("vrf_sign stage %d: %w", i, err)
		}
		stages = append(stages, stage)
	}
	decoded, err := pluginutil.VRFStagesB64(stages)
	if err != nil {
		return nil, fmt.Errorf("vrf_sign stages: %w", err)
	}
	return pluginutil.VRFSign(apiPath, params, decoded), nil
}
