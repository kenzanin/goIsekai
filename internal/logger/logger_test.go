package logger

import (
	"context"
	"log/slog"
	"testing"
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
