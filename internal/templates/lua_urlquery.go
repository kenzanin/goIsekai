package templates

import (
	"net/url"
	"sort"
	"strings"

	"github.com/mmcdole/lunar"
)

// urlQueryHelper builds an escaped query string from a table, skipping entries
// whose value is an empty string:
//
//	urlQuery({q = "isekai", sort = "read", status = ""})
//	  -> "q=isekai&sort=read"
//
// It exists because a template that changes one filter while keeping the others
// cannot assemble that by concatenation. Appending to a base that already carries
// the parameter emits "?status=reading&status=done", and the server reads the
// first - so the link looks right and does nothing. Building the whole query from
// the parameter set, with the changed key replaced, cannot produce that.
func urlQueryHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		t, ok := frame.Table(0)
		if !ok {
			return frame.ReturnString("")
		}
		type kv struct{ k, v string }
		var items []kv
		after := lua.Nil()
		for k, v, ok, _ := t.Next(after); ok; k, v, ok, _ = t.Next(k) {
			key, keyErr := frame.ToString(k)
			val, valErr := frame.ToString(v)
			key, val = strings.TrimSpace(key), strings.TrimSpace(val)
			if keyErr != nil || valErr != nil || key == "" || val == "" {
				continue
			}
			items = append(items, kv{key, val})
		}
		// Lua table order is not stable; sorting keeps the produced URL the same
		// across renders, which matters once a link is compared in a test.
		sort.Slice(items, func(i, j int) bool { return items[i].k < items[j].k })
		parts := make([]string, len(items))
		for i, it := range items {
			parts[i] = url.QueryEscape(it.k) + "=" + url.QueryEscape(it.v)
		}
		return frame.ReturnString(strings.Join(parts, "&"))
	}
}

// linkHelper joins a path and a query string produced by urlQuery, so a template
// does not have to decide between "?" and "&" itself.
func linkHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		path, _ := frame.CoerceString(0)
		query, _ := frame.CoerceString(1)
		if query == "" {
			return frame.ReturnString(path)
		}
		return frame.ReturnString(path + "?" + query)
	}
}
