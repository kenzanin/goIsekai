package bridge

// Task 2.3: the image path treats a 200 HTML interstitial as a block, walks
// the plugin's identity ladder, and surfaces a challenge error (not "invalid
// image data"); a truncated JPEG keeps the old corrupt-bytes retry; a blocked
// response never creates a cache file.

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"goisekai/internal/hostnet"
)

// serveInterstitialJPEG answers any request carrying a blockedUA with a 200
// HTML verification page, and everything else with a valid JPEG, recording
// each request's UA. The block is per-UA, not per-hit: the proxy's ladder may
// re-probe the same UA on another TLS profile and must stay blocked.
func serveInterstitialJPEG(t *testing.T, blocked func(ua string) bool, uas *[]string, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}
	const page = "<!DOCTYPE html>\n<html><head><title>Browser verification</title></head><body>Checking\u2026</body></html>\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hits.Add(1)
		ua := r.Header.Get("User-Agent")
		*uas = append(*uas, ua)
		if blocked == nil || blocked(ua) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(page))
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpg.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestImageHTMLInterstitialFallsToNextUA(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := serveInterstitialJPEG(t, func(ua string) bool { return ua == "ua-blocked" }, &uas, &hits)

	s := newTestServiceWithCache(t)
	s.proxy.SetUserAgents("plugin-x", []string{"ua-blocked", "ua-clears"})

	data, err := s.GetImage(context.Background(), "plugin-x", srv.URL+"/img", nil, "", "", PrioLow)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty image bytes")
	}
	if got, want := uas[len(uas)-1], "ua-clears"; got != want {
		t.Fatalf("final fetch UA = %q, want %q; uas=%v hits=%d", got, want, uas, hits.Load())
	}
}

func TestImageExhaustedLadderYieldsChallengeError(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := serveInterstitialJPEG(t, func(ua string) bool { return true }, &uas, &hits)

	s := newTestServiceWithCache(t)
	s.proxy.SetUserAgents("plugin-x", []string{"ua-a", "ua-b"})

	data, err := s.GetImage(context.Background(), "plugin-x", srv.URL+"/img", nil, "", "", PrioLow)
	if err == nil {
		t.Fatalf("expected an error for an all-blocked interstitial; got %d bytes, uas=%v, hits=%d", len(data), uas, hits.Load())
	}
	if strings.Contains(err.Error(), "invalid image data") {
		t.Fatalf("err = %v, must not be the corrupt-image error", err)
	}
	if !errors.Is(err, hostnet.ErrChallenge) {
		t.Fatalf("err = %v, want a challenge error", err)
	}
}

func TestImageTruncatedJPEGKeepsOldRetry(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}
	truncated := jpg.Bytes()[:len(jpg.Bytes())/2]

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1))
		w.Header().Set("Content-Type", "image/jpeg")
		if n < 3 {
			_, _ = w.Write(truncated)
			return
		}
		_, _ = w.Write(jpg.Bytes())
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)
	if _, err := s.GetImage(context.Background(), "plugin-x", srv.URL+"/img", nil, "", "", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if int(hits.Load()) < 2 {
		t.Fatalf("expected corrupt-image retries; requests=%d", hits.Load())
	}
}

func TestImageBlockedResponseWritesNoCacheFile(t *testing.T) {
	var hits atomic.Int32
	var uas []string
	srv := serveInterstitialJPEG(t, func(ua string) bool { return true }, &uas, &hits)

	s := newTestServiceWithCache(t)
	_, err := s.GetImage(context.Background(), "plugin-x", srv.URL+"/img", nil, "m1", "c1", PrioLow)
	if err == nil {
		t.Fatalf("expected a challenge error; uas=%v, hits=%d", uas, hits.Load())
	}
	base := s.diskCachePath("plugin-x", "m1", "c1", srv.URL+"/img")
	if base == "" {
		t.Fatal("no cache path resolved")
	}
	for _, ext := range []string{".avif", ".webp", ".img", ".jxl"} {
		if _, statErr := os.Stat(base + ext); statErr == nil {
			t.Errorf("blocked response wrote a cache file: %s%s", base, ext)
		}
	}
	if entries, _ := os.ReadDir(filepath.Dir(base)); len(entries) != 0 {
		t.Errorf("cache dir for a blocked URL is not empty: %v", entries)
	}
}
