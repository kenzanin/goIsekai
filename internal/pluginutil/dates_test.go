package pluginutil

import (
	"testing"
	"time"
)

// ParseDate/DateToISO are the single funnel every plugin's chapter dates flow
// through (host.text.date_to_iso). A silent regression here blanks
// released_at across all plugins — so each shape is pinned by vector.
func TestParseDateVectors(t *testing.T) {
	now := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   string
		want time.Time
	}{
		{"unix seconds", "1700000000", time.Unix(1700000000, 0)},
		{"unix millis", "1700000000000", time.UnixMilli(1700000000000)},
		{"just now", "just now", now},
		{"today", "today", now},
		{"yesterday", "yesterday", now.Add(-24 * time.Hour)},
		{"relative days", "2 days ago", now.Add(-48 * time.Hour)},
		{"relative singular hour", "1 hour ago", now.Add(-time.Hour)},
		{"relative weeks", "3 weeks ago", now.Add(-3 * 7 * 24 * time.Hour)},
		{"iso date", "2024-03-05", time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)},
		{"us written", "Jan 2, 2006", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC)},
		{"us written no space", "Jan 2,2006", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC)},
		{"rfc3339", "2024-03-05T10:30:00Z", time.Date(2024, 3, 5, 10, 30, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseDate(tc.in, now)
			if !ok {
				t.Fatalf("ParseDate(%q) = not ok, want %v", tc.in, tc.want)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("ParseDate(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// Slash-only dates are ambiguous without the site's locale (day-first vs
// month-first), so they must never resolve — resolving them would silently
// swap day and month for half the sites.
func TestParseDateRejects(t *testing.T) {
	now := time.Now()
	for _, in := range []string{"", "   ", "garbage", "01/02/2024", "2024/03/05", "12345678", "123456789012345"} {
		if _, ok := ParseDate(in, now); ok {
			t.Fatalf("ParseDate(%q) = ok, want reject", in)
		}
	}
}

func TestDateToISOVectors(t *testing.T) {
	now := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)
	if got := DateToISO("2024-03-05", now); got != "2024-03-05T00:00:00Z" {
		t.Fatalf("DateToISO = %q, want 2024-03-05T00:00:00Z", got)
	}
	// Unparsable input must yield "" so the caller omits released_at: the ABI
	// decodes it into a time.Time, which rejects "" and would fail the whole
	// chapter list.
	if got := DateToISO("not a date", now); got != "" {
		t.Fatalf("DateToISO(garbage) = %q, want empty", got)
	}
	if got := DateToISO("01/02/2024", now); got != "" {
		t.Fatalf("DateToISO(slash) = %q, want empty", got)
	}
}
