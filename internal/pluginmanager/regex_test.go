package pluginmanager

import (
	"strings"
	"testing"

	lua "github.com/mmcdole/lunar"
)

// TestHostRegexReplaceFunction pins the replacement-function form of
// host.regex.replace in both runtimes: the callback runs once per match, gets
// the match's captures (or the whole match when the pattern has none), and a
// failure inside it surfaces instead of leaving a half-replaced string. The
// first case is the mangabuddy genre label, the call site that used to have to
// stay on Lua's pattern-based string.gsub for want of this.
func TestHostRegexReplaceFunction(t *testing.T) {
	runLua(t, `
		return host.regex.replace("slice-of-life", [==[([A-Za-z])([\w']*)]==], function(a, b)
			return a:upper() .. b
		end)
	`, wantLuaString("Slice-Of-Life"))

	runLua(t, `
		return host.regex.replace("a1b2", [[\d]], function(m) return "[" .. m .. "]" end)
	`, wantLuaString("a[1]b[2]"))

	// A string replacement still lands on the same helper.
	runLua(t, `
		return host.regex.replace("a1b2", [[\d]], "-")
	`, wantLuaString("a-b-"))

	runLua(t, `
		local out, err = host.regex.replace("a1", [[\d]], function() error("boom") end)
		return tostring(out) .. "|" .. tostring(err)
	`, func(t *testing.T, first lua.Value) {
		t.Helper()
		got, ok := first.AsString()
		if !ok {
			t.Fatalf("expected a string, got %v", first)
		}
		if !strings.HasPrefix(got, "nil|") || !strings.Contains(got, "boom") {
			t.Fatalf("callback error not reported: %q", got)
		}
	})

	wantJSString(t, runJS(t, `
		host.regex.replace("slice-of-life", "([A-Za-z])([\\w']*)", function(a, b) {
			return a.toUpperCase() + b;
		});
	`).String(), "Slice-Of-Life")

	wantJSString(t, runJS(t, `
		host.regex.replace("a1b2", "\\d", function(m) { return "[" + m + "]"; });
	`).String(), "a[1]b[2]")
}
