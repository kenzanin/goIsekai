package bridge

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jxl"
	"github.com/gen2brain/webp"
)

func validJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	return jpegOf(t, img)
}

func jpegOf(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestEncodeForCacheCoverDownscales(t *testing.T) {
	// "Fit" scales the longer side down to maxDim and keeps the aspect ratio.
	got, converted := encodeForCache(validJPEG(t, 900, 1400), FormatWebP, true, 720, false, nil)
	if !converted {
		t.Fatal("expected conversion")
	}
	cfg, err := webpConfig(got)
	if err != nil {
		t.Fatalf("decode webp config: %v", err)
	}
	if cfg.Height != 720 || cfg.Width != 462 {
		t.Errorf("dimensions = %dx%d, want 462x720", cfg.Width, cfg.Height)
	}
}

func TestEncodeForCachePageNotResized(t *testing.T) {
	got, _ := encodeForCache(validJPEG(t, 900, 1400), FormatWebP, false, 720, false, nil)
	cfg, err := webpConfig(got)
	if err != nil {
		t.Fatalf("decode webp config: %v", err)
	}
	if cfg.Width != 900 || cfg.Height != 1400 {
		t.Errorf("dimensions = %dx%d, want 900x1400 (pages never resize)", cfg.Width, cfg.Height)
	}
}

func TestEncodeForCacheSmallCoverLeftAlone(t *testing.T) {
	got, _ := encodeForCache(validJPEG(t, 300, 400), FormatWebP, true, 720, false, nil)
	cfg, err := webpConfig(got)
	if err != nil {
		t.Fatalf("decode webp config: %v", err)
	}
	if cfg.Width != 300 || cfg.Height != 400 {
		t.Errorf("dimensions = %dx%d, want 300x400 (already under cap)", cfg.Width, cfg.Height)
	}
}

func TestEncodeForCacheAvif(t *testing.T) {
	got, converted := encodeForCache(validJPEG(t, 900, 1400), FormatAVIF, true, 720, false, nil)
	if !converted {
		t.Fatal("expected conversion to avif")
	}
	if !isAVIF(got) {
		t.Fatalf("output lacks avif ftyp brand: %q", got[:min(16, len(got))])
	}
	cfg, err := avif.DecodeConfig(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode avif config: %v", err)
	}
	if cfg.Width != 462 || cfg.Height != 720 {
		t.Errorf("dimensions = %dx%d, want 462x720", cfg.Width, cfg.Height)
	}
}

func TestEncodeForCacheJXL(t *testing.T) {
	got, converted := encodeForCache(validJPEG(t, 900, 1400), FormatJXL, true, 720, false, nil)
	if !converted {
		t.Fatal("expected conversion to jxl")
	}
	if !isJXL(got) {
		t.Fatalf("output lacks a jxl signature: %q", got[:min(16, len(got))])
	}
	cfg, err := jxl.DecodeConfig(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode jxl config: %v", err)
	}
	if cfg.Width != 462 || cfg.Height != 720 {
		t.Errorf("dimensions = %dx%d, want 462x720", cfg.Width, cfg.Height)
	}
	// The L2 read path validates cached bytes through image.DecodeConfig, which
	// only recognises JXL because the codec registers itself on import. Losing
	// that would make every cached file look corrupt and get deleted.
	if _, _, err := decodeImage(got); err != nil {
		t.Fatalf("cached jxl does not decode: %v", err)
	}
}

func TestEncodeForCacheDoesNotReEncodeJXL(t *testing.T) {
	once, _ := encodeForCache(validJPEG(t, 400, 600), FormatJXL, true, 0, false, nil)
	twice, converted := encodeForCache(once, FormatJXL, true, 0, false, nil)
	if converted {
		t.Error("jxl input under jxl format should not be re-encoded")
	}
	if !bytes.Equal(once, twice) {
		t.Error("re-encoding changed bytes")
	}
}

func TestEncodeForCacheOriginalFormatUntouched(t *testing.T) {
	src := validJPEG(t, 900, 1400)
	got, converted := encodeForCache(src, FormatOriginal, true, 720, false, nil)
	if converted {
		t.Error("original format should never report conversion")
	}
	if !bytes.Equal(got, src) {
		t.Error("original format should return the input bytes unchanged")
	}
}

func TestEncodeForCacheGifAndGarbagePassThrough(t *testing.T) {
	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"gif", gif},
		{"garbage", []byte("definitely not an image")},
		{"avif mode, gif input", gif},
	} {
		format := FormatWebP
		if tc.name == "avif mode, gif input" {
			format = FormatAVIF
		}
		got, converted := encodeForCache(tc.in, format, true, 720, false, nil)
		if converted {
			t.Errorf("%s: reported conversion", tc.name)
		}
		if !bytes.Equal(got, tc.in) {
			t.Errorf("%s: bytes changed", tc.name)
		}
	}
}

func TestEncodeForCacheDoesNotReEncodeSameFormat(t *testing.T) {
	once, _ := encodeForCache(validJPEG(t, 400, 600), FormatWebP, true, 720, false, nil)
	twice, converted := encodeForCache(once, FormatWebP, true, 720, false, nil)
	if converted {
		t.Error("webp input under webp format should not be re-encoded")
	}
	if !bytes.Equal(once, twice) {
		t.Error("re-encoding changed bytes")
	}

	avifOnce, _ := encodeForCache(validJPEG(t, 400, 600), FormatAVIF, true, 0, false, nil)
	avifTwice, converted := encodeForCache(avifOnce, FormatAVIF, true, 0, false, nil)
	if converted || !bytes.Equal(avifOnce, avifTwice) {
		t.Error("avif input under avif format should pass through unchanged")
	}
}

