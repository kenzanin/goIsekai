package pluginutil

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	relativeDate = regexp.MustCompile(
		`(?i)^(\d+)\s*(minutes?|mins?|hours?|hrs?|days?|weeks?|months?|years?)\s+ago\b`)

	dateLayouts = []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"02 Jan 2006",
		"2 Jan 2006",
		"Jan 2, 2006",
		"Jan 2 2006",
		"January 2, 2006",
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
	}
)

// DateToISO normalizes a release date to RFC 3339 in UTC, so plugins hand the
// host one shape instead of each guessing a layout. now anchors the relative
// phrases ("2 days ago", "yesterday").
//
// Returns "" when nothing parses, and the caller must then leave released_at
// out of the payload entirely: the ABI decodes it into a time.Time, which
// rejects "" and would fail the whole chapter list.
func DateToISO(s string, now time.Time) string {
	t, ok := ParseDate(s, now)
	if !ok {
		return ""
	}
	return formatISO(t)
}

// ParseDate resolves the date shapes sites put in markup and JSON: unix seconds
// or milliseconds, the relative phrases listing pages use for their newest
// chapters, and the layouts in dateLayouts. Slash-only dates never resolve:
// day-first and month-first are indistinguishable without the site's locale.
func ParseDate(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if isDigits(s) {
		n, err := strconv.ParseInt(s, 10, 64)
		switch l := len(s); {
		case err == nil && l >= 9 && l <= 11:
			return time.Unix(n, 0), true
		case err == nil && l >= 12 && l <= 14:
			return time.UnixMilli(n), true
		}
	}
	if d, ok := parseRelative(s, now); ok {
		return d, true
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// relativeUnits maps a site's unit word to its duration. Months and years use
// the calendar averages, which is as precise as "3 months ago" ever gets.
var relativeUnits = map[string]time.Duration{
	"minute": time.Minute, "min": time.Minute,
	"hour": time.Hour, "hr": time.Hour,
	"day": 24 * time.Hour, "week": 7 * 24 * time.Hour,
	"month": 30 * 24 * time.Hour, "year": 365 * 24 * time.Hour,
}

// parseRelative resolves the relative phrases listing pages use for their
// newest chapters.
func parseRelative(s string, now time.Time) (time.Time, bool) {
	switch strings.ToLower(s) {
	case "just now", "today", "moments ago":
		return now, true
	case "yesterday":
		return now.Add(-24 * time.Hour), true
	}
	m := relativeDate.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return time.Time{}, false
	}
	unit := strings.TrimSuffix(strings.ToLower(m[2]), "s")
	d, ok := relativeUnits[unit]
	if !ok {
		return time.Time{}, false
	}
	return now.Add(-time.Duration(n) * d), true
}

func formatISO(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// DateToISONow is DateToISO anchored at the current time, which is the shape
// the plugin runtimes expose as host.text.date_to_iso.
func DateToISONow(s string) string {
	return DateToISO(s, time.Now())
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
