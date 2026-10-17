package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/pkg/types"
)

// Read-ahead moved to the server has exactly one job: land the bytes in L2 so
// the reader's later /image request for that page is a cache hit instead of a
// second upstream fetch. Warming and then reading must touch the network once.
func TestWarmPagesLandsInCacheAndSkipsRefetch(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpegBytes(t, greyPage(32, 32)))
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithFormat(t, FormatWebP)
	pages := []types.Page{
		{Index: 0, URL: srv.URL + "/0.jpg"},
		{Index: 1, URL: srv.URL + "/1.jpg"},
	}
	s.WarmPages("plugin-x", "m1", "c1", pages)

	deadline := time.Now().Add(15 * time.Second)
	for _, p := range pages {
		base := s.diskCachePath("plugin-x", "m1", "c1", p.URL)
		for {
			if _, err := os.Stat(base + ".webp"); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("read-ahead never wrote %s.webp", base)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	// The reader's own fetch for a warmed page must not hit the network: the
	// L1/L2 entry the warm just created is the whole point of moving it here.
	before := hits.Load()
	if _, err := s.GetImage(context.Background(), "plugin-x", pages[0].URL, nil, "m1", "c1", PrioLow); err != nil {
		t.Fatalf("GetImage after warm: %v", err)
	}
	if after := hits.Load(); after != before {
		t.Errorf("upstream hits went %d -> %d; a warmed page must be served from cache", before, after)
	}
}
