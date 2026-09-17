package pluginutil

import (
	"reflect"
	"testing"
)

// TestRegexFind pins the string.match contract the plugins rely on: captures
// when the pattern has them, the whole match when it does not, nil when nothing
// matches. The patterns are the real ones the plugin migration uses, so a
// translation that only looks right fails here.
func TestRegexFind(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		pattern string
		want    []string
	}{
		{"no capture returns whole match", "chapterId = 42", `chapterId\s*=\s*\d+`, []string{"chapterId = 42"}},
		{"one capture", "chapterId = 42", `chapterId\s*=\s*(\d+)`, []string{"42"}},
		{"two captures", `<a href="/manga/x/" title="X">`, `^<a\s+href="([^"]+)"\s+title="([^"]*)"`, []string{"/manga/x/", "X"}},
		{"heading text", "<h1>Solo Leveling</h1>", `<h1>([^<]+)</h1>`, []string{"Solo Leveling"}},
		{"non-greedy stops at first closer", "a<b>1</b><b>2</b>", `<b>(.*?)</b>`, []string{"1"}},
		{"attribute with fallback alternation", `x src="https://a/b.png"`, `(?:data-src|src)="(https?://[^"]+)"`, []string{"https://a/b.png"}},
		{"anchored suffix", "logo.png", `\.png$`, []string{".png"}},
		{"no match is nil", "abc", `(\d+)`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RegexFind(tt.subject, tt.pattern)
			if err != nil {
				t.Fatalf("RegexFind(%q, %q): %v", tt.subject, tt.pattern, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("RegexFind(%q, %q) = %#v, want %#v", tt.subject, tt.pattern, got, tt.want)
			}
		})
	}
}

// TestRegexFindRejectsBadPattern keeps a broken pattern distinguishable from a
// pattern that merely did not match: one is an error, the other is nil.
func TestRegexFindRejectsBadPattern(t *testing.T) {
	if _, err := RegexFind("abc", `(unclosed`); err == nil {
		t.Fatal("want error for unclosed group, got nil")
	}
	got, err := RegexFind("abc", `(\d+)`)
	if err != nil {
		t.Fatalf("no match should not error: %v", err)
	}
	if got != nil {
		t.Fatalf("no match should be nil, got %#v", got)
	}
}

func TestRegexMatch(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		pattern string
		want    bool
	}{
		{"hit", "https://x/1/2.jpg", `\.(?:jpg|png)$`, true},
		{"miss", "https://x/1/2.webp", `\.(?:jpg|png)$`, false},
		{"unanchored hit", "class=\"post-title\"", `post-title`, true},
		{"literal dot is literal", "a.svgx", `\.svg$`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RegexMatch(tt.subject, tt.pattern)
			if err != nil {
				t.Fatalf("RegexMatch: %v", err)
			}
			if got != tt.want {
				t.Fatalf("RegexMatch(%q, %q) = %v, want %v", tt.subject, tt.pattern, got, tt.want)
			}
		})
	}
}

