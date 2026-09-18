package httpserver

import (
	"strings"
	"testing"
)

func TestRenderMarkdownREADMESubset(t *testing.T) {
	md := "# Title\n\nIntro with **bold**, *it*, `code`, [link](https://x.y) and ![img](b.png).\n\n## Section\n\n- bullet **one**\n- bullet two\n\n```sh\njust build\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n---\n\n<script>alert(1)</script>"
	out := renderMarkdown(md)
	for _, want := range []string{
		"<h1", "<h2", "<strong>bold</strong>", "<em>it</em>", "<code>code</code>",
		`href="https://x.y"`, "<li", "just build", "<table", "<hr", "alert(1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output", want)
		}
	}
	// Script must arrive escaped, never executable.
	if strings.Contains(out, "<script>") {
		t.Error("raw <script> made it through")
	}
	// Badge image renders as a normal img element with the alt text.
	if !strings.Contains(out, `alt="img"`) {
		t.Error("badge image not rendered with alt text")
	}
}

func TestLoadReadmeMissing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if got := loadReadme(); !strings.Contains(got, "not found") {
		t.Fatalf("want not-found note, got %q", got)
	}
}
