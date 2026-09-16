package pluginmanager

import (
	"strings"

	"goisekai/pkg/types"
)

// yaegiRequired are the ABI function names every yaegi plugin MUST define.
var yaegiRequired = []string{
	types.SearchFunc,
	types.GetMangaDetailFunc,
	types.GetChapterListFunc,
	types.GetPageListFunc,
}

// initWrapperSrc returns the gskCall_Init wrapper matching the plugin's
// Init signature, or "" when the plugin has no Init. Init is optional and
// its signature varies: no arg or one arg, returning string or (string, error).
func initWrapperSrc(srcStr string) string {
	if !strings.Contains(srcStr, "func "+types.InitFunc+"(") {
		return ""
	}
	// Locate the param list between "func Init(" and its matching ")".
	o := strings.Index(srcStr, "func "+types.InitFunc+"(")
	o += len("func " + types.InitFunc + "(")
	depth := 1
	j := o
	for j < len(srcStr) && depth > 0 {
		switch srcStr[j] {
		case '(':
			depth++
		case ')':
			depth--
		}
		j++
	}
	params := strings.TrimSpace(srcStr[o : j-1])
	rest := strings.TrimSpace(srcStr[j:])
	hasErr := strings.HasPrefix(rest, "(string, error)")
	call := "Init()"
	if params != "" {
		call = "Init(arg)"
	}
	if hasErr {
		return "func gskCall_Init(arg string) string { s, e := " + call + "; if e != nil { panic(\"plugin error: \" + e.Error()) }; return s }"
	}
	return "func gskCall_Init(arg string) string { return " + call + " }"
}

// isGoStdlib checks if the import path is a Go standard library package.
func isGoStdlib(path string) bool {
	if strings.HasPrefix(path, "github.com/") ||
		strings.HasPrefix(path, "golang.org/x/") ||
		strings.HasPrefix(path, "gopkg.in/") ||
		strings.HasPrefix(path, "gitea.com/") ||
		strings.HasPrefix(path, "bitbucket.org/") {
		return false
	}
	// Also block goisekai internal paths that aren't the hostnet bridge.
	if strings.HasPrefix(path, "goisekai/") {
		return false
	}
	return true
}
