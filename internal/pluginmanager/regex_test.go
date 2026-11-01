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

	wantJSString(t, jsValString(runJS(t, `
		host.regex.replace("slice-of-life", "([A-Za-z])([\\w']*)", function(a, b) {
			return a.toUpperCase() + b;
		});
	`)), "Slice-Of-Life")

	wantJSString(t, jsValString(runJS(t, `
		host.regex.replace("a1b2", "\\d", function(m) { return "[" + m + "]"; });
	`)), "a[1]b[2]")
}

// TestHostRegexReplaceFunctionErrors pins the error path of host.regex.replace
// in JS. A callback that throws must surface to the host call as an error that
// carries the thrown message, with no partial output. A bad pattern must fail
// the same way in both forms, and the wrapper must keep working after a throw.
func TestHostRegexReplaceFunctionErrors(t *testing.T) {
	_, err := runJSString(t, `
		host.regex.replace("a1", "\\d", function(m) { throw new Error("boom"); });
	`)
	if err == nil {
		t.Fatal("a throwing replacer should fail the host call")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("thrown message missing from error: %v", err)
	}

	for _, call := range []string{
		`host.regex.replace("a1", "(", "x")`,
		`host.regex.replace("a1", "(", function() { return "x"; })`,
	} {
		if _, err := runJSString(t, call); err == nil || !strings.Contains(err.Error(), "error parsing regexp") {
			t.Fatalf("%s: expected a regexp parse error, got %v", call, err)
		}
	}

	// Same VM: a throw is caught, then a normal replace still works.
	wantJSString(t, jsValString(runJS(t, `
		var caught;
		try {
			host.regex.replace("a1", "\\d", function() { throw new Error("boom"); });
		} catch (e) { caught = e.message; }
		caught + "|" + host.regex.replace("a1b2", "\\d", function(m) { return "[" + m + "]"; });
	`)), "boom|a[1]b[2]")
}
