package templates

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	lua "github.com/mmcdole/lunar"
)

// pageWindowHelper returns a Lua table of page numbers to display.
func pageWindowHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		current, _ := frame.CoerceNumber(0)
		total, _ := frame.CoerceNumber(1)
		pages := pageWindow(int(current), int(total))

		t, err := S.NewTableWithCapacity(len(pages), 0)
		if err != nil {
			return frame.ReturnNil()
		}
		for i, p := range pages {
			if err := t.RawSetInt(i+1, lua.Number(float64(p))); err != nil {
				return frame.ReturnNil()
			}
		}
		return frame.ReturnValue(t.Value())
	}
}

// pageURLHelper builds a paginated URL from either:
//
//  1. pageURL(base, page) — simple: base string + "?page=N"
//  2. pageURL(paginationTable, page) — full: extracts Base, Param, Extra from table.
func pageURLHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		pageNum, _ := frame.CoerceNumber(1)
		p := strconv.Itoa(int(pageNum))

		// Simple form: pageURL(base, page)
		arg0, _ := frame.Argument(0)
		if arg0.Kind() != lua.TableKind {
			base, _ := frame.CoerceString(0)
			sep := "?"
			if strings.Contains(base, "?") {
				sep = "&"
			}
			return frame.ReturnString(base + sep + "page=" + p)
		}

		// Table form: pageURL(paginationTable, page)
		t, ok := frame.Table(0)
		if !ok {
			return frame.ReturnString("?page=" + p)
		}

		base := ""
		param := "page"

		// Read Base field
		if v, err := frame.Index(t.Value(), lua.String("Base")); err == nil && v.Kind() == lua.StringKind {
			base, _ = v.AsString()
		}

		// Read Param field
		if v, err := frame.Index(t.Value(), lua.String("Param")); err == nil && v.Kind() == lua.StringKind {
			param, _ = v.AsString()
		}

		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		var b strings.Builder
		b.WriteString(base + sep + param + "=" + p)

		// Read Extra field and append as key=value pairs. Extra is a flat
		// array {key1, val1, key2, val2, ...} — pair consecutive entries so
		// pagination links keep query params (q, pluginID) across pages.
		if v, err := frame.Index(t.Value(), lua.String("Extra")); err == nil && v.Kind() == lua.TableKind {
			if vt, ok := v.AsTable(); ok {
				type kv struct{ k, val string }
				var items []kv
				nilVal := lua.Nil()
				for k, val, ok, _ := vt.Next(nilVal); ok; k, val, ok, _ = vt.Next(k) {
					ks, _ := frame.ToString(k)
					vs, _ := frame.ToString(val)
					items = append(items, kv{ks, vs})
				}
				sort.Slice(items, func(a, c int) bool {
					ia, _ := strconv.Atoi(items[a].k)
					ic, _ := strconv.Atoi(items[c].k)
					return ia < ic
				})
				for i := 0; i+1 < len(items); i += 2 {
					b.WriteString("&" + url.QueryEscape(items[i].val) + "=" + url.QueryEscape(items[i+1].val))
				}
			}
		}
		return frame.ReturnString(b.String())
	}
}
