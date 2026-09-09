package templates

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	lua "github.com/mmcdole/lunar"
)

// luaHelpers returns all helper functions to register as Lua globals.
func luaHelpers(S *lua.State) map[string]lua.NativeFunc {
	return map[string]lua.NativeFunc{
		"h":                hHelper(S),
		"formatDate":       formatDateHelper(S),
		"formatChapterNum": formatChapterNumHelper(S),
		"getInitials":      getInitialsHelper(S),
		"formatBytes":      formatBytesHelper(S),
		"pageWindow":       pageWindowHelper(S),
		"pageURL":          pageURLHelper(S),
	}
}

// registerHelpers installs all Go helpers as Lua globals.
func registerHelpers(S *lua.State) error {
	for name, fn := range luaHelpers(S) {
		nf, err := S.NewNativeFunction(fn)
		if err != nil {
			return fmt.Errorf("create helper %s: %w", name, err)
		}
		if err := S.SetGlobal(name, nf.Value()); err != nil {
			return fmt.Errorf("register helper %s: %w", name, err)
		}
	}
	return nil
}

// hHelper returns a Lua native function that HTML-escapes its argument.
func hHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		arg, ok := frame.Argument(0)
		if !ok {
			return frame.ReturnString("")
		}
		s, err := frame.ToString(arg)
		if err != nil {
			s = arg.String()
		}
		return frame.ReturnString(html.EscapeString(s))
	}
}

// formatDateHelper formats an ISO-8601 timestamp as "Jan 2, 2006".
func formatDateHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		arg, ok := frame.Argument(0)
		if !ok || arg.IsNil() {
			return frame.ReturnString("—")
		}
		s, _ := frame.ToString(arg)
		s = strings.TrimSpace(s)
		if s == "" {
			return frame.ReturnString("—")
		}
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, s); err == nil {
				return frame.ReturnString(t.Format("Jan 2, 2006"))
			}
		}
		return frame.ReturnString("—")
	}
}

// formatChapterNum formats a chapter number: strip trailing zeros, etc.
func formatChapterNumHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		arg, ok := frame.Argument(0)
		if !ok || arg.IsNil() {
			return frame.ReturnString("—")
		}
		switch arg.Kind() {
		case lua.NumberKind:
			f, _ := arg.AsNumber()
			return frame.ReturnString(chapterFloat(f))
		case lua.StringKind:
			s, _ := arg.AsString()
			s = strings.TrimSpace(s)
			if s == "" {
				return frame.ReturnString("—")
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return frame.ReturnString(s)
			}
			return frame.ReturnString(chapterFloat(f))
		default:
			return frame.ReturnString("—")
		}
	}
}

// getInitialsHelper returns the uppercase first letter of the first two words.
func getInitialsHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		arg, ok := frame.Argument(0)
		if !ok || arg.IsNil() {
			return frame.ReturnString("")
		}
		s, _ := frame.ToString(arg)
		parts := strings.Fields(s)
		initials := make([]byte, 0, 2)
		for i := range 2 {
			if i >= len(parts) {
				break
			}
			r := []rune(parts[i])
			if len(r) == 0 {
				continue
			}
			u := strings.ToUpper(string(r[0]))
			if len(u) > 0 {
				initials = append(initials, u[0])
			}
		}
		return frame.ReturnString(string(initials))
	}
}

// formatBytesHelper formats a byte count as "1.2 MB" etc.
func formatBytesHelper(S *lua.State) lua.NativeFunc {
	return func(frame lua.Frame) lua.Outcome {
		arg, ok := frame.Argument(0)
		if !ok || arg.IsNil() {
			return frame.ReturnString("0 B")
		}
		var n int64
		switch arg.Kind() {
		case lua.NumberKind:
			f, _ := arg.AsNumber()
			n = int64(f)
		case lua.StringKind:
			s, _ := arg.AsString()
			val, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return frame.ReturnString("0 B")
			}
			n = val
		default:
			return frame.ReturnString("0 B")
		}
		const unit = 1024
		if n < unit {
			return frame.ReturnString(strconv.FormatInt(n, 10) + " B")
		}
		div, exp := int64(unit), 0
		for m := n / unit; m >= unit && exp < 3; m /= unit {
			div *= unit
			exp++
		}
		return frame.ReturnString(
			strconv.FormatFloat(float64(n)/float64(div), 'f', 1, 64) + " " + string("KMG"[exp]) + "B",
		)
	}
}

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
		var url strings.Builder
		url.WriteString(base + sep + param + "=" + p)

		// Read Extra field and append as key=value pairs
		if v, err := frame.Index(t.Value(), lua.String("Extra")); err == nil && v.Kind() == lua.TableKind {
			if vt, ok := v.AsTable(); ok {
				nilVal := lua.Nil()
				for k, val, ok, _ := vt.Next(nilVal); ok; k, val, ok, _ = vt.Next(k) {
					ev, _ := frame.ToString(val)
					url.WriteString("&" + ev)
				}
			}
		}
		return frame.ReturnString(url.String())
	}
}

// pageWindow returns a window of page numbers to display around current.
