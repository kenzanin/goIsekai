package pluginmanager

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"modernc.org/quickjs"
)

// captureSlog records every slog record so a test can assert on level and
// payload instead of merely that the call did not panic.
type captureSlog struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (c *captureSlog) Enabled(context.Context, slog.Level) bool { return true }

func (c *captureSlog) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recs = append(c.recs, r)
	return nil
}

func (c *captureSlog) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *captureSlog) WithGroup(string) slog.Handler      { return c }

// attr reads one attribute value off a captured record.
func (c *captureSlog) attr(r slog.Record, key string) (string, bool) {
	var out string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			out, found = a.Value.String(), true
			return false
		}
		return true
	})
	return out, found
}

// setSlogCapture installs capture as the process default logger and restores
// the previous one on cleanup. Global by necessity — logger.Debug/Info/Warn
// log through slog.Default — so this test does not run parallel.
func setSlogCapture(t *testing.T) *captureSlog {
	t.Helper()
	prev := slog.Default()
	c := &captureSlog{}
	slog.SetDefault(slog.New(c))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return c
}

// All five console methods must reach the host logger at the matching level,
// with JS arguments joined the way JS console.log joins them. A plugin's
// console output is how an author debugs a scrape, so a method that silently
// swallowed its arguments would be worse than useless.
func TestJSConsoleRoutesLevelsAndJoinsArgs(t *testing.T) {
	c := setSlogCapture(t)

	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	defer func() { _ = vm.Close() }()

	// The real injection site is loadJS; this mirrors it.
	if err := registerJSConsole(vm, "jsconsole-test"); err != nil {
		t.Fatalf("registerJSConsole: %v", err)
	}

	cases := []struct {
		call  string
		level slog.Level
		want  string
	}{
		{`console.log("hello", 42)`, slog.LevelInfo, "hello 42"},
		{`console.info("note")`, slog.LevelInfo, "note"},
		{`console.warn("careful", true)`, slog.LevelWarn, "careful true"},
		{`console.error("boom")`, slog.LevelError, "boom"},
		{`console.debug("tick")`, slog.LevelDebug, "tick"},
	}
	for _, tc := range cases {
		if _, err := vm.Eval(tc.call, quickjs.EvalGlobal); err != nil {
			t.Fatalf("%s: %v", tc.call, err)
		}
	}

	c.mu.Lock()
	recs := append([]slog.Record(nil), c.recs...)
	c.mu.Unlock()

	if len(recs) != len(cases) {
		t.Fatalf("captured %d records, want %d", len(recs), len(cases))
	}
	for i, tc := range cases {
		r := recs[i]
		if r.Level != tc.level {
			t.Errorf("%s: level = %v, want %v", tc.call, r.Level, tc.level)
		}
		if r.Message != "plugin=jsconsole-test" {
			t.Errorf("%s: message = %q, want plugin=jsconsole-test", tc.call, r.Message)
		}
		if got, ok := c.attr(r, "msg"); !ok || got != tc.want {
			t.Errorf("%s: msg = %q (present=%v), want %q", tc.call, got, ok, tc.want)
		}
	}
}

// The object exposes exactly the five familiar lowercase names. A Go struct
// here would have mapped methods by exported name and left console.log missing.
func TestJSConsoleExposesLowercaseNames(t *testing.T) {
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	defer func() { _ = vm.Close() }()

	if err := registerJSConsole(vm, "jsconsole-names"); err != nil {
		t.Fatalf("registerJSConsole: %v", err)
	}

	val, err := vm.Eval("Object.keys(console).length", quickjs.EvalGlobal)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if n, ok := val.(int); !ok || n != 5 {
		t.Fatalf("console has %v members, want 5", val)
	}
}
