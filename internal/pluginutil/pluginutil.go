// Package pluginutil holds pure helpers shared by every plugin runtime
// (Lua/Lunar, JS/goja, Yaegi). The host exposes these as
// natives so the same logic is not re-implemented per plugin per language.
package pluginutil

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	brTag   = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTag = regexp.MustCompile(`<[^>]*>`)

	mdLink      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdAutolink  = regexp.MustCompile(`<(https?://[^>]+)>`)
	mdBold      = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdUnderline = regexp.MustCompile(`__([^_]+)__`)
	mdItalic    = regexp.MustCompile(`\*([^*]+)\*`)
	mdEmphasis  = regexp.MustCompile(`_([^_]+)_`)
	mdHeading   = regexp.MustCompile(`(?m)^#+\s*`)
	mdRuleU     = regexp.MustCompile(`(?m)^_+\s*$`)
	mdRuleD     = regexp.MustCompile(`(?m)^-+\s*$`)
	mdBlankRuns = regexp.MustCompile(`\n{3,}`)
	mdTrailWS   = regexp.MustCompile(`[ \t]+\n`)
)

// URLEncode percent-encodes every byte outside the RFC 3986 unreserved set
// (A-Za-z0-9-._~), uppercase hex. Mirrors the Lua plugins' url_encode.
func URLEncode(s string) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperhex[c>>4])
		b.WriteByte(upperhex[c&0x0F])
	}
	return b.String()
}

// URLDecode percent-decodes %XX where the byte is >= 32 (printable and UTF-8
// continuation bytes), leaving control-byte and malformed sequences as-is.
// Non-ASCII sequences reassemble into valid UTF-8.
func URLDecode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := hexVal(s[i+1])
			lo, ok2 := hexVal(s[i+2])
			if v := hi<<4 | lo; ok1 && ok2 && v >= 32 {
				b.WriteByte(v)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// HTMLDecode decodes named and numeric HTML entities in a single pass.
func HTMLDecode(s string) string {
	return html.UnescapeString(s)
}

// StripHTML turns <br> variants into newlines, removes all other tags, decodes
// entities, and trims the result.
func StripHTML(s string) string {
	s = brTag.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

// StripMarkdown reduces inline markdown (links, emphasis) and block markdown
// (headings, horizontal rules) to plain text, collapsing blank-line runs.
//
// ponytail: regex stripper, faithful to the Lua/JS plugins it replaces. It
// still mangles intra-word underscores (snake_case) and nested emphasis.
// Upgrade path: a gomarkdown AST text renderer if descriptions ever need
// CommonMark-correct stripping (memo: gomarkdown has no built-in text renderer).
func StripMarkdown(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdAutolink.ReplaceAllString(s, "$1")
	s = mdBold.ReplaceAllString(s, "$1")
	s = mdUnderline.ReplaceAllString(s, "$1")
	s = mdItalic.ReplaceAllString(s, "$1")
	s = mdEmphasis.ReplaceAllString(s, "$1")
	s = mdHeading.ReplaceAllString(s, "")
	s = mdRuleU.ReplaceAllString(s, "")
	s = mdRuleD.ReplaceAllString(s, "")
	s = mdBlankRuns.ReplaceAllString(s, "\n\n")
	s = mdTrailWS.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}

// Titlecase uppercases the first rune only.
func Titlecase(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// HTMLEntityUnescape decodes named and numeric HTML entities (&amp; &#039; &#x27; etc.)
// to their UTF-8 characters. It is an alias for HTMLDecode with a more explicit name.
func HTMLEntityUnescape(s string) string {
	return html.UnescapeString(s)
}

// LuaEscape escapes every Lua pattern magic character (- . + [ ] ( ) $ ^ % ? *)
// with a leading %, so the result is a literal for string.find/gsub/match.
func LuaEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 10)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '-', '.', '+', '[', ']', '(', ')', '$', '^', '%', '?', '*':
			b.WriteByte('%')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
