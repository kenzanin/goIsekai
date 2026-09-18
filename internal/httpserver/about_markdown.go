package httpserver

import (
	"os"
	"path/filepath"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// renderMarkdown converts README.md to HTML: CommonMark plus tables, fenced
// code, and autolinks. SkipHTML drops raw HTML blocks (script injection),
// HrefTargetBlank opens links in new tabs.
func renderMarkdown(md string) string {
	p := parser.NewWithExtensions(parser.CommonExtensions)
	doc := p.Parse([]byte(md))
	opts := html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank | html.SkipHTML,
	}
	return string(markdown.Render(doc, html.NewRenderer(opts)))
}

// loadReadme reads README.md from the working directory. An error returns the
// message as content so the About page never renders empty.
func loadReadme() string {
	for _, p := range []string{"README.md", filepath.Join("docs", "README.md")} {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			return string(data)
		}
	}
	return "README.md not found next to the binary (working directory)."
}
