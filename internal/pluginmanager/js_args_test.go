package pluginmanager

import "testing"

// A JS number or bool passed to a string helper must reach the Go helper as its
// JS string form, not as "". host.text.url_encode is a plain kindStr1 helper, so
// its output shows exactly what the argument became.
func TestJSStringHelperCoercesNonStringArgs(t *testing.T) {
	cases := []struct {
		name string
		arg  string // JS literal passed to the helper
		as   string // the same value written as a JS string
	}{
		{"float", "12.5", `"12.5"`},
		{"integer", "7", `"7"`},
		{"negative", "-3", `"-3"`},
		{"bool true", "true", `"true"`},
		{"bool false", "false", `"false"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jsString(t, "host.text.url_encode("+tc.arg+")")
			want := jsString(t, "host.text.url_encode("+tc.as+")")
			if got != want {
				t.Fatalf("url_encode(%s) = %q, want %q (same as the string form)", tc.arg, got, want)
			}
			if got == "" {
				t.Fatalf("url_encode(%s) returned the empty-argument result", tc.arg)
			}
		})
	}
}
