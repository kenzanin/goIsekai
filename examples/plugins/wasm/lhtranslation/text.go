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

func numStr(v any) string {
	switch x := v.(type) {
	case float64:
		if x == float64(int(x)) {
			return itoa(int(x))
		}
		return strings.TrimRight(strings.TrimRight(
			strings.TrimRight(fmtSprintfF(x), "0"), "."), "")
	case string:
		return x
	}
	return ""
}

func fmtSprintfF(f float64) string {
	return strings.TrimSuffix(jsonFloatShortest(f), "")
}

func jsonFloatShortest(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
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

func escapeGQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
