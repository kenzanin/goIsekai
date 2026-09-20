package pluginmanager

import (
	lua "github.com/mmcdole/lunar"
	"goisekai/internal/pluginutil"
)

// regexGroup builds the host.regex table. Lua's string library matches Lua
// patterns, not regular expressions, so every scraping plugin hand-rolls the
// same brittle escaping and loses readability. These four functions move the
// matching onto one Go engine (internal/pluginutil) so a pattern is written in
// one syntax and behaves the same in every runtime.
func regexGroup(state *lua.State) *lua.Table {
	group, _ := state.NewTable()
	_ = group.RawSetString("find", luaRegexFind(state))
	_ = group.RawSetString("match", luaRegexMatch(state))
	_ = group.RawSetString("find_all", luaRegexFindAll(state))
	_ = group.RawSetString("gmatch", luaRegexGmatch(state))
	_ = group.RawSetString("find_index", luaRegexFindIndex(state))
	_ = group.RawSetString("quote", luaRegexQuote(state))
	_ = group.RawSetString("replace", luaRegexReplace(state))
	return group
}

// luaRegexArgs reads the subject and pattern every regex native takes. Missing
// arguments coerce to "", matching the other host helpers.
func luaRegexArgs(frame lua.Frame) (string, string) {
	subject, _ := frame.CoerceString(0)
	pattern, _ := frame.CoerceString(1)
	return subject, pattern
}

// luaRegexFind wraps host.regex.find(subject, pattern): the capture groups of
// the first match the way string.match returns them, or nil when nothing
// matched. A pattern that does not compile reports the reason as the second
// return value, so a mistyped pattern is not mistaken for a miss.
func luaRegexFind(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		found, err := pluginutil.RegexFind(luaRegexArgs(frame))
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		if len(found) == 0 {
			return frame.ReturnValue(lua.Nil())
		}
		values := make([]lua.Value, len(found))
		for i, capture := range found {
			values[i] = lua.String(capture)
		}
		return frame.ReturnValues(values...)
	})
	return v.Value()
}

// luaRegexMatch wraps host.regex.match(subject, pattern) -> boolean.
func luaRegexMatch(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		ok, err := pluginutil.RegexMatch(luaRegexArgs(frame))
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.Bool(ok))
	})
	return v.Value()
}

// luaRegexFindAll wraps host.regex.find_all(subject, pattern): an array holding
// one entry per match, each the match's captures. A pattern with a single
// capture gives an array of plain strings, so the common
// `for _, slug in ipairs(host.regex.find_all(html, pattern))` loop reads like
// the gmatch loop it replaces.
func luaRegexFindAll(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		rows, err := pluginutil.RegexFindAll(luaRegexArgs(frame))
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		out, _ := state.NewTable()
		for i, row := range rows {
			_ = out.RawSetInt(i+1, regexRow(state, row))
		}
		return frame.ReturnValue(out.Value())
	})
	return v.Value()
}

// regexRow packs one match's captures: with a single capture the value is the
// capture itself, so a one-group pattern yields an array of strings; with
// several it yields an array of rows.
func regexRow(state *lua.State, row []string) lua.Value {
	if len(row) == 1 {
		return lua.String(row[0])
	}
	cells, _ := state.NewTable()
	for i, cell := range row {
		_ = cells.RawSetInt(i+1, lua.String(cell))
	}
	return cells.Value()
}

// luaRegexGmatch wraps host.regex.gmatch(subject, pattern): the iterator triple
// a Lua generic for consumes, so a plugin keeps the `for a, b in ... do` shape
// it already had and the captures arrive as separate loop variables. JS has no
// generic for, so its runtimes use find_all instead.
func luaRegexGmatch(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		rows, err := pluginutil.RegexFindAll(luaRegexArgs(frame))
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		next := 0
		iter, _ := state.NewNativeFunction(func(f lua.Frame) lua.Outcome {
			if next >= len(rows) {
				return f.ReturnValue(lua.Nil())
			}
			row := rows[next]
			next++
			values := make([]lua.Value, len(row))
			for i, capture := range row {
				values[i] = lua.String(capture)
			}
			return f.ReturnValues(values...)
		})
		return frame.ReturnValues(iter.Value(), lua.Nil(), lua.Nil())
	})
	return v.Value()
}

// luaRegexFindIndex wraps host.regex.find_index(subject, pattern, init): the
// 1-based start and end byte offsets of the first match at or after init, or nil
// when nothing matched. It is the positional half of string.find, for the
// cursor scans that walk markup tag by tag.
func luaRegexFindIndex(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		subject, pattern := luaRegexArgs(frame)
		init := 1
		if n, ok := frame.Number(2); ok {
			init = int(n)
		}
		start, end, found, err := pluginutil.RegexFindIndex(subject, pattern, init)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		if !found {
			return frame.ReturnValue(lua.Nil())
		}
		return frame.ReturnValues(lua.Number(float64(start)), lua.Number(float64(end)))
	})
	return v.Value()
}

// luaRegexQuote wraps host.regex.quote(s) -> string.
func luaRegexQuote(state *lua.State) lua.Value {
	return luaStr1(state, pluginutil.RegexQuote)
}

// luaRegexReplace wraps host.regex.replace(subject, pattern, repl) -> string.
// Capture references in a string repl use Go's $1 syntax. A function repl is
// called once per match with the match's captures, or with the whole match when
// the pattern has none, and whatever it returns (by tostring) replaces the
// match. That is the shape string.gsub gives a replacement function, so a
// computed replacement no longer forces the plugin back onto Lua patterns.
func luaRegexReplace(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		subject, pattern := luaRegexArgs(frame)
		out, err := luaRegexReplaceRun(frame, subject, pattern)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}

// luaRegexReplaceRun picks the replacement form from the third argument: a
// function is called per match, and anything else is read as the replacement
// string the same way the other host helpers coerce an argument.
func luaRegexReplaceRun(frame lua.Frame, subject, pattern string) (string, error) {
	replacement, _ := frame.Argument(2)
	function, isFunction := replacement.AsFunction()
	if !isFunction {
		repl, _ := frame.CoerceString(2)
		return pluginutil.RegexReplace(subject, pattern, repl)
	}
	return pluginutil.RegexReplaceFunc(subject, pattern, func(captures []string) (string, error) {
		args := make([]lua.Value, len(captures))
		for i, capture := range captures {
			args[i] = lua.String(capture)
		}
		result, err := frame.CallOne(function.Value(), args...)
		if err != nil {
			return "", err
		}
		return frame.ToString(result)
	})
}
