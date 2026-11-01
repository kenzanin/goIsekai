package pluginmanager

import (
	"fmt"
	"reflect"
	"testing"

	"goisekai/internal/hostnet"

	"modernc.org/quickjs"
)

// jsTypesVM returns a fresh VM with the shared host object installed, the way a
// plugin gets one at load time, and the given Go functions registered as globals.
func jsTypesVM(t *testing.T, fns map[string]quickjs.HostFunc) *quickjs.VM {
	t.Helper()
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	t.Cleanup(func() { _ = vm.Close() })

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "jstypes"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	for name, fn := range fns {
		if err := vm.RegisterHostFunc(name, fn); err != nil {
			t.Fatalf("RegisterHostFunc(%s): %v", name, err)
		}
	}
	return vm
}

// convertJSArg applies the real helper a host function would use to read a
// *quickjs.Object argument. An empty how means the argument is already a Go value.
func convertJSArg(t *testing.T, arg any, how string) any {
	t.Helper()
	switch how {
	case "":
		return arg
	case "array":
		arr, err := arrayToSlice(arg.(*quickjs.Object))
		if err != nil {
			t.Fatalf("arrayToSlice: %v", err)
		}
		return arr
	case "object":
		m, err := objectToMap(arg.(*quickjs.Object))
		if err != nil {
			t.Fatalf("objectToMap: %v", err)
		}
		return m
	}
	t.Fatalf("unknown conversion %q", how)
	return nil
}

// TestJSTypesHostArgs: each JS argument kind must reach a host function as one
// fixed Go dynamic type, with the value and string form the helpers expect.
func TestJSTypesHostArgs(t *testing.T) {
	cases := []struct {
		name    string
		literal string
		wantT   string // %T of the value the host receives
		convert string // "array" or "object" when wantT is *quickjs.Object
		want    any    // expected value, after convert
		wantStr string // jsArgString of the received value
	}{
		{"string", `"héllo"`, "string", "", "héllo", "héllo"},
		{"integer", `7`, "int", "", 7, "7"},
		{"negative integer", `-3`, "int", "", -3, "-3"},
		{"integral float literal", `7.0`, "int", "", 7, "7"},
		{"fraction", `12.5`, "float64", "", 12.5, "12.5"},
		{"bool", `true`, "bool", "", true, "true"},
		{"null", `null`, "<nil>", "", nil, ""},
		{"undefined", `undefined`, "quickjs.Undefined", "", quickjs.Undefined{}, ""},
		{"array", `[1, "a", true]`, "*quickjs.Object", "array",
			[]any{float64(1), "a", true}, ""},
		{"nested object", `{"a": {"b": [1, 2]}, "n": 2}`, "*quickjs.Object", "object",
			map[string]any{"a": map[string]any{"b": []any{float64(1), float64(2)}}, "n": float64(2)}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []any
			vm := jsTypesVM(t, map[string]quickjs.HostFunc{
				"record": func(args []any) (any, error) {
					got = args
					return nil, nil
				},
			})
			if _, err := vm.Eval("record("+tc.literal+")", quickjs.EvalGlobal); err != nil {
				t.Fatalf("eval: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("host got %d args, want 1", len(got))
			}
			if gotT := fmt.Sprintf("%T", got[0]); gotT != tc.wantT {
				t.Fatalf("host arg type = %s, want %s", gotT, tc.wantT)
			}
			value := convertJSArg(t, got[0], tc.convert)
			if !reflect.DeepEqual(value, tc.want) {
				t.Fatalf("host arg value = %#v, want %#v", value, tc.want)
			}
			if s := jsArgString(got[0]); s != tc.wantStr {
				t.Fatalf("jsArgString = %q, want %q", s, tc.wantStr)
			}
		})
	}
}

// TestJSTypesHostReturns: a Go value returned by a host helper must reach JS as
// the matching JS type. The check runs inside the VM, so it sees what a plugin sees.
func TestJSTypesHostReturns(t *testing.T) {
	cases := []struct {
		name string
		ret  any
		want string // typeof | Array.isArray | JSON.stringify, as JS sees the value
	}{
		{"string", "hi", `string|false|"hi"`},
		{"int", 7, `number|false|7`},
		{"float", 12.5, `number|false|12.5`},
		{"integral float64", float64(7), `number|false|7`},
		{"int64 max safe", int64(9007199254740991), `number|false|9007199254740991`},
		{"bool", true, `boolean|false|true`},
		{"nil", nil, `object|false|null`},
		{"map sorted keys", map[string]any{"b": 2, "a": "x", "c": true},
			`object|false|{"a":"x","b":2,"c":true}`},
		{"[]any", []any{1, "two", nil}, `object|true|[1,"two",null]`},
		{"[]string", []string{"a", "b"}, `object|true|["a","b"]`},
		{"nested map", map[string]any{"list": []string{"x"}}, `object|false|{"list":["x"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vm := jsTypesVM(t, map[string]quickjs.HostFunc{
				"ret": func(args []any) (any, error) { return tc.ret, nil },
			})
			val, err := vm.Eval(`(function () {
				var x = ret();
				return typeof x + "|" + Array.isArray(x) + "|" + JSON.stringify(x);
			})()`, quickjs.EvalGlobal)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if got, _ := val.(string); got != tc.want {
				t.Fatalf("JS saw %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJSTypesIntVsFloat: integer, integral-float and fractional numbers keep
// their precision through a host round trip. The host echoes the argument back
// unchanged, so the JS side compares the returned value with the literal.
func TestJSTypesIntVsFloat(t *testing.T) {
	cases := []struct {
		name     string
		literal  string
		wantT    string // %T of the argument the host receives
		wantText string // String(x) in JS after the round trip
	}{
		{"integer", `7`, "int", "7"},
		{"integral float literal", `7.0`, "int", "7"},
		{"fraction", `12.5`, "float64", "12.5"},
		{"above int32", `3000000000`, "float64", "3000000000"},
		{"max safe integer", `9007199254740991`, "float64", "9007199254740991"},
		{"max safe integer expression", `2 ** 53 - 1`, "float64", "9007199254740991"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotT string
			vm := jsTypesVM(t, map[string]quickjs.HostFunc{
				"echo": func(args []any) (any, error) {
					gotT = fmt.Sprintf("%T", args[0])
					return args[0], nil
				},
			})
			val, err := vm.Eval(`(function () {
				var x = echo(`+tc.literal+`);
				return String(x) + "|" + (x === `+tc.literal+`);
			})()`, quickjs.EvalGlobal)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if gotT != tc.wantT {
				t.Fatalf("host arg type = %s, want %s", gotT, tc.wantT)
			}
			if got, want := val, tc.wantText+"|true"; got != want {
				t.Fatalf("round trip = %v, want %q", got, want)
			}
		})
	}
}
