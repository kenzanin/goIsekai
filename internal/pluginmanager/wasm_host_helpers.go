package pluginmanager

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
	"github.com/tetratelabs/wazero/api"

	"goisekai/internal/htmldoc"
	"goisekai/internal/pluginutil"
)

// The wasm runtime exposes the Lua/JS `host` object through ONE generic import,
// env.host_call, rather than one import per helper. The guest passes
// {"fn":"<namespace>.<name>","args":[...]} and gets the JSON result back; every
// case below calls the same Go implementation the Lua/JS natives call, so the
// three runtimes cannot drift. See docs/plugin-wasm-helpers.md.

// wasmHostCallRequest is the envelope env.host_call decodes.
type wasmHostCallRequest struct {
	Fn   string   `json:"fn"`
	Args []string `json:"args"`
}

// hostCall is the env.host_call host import. The guest passes (ptr, len) of a
// JSON request and receives a packed i64 (ptr|len<<32) buffer holding the JSON
// result, or {"error":"..."} on failure. A zero return means the request could
// not be read or the response could not be allocated.
func (m *Manager) hostCall(ctx context.Context, mod api.Module, stack []uint64) {
	ptr, length := uint32(stack[0]), uint32(stack[1])
	reqBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		stack[0] = pack(0, 0)
		return
	}
	var req wasmHostCallRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		stack[0] = writeWasmResult(ctx, mod, wasmHostError("host_call: "+err.Error()))
		return
	}
	value, err := m.dispatchHostCall(ctx, mod.Name(), req.Fn, req.Args)
	if err != nil {
		stack[0] = writeWasmResult(ctx, mod, wasmHostError(err.Error()))
		return
	}
	out, err := json.Marshal(value)
	if err != nil {
		stack[0] = writeWasmResult(ctx, mod, wasmHostError("host_call "+req.Fn+": "+err.Error()))
		return
	}
	stack[0] = writeWasmResult(ctx, mod, out)
}

// writeWasmResult allocates the response in the guest's memory and returns the
// packed i64, or a zero pointer when allocation or the write failed.
func writeWasmResult(ctx context.Context, mod api.Module, out []byte) uint64 {
	respPtr, ok := wasmAlloc(ctx, mod, uint32(len(out)))
	if !ok || !mod.Memory().Write(respPtr, out) {
		return pack(0, 0)
	}
	return pack(respPtr, uint32(len(out)))
}

// wasmHostError is the JSON error envelope env.host_call returns.
func wasmHostError(msg string) []byte {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return b
}

// wasmHostStr1 maps every fixed one-string host helper straight to the Go
// implementation the Lua/JS natives call, keyed by "<namespace>.<name>".
var wasmHostStr1 = map[string]func(string) string{
	"text.url_encode":         pluginutil.URLEncode,
	"text.url_decode":         pluginutil.URLDecode,
	"text.html_decode":        pluginutil.HTMLDecode,
	"text.strip_html":         pluginutil.StripHTML,
	"text.strip_markdown":     pluginutil.StripMarkdown,
	"text.titlecase":          pluginutil.Titlecase,
	"text.normalize_title":    pluginutil.NormalizeTitle,
	"text.strip_link_blocks":  pluginutil.StripLinkBlocks,
	"text.unescape":           pluginutil.HTMLEntityUnescape,
	"text.trim":               strings.TrimSpace,
	"text.lua_escape":         pluginutil.LuaEscape,
	"text.date_to_iso":        pluginutil.DateToISONow,
	"codecs.base64_encode":    pluginutil.Base64Encode,
	"codecs.base64url_encode": pluginutil.Base64URLEncode,
	"codecs.hex_encode":       pluginutil.HexEncode,
	"crypto.sha256_hex":       pluginutil.SHA256Hex,
	"crypto.md5_hex":          pluginutil.MD5Hex,
	"crypto.utf8_hex":         pluginutil.UTF8Hex,
	"regex.quote":             pluginutil.RegexQuote,
}

// wasmHostStr1Err are the one-string helpers whose Go implementation can fail
// (a bad encoding, a bad hex string) instead of always returning a value.
var wasmHostStr1Err = map[string]func(string) (string, error){
	"codecs.base64_decode":     pluginutil.Base64Decode,
	"codecs.base64url_decode":  pluginutil.Base64URLDecode,
	"codecs.hex_decode":        pluginutil.HexDecode,
	"codecs.b64_decode_hex":    pluginutil.B64DecodeHex,
	"codecs.b64url_encode_hex": pluginutil.B64URLEncodeHex,
	"codecs.b64url_decode_hex": pluginutil.B64URLDecodeHex,
}

// wasmHostStr2 are the two-string helpers with no error return.
var wasmHostStr2 = map[string]func(string, string) string{
	"text.json_blob":         pluginutil.JSONBlob,
	"crypto.hmac_sha256_hex": pluginutil.HMACSHA256Hex,
}

// wasmHostStr2Err are the two-string helpers that can fail.
var wasmHostStr2Err = map[string]func(string, string) (string, error){
	"crypto.xor": pluginutil.XORHex,
}

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

// wasmArg reads one positional argument, defaulting to "" like the Lua/JS
// natives do for a missing one.
func wasmArg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
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
