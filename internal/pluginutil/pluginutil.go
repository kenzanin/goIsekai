// Package pluginutil holds pure helpers shared by every plugin runtime
// (Lua/Lunar, JS/goja, WASM/Extism, Scriggo). The host exposes these as
// natives so the same logic is not re-implemented per plugin per language.
package pluginutil

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// Base64Encode returns standard padded base64.
func Base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// Base64Decode decodes standard padded base64.
func Base64Decode(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Base64URLEncode returns unpadded base64url (JWT/VRF style).
func Base64URLEncode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// Base64URLDecode decodes unpadded base64url, tolerating padded input.
func Base64URLDecode(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HexEncode returns lowercase hex.
func HexEncode(s string) string {
	return hex.EncodeToString([]byte(s))
}

// HexDecode decodes lowercase or uppercase hex.
func HexDecode(s string) (string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SHA256Hex returns the lowercase hex SHA-256 digest.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// MD5Hex returns the lowercase hex MD5 digest.
func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HMACSHA256Hex returns the lowercase hex HMAC-SHA256 of msg under key.
func HMACSHA256Hex(key, msg string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// XOR returns the byte-wise XOR of a and b as a new byte slice.
// If lengths differ, XORs up to the shorter length.
// ponytail: could accept varargs for multi-source XOR, add when mangafire needs it.
func XOR(a, b []byte) []byte {
	out := make([]byte, len(a))
	n := min(len(b), len(a))
	for i := 0; i < n; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// XORHex returns the byte-wise XOR of two hex-encoded strings as a new hex string.
// If lengths differ, XORs up to the shorter length.
// ponytail: could accept varargs for multi-source XOR, add when mangafire needs it.
func XORHex(a, b string) (string, error) {
	aa, err := hex.DecodeString(a)
	if err != nil {
		return "", err
	}
	bb, err := hex.DecodeString(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(XOR(aa, bb)), nil
}

// UTF8Hex encodes a string as its UTF-8 bytes, returned as lowercase hex.
func UTF8Hex(s string) string {
	return hex.EncodeToString([]byte(s))
}

// B64DecodeHex decodes standard padded base64 and returns the result as hex.
func B64DecodeHex(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// B64URLEncodeHex encodes a hex string as bytes and returns base64url (unpadded).
func B64URLEncodeHex(h string) (string, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// B64URLDecodeHex decodes unpadded base64url and returns the result as hex.
func B64URLDecodeHex(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
