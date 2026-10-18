package bridge

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Only MangaDex at-home nodes burst-reject; pacing everything else serialized
// the cover picker at ~1.1s per image.
func TestHostNeedsPacing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host string
		want bool
	}{
		{"cmdxd98sb0x3yprd.mangadex.network", true},
		{"mangadex.network", true},
		{"uploads.mangadex.org", false}, // static cover CDN: serves bursts fine
		{"mangadex.org", false},
		{"resmk.my.id", false},
		{"evil-mangadex.network.attacker.test", false}, // suffix, not substring
		{"", false},
	}
	for _, c := range cases {
		if got := hostNeedsPacing(c.host); got != c.want {
			t.Errorf("hostNeedsPacing(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// Fetches to a burst-tolerant host must not be spaced by the 1.1s pace gap.
// Pacing them made the cover picker take ~N seconds for N covers, so a click on
// an already-loaded cover queued its full-size download behind the whole grid.
//
// The handler answers immediately, so the per-host admission gate (2 at a time)
// frees each slot at once and only pacing could spread the arrivals out.
func TestCoverCDNFetchesAreNotPaced(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	payload := buf.Bytes()

	const n = 3
	arrivals := make(chan time.Time, n)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivals <- time.Now()
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	s := newTestServiceWithCache(t)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Distinct paths so each is a cache miss on a distinct URL.
			_, _ = s.GetImage(context.Background(), "plugin-x",
				srv.URL+"/cover/"+strconv.Itoa(i)+".png", nil, "", "", PrioHigh)
		}()
	}
	wg.Wait()
	close(arrivals)

	first, last := time.Time{}, time.Time{}
	for ts := range arrivals {
		if first.IsZero() || ts.Before(first) {
			first = ts
		}
		if last.IsZero() || ts.After(last) {
			last = ts
		}
	}
	// Paced, n requests arrive >= (n-1)*1100ms = 2.2s apart; unpaced they bunch
	// within milliseconds. 1100ms cleanly separates the two.
	if spread := last.Sub(first); spread >= 1100*time.Millisecond {
		t.Fatalf("%d fetches to a cover CDN arrived %v apart — host is being paced", n, spread)
	}
}
