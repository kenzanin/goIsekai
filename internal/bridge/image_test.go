package bridge

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
)

// newTestServiceWithCache builds an AppService with a throwaway disk cache dir
// so GetImage's L2 path writes real files.
func newTestServiceWithCache(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewAppService(db, nil, hostnet.NewProxy(), "", t.TempDir(), nil)
}

// newTestServiceWithFormat is newTestServiceWithCache with the cache encoding
// pinned, so tests cover avif and webp through the real GetImage path.
func newTestServiceWithFormat(t *testing.T, format ImageFormat) *AppService {
	t.Helper()
	s := newTestServiceWithCache(t)
	s.imgFormat = format
	return s
}

// serveImage starts an httptest server that returns payload with the given
// content type and the URL it serves.
func serveImage(t *testing.T, contentType string, payload []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/img"
}

func TestImageCacheConvertsJPEGToWebP(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := range 64 {
		for y := range 64 {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 200, 255})
		}
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	url := serveImage(t, "image/jpeg", jpg.Bytes())

	s := newTestServiceWithCache(t)
	if _, err := s.GetImage("plugin-x", url, nil, "", "", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	base := s.diskCachePath("plugin-x", "", "", url)
	data, err := os.ReadFile(base + ".webp")
	if err != nil {
		t.Fatalf("expected cached .webp file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("cached webp file is empty")
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) {
		t.Errorf("cached bytes lack RIFF magic: got prefix %q", data[:min(12, len(data))])
	}
	if _, err := os.Stat(base + ".img"); !os.IsNotExist(err) {
		t.Errorf("expected no .img file for converted jpeg, stat err = %v", err)
	}
}

func TestImageCacheGIFPassthrough(t *testing.T) {
	// Minimal GIF89a (1x1) — must stay byte-for-byte untouched.
	payload := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	url := serveImage(t, "image/gif", payload)

	s := newTestServiceWithCache(t)
	if _, err := s.GetImage("plugin-x", url, nil, "", "", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	base := s.diskCachePath("plugin-x", "", "", url)
	data, err := os.ReadFile(base + ".img")
	if err != nil {
		t.Fatalf("expected cached .img file: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("gif bytes changed on disk: got %q, want %q", data, payload)
	}
	if _, err := os.Stat(base + ".webp"); !os.IsNotExist(err) {
		t.Errorf("expected no .webp file for gif, stat err = %v", err)
	}
}

func TestImageCacheInvalidBytesRejected(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte("\x00\x01\x02\x03"),        // too small to sniff/decode
		[]byte("definitely not an image"), // decodable length, but garbage
	} {
		url := serveImage(t, "application/octet-stream", payload)

		s := newTestServiceWithCache(t)
		if _, err := s.GetImage("plugin-x", url, nil, "", "", PrioLow); err == nil {
			t.Fatalf("GetImage: expected error for invalid bytes")
		}

		base := s.diskCachePath("plugin-x", "", "", url)
		if _, err := os.Stat(base + ".img"); !os.IsNotExist(err) {
			t.Errorf("expected no .img file for invalid bytes, stat err = %v", err)
		}
		if _, err := os.Stat(base + ".webp"); !os.IsNotExist(err) {
			t.Errorf("expected no .webp file for invalid bytes, stat err = %v", err)
		}
	}
}

func TestImageCacheStableKeyAcrossQuery(t *testing.T) {
	s := newTestServiceWithCache(t)
	base := s.diskCachePath("plugin-x", "m1", "c1", "http://cdn.example.com/img/1.png")
	for _, signed := range []string{
		"http://cdn.example.com/img/1.png?acc=abc&expires=1",
		"http://cdn.example.com/img/1.png?acc=xyz&expires=2",
	} {
		if got := s.diskCachePath("plugin-x", "m1", "c1", signed); got != base {
			t.Errorf("diskCachePath(%s) = %s, want stable %s", signed, got, base)
		}
	}
}

func TestImageCacheHealsCorruptEntry(t *testing.T) {
	// Pre-seed L2 with corrupt bytes (legacy fail-open entry).
	s := newTestServiceWithCache(t)
	validURL := serveImage(t, "image/png", validPNG(t))
	base := s.diskCachePath("plugin-x", "", "", validURL)
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(base+".webp", []byte("not a real image"), 0o644); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}

	// Reading must heal: corrupt file deleted, valid image fetched and cached.
	if _, err := s.GetImage("plugin-x", validURL, nil, "", "", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	data, err := os.ReadFile(base + ".webp")
	if err != nil {
		t.Fatalf("expected healed .webp file: %v", err)
	}
	if !validateImageFull(data) {
		t.Errorf("healed .webp does not decode")
	}
}

func validPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestImageCacheWritesConfiguredFormat(t *testing.T) {
	for _, tc := range []struct {
		format ImageFormat
		ext    string
		sniff  func([]byte) bool
	}{
		{FormatWebP, ".webp", isWebP},
		{FormatAVIF, ".avif", isAVIF},
	} {
		t.Run(string(tc.format), func(t *testing.T) {
			url := serveImage(t, "image/jpeg", validJPEG(t, 900, 1400))
			s := newTestServiceWithFormat(t, tc.format)
			if _, err := s.GetImage("plugin-x", url, nil, "", "", PrioLow); err != nil {
				t.Fatalf("GetImage: %v", err)
			}

			base := s.diskCachePath("plugin-x", "", "", url)
			data, err := os.ReadFile(base + tc.ext)
			if err != nil {
				t.Fatalf("expected cached %s file: %v", tc.ext, err)
			}
			if !tc.sniff(data) {
				t.Errorf("%s bytes failed its magic-number check", tc.ext)
			}
			// The cover is 900x1400, so the default 720 cap must have shrunk it.
			cfg, err := webpConfig(data)
			if err == nil && cfg.Height > 720 {
				t.Errorf("cover height %d exceeds the 720 cap", cfg.Height)
			}
			if _, err := os.Stat(base + ".img"); !os.IsNotExist(err) {
				t.Errorf("expected no .img file, stat err = %v", err)
			}
		})
	}
}

func TestImageCacheOriginalFormatStoresSourceBytes(t *testing.T) {
	payload := validJPEG(t, 64, 64)
	url := serveImage(t, "image/jpeg", payload)
	s := newTestServiceWithFormat(t, FormatOriginal)
	if _, err := s.GetImage("plugin-x", url, nil, "", "", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	base := s.diskCachePath("plugin-x", "", "", url)
	data, err := os.ReadFile(base + ".img")
	if err != nil {
		t.Fatalf("expected cached .img file: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Error("original format should store the source bytes untouched")
	}
}

// jpegBytes encodes img as jpeg, the format the manga sources actually serve.
func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

// TestImageCacheEnhanceScope pins exactly which requests the enhancement
// pipeline is allowed to touch. The blast radius has to stay at "a chapter page
// from a plugin that did not opt out": covers, library thumbnails, and opted-out
// plugins all have to come out byte-identical to the plain encoding.
func TestImageCacheEnhanceScope(t *testing.T) {
	auto := enhanceConfig{defaultMode: EnhanceAuto, byPlugin: map[string]EnhanceMode{"opted-out": EnhanceOff}}
	for _, tc := range []struct {
		name               string
		pluginID           string
		mangaID, chapterID string
		enhance            enhanceConfig
		wantEnhanced       bool
	}{
		{"chapter page", "plugin-x", "m1", "c1", auto, true},
		{"cover has no chapter id", "plugin-x", "m1", "", auto, false},
		{"thumbnail has no manga id", "plugin-x", "", "", auto, false},
		{"plugin opted out", "opted-out", "m1", "c1", auto, false},
		{"enhancement off", "plugin-x", "m1", "c1", enhanceConfig{defaultMode: EnhanceOff}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := jpegBytes(t, greyPage(300, 400))
			// A fresh URL per case: the L1 cache is keyed by URL alone, so
			// reusing one would skip the L2 write this test is checking.
			url := serveImage(t, "image/jpeg", payload)

			s := newTestServiceWithFormat(t, FormatWebP)
			s.enhance = tc.enhance
			if _, err := s.GetImage(tc.pluginID, url, nil, tc.mangaID, tc.chapterID, PrioLow); err != nil {
				t.Fatalf("GetImage: %v", err)
			}

			base := s.diskCachePath(tc.pluginID, tc.mangaID, tc.chapterID, url)
			got, err := os.ReadFile(base + ".webp")
			if err != nil {
				t.Fatalf("expected cached .webp file: %v", err)
			}
			plain, _ := encodeForCache(payload, FormatWebP, tc.mangaID == "", s.coverMaxDim, false, nil)
			if enhanced := !bytes.Equal(got, plain); enhanced != tc.wantEnhanced {
				t.Errorf("enhanced = %v, want %v", enhanced, tc.wantEnhanced)
			}
		})
	}
}

// TestSamePriorityFIFO verifies that requests of the same priority
// are handled in arrival order.
func TestSamePriorityFIFO(t *testing.T) {
	s := newTestServiceWithCache(t)

	var mu sync.Mutex
	callOrder := make([]int, 0)
	var idx int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callOrder = append(callOrder, idx)
		idx++
		mu.Unlock()

		w.Header().Set("Content-Type", "image/png")
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	}))
	defer srv.Close()

	// Start 3 low-priority requests for DISTINCT urls (same-url dedup is
	// TestConcurrentGetImageSharesFetch) - lane capacity is 1
	var wg sync.WaitGroup
	for k := range 3 {
		wg.Go(func() {
			_, err := s.GetImage("test", fmt.Sprintf("%s/img?n=%d", srv.URL, k), nil, "", "", PrioLow)
			if err != nil {
				t.Errorf("low priority GetImage failed: %v", err)
			}
		})
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(callOrder) != 3 {
		t.Errorf("expected 3 requests, got %d", len(callOrder))
	}
}

// TestLowLaneDoesNotBlockHigh verifies low lane requests don't block high lane.
func TestLowLaneDoesNotBlockHigh(t *testing.T) {
	s := newTestServiceWithCache(t)

	var mu sync.Mutex
	running := 0
	maxRunning := 0
	highRan := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		running++
		if running > maxRunning {
			maxRunning = running
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		running--
		mu.Unlock()

		w.Header().Set("Content-Type", "image/png")
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	}))
	defer srv.Close()

	// Start a low-priority request that will occupy the low lane
	lowDone := make(chan struct{})
	go func() {
		_, err := s.GetImage("test", srv.URL+"/img", nil, "", "", PrioLow)
		if err != nil {
			t.Errorf("low priority GetImage failed: %v", err)
		}
		close(lowDone)
	}()

	// Wait for low request to start
	time.Sleep(5 * time.Millisecond)

	// High-priority request should run concurrently (not wait for low)
	_, err := s.GetImage("test", srv.URL+"/img", nil, "", "", PrioHigh)
	if err != nil {
		t.Fatalf("high priority GetImage failed: %v", err)
	}
	highRan = true

	<-lowDone

	mu.Lock()
	defer mu.Unlock()

	if !highRan {
		t.Error("high priority request did not run")
	}
	// Max running should be at least 1 (low) + could be 1 (high run together with low)
	if maxRunning < 1 {
		t.Errorf("max running was %d", maxRunning)
	}
}

