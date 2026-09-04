package logger

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warning", slog.LevelWarn},
		{"warn", slog.LevelWarn},      // alias
		{"DEBUG", slog.LevelDebug},    // case-insensitive
		{"  Info ", slog.LevelInfo},   // trims whitespace
		{" WARNING ", slog.LevelWarn}, // case + whitespace together
		{"warning", slog.LevelWarn},   // warning maps to LevelWarn
	}
	for _, c := range cases {
		got, err := ParseLevel(c.in)
		if err != nil {
			t.Errorf("ParseLevel(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseLevelUnknown(t *testing.T) {
	if _, err := ParseLevel("foo"); err == nil {
		t.Fatal("ParseLevel(\"foo\") = nil error, want error")
	}
}

func TestInit(t *testing.T) {
	if err := Init("debug"); err != nil {
		t.Fatalf("Init(debug): %v", err)
	}
	h := slog.Default().Handler()
	if h == nil {
		t.Fatal("Init did not install a default handler")
	}
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be enabled at debug level")
	}

	if err := Init("warn"); err != nil {
		t.Fatalf("Init(warn): %v", err)
	}
	h = slog.Default().Handler()
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be disabled at warn level")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn should be enabled at warn level")
	}

	if err := Init("nonsense"); err == nil {
		t.Fatal("Init(nonsense) = nil error, want error")
	}
}

func TestHelpersDontPanic(t *testing.T) {
	// Installing a known level so the helpers have a handler to route to.
	if err := Init("info"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Debug("d", "k", "v")
	Info("i", "k", "v")
	Warn("w", "k", "v")
	Error("e", "k", "v")
}

func TestCaptureBuffer(t *testing.T) {
	// The capture handler appends every record to the ring buffer (GetLines).
	if err := Init("info"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Clear()
	Error("boom", "code", 7)
	Info("hello", "k", "v")

	lines := GetLines()
	if len(lines) != 2 {
		t.Fatalf("GetLines() = %d lines, want 2", len(lines))
	}
	// oldest first
	if !contains(lines[0], "ERROR boom") {
		t.Errorf("line0 = %q, want to contain 'ERROR boom'", lines[0])
	}
	if !contains(lines[1], "hello") || !contains(lines[1], "k=v") {
		t.Errorf("line1 = %q, want msg + attrs", lines[1])
	}
}

func TestCaptureBufferEviction(t *testing.T) {
	if err := Init("info"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// Bypass the global mutex to force the ring over its cap cheaply.
	mu.Lock()
	lines = make([]string, 0, bufSize)
	mu.Unlock()

	// Log bufSize+1 records; the oldest must be evicted.
	for i := range bufSize + 1 {
		Info("line", "n", i)
	}
	got := GetLines()
	if len(got) != bufSize {
		t.Fatalf("GetLines() = %d lines, want ring cap %d", len(got), bufSize)
	}
	// Oldest (n=0) must be gone, newest (n=bufSize) must be present.
	if !contains(got[0], "n=1") {
		t.Errorf("first retained line = %q, want n=1 (oldest evicted)", got[0])
	}
	if !contains(got[len(got)-1], "n="+itoa(bufSize)) {
		t.Errorf("last line = %q, want n=%d", got[len(got)-1], bufSize)
	}
}

func TestFormatRecord(t *testing.T) {
	r := slog.NewRecord(time.Now(), slog.LevelWarn, "something bad", 0)
	r.AddAttrs(slog.String("k", "v"))
	s := formatRecord(r)
	if !contains(s, "WARN") || !contains(s, "something bad") || !contains(s, "k=v") {
		t.Errorf("formatRecord = %q, want level+msg+attr", s)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
