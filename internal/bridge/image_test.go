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
	"path/filepath"
	"testing"

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
	if _, err := s.GetImage("plugin-x", url, nil, "", ""); err != nil {
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
	if _, err := s.GetImage("plugin-x", url, nil, "", ""); err != nil {
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
		if _, err := s.GetImage("plugin-x", url, nil, "", ""); err == nil {
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
	if _, err := s.GetImage("plugin-x", validURL, nil, "", ""); err != nil {
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
			if _, err := s.GetImage("plugin-x", url, nil, "", ""); err != nil {
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
	if _, err := s.GetImage("plugin-x", url, nil, "", ""); err != nil {
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
			if _, err := s.GetImage(tc.pluginID, url, nil, tc.mangaID, tc.chapterID); err != nil {
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
