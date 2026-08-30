// Command frontendgen assembles cmd/goisekai/frontend/index.html from
// shell.html + the views/ partials, so large Alpine view sections live in
// small separate files instead of one huge index.html.
//
// Usage: go run ./cmd/frontendgen
//
// The generated index.html is committed; go:embed reads it at compile time.
// wiregen is not required at build time unless a partial changed — run it,
// then rebuild. Guarded by Makefile gen-frontend.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := "cmd/goisekai/frontend"
	if _, err := os.Stat(dir); err != nil {
		// allow running from the repo root's parent or via go run path
		if _, err2 := os.Stat("frontend"); err2 == nil {
			dir = "frontend"
		}
	}

	shell, err := os.ReadFile(filepath.Join(dir, "shell.html"))
	if err != nil {
		fatal(err)
	}

	out := string(shell)
	// Substituting all <!-- @partial:NAME --> markers with views/NAME.html.
	i := 0
	for {
		start := strings.Index(out, "<!-- @partial:")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "-->")
		if end < 0 {
			fatal(fmt.Errorf("unterminated partial marker near offset %d", start))
		}
		end += start
		mk := strings.TrimSuffix(strings.TrimPrefix(out[start+len("<!-- @partial:"):end], " "), " ")
		name := strings.TrimPrefix(strings.TrimPrefix(mk, "@partial:"), "@partial:")
		content, err := os.ReadFile(filepath.Join(dir, "views", name+".html"))
		if err != nil {
			fatal(fmt.Errorf("reading partial %q: %w", name, err))
		}
		out = out[:start] + string(content) + out[end+len("-->"):]
		i++
		if i > 20 {
			fatal(fmt.Errorf("too many substitutions"))
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(out), 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("frontendgen: assembled index.html from shell.html + %d partials\n", i)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "frontendgen:", err)
	os.Exit(1)
}
