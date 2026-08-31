package bridge

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
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
	return NewAppService(db, nil, hostnet.NewProxy(), "", t.TempDir())
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
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
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

func TestImageCacheInvalidBytesFailOpen(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte("\x00\x01\x02\x03"),        // too small to sniff/decode
		[]byte("definitely not an image"), // decodable length, but garbage
	} {
		url := serveImage(t, "application/octet-stream", payload)

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
			t.Errorf("invalid bytes changed on disk: got %q, want %q", data, payload)
		}
		if _, err := os.Stat(base + ".webp"); !os.IsNotExist(err) {
			t.Errorf("expected no .webp file for invalid bytes, stat err = %v", err)
		}
	}
}