// maxDim is the cover cap and covers are the only thing downscaled, so a page
// image taller than that cap must not be decoded and re-encoded: the round trip
// produces a larger file at lower quality for identical pixels.
func TestEncodeForCacheKeepsPageOverTheCoverCap(t *testing.T) {
	page, converted := encodeForCache(validJPEG(t, 1500, 2125), FormatWebP, false, 720, false, nil)
	if !converted {
		t.Fatal("setup: expected the page to be converted to webp")
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(page))
	if err != nil {
		t.Fatalf("setup: produced bytes are not webp: %v", err)
	}
	if cfg.Width <= 720 && cfg.Height <= 720 {
		t.Fatalf("setup: page %dx%d is under the cap, it cannot exercise this", cfg.Width, cfg.Height)
	}

	got, converted := encodeForCache(page, FormatWebP, false, 720, false, nil)
	if converted {
		t.Error("a page webp over the cover cap was re-encoded")
	}
	if !bytes.Equal(got, page) {
		t.Errorf("page bytes changed for nothing: %d -> %d", len(page), len(got))
	}
}

// The other half of the same guard: a cover still gets fitted under the cap even
// when it is already in the target format.
func TestEncodeForCacheStillFitsAnOversizedCover(t *testing.T) {
	big, converted := encodeForCache(validJPEG(t, 1800, 2800), FormatWebP, false, 720, false, nil)
	if !converted {
		t.Fatal("setup: expected the oversized cover to be converted to webp")
	}
	got, converted := encodeForCache(big, FormatWebP, true, 720, false, nil)
	if !converted {
		t.Error("an oversized cover must still be re-encoded so it can be fitted")
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Width > 720 || cfg.Height > 720 {
		t.Errorf("cover %dx%d was not fitted under the 720 cap", cfg.Width, cfg.Height)
	}
}

// enhance rewrites pixels, so a same-format page must still go through the
// pipeline instead of riding the pass-through added for plain pages.
func TestEncodeForCacheStillEnhancesASameFormatPage(t *testing.T) {
	grey, converted := encodeForCache(jpegOf(t, greyPage(900, 1400)), FormatWebP, false, 720, false, nil)
	if !converted {
		t.Fatal("setup: expected the greyscale page to be converted to webp")
	}

	got, converted := encodeForCache(grey, FormatWebP, false, 720, true, nil)
	if !converted {
		t.Error("enhance was requested but the page passed straight through")
	}
	if bytes.Equal(got, grey) {
		t.Error("enhance ran but produced identical bytes")
	}
}

// enhance defaults to auto, so this is the common path: a colour page already
// in the target format. isColourPage skips the cleanup, which means nothing about
// the pixels changed, so the encode must be skipped too - otherwise every colour
// page pays a generation of loss and comes out larger than it arrived.
func TestEncodeForCacheKeepsAColourPageUnderEnhance(t *testing.T) {
	colour, converted := encodeForCache(validJPEG(t, 1500, 2125), FormatWebP, false, 720, false, nil)
	if !converted {
		t.Fatal("setup: expected the page to be converted to webp")
	}

	got, converted := encodeForCache(colour, FormatWebP, false, 720, true, nil)
	if converted {
		t.Error("a colour page under enhance was re-encoded for nothing")
	}
	if !bytes.Equal(got, colour) {
		t.Errorf("colour page bytes changed: %d -> %d", len(colour), len(got))
	}
}

func TestFormatExtension(t *testing.T) {
	for format, want := range map[ImageFormat]string{
		FormatWebP:     ".webp",
		FormatAVIF:     ".avif",
		FormatJXL:      ".jxl",
		FormatOriginal: ".img",
	} {
		if got := format.extension(); got != want {
			t.Errorf("%s.extension() = %q, want %q", format, got, want)
		}
	}
	// An unknown format falls back to the legacy extension.
	if got := ImageFormat("garbage").extension(); got != ".webp" {
		t.Errorf("unknown extension() = %q, want .webp", got)
	}
}

// TestLoadImageFormat guards the .ini handshake: a format the switch does not
// name is silently ignored and the cache falls back to webp, so a new value has
// to be registered here as well as in the codec.
func TestLoadImageFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		ini  string
		want ImageFormat
	}{
		{"jxl", "image_format = jxl\n", FormatJXL},
		{"avif", "image_format = avif\n", FormatAVIF},
		{"webp", "image_format = webp\n", FormatWebP},
		{"original", "image_format = original\n", FormatOriginal},
		{"unknown falls back to webp", "image_format = heif\n", FormatWebP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "goisekai.ini")
			if err := os.WriteFile(path, []byte("[app]\n"+tc.ini), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := loadImageFormat(path); got != tc.want {
				t.Errorf("loadImageFormat = %q, want %q", got, tc.want)
			}
		})
	}
	if got := loadImageFormat(filepath.Join(t.TempDir(), "missing.ini")); got != FormatWebP {
		t.Errorf("missing config = %q, want webp", got)
	}
}

// webpConfig reads the dimensions back out of encoded webp bytes.
func webpConfig(data []byte) (image.Config, error) {
	return webp.DecodeConfig(bytes.NewReader(data))
}
