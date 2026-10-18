package bridge

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func loadMangadexPlaceholder(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/mangadex_placeholder.jpg")
	if err != nil {
		t.Fatalf("read placeholder fixture: %v", err)
	}
	return data
}

// The MangaDex placeholder is a real JPEG (HTTP 200, 59480 bytes, 600x642) so
// validateImageFull cannot reject it. isPlaceholderImage matches it by hash so
// the fetch loop retries another Referer instead of caching the placeholder.
func TestIsPlaceholderImageRejectsRealJPEG(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if isPlaceholderImage(buf.Bytes()) {
		t.Fatal("a real JPEG was misdetected as the MangaDex placeholder")
	}
}

func TestIsPlaceholderImageSizeGate(t *testing.T) {
	t.Parallel()
	// Wrong size short-circuits before hashing; any non-59480 payload is not it.
	if isPlaceholderImage(bytes.Repeat([]byte{0}, 59479)) {
		t.Fatal("59479 bytes must not match the placeholder")
	}
	if isPlaceholderImage(nil) {
		t.Fatal("nil must not match the placeholder")
	}
}

// isPlaceholderImage must positively identify the exact placeholder bytes. The
// fixture is checked in so the hash constant and the detection stay in sync.
func TestIsPlaceholderImagePositive(t *testing.T) {
	t.Parallel()
	fixture := loadMangadexPlaceholder(t)
	if !isPlaceholderImage(fixture) {
		t.Fatal("the known MangaDex placeholder fixture was not detected")
	}
	// One flipped byte must drop detection: the match is the exact bytes.
	mutated := bytes.Clone(fixture)
	mutated[len(mutated)-1] ^= 0xff
	if isPlaceholderImage(mutated) {
		t.Fatal("a mutated placeholder must not match")
	}
}

// The reported bug: uploads.mangadex.org answers a foreign Referer with the
// placeholder under HTTP 200, so the fetch loop accepted it as success and
// cached it. Here the first (origin) Referer yields the placeholder and the
// fallthrough Referer yields the real image: the loop must skip the first and
// keep the real bytes.
func TestPlaceholderRefererFallsThroughToRealImage(t *testing.T) {
	placeholder := loadMangadexPlaceholder(t)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	real := buf.Bytes()

	var hits atomic.Int32
	var winningReferer atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Referer") != "" {
			// Foreign Referer: HTTP 200, but the placeholder body.
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(placeholder)
			return
		}
		winningReferer.Store("<none>")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(real)
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)
	if _, err := s.GetImage(t.Context(), "p", srv.URL+"/cover.jpg", nil, "", "", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	// Two upstream calls: the origin-Referer placeholder, then the no-Referer
	// real image. Accepting the placeholder would stop at one.
	if got := hits.Load(); got != 2 {
		t.Errorf("upstream hits = %d, want 2 (placeholder rejected, then real image)", got)
	}
	if v, _ := winningReferer.Load().(string); v != "<none>" {
		t.Error("the placeholder-bearing Referer was accepted instead of falling through")
	}
}
