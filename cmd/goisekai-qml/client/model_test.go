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