// TestRegexFindAll pins the gmatch contract: every match in order, each row
// carrying the captures (or the whole match when the pattern has none), with
// every row of one pattern the same length so callers can read a one-value row
// as a plain string.
func TestRegexFindAll(t *testing.T) {
	subject := `<a href="/manga/one/" title="One">` +
		`<a href="/manga/two/" title="Two">` +
		`<a href="/manga/three/" title="Three">`

	t.Run("two captures per row", func(t *testing.T) {
		got, err := RegexFindAll(subject, `<a\s+href="([^"]+)"\s+title="([^"]+)"`)
		if err != nil {
			t.Fatal(err)
		}
		want := [][]string{
			{"/manga/one/", "One"},
			{"/manga/two/", "Two"},
			{"/manga/three/", "Three"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("one capture per row", func(t *testing.T) {
		got, err := RegexFindAll(subject, `href="/(manga/[^"]+)"`)
		if err != nil {
			t.Fatal(err)
		}
		want := [][]string{{"manga/one/"}, {"manga/two/"}, {"manga/three/"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("no capture returns whole matches", func(t *testing.T) {
		got, err := RegexFindAll("a1 b2 c3", `[a-z]\d`)
		if err != nil {
			t.Fatal(err)
		}
		want := [][]string{{"a1"}, {"b2"}, {"c3"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("no match is empty, not nil", func(t *testing.T) {
		got, err := RegexFindAll("abc", `\d+`)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("want no rows, got %#v", got)
		}
	})
}

func TestRegexReplace(t *testing.T) {
	got, err := RegexReplace("a  b\tc", `\s+`, " ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a b c" {
		t.Fatalf("got %q", got)
	}

	got, err = RegexReplace("<b>x</b>", `</?b>`, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "x" {
		t.Fatalf("got %q", got)
	}

	if _, err := RegexReplace("x", `(`, ""); err == nil {
		t.Fatal("want error for bad pattern")
	}
}

// TestRegexFindIndex pins the string.find contract the cursor scans rely on:
// 1-based byte offsets, inclusive end, and no match reported without an error.
func TestRegexFindIndex(t *testing.T) {
	const subject = `<a href="/x/">x</a><a href="/y/">y</a>`
	const link = `href="([^"]+)"`

	start, end, ok, err := RegexFindIndex(subject, link, 1)
	if err != nil || !ok {
		t.Fatalf("first call: ok=%v err=%v", ok, err)
	}
	if start != 4 || end != 13 {
		t.Fatalf("first call = %d..%d, want 4..13", start, end)
	}
	if got := subject[start-1 : end]; got != `href="/x/"` {
		t.Fatalf("first match sliced %q", got)
	}

	// Starting one past the match's first byte finds the next link, so a cursor
	// scan advances even though the match is longer than one byte.
	start, end, ok, err = RegexFindIndex(subject, link, start+1)
	if err != nil || !ok {
		t.Fatalf("second call: ok=%v err=%v", ok, err)
	}
	if start != 23 || end != 32 {
		t.Fatalf("second call = %d..%d, want 23..32", start, end)
	}
	if got := subject[start-1 : end]; got != `href="/y/"` {
		t.Fatalf("second match sliced %q", got)
	}

	if _, _, ok, err = RegexFindIndex(subject, link, 40); ok || err != nil {
		t.Fatalf("past-the-end init: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err = RegexFindIndex(subject, `nope`, 1); ok || err != nil {
		t.Fatalf("no match should be ok=false and no error, got ok=%v err=%v", ok, err)
	}
	if _, _, _, err = RegexFindIndex(subject, `(`, 1); err == nil {
		t.Fatal("want error for bad pattern")
	}

	// Byte offsets, not rune offsets: the scan position stays usable as a
	// string.sub bound on a page holding non-ASCII text.
	start, end, ok, err = RegexFindIndex("é<img>", `<img>`, 1)
	if err != nil || !ok {
		t.Fatalf("utf-8: ok=%v err=%v", ok, err)
	}
	if start != 3 || end != 7 {
		t.Fatalf("utf-8 = %d..%d, want 3..7", start, end)
	}
}

// TestRegexQuote checks a needle full of metacharacters matches itself once
// quoted, which is what a literal search over a scraped slug needs.
func TestRegexQuote(t *testing.T) {
	const needle = `a+b(c)[d].jpg?x=1|2`
	const subject = "prefix " + needle + " suffix"

	quoted := RegexQuote(needle)
	start, end, ok, err := RegexFindIndex(subject, quoted, 1)
	if err != nil || !ok {
		t.Fatalf("quoted needle did not match: ok=%v err=%v", ok, err)
	}
	if got := subject[start-1 : end]; got != needle {
		t.Fatalf("matched %q, want %q", got, needle)
	}

	// The unquoted needle is either a compile error or matches the wrong text,
	// never itself. If that stops holding this test has stopped proving
	// anything and the assertion above is no longer meaningful.
	s, e, matched, err := RegexFindIndex(subject, needle, 1)
	if err == nil && matched && subject[s-1:e] == needle {
		t.Fatal("unquoted needle matched itself; quote is no longer load-bearing here")
	}
}

// TestRegexCompileIsCached checks the cache returns the identical compiled
// pattern, so repeated plugin calls do not pay for a compile each time.
func TestRegexCompileIsCached(t *testing.T) {
	const pattern = `<h1>([^<]+)</h1>`
	first, err := regexCompile(pattern)
	if err != nil {
		t.Fatal(err)
	}
	second, err := regexCompile(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("second compile did not reuse the cached pattern")
	}
}
