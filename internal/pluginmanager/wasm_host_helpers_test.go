package pluginmanager

import (
	"context"
	"encoding/json"
	"testing"
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
