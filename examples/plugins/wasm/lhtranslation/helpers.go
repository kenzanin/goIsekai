//go:build wasip1

package main

import (
	"encoding/json"
)

// ─── host regex (RE2, run on the host; one IPC round-trip per call) ────────

// firstCap returns the first capture group of pattern in s, or "" when the
// pattern does not match. The host's regex.find reports the capture ranges,
// so this reads range 1.
func firstCap(s, pattern string) string {
	res := hostCall("regex.find", s, pattern)
	if res == "null" || res == "" {
		return ""
	}
	var ranges []int
	if json.Unmarshal([]byte(res), &ranges) != nil || len(ranges) < 2 {
		return ""
	}
	return s[ranges[1]:ranges[2]]
}

// hostAll asks the host for every match of pattern in s. The host's
// regex.find_all returns rows of capture groups; when the pattern has no
// groups, row[0] is the whole match; when it has groups, row[0] is the
// first capture. We return row[0] (first capture or whole match) from each row.
func hostAll(pattern, s string) []string {
	res := hostCall("regex.find_all", s, pattern)
	if res == "null" || res == "" {
		return nil
	}
	var rows []json.RawMessage
	if json.Unmarshal([]byte(res), &rows) != nil {
		return nil
	}
	var out []string
	for _, row := range rows {
		// row is []string (one capture), or string (whole match only)
		var vals []string
		if json.Unmarshal(row, &vals) == nil {
			if len(vals) > 0 && vals[0] != "" {
				out = append(out, vals[0])
			}
			continue
		}
		// single value (no groups → whole match)
		var single string
		if json.Unmarshal(row, &single) == nil && single != "" {
			out = append(out, single)
		}
	}
	return out
}

// firstCapAll returns the first capture group of every match of pattern in s.
func firstCapAll(s, pattern string) []string {
	res := hostCall("regex.find_all", s, pattern)
	if res == "null" || res == "" {
		return nil
	}
	var rows []json.RawMessage
	if json.Unmarshal([]byte(res), &rows) != nil {
		return nil
	}
	var out []string
	for _, row := range rows {
		var vals []string
		if json.Unmarshal(row, &vals) == nil {
			if len(vals) > 0 && vals[0] != "" {
				out = append(out, vals[0])
			}
			continue
		}
		var single string
		if json.Unmarshal(row, &single) == nil && single != "" {
			out = append(out, single)
		}
	}
	return out
}

// ─── host http (body-only forms: nil/"" on transport or non-2xx) ────────────

func htmlGet(url string) string {
	res := hostCall("http.get_body", url)
	if res == "null" || res == "" {
		return ""
	}
	var s string
	if json.Unmarshal([]byte(res), &s) != nil {
		return ""
	}
	return s
}

func htmlPost(url, body, headersJSON string) string {
	res := hostCall("http.post_body", url, body, headersJSON)
	if res == "null" || res == "" {
		return ""
	}
	var s string
	if json.Unmarshal([]byte(res), &s) != nil {
		return ""
	}
	return s
}

// ─── host text helpers (one IPC round-trip each) ────────────────────────────

func trim(s string) string {
	var out string
	if json.Unmarshal([]byte(hostCall("text.trim", s)), &out) != nil {
		return s
	}
	return out
}

func unescape(s string) string {
	var out string
	if json.Unmarshal([]byte(hostCall("text.unescape", s)), &out) != nil {
		return s
	}
	return out
}

func stripHTML(s string) string {
	var out string
	if json.Unmarshal([]byte(hostCall("text.strip_html", s)), &out) != nil {
		return s
	}
	return out
}

func urlEncode(s string) string {
	var out string
	if json.Unmarshal([]byte(hostCall("text.url_encode", s)), &out) != nil {
		return s
	}
	return out
}

// decodeString decodes a JSON string arg, falling back to the raw arg.
func decodeString(arg string) string {
	var s string
	if err := json.Unmarshal([]byte(arg), &s); err == nil {
		return s
	}
	return arg
}

// gAll returns the first capture group of every match of pattern in s.
// (Alias of firstCapAll for call-site readability.)
func gAll(s, pattern string) []string {
	return firstCapAll(s, pattern)
}