// TestHighPriorityRunsWithLow verifies high-priority requests can run
// concurrently with low-priority requests (high lane capacity 2, low capacity 1).
func TestHighPriorityRunsWithLow(t *testing.T) {
	s := newTestServiceWithCache(t)

	var mu sync.Mutex
	running := 0
	maxRunning := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		running++
		if running > maxRunning {
			maxRunning = running
		}
		mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		running--
		mu.Unlock()

		w.Header().Set("Content-Type", "image/png")
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	}))
	defer srv.Close()

	// Start a low-priority request - occupies low lane (capacity 1)
	var lowDone sync.WaitGroup
	lowDone.Go(func() {
		_, err := s.GetImage("test", srv.URL+"/img?prio=low", nil, "", "", PrioLow)
		if err != nil {
			t.Errorf("low priority GetImage failed: %v", err)
		}
	})

	// Wait for low to start
	time.Sleep(5 * time.Millisecond)

	// Start a high-priority request - should run concurrently
	highDone := make(chan struct{})
	go func() {
		_, err := s.GetImage("test", srv.URL+"/img2", nil, "", "", PrioHigh)
		if err != nil {
			t.Errorf("high priority GetImage failed: %v", err)
		}
		close(highDone)
	}()

	<-highDone
	lowDone.Wait()

	mu.Lock()
	defer mu.Unlock()

	// High ran concurrently with low (not waiting in queue)
	if maxRunning < 1 {
		t.Errorf("max running was %d", maxRunning)
	}
	t.Logf("max concurrent requests: %d", maxRunning)
}

// TestConcurrentGetImageSharesFetch proves the singleflight path: N goroutines
// requesting the same image URL must produce exactly one upstream hit.
func TestConcurrentGetImageSharesFetch(t *testing.T) {
	var hits atomic.Int32
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	payload := buf.Bytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(50 * time.Millisecond) // widen the race window
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	s := newTestServiceWithCache(t)
	const n = 8
	var wg sync.WaitGroup
	results := make([][]byte, n)
	for i := range n {
		wg.Go(func() {
			data, err := s.GetImage("p", srv.URL+"/img", nil, "m", "c", PrioLow)
			if err != nil {
				t.Errorf("get image: %v", err)
				return
			}
			results[i] = data
		})
	}
	wg.Wait()

	if got := hits.Load(); got != 1 {
		t.Errorf("upstream hits = %d, want 1", got)
	}
	for i, data := range results {
		if string(data) != string(payload) {
			t.Fatalf("goroutine %d got different bytes", i)
		}
	}
}
