package main

import (
	"encoding/json"
	"strings"
	"time"
)

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

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

func decodeString(arg string) string {
	var s string
	if json.Unmarshal([]byte(arg), &s) == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(arg)
}
