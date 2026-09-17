package pluginmanager

import (
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// wantHostPayload is every host native's output for the shared fixture input,
// joined by "|". Both the Lua and JS fixtures must produce exactly this.
const wantHostPayload = "a%20b|a b|&|hi|bold x|Abc|aGk=|hi|YT9i|6869|hi|" +
	"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad|" +
	"900150983cd24fb0d6963f7d28e17f72|" +
	"f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8|" +
	"0206|6869|6869|aGk|6869|HjB2RwY5YAtJKGYJESc|" +
	`12.5|7.5|{"a":[1,2]}|2026-01-02T00:00:00Z|` +
	`{"a":1,"b":"x"}|[1,2,3]|{"a":1,"b":"x"}|` +
	`42,Solo Leveling,true,false,a b,/m/a/,A;/m/b/,B,4,13,a\+b|true`

// hostFixtureDetail copies a host-native fixture into a temp dir and returns the
// manga detail it produces. The fixture reports the JSON natives' failure texts
// in the title.
func hostFixtureDetail(t *testing.T, fixture string) types.Manga {
	t.Helper()
	dir := t.TempDir()
	if err := copyDir("testdata/"+fixture, filepath.Join(dir, fixture)); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	d, err := mgr.GetMangaDetail(fixture, "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	return d
}

func TestLuaHostNatives(t *testing.T) {
	if got := strings.TrimSpace(hostFixtureDetail(t, "luahost").Description); got != wantHostPayload {
		t.Fatalf("lua host payload mismatch:\n got %q\nwant %q", got, wantHostPayload)
	}
}

func TestJSHostNatives(t *testing.T) {
	if got := strings.TrimSpace(hostFixtureDetail(t, "jshost").Description); got != wantHostPayload {
		t.Fatalf("js host payload mismatch:\n got %q\nwant %q", got, wantHostPayload)
	}
}

// TestHostJSONFailureTextIdentical machine-checks the cross-runtime identity
// requirement: the same malformed document, the same non-string argument and the
// same unrepresentable value must each report the same text in Lua and JS.
func TestHostJSONFailureTextIdentical(t *testing.T) {
	lua := hostFixtureDetail(t, "luahost").Title
	js := hostFixtureDetail(t, "jshost").Title
	if lua != js {
		t.Fatalf("json failure text differs:\n lua %q\n  js %q", lua, js)
	}
	parts := strings.Split(lua, "~")
	if len(parts) != 3 {
		t.Fatalf("want 3 failure texts, got %d: %q", len(parts), lua)
	}
	for i, part := range parts {
		if part == "" || part == "no error" {
			t.Fatalf("failure %d was not reported: %q", i, part)
		}
	}
}

// TestHTTPBodyHelper pins host.http.get_body's contract: only a 200 with a
// usable body counts as success, so a plugin can treat nil as "the fetch
// failed" and drop the status guard it otherwise repeats at every call site.
// The host logs the reason, so no plugin-side status handling is needed.
func TestHTTPBodyHelper(t *testing.T) {
	tests := []struct {
		name string
		resp map[string]any
		want string
		ok   bool
	}{
		{"200 with body", map[string]any{"status": float64(200), "body": "hi"}, "hi", true},
		{"200 with empty body", map[string]any{"status": float64(200)}, "", true},
		{"404", map[string]any{"status": float64(404), "body": "gone"}, "", false},
		{"transport failure", map[string]any{"status": float64(0), "error": "boom"}, "", false},
		{"missing status", map[string]any{"body": "hi"}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := responseBody(tc.resp)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("responseBody = %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
