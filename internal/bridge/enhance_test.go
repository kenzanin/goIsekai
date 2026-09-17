package bridge

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// greyPage returns a w x h image whose channels are equal: a black-and-white
// scan as far as the colour detector is concerned.
func greyPage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8((x + y) % 200)
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

// nearGreyPage keeps a two-level channel spread, the noise floor real scans sit
// at. It must still count as grey.
func nearGreyPage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8((x + y) % 200)
			img.Set(x, y, color.RGBA{v, v + 1, v + 2, 255})
		}
	}
	return img
}

// patchPage is a grey page with a size x size block of saturated red in its
// corner: the "one colour page per chapter" case, and the only reason the
// detector needs a second test beyond the mean spread.
func patchPage(w, h, size int) *image.RGBA {
	img := greyPage(w, h)
	for y := range size {
		for x := range size {
			img.Set(x, y, color.RGBA{220, 30, 30, 255})
		}
	}
	return img
}

func solidPage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestIsColourPage(t *testing.T) {
	for _, tc := range []struct {
		name string
		img  image.Image
		want bool
	}{
		{"grey ramp", greyPage(96, 96), false},
		{"near-grey noise", nearGreyPage(96, 96), false},
		{"saturated colour", solidPage(96, 96, color.RGBA{200, 40, 40, 255}), true},
		{"half grey half colour", solidPage(96, 96, color.RGBA{255, 0, 255, 255}), true},
		{"grey with a colour patch", patchPage(96, 96, 12), true},
		{"grey with a colour speck", patchPage(96, 96, 4), false},
		{"empty", image.NewRGBA(image.Rect(0, 0, 0, 0)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isColourPage(tc.img); got != tc.want {
				t.Errorf("isColourPage = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSoftLevels(t *testing.T) {
	pixel := func(v uint8) *image.Gray {
		g := image.NewGray(image.Rect(0, 0, 1, 1))
		g.SetGray(0, 0, color.Gray{Y: v})
		return g
	}
	for _, tc := range []struct {
		in   uint8
		want uint8
	}{
		{0, 0},  // deep ink
		{60, 0}, // lo is black, inclusive
		{61, 1}, // just above lo starts climbing
		{125, 127},
		{189, 253},
		{190, 255}, // hi is white, inclusive
		{255, 255},
	} {
		got := softLevels(pixel(tc.in), 60, 190).GrayAt(0, 0).Y
		if got != tc.want {
			t.Errorf("softLevels(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestSoftLevelsKeepsMidtones is the whole point of soft levels over a hard
// threshold: the range between paper and ink has to survive. A threshold would
// collapse every input to the same two values.
func TestSoftLevelsKeepsMidtones(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 256, 1))
	for x := range 256 {
		src.SetGray(x, 0, color.Gray{Y: uint8(x)})
	}
	got := softLevels(src, 60, 190)
	var black, white, mid int
	for x := range 256 {
		switch v := got.GrayAt(x, 0).Y; {
		case v == 0:
			black++
		case v == 255:
			white++
		default:
			mid++
		}
	}
	if black == 0 || white == 0 {
		t.Errorf("levels did not reach both ends: black=%d white=%d", black, white)
	}
	if mid < 100 {
		t.Errorf("only %d of 256 levels survived between black and white", mid)
	}
}

func TestEnhanceScanIsGreyscaleAndSatBounded(t *testing.T) {
	got := enhanceScan(greyPage(120, 160))
	if got.Bounds().Dx() != 120 || got.Bounds().Dy() != 160 {
		t.Fatalf("bounds changed: %v", got.Bounds())
	}
	for y := range 160 {
		for x := range 120 {
			r, g, b, _ := got.At(x, y).RGBA()
			if r != g || g != b {
				t.Fatalf("enhanceScan left colour at (%d,%d): %d,%d,%d", x, y, r, g, b)
			}
		}
	}
}

func TestEnhanceConfigModeFor(t *testing.T) {
	cfg := enhanceConfig{
		defaultMode: EnhanceAuto,
		byPlugin:    map[string]EnhanceMode{"mangadex": EnhanceOff},
	}
	if got := cfg.modeFor("mangadex"); got != EnhanceOff {
		t.Errorf("mangadex = %q, want off", got)
	}
	if got := cfg.modeFor("kaliscan"); got != EnhanceAuto {
		t.Errorf("kaliscan = %q, want the default auto", got)
	}
	empty := enhanceConfig{}
	if got := empty.modeFor("kaliscan"); got != "" {
		t.Errorf("unset default = %q, want the zero mode", got)
	}
}

func writeINI(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "goisekai.ini")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadEnhanceConfig(t *testing.T) {
	got := loadEnhanceConfig(writeINI(t, "[enhance]\ndefault = off\nmangadex = auto\n"))
	if got.defaultMode != EnhanceOff {
		t.Errorf("default = %q, want off", got.defaultMode)
	}
	if got.byPlugin["mangadex"] != EnhanceAuto {
		t.Errorf("mangadex = %q, want auto", got.byPlugin["mangadex"])
	}
	// A typo must not become a third mode; the line is dropped and the plugin
	// keeps the global setting.
	mode := loadEnhanceConfig(writeINI(t, "[enhance]\ndefault = auto\nmangadex = maybe\n"))
	if _, ok := mode.byPlugin["mangadex"]; ok {
		t.Error("an unrecognized mode should be ignored")
	}
	if mode.modeFor("mangadex") != EnhanceAuto {
		t.Error("an ignored override should fall back to the default")
	}
	// A missing INI is the first-run state, so it follows the defaults.
	if missing := loadEnhanceConfig(filepath.Join(t.TempDir(), "missing.ini")); missing.modeFor("x") != EnhanceAuto {
		t.Error("a missing config should use the default mode")
	}
}

// TestEncodeForCacheSkipsColourPage pins the promise made for colour pages:
// with enhancement on they are byte-identical to what the plain path produces.
func TestEncodeForCacheSkipsColourPage(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, solidPage(300, 400, color.RGBA{30, 200, 90, 255}), nil); err != nil {
		t.Fatal(err)
	}
	src := buf.Bytes()
	plain, _ := encodeForCache(src, FormatWebP, false, 720, false, nil)
	enhanced, _ := encodeForCache(src, FormatWebP, false, 720, true, nil)
	if !bytes.Equal(plain, enhanced) {
		t.Error("a colour page must not be touched by enhancement")
	}
}

func TestEncodeForCacheEnhancesGreyPage(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, greyPage(300, 400), nil); err != nil {
		t.Fatal(err)
	}
	src := buf.Bytes()
	plain, _ := encodeForCache(src, FormatWebP, false, 720, false, nil)
	enhanced, _ := encodeForCache(src, FormatWebP, false, 720, true, nil)
	if bytes.Equal(plain, enhanced) {
		t.Fatal("a grey page was not enhanced")
	}
	img, _, err := image.Decode(bytes.NewReader(enhanced))
	if err != nil {
		t.Fatalf("enhanced page does not decode: %v", err)
	}
	r, g, b, _ := img.At(10, 10).RGBA()
	if r != g || g != b {
		t.Errorf("enhanced page is not greyscale: %d,%d,%d", r, g, b)
	}
}

// TestEncodeForCacheStats checks the timings the debug log reports: the two
// stages have to be distinguishable, or the log line is worse than useless.
// Reading enhance is how you tell "skipped" from "ran and cost nothing".
func TestEncodeForCacheStats(t *testing.T) {
	grey := jpegBytes(t, greyPage(400, 600))
	colour := jpegBytes(t, solidPage(400, 600, color.RGBA{30, 200, 90, 255}))
	for _, tc := range []struct {
		name         string
		src          []byte
		enhance      bool
		wantEnhanced bool
	}{
		{"grey page", grey, true, true},
		{"colour page", colour, true, false},
		{"enhancement off", grey, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stats encodeStats
			if _, converted := encodeForCache(tc.src, FormatWebP, false, 720, tc.enhance, &stats); !converted {
				t.Fatal("expected a converted result")
			}
			if stats.encode <= 0 {
				t.Error("encode stage was not timed")
			}
			if gotEnhanced := stats.enhance > 0; gotEnhanced != tc.wantEnhanced {
				t.Errorf("enhance stage timed = %v (%v), want %v", gotEnhanced, stats.enhance, tc.wantEnhanced)
			}
		})
	}
}
