package pluginmanager

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"goisekai/internal/logger"
)

// TestWasmHostCallDispatch pins the env.host_call surface: one call per
// namespace proves the wasm ABI reaches the same Go helpers the Lua/JS natives
// use. HTTP is not covered here (it needs the proxy) and is exercised through
// the shared natives tests.
func TestWasmHostCallDispatch(t *testing.T) {
	m := &Manager{}
	cases := []struct {
		fn   string
		args []string
		want string
	}{
		{"text.titlecase", []string{"hello world"}, `"Hello world"`},
		{"text.normalize_status", []string{"ongoing"}, `"Ongoing"`},
		{"text.normalize_status", []string{`{"serialised":"Ongoing"}`, "serialised"}, `"Ongoing"`},
		{"text.normalize_status", []string{`{}`, "completed"}, `"Completed"`},
		{"text.chapter_num", []string{"Chapter 230.5"}, `230.5`},
		{"text.json_blob", []string{"xx {" + `"a":1` + "} yy", ""}, `"{\"a\":1}"`},
		{"codecs.base64_encode", []string{"hi"}, `"aGk="`},
		{"codecs.hex_decode", []string{"6869"}, `"hi"`},
		{"crypto.sha256_hex", []string{"x"}, `"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881"`},
		{"regex.match", []string{"Chapter 12", "(?i)^chapter"}, `true`},
		{"regex.find", []string{"a-b", `(\w)-(\w)`}, `["a","b"]`},
		{"regex.find_index", []string{"xxabc", "abc", "1"}, `[3,5]`},
		{"json.decode", []string{`{"a":1}`}, `{"a":1}`},
		{"json.encode", []string{`{ "a" : 1 }`}, `"{\"a\":1}"`},
		{"html.find_text", []string{"<h1>  Hi  </h1>", "h1"}, `"Hi"`},
		{"html.find_list_attr", []string{`<a href="/x">x</a>`, "a", "href"}, `["/x"]`},
	}
	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := m.dispatchHostCall(context.Background(), "test", tc.fn, tc.args)
			if err != nil {
				t.Fatalf("dispatchHostCall(%s): %v", tc.fn, err)
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if string(b) != tc.want {
				t.Errorf("%s = %s, want %s", tc.fn, b, tc.want)
			}
		})
	}
}

// TestWasmHostCallUnknownFunction pins that an unknown name is reported rather
// than silently returning null, so a typo surfaces at the plugin call site.
func TestWasmHostCallUnknownFunction(t *testing.T) {
	if _, err := (&Manager{}).dispatchHostCall(context.Background(), "test", "text.nope", nil); err == nil {
		t.Fatal("an unknown host function should error")
	}
}

// TestWasmHostCallLog pins that a wasm plugin can log at every level and that
// the plugin id reaches the host logger. Before this existed a wasm plugin had
// no way to emit a diagnostic at all.
func TestWasmHostCallLog(t *testing.T) {
	// The logger's ring buffer is the observable side effect: Init installs
	// the capture handler that appends every record, so the lines the plugin
	// logged can be read back without a test-only hook.
	if err := logger.Init("debug"); err != nil {
		t.Fatalf("logger.Init: %v", err)
	}
	logger.Clear()

	m := &Manager{}
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		if _, err := m.dispatchHostCall(context.Background(), "wasmtest", "log."+lvl, []string{"hello from wasm"}); err != nil {
			t.Fatalf("log.%s: %v", lvl, err)
		}
	}

	lines := logger.GetLines()
	if len(lines) != 4 {
		t.Fatalf("expected 4 log lines, got %d: %v", len(lines), lines)
	}
	for i, lvl := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		line := lines[i]
		if !strings.Contains(line, lvl) {
			t.Errorf("line %d = %q, want level %s", i, line, lvl)
		}
		if !strings.Contains(line, "hello from wasm") {
			t.Errorf("line %d = %q, want the plugin message", i, line)
		}
		// The Lua/JS globals tag every record with plugin=<id>; wasm must too,
		// otherwise a noisy plugin cannot be told apart in the logs view.
		if !strings.Contains(line, "plugin=wasmtest") {
			t.Errorf("line %d = %q, want plugin=wasmtest", i, line)
		}
	}
}
