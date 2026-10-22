package client

import (
	"net/url"
	"testing"
)

func TestMangaHrefEscapesPathSegments(t *testing.T) {
	// batcave's source_manga_id is a full permalink containing slashes.
	got := mangaHref("http://h", "batcave", "https://batcave.biz/33758-batman.html")
	const wantPath = "/view/manga/batcave/https:%2F%2Fbatcave.biz%2F33758-batman.html"
	if want := "http://h" + wantPath; got != want {
		t.Fatalf("mangaHref = %q, want %q", got, want)
	}
	// EscapedPath, not Path: Path is already decoded, which is exactly the
	// form a router must never see.
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.EscapedPath() != wantPath {
		t.Fatalf("EscapedPath = %q, want %q", u.EscapedPath(), wantPath)
	}
}

func TestCoverProxyURLRoundTripsQuery(t *testing.T) {
	const cover = "https://x/c.jpg?token=abc&ttl=1"
	got := coverProxyURL("http://h", "mangahere", cover)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Path != "/image" {
		t.Fatalf("path = %q, want /image", u.Path)
	}
	q := u.Query()
	if q.Get("pluginID") != "mangahere" {
		t.Fatalf("pluginID = %q", q.Get("pluginID"))
	}
	// The inner URL's own query must survive encoding intact, or the CDN token
	// is truncated and every cover 403s.
	if inner := q.Get("url"); inner != cover {
		t.Fatalf("url = %q, want %q", inner, cover)
	}
}

func TestCoverProxyURLEmptyCover(t *testing.T) {
	if got := coverProxyURL("http://h", "p", ""); got != "" {
		t.Fatalf("empty cover should stay empty, got %q", got)
	}
}

func TestMatchesStatusMirrorsServer(t *testing.T) {
	cases := []struct {
		name   string
		item   Item
		status string
		want   bool
	}{
		{"reading needs progress", Item{Read: 3, Total: 10}, StatusReading, true},
		{"reading excludes unread", Item{Read: 0, Total: 10}, StatusReading, false},
		{"reading excludes finished", Item{Read: 10, Total: 10}, StatusReading, false},
		{"reading excludes unknown total", Item{Read: 1, Total: 0}, StatusReading, false},
		{"unread is read==0", Item{Read: 0, Total: 5}, StatusUnread, true},
		{"unread excludes started", Item{Read: 1, Total: 5}, StatusUnread, false},
		{"done needs total", Item{Read: 5, Total: 5}, StatusDone, true},
		{"done excludes total==0", Item{Read: 0, Total: 0}, StatusDone, false},
		{"new uses the badge", Item{HasNew: true}, StatusNew, true},
		{"new excludes absent badge", Item{HasNew: false}, StatusNew, false},
		{"all keeps everything", Item{}, StatusAll, true},
		{"unknown status keeps everything", Item{}, "bogus", true},
	}
	for _, c := range cases {
		if got := MatchesStatus(c.item, c.status); got != c.want {
			t.Errorf("%s: MatchesStatus = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestItemProgressGuardsZeroTotal(t *testing.T) {
	if got := (Item{Read: 3, Total: 0}).Progress(); got != 0 {
		t.Fatalf("Progress = %v, want 0", got)
	}
	if got := (Item{Read: 5, Total: 10}).Progress(); got != 0.5 {
		t.Fatalf("Progress = %v, want 0.5", got)
	}
	// Read can exceed total when a source renumbers chapters.
	if got := (Item{Read: 12, Total: 10}).Progress(); got != 1 {
		t.Fatalf("Progress = %v, want clamped to 1", got)
	}
}

func TestDupGroupKey(t *testing.T) {
	if got := DupGroupKey(Item{DupGroup: -1}); got != "" {
		t.Fatalf("non-duplicate key = %q, want empty", got)
	}
	if got := DupGroupKey(Item{DupGroup: 2}); got != "2" {
		t.Fatalf("duplicate key = %q, want 2", got)
	}
}
