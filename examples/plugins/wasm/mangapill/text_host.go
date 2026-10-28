//go:build wasip1

package main

import (
	"encoding/json"
	"regexp"
	"strconv"
)

// chapterNum delegates to the host's text.chapter_num helper so the host stays
// the single source of chapter-number parsing, with a local regex fallback when
// the helper is unavailable.
func chapterNum(s string) float64 {
	var f float64
	if json.Unmarshal([]byte(hostCall("text.chapter_num", s)), &f) == nil && f != 0 {
		return f
	}
	if num := reNum.FindString(s); num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			return v
		}
	}
	return 0
}

// dateToISO delegates to the host's text.date_to_iso helper.
func dateToISO(s string) string { return hostString("text.date_to_iso", s) }

var reNum = regexp.MustCompile(`\d+(?:\.\d+)?`)

// itoa is a TinyGo-safe integer->string (strconv.Itoa is missing in some
// TinyGo libc builds).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// floatString formats a float for URL building without strconv in tinygo.
func floatString(f float64) string {
	if f == float64(int(f)) {
		return itoa(int(f))
	}
	b, _ := json.Marshal(f)
	return string(b)
}
