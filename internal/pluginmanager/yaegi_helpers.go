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
