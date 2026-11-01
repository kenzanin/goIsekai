package pluginmanager

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"goisekai/internal/logger"
	"modernc.org/quickjs"
)

var jsScratchSeq atomic.Int64

// jsScratchName is a unique scratch-global name for one host function.
func jsScratchName(ns, name string) string {
	return fmt.Sprintf("__pm_fn_%d_%s_%s", jsScratchSeq.Add(1), ns, name)
}

// jsSetHostFunc makes obj.name a JS function wrapping fn.
//
// SetProperty marshals its `any` argument through JSON, which a HostFunc cannot
// survive, and quickjs exposes no constructor for function values. RegisterHostFunc
// is the only way in, and it defines on the global object — so fn lands there under
// a scratch name, is read back as a Value, and is then copied onto obj. The caller
// drops the scratch global with jsDropScratch once the group is attached.
func jsSetHostFunc(vm *quickjs.VM, obj quickjs.Value, scratch, name string, fn quickjs.HostFunc) error {
	if err := vm.RegisterHostFunc(scratch, fn); err != nil {
		return fmt.Errorf("register %s: %w", scratch, err)
	}
	scratchAtom, err := vm.NewAtom(scratch)
	if err != nil {
		return fmt.Errorf("new atom %s: %w", scratch, err)
	}
	val, err := vm.GetPropertyValue(vm.GlobalObject(), scratchAtom)
	if err != nil {
		return fmt.Errorf("read %s: %w", scratch, err)
	}
	atom, err := vm.NewAtom(name)
	if err != nil {
		return fmt.Errorf("new atom %s: %w", name, err)
	}
	return vm.SetPropertyValue(obj, atom, val)
}

// jsDropScratch deletes the scratch globals jsSetHostFunc left on the global
// object, so a plugin only ever sees its own namespaces.
func jsDropScratch(vm *quickjs.VM, scratch ...string) error {
	if len(scratch) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("(function(){")
	for _, s := range scratch {
		fmt.Fprintf(&b, "delete globalThis[%q];", s)
	}
	b.WriteString("})();")
	_, err := vm.Eval(b.String(), quickjs.EvalGlobal)
	return err
}

// registerJSConsole sets console.debug/info/warn/error on the global object.
// The four functions are closures over id, mirroring the Lua log table.
// Kept for the test suite, which builds a bare VM and sets console by hand
// instead of going through loadJS.
//
//nolint:unused // used only by tests; golangci runs with --tests=false
func registerJSConsole(vm *quickjs.VM, id string) error {
	level := func(lv string) quickjs.HostFunc {
		return func(args []any) (any, error) {
			if len(args) == 0 {
				return quickjs.Undefined{}, nil
			}
			msg := fmt.Sprintln(args...)
			// Sprintln appends a trailing newline. Drop it so the message is
			// the same as JS's space-joined console.log format.
			msg = strings.TrimSuffix(msg, "\n")
			switch lv {
			case "debug":
				logger.Debug("plugin="+id, "msg", msg)
			case "info":
				logger.Info("plugin="+id, "msg", msg)
			case "warn":
				logger.Warn("plugin="+id, "msg", msg)
			case "error":
				logger.Error("plugin="+id, "msg", msg)
			}
			return quickjs.Undefined{}, nil
		}
	}
	console, err := vm.NewObjectValue()
	if err != nil {
		return err
	}
	var scratch []string
	// The five familiar names: log is an alias of info.
	levels := map[string]string{
		"log":   "info",
		"info":  "info",
		"warn":  "warn",
		"error": "error",
		"debug": "debug",
	}
	for lv, target := range levels {
		s := jsScratchName("console", lv)
		if err := jsSetHostFunc(vm, console, s, lv, level(target)); err != nil {
			return err
		}
		scratch = append(scratch, s)
	}
	if err := jsDropScratch(vm, scratch...); err != nil {
		return err
	}
	atom, err := vm.NewAtom("console")
	if err != nil {
		return err
	}
	return vm.SetProperty(vm.GlobalObject(), atom, console)
}

// jsArgString returns the JS string form of a scalar argument, the same text
// goja's .String() produced. Strings pass through; numbers and bools are
// formatted; null, undefined and objects give "".
func jsArgString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

// jsStr1 wraps a string→string host helper as a quickjs host function.
// The JS argument arrives as a Go string; the result goes back as a JS string.
func jsStr1(fn func(string) string) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return "", nil
		}
		return fn(jsArgString(args[0])), nil
	}
}

// jsStr1Err wraps a string→(string, error) host helper.
func jsStr1Err(fn func(string) (string, error)) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return "", nil
		}
		return fn(jsArgString(args[0]))
	}
}

// jsStr2 wraps a (string, string)→string host helper.
func jsStr2(fn func(string, string) string) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 2 {
			return "", nil
		}
		return fn(jsArgString(args[0]), jsArgString(args[1])), nil
	}
}

// jsStr2Err wraps a (string, string)→(string, error) host helper.
func jsStr2Err(fn func(string, string) (string, error)) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 2 {
			return "", nil
		}
		return fn(jsArgString(args[0]), jsArgString(args[1]))
	}
}

// jsStr3Err wraps a (string, string, string)→(string, error) host helper.
func jsStr3Err(fn func(string, string, string) (string, error)) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 3 {
			return "", nil
		}
		return fn(jsArgString(args[0]), jsArgString(args[1]), jsArgString(args[2]))
	}
}

// jsStr4Err wraps a (string, string, string, string)→(string, error) host helper.
func jsStr4Err(fn func(string, string, string, string) (string, error)) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) < 4 {
			return "", nil
		}
		return fn(jsArgString(args[0]), jsArgString(args[1]), jsArgString(args[2]), jsArgString(args[3]))
	}
}

// jsFloat1 wraps a string→float64 host helper. A JS number argument is turned
// into its JS string form first, so chapter_num(7.5) sees "7.5" as it would
// from Lua and goja.
func jsFloat1(fn func(string) float64) quickjs.HostFunc {
	return func(args []any) (any, error) {
		if len(args) == 0 {
			return 0.0, nil
		}
		return fn(jsArgString(args[0])), nil
	}
}

// objectToMap converts a *quickjs.Object into map[string]any via Into.
func objectToMap(obj *quickjs.Object) (map[string]any, error) {
	var m map[string]any
	if err := obj.Into(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// arrayToSlice converts a *quickjs.Object that is a JS array into []any.
func arrayToSlice(obj *quickjs.Object) ([]any, error) {
	var arr []any
	if err := obj.Into(&arr); err != nil {
		return nil, err
	}
	return arr, nil
}
