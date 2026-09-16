package templates

import (
	"strconv"
	"strings"
	"time"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/pluginutil"
)

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
		if t, ok := pluginutil.ParseDate(s, time.Now()); ok {
			return frame.ReturnString(t.Format("Jan 2, 2006"))
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
