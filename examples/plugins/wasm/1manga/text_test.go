package main

import (
	"testing"
	"time"
)

func TestNumStr(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{float64(0), "0"},
		{float64(42), "42"},
		{float64(23.5), "23.5"},
		{float64(1.5), "1.5"},
		{"3862", "3862"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := numStr(c.in); got != c.want {
			t.Errorf("numStr(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestItOA(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-7, "-7"},
		{100, "100"},
		{1234, "1234"},
	}
	for _, c := range cases {
		if got := itoa(c.in); got != c.want {
			t.Errorf("itoa(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDecodeString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"martial-peak"`, "martial-peak"},
		{`"  padded  "`, "padded"},
		{``, ""},
		{`not-a-json-string`, ""},
		{`123`, ""},
	}
	for _, c := range cases {
		if got := decodeString(c.in); got != c.want {
			t.Errorf("decodeString(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEscapeGQL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`he said "hi"`, `he said \"hi\"`},
		{`back\slash`, `back\\slash`},
		{`plain`, `plain`},
		{``, ``},
	}
	for _, c := range cases {
		if got := escapeGQL(c.in); got != c.want {
			t.Errorf("escapeGQL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseTime(t *testing.T) {
	rfc := "2025-12-22T15:50:56Z"
	got := parseTime(rfc)
	want := time.Date(2025, 12, 22, 15, 50, 56, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseTime(%q) = %v, want %v", rfc, got, want)
	}
	// date-only layout
	got2 := parseTime("2025-01-01")
	if !got2.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("parseTime(2025-01-01) = %v", got2)
	}
	// garbage
	if !parseTime("not-a-date").IsZero() {
		t.Error("parseTime(garbage) should be zero time")
	}
}
