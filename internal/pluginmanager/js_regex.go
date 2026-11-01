package pluginmanager

import (
	"errors"

	"goisekai/internal/pluginutil"
	"modernc.org/quickjs"
)

// regexGroupJS mirrors the Lua host.regex table on the quickjs runtime. Both
// runtimes call the same Go implementation, so a pattern reads the same in Lua
// and JS and returns the same values; only the shape of "no match" differs, per
// each language's idiom (null in JS, nil in Lua).
func regexGroupJS(vm *quickjs.VM) (quickjs.Value, error) {
	obj, err := vm.NewObjectValue()
	if err != nil {
		return quickjs.UndefinedValue, err
	}
	var scratch []string
	setFn := func(name string, fn quickjs.HostFunc) error {
		s := jsScratchName("regex", name)
		if err := jsSetHostFunc(vm, obj, s, name, fn); err != nil {
			return err
		}
		scratch = append(scratch, s)
		return nil
	}
	fns := map[string]quickjs.HostFunc{
		"find":       jsRegexFind(vm),
		"match":      jsRegexMatch(vm),
		"find_all":   jsRegexFindAll(vm),
		"find_index": jsRegexFindIndex(vm),
		"quote":      jsRegexQuote(vm),
		"replace":    jsRegexReplace(vm),
		"split":      jsRegexSplit(vm),
	}
	for name, fn := range fns {
		if err := setFn(name, fn); err != nil {
			return quickjs.UndefinedValue, err
		}
	}
	if err := jsDropScratch(vm, scratch...); err != nil {
		return quickjs.UndefinedValue, err
	}
	install, err := vm.EvalValue(jsRegexReplaceWrap, quickjs.EvalGlobal)
	if err != nil {
		return quickjs.UndefinedValue, err
	}
	defer install.Free()
	if _, err := install.Call(quickjs.UndefinedValue, obj); err != nil {
		return quickjs.UndefinedValue, err
	}
	return obj, nil
}

// jsRegexReplaceWrap runs once per VM and receives host.regex. quickjs cannot
// hand a JS function to a Go host function (it arrives as a JSON snapshot), so
// the function form of replace is built here: a string repl goes to the native
// replace, and a function repl is called per match from JS, which lets a throw
// inside it surface as a normal JS exception. split is the native that yields
// the match pieces, and it is removed from the table so only replace is exposed.
const jsRegexReplaceWrap = `(function (regex) {
	var native = regex.replace, split = regex.split;
	delete regex.split;
	regex.replace = function (subject, pattern, repl) {
		if (typeof repl !== "function") return native(subject, pattern, repl);
		var parts = split(subject, pattern), out = parts[0];
		for (var i = 1; i < parts.length; i += 2) {
			out += String(repl.apply(undefined, parts[i])) + parts[i + 1];
		}
		return out;
	};
})`

// jsRegexArgs reads the subject and pattern every regex native takes.
func jsRegexArgs(args []any) (string, string) {
	a, _ := args[0].(string)
	b, _ := args[1].(string)
	return a, b
}

// jsRegexFind wraps host.regex.find(subject, pattern): the single capture as a
// string, several captures as an array, and null when nothing matched.
func jsRegexFind(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		subject, pattern := jsRegexArgs(args)
		found, err := pluginutil.RegexFind(subject, pattern)
		if err != nil {
			return nil, err
		}
		switch len(found) {
		case 0:
			return nil, nil
		case 1:
			return found[0], nil
		default:
			return found, nil
		}
	}
}

// jsRegexMatch wraps host.regex.match(subject, pattern) -> boolean.
func jsRegexMatch(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		subject, pattern := jsRegexArgs(args)
		ok, err := pluginutil.RegexMatch(subject, pattern)
		if err != nil {
			return nil, err
		}
		return ok, nil
	}
}

// jsRegexFindAll wraps host.regex.find_all(subject, pattern): an array with one
// entry per match, each either the matched text or its capture array.
func jsRegexFindAll(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		subject, pattern := jsRegexArgs(args)
		rows, err := pluginutil.RegexFindAll(subject, pattern)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(rows))
		for _, row := range rows {
			if len(row) == 1 {
				out = append(out, row[0])
				continue
			}
			out = append(out, row)
		}
		return out, nil
	}
}

// jsRegexFindIndex wraps host.regex.find_index(subject, pattern, init): a
// [start, end] pair of 1-based byte offsets, or null when nothing matched. JS
// has no multiple return values, so the two positions arrive as one array.
func jsRegexFindIndex(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		subject, pattern := jsRegexArgs(args)
		init := 1
		if len(args) > 2 {
			if n, ok := args[2].(int); ok {
				init = n
			}
		}
		start, end, found, err := pluginutil.RegexFindIndex(subject, pattern, init)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		return []int{start, end}, nil
	}
}

// jsRegexQuote wraps host.regex.quote(s) -> string.
func jsRegexQuote(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		s, _ := args[0].(string)
		return pluginutil.RegexQuote(s), nil
	}
}

// jsRegexSplit wraps the split native behind the function form of replace: the
// text between matches and each match's captures, interleaved as
// [gap, captures, gap, ..., gap] so one array carries the whole walk.
func jsRegexSplit(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		subject, pattern := jsRegexArgs(args)
		gaps, caps, err := pluginutil.RegexSplitMatches(subject, pattern)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, 2*len(caps)+1)
		for i, gap := range gaps {
			out = append(out, gap)
			if i < len(caps) {
				out = append(out, caps[i])
			}
		}
		return out, nil
	}
}

// jsRegexReplace wraps host.regex.replace(subject, pattern, repl) -> string for
// a string repl, where capture references use Go's $1 syntax. The function form
// is handled in JS by jsRegexReplaceWrap, which only calls this for strings.
func jsRegexReplace(vm *quickjs.VM) quickjs.HostFunc {
	return func(args []any) (any, error) {
		subject, pattern := jsRegexArgs(args)
		if len(args) < 3 {
			return "", errors.New("replace: missing repl")
		}
		repl, _ := args[2].(string)
		out, err := pluginutil.RegexReplace(subject, pattern, repl)
		if err != nil {
			return "", err
		}
		return out, nil
	}
}
