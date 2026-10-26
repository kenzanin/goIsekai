//go:build wasip1

package main

import (
	"encoding/json"
	"strings"
)

// gAll returns every full match of a global regex pattern in s. The host's
// regex.find_all reports capture ranges per match; these patterns are all
// anchored on the wanted text as group 1, so this reads range 1.
func gAll(s, pattern string) []string {
	res := hostCall("regex.find_all", s, pattern)
	if res == "" || res == "null" {
		return nil
	}
	var ranges []int
	if json.Unmarshal([]byte(res), &ranges) == nil {
		var out []string
		for i := 0; i+1 < len(ranges); i += 2 {
			out = append(out, s[ranges[i]:ranges[i+1]])
		}
		return out
	}
	var group0s [][]int
	if json.Unmarshal([]byte(res), &group0s) == nil {
		var out []string
		for _, r := range group0s {
			if len(r) >= 2 {
				out = append(out, s[r[0]:r[1]])
			}
		}
		return out
	}
	return nil
}

// ponytail: text helpers are one-line host delegations; ceiling is one IPC
// round-trip each, upgrade to local impls only if profiling shows it matters.

func trim(s string) string   { return hostString("text.trim", s) }

func unescape(s string) string { return hostString("text.unescape", s) }

func urlEncode(s string) string { return hostString("text.url_encode", s) }

// ponytail: strings import kept for the detail/chapters helpers.
var _ = strings.TrimSpace
