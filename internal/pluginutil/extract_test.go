package pluginutil

import (
	"testing"
	"time"
)

func TestChapterNum(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"Chapter 12.5", 12.5},
		{"chapter 12", 12},
		{"ch12", 12},
		{"Ch. 12", 12},
		{"#12", 12},
		{"Episode 7", 7},
		{"Vol. 3 Ch. 12", 12},
		{"/manga/one-piece/chapter-1085", 1085},
		{"https://site.io/c12.5", 12.5},
		{"12,5", 12.5},
		{"One Piece 1085", 1085},
		{"chapter-12-5", 12},
		{"Vol. 3", 0},
		{"Volume 2 Extra", 0},
		{"No numbers here", 0},
		{"", 0},
		{"search12", 12},
	}
	for _, tt := range tests {
		if got := ChapterNum(tt.in); got != tt.want {
			t.Errorf("ChapterNum(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestDateToISO(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want string
	}{
		{"2026-01-02T15:04:05Z", "2026-01-02T15:04:05Z"},
		{"2026-01-02T15:04:05+07:00", "2026-01-02T08:04:05Z"},
		{"2026-01-02 15:04:05", "2026-01-02T15:04:05Z"},
		{"2026-01-02", "2026-01-02T00:00:00Z"},
		{"2 Jan 2006", "2006-01-02T00:00:00Z"},
		{"Jan 2, 2006", "2006-01-02T00:00:00Z"},
		{"Thu, 02 Jan 2006 15:04:05 UTC", "2006-01-02T15:04:05Z"},
		{"1789482933", "2026-09-15T14:35:33Z"},
		{"2 days ago", "2026-09-14T12:00:00Z"},
		{"3 hours ago", "2026-09-16T09:00:00Z"},
		{"1 month ago", "2026-08-17T12:00:00Z"},
		{"yesterday", "2026-09-15T12:00:00Z"},
		{"just now", "2026-09-16T12:00:00Z"},
		{" 2026-01-02 ", "2026-01-02T00:00:00Z"},
		{"", ""},
		{"not a date", ""},
		{"12", ""},
		{"02/01/2026", ""},
	}
	for _, tt := range tests {
		if got := DateToISO(tt.in, now); got != tt.want {
			t.Errorf("DateToISO(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestJSONBlob(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		marker string
		want   string
	}{
		{"object", `x {"a":1} y`, "", `{"a":1}`},
		{"array", `x [1,2] y`, "", `[1,2]`},
		{"nested", `{"a":{"b":[1,2]}}`, "", `{"a":{"b":[1,2]}}`},
		{"brace in string", `{"a":"}"}`, "", `{"a":"}"}`},
		{"escaped quote in string", `{"a":"\"}"}`, "", `{"a":"\"}"}`},
		{"marker picks the blob", `<script>var x=0;</script><script id="__NEXT_DATA__">{"p":1}</script>`, "__NEXT_DATA__", `{"p":1}`},
		{"marker absent", `{"a":1}`, "__NEXT_DATA__", ""},
		{"unbalanced", `{"a":1`, "", ""},
		{"no json", "plain text", "", ""},
		{"empty", "", "", ""},
	}
	for _, tt := range tests {
		if got := JSONBlob(tt.in, tt.marker); got != tt.want {
			t.Errorf("%s: JSONBlob(%q, %q) = %q, want %q", tt.name, tt.in, tt.marker, got, tt.want)
		}
	}
}
