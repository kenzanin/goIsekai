package pluginutil

import (
	"regexp"
	"strconv"
	"strings"
)

// Chapter numbering and embedded-JSON extraction: the two
// patterns every scraper re-derives from its own site's markup. They are pure
// and site-agnostic, so they live here and reach plugins through host.text.*.

var (
	// chapterMarker prefers the number attached to a chapter word, so
	// "Vol. 3 Ch. 12" resolves to 12 rather than to the volume.
	chapterMarker = regexp.MustCompile(
		`(?i)(?:\b(?:chapters?|chaps?|chs?|episodes?|eps?)\s*\.?\s*|#\s*)(\d+(?:[.,]\d+)?)`)
	volumeOnly  = regexp.MustCompile(`(?i)\b(?:volumes?|vols?|books?)\b`)
	firstNumber = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

// ChapterNum extracts a chapter number from a title, slug or URL.
//
// The number attached to a chapter word wins ("Chapter 12.5", "ch12", "#12"),
// otherwise the first number in the string is used ("One Piece 1085"). A string
// that names only a volume ("Vol. 3") returns 0, because numbering a chapter
// from a volume would corrupt reading order. 0 means "no chapter number here",
// which is what plugins already wrote by hand (`tonumber(match) or 0`).
//
// A slug like "chapter-12-5" reads as 12: whether the tail is a decimal part or
// a sub-chapter is unknowable from the string alone.
func ChapterNum(s string) float64 {
	if m := chapterMarker.FindStringSubmatch(s); m != nil {
		return parseDecimal(m[1])
	}
	if volumeOnly.MatchString(s) {
		return 0
	}
	return parseDecimal(firstNumber.FindString(s))
}

// parseDecimal reads an unsigned decimal, accepting a comma as the separator.
func parseDecimal(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return 0
	}
	return f
}

// JSONBlob returns the first balanced JSON object or array embedded in text,
// starting the search at the first occurrence of marker ("" searches from the
// start). Sites embed their payload in an inline <script>, so plugins need to
// pull one blob out of a larger document: marker is the prefix that identifies
// it ("application/ld+json", "__NEXT_DATA__").
//
// Quotes and escapes are tracked, so a brace inside a string never closes the
// blob early. Returns "" when no balanced blob follows the marker.
func JSONBlob(text, marker string) string {
	if marker != "" {
		i := strings.Index(text, marker)
		if i < 0 {
			return ""
		}
		text = text[i+len(marker):]
	}
	start := strings.IndexAny(text, "{[")
	if start < 0 {
		return ""
	}
	open := text[start]
	closer := byte('}')
	if open == '[' {
		closer = ']'
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}
