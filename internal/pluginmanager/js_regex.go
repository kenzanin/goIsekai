package pluginmanager

import (
	"github.com/dop251/goja"
	"goisekai/internal/pluginutil"
)

// regexGroupJS mirrors the Lua host.regex table on the goja runtime. Both
// runtimes call the same Go implementation, so a pattern reads the same in Lua
// and JS and returns the same values; only the shape of "no match" differs, per
// each language's idiom (null in JS, nil in Lua).
func regexGroupJS(vm *goja.Runtime) (*goja.Object, error) {
	obj := vm.NewObject()
	fns := map[string]any{
		"find":     jsRegexFind(vm),
		"match":    jsRegexMatch(vm),
		"find_all": jsRegexFindAll(vm),
		"replace":  jsRegexReplace(vm),
	}
	for name, fn := range fns {
		if err := obj.Set(name, fn); err != nil {
			return nil, err
		}
	}
	return obj, nil
}

// jsRegexArgs reads the subject and pattern every regex native takes.
func jsRegexArgs(call goja.FunctionCall) (string, string) {
	return call.Arguments[0].String(), call.Arguments[1].String()
}

// jsRegexFind wraps host.regex.find(subject, pattern): the single capture as a
// string, several captures as an array, and null when nothing matched.
func jsRegexFind(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		found, err := pluginutil.RegexFind(jsRegexArgs(call))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		switch len(found) {
		case 0:
			return goja.Null()
		case 1:
			return vm.ToValue(found[0])
		default:
			return vm.ToValue(found)
		}
	}
}

// jsRegexMatch wraps host.regex.match(subject, pattern) -> boolean.
func jsRegexMatch(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		ok, err := pluginutil.RegexMatch(jsRegexArgs(call))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(ok)
	}
}

// jsRegexFindAll wraps host.regex.find_all(subject, pattern): an array with one
// entry per match, each either the matched text or its capture array.
func jsRegexFindAll(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		rows, err := pluginutil.RegexFindAll(jsRegexArgs(call))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		out := make([]any, 0, len(rows))
		for _, row := range rows {
			if len(row) == 1 {
				out = append(out, row[0])
				continue
			}
			out = append(out, row)
		}
		return vm.ToValue(out)
	}
}

// jsRegexReplace wraps host.regex.replace(subject, pattern, repl) -> string.
func jsRegexReplace(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		subject, pattern := jsRegexArgs(call)
		out, err := pluginutil.RegexReplace(subject, pattern, call.Arguments[2].String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(out)
	}
}
