//go:build wasip1

package main

import (
	"encoding/json"
	"strings"
)

// gAll returns every full match of a global regex pattern in s. The host's
// regex.find_all reports capture ranges per match; these patterns are all
// anchored on the wanted text as group 1, so this reads range 1.
// gAll extracts every capture group 1 match of pattern in s.
// regex.find_all returns rows of capture groups ([][string]), so this parses
// the JSON as []json.RawMessage rows then []string per row and takes vals[0].
func gAll(s, pattern string) []string {
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

// ponytail: text helpers are one-line host delegations; ceiling is one IPC
// round-trip each, upgrade to local impls only if profiling shows it matters.

func trim(s string) string   { return hostString("text.trim", s) }

func unescape(s string) string { return hostString("text.unescape", s) }

func urlEncode(s string) string { return hostString("text.url_encode", s) }

// ponytail: strings import kept for the detail/chapters helpers.
var _ = strings.TrimSpace
