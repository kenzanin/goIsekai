package bridge

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jxl"
	"github.com/gen2brain/webp"
)

// testImgSmall creates a small test image with colored pixels.
func testImgSmall(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	return img
}

// validGIFSource returns bytes of a minimal 4x4 GIF.
func validGIFSource(t *testing.T) []byte {
	t.Helper()
	img := testImgSmall(4, 4)
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif.Encode: %v", err)
	}
	return buf.Bytes()
}

// validWebPSource returns bytes of a WebP-encoded image.
func validWebPSource(t *testing.T, w, h int) []byte {
	t.Helper()
	img := testImgSmall(w, h)
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: 85}); err != nil {
		t.Fatalf("webp.Encode: %v", err)
	}
	return buf.Bytes()
}

// validAVIFSource returns bytes of an AVIF-encoded image.
func validAVIFSource(t *testing.T, w, h int) []byte {
	t.Helper()
	img := testImgSmall(w, h)
	var buf bytes.Buffer
	if err := avif.Encode(&buf, img, avif.Options{Quality: 60, Speed: 6}); err != nil {
		t.Fatalf("avif.Encode: %v", err)
	}
	return buf.Bytes()
}

// validJXLSource returns bytes of a JXL-encoded image.
func validJXLSource(t *testing.T, w, h int) []byte {
	t.Helper()
	img := testImgSmall(w, h)
	var buf bytes.Buffer
	if err := jxl.Encode(&buf, img, jxl.EncodeOptions{Quality: 85, Effort: 4}); err != nil {
		t.Fatalf("jxl.Encode: %v", err)
	}
	return buf.Bytes()
}

// TestImageExtAllSourceFormats verifies imageExt correctly identifies all
// source formats by their magic bytes.
func TestImageExtAllSourceFormats(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"jpeg", validJPEG(t, 8, 8), ".jpg"}, // JPEG uses .jpg extension in imageExt
		{"png", validPNG(t), ".png"},
		{"gif", validGIFSource(t), ".gif"},
		{"webp", validWebPSource(t, 8, 8), ".webp"},
		{"avif", validAVIFSource(t, 8, 8), ".avif"},
		{"jxl", validJXLSource(t, 8, 8), ".jxl"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := imageExt(tc.data)
			if got != tc.want {
				t.Errorf("imageExt() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestValidateImageFastAllSourceFormats verifies validateImageFast accepts
// all standard source formats.
func TestValidateImageFastAllSourceFormats(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"jpeg", validJPEG(t, 8, 8)},
		{"png", validPNG(t)},
		{"gif", validGIFSource(t)},
		{"webp", validWebPSource(t, 8, 8)},
		{"avif", validAVIFSource(t, 8, 8)},
		{"jxl", validJXLSource(t, 8, 8)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !validateImageFast(tc.data) {
				t.Errorf("validateImageFast() rejected %s", tc.name)
			}
		})
	}
}

// TestValidateImageFullAllSourceFormats verifies validateImageFull accepts
// all standard source formats.
func TestValidateImageFullAllSourceFormats(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"jpeg", validJPEG(t, 8, 8)},
		{"png", validPNG(t)},
		{"gif", validGIFSource(t)},
		{"webp", validWebPSource(t, 8, 8)},
		{"avif", validAVIFSource(t, 8, 8)},
		{"jxl", validJXLSource(t, 8, 8)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !validateImageFull(tc.data) {
				t.Errorf("validateImageFull() rejected %s", tc.name)
			}
		})
	}
}

// TestEncodeForCacheSourceFormatMatrix verifies encodeForCache behavior
// when given each source format as input. It tests:
//   - imageExt(out) matches the configured format extension
//   - validateImageFast(out) succeeds for converted output
//   - GIF source passes through unchanged (GIF is PASSTHROUGH in imgcodec)
//   - FormatOriginal preserves source bytes
//   - AVIF/JXL source pass through (no re-encode per design)
//   - WebP source may pass through if no downscale needed
//
// Note: The source format determines if passthrough occurs:
//   - GIF: always passthrough
//   - AVIF: always passthrough (line 103-105 in imgcodec.go)
//   - JXL: always passthrough (line 103-105 in imgcodec.go)
//   - WebP: passthrough only if no downscale needed (covers only)
//   - JPEG/PNG: always converted to target format
func TestEncodeForCacheSourceFormatMatrix(t *testing.T) {
	t.Parallel()

	type srcFormat struct {
		name string
		data []byte
		ext  string
	}
	sourceFormats := []srcFormat{
		{"jpeg", validJPEG(t, 8, 8), ".jpeg"},
		{"png", validPNG(t), ".png"},
		{"gif", validGIFSource(t), ".gif"},
		{"webp", validWebPSource(t, 8, 8), ".webp"},
		{"avif", validAVIFSource(t, 8, 8), ".avif"},
		{"jxl", validJXLSource(t, 8, 8), ".jxl"},
	}

	for _, tgtFormat := range []ImageFormat{FormatWebP, FormatAVIF, FormatJXL, FormatOriginal} {
		tgtName := string(tgtFormat)
		t.Run(tgtName, func(t *testing.T) {
			var wantExt string
			var tgtSniff func([]byte) bool

			switch tgtFormat {
			case FormatWebP:
				wantExt = ".webp"
				tgtSniff = isWebP
			case FormatAVIF:
				wantExt = ".avif"
				tgtSniff = isAVIF
			case FormatJXL:
				wantExt = ".jxl"
				tgtSniff = isJXL
			case FormatOriginal:
				wantExt = ".img"
				tgtSniff = nil
			}

			for _, src := range sourceFormats {
				name := src.name + "_to_" + tgtName
				t.Run(name, func(t *testing.T) {
					got, converted := encodeForCache(src.data, tgtFormat, false, 720, false, nil)

					// FormatOriginal: bytes should be unchanged for all sources
					if tgtFormat == FormatOriginal {
						if converted {
							t.Error("original format: expected converted=false")
						}
						if !bytes.Equal(got, src.data) {
							t.Error("original format: bytes changed")
						}
						return
					}

					// GIF source: should pass through unchanged
					if src.ext == ".gif" {
						if converted {
							t.Error("gif source: expected passthrough (converted=false)")
						}
						if !bytes.Equal(got, src.data) {
							t.Error("gif source: bytes changed during passthrough")
						}
						if gotExt := imageExt(got); gotExt != ".gif" {
							t.Errorf("gif source imageExt = %q, want .gif", gotExt)
						}
						return
					}

					// AVIF source: always passthrough
					if src.ext == ".avif" {
						if converted {
							t.Error("avif source: expected passthrough (converted=false)")
						}
						if !bytes.Equal(got, src.data) {
							t.Error("avif source: bytes changed during passthrough")
						}
						return
					}

					// JXL source: always passthrough
					if src.ext == ".jxl" {
						if converted {
							t.Error("jxl source: expected passthrough (converted=false)")
						}
						if !bytes.Equal(got, src.data) {
							t.Error("jxl source: bytes changed during passthrough")
						}
						return
					}

					// WebP source: may pass through if no downscale needed
					// For 8x8 image with maxDim=720, no downscale needed
					if src.ext == ".webp" {
						// WebP may pass through or convert - both are valid
						// Just verify whatever we got is valid
						if !validateImageFast(got) {
							t.Errorf("webp <- webp: output failed validateImageFast")
						}
						return
					}

					// JPEG and PNG should be converted
					if !converted {
						t.Errorf("%s <- %s: expected conversion to %s", tgtName, src.name, tgtName)
					}

					if !validateImageFast(got) {
						t.Errorf("%s <- %s: output failed validateImageFast", tgtName, src.name)
					}

					if ext := imageExt(got); ext != wantExt {
						t.Errorf("%s <- %s: imageExt = %q, want %q", tgtName, src.name, ext, wantExt)
					}

					if tgtSniff != nil && !tgtSniff(got) {
						t.Errorf("%s <- %s: output failed %s sniff", tgtName, src.name, tgtName)
					}
				})
			}
		})
	}
}

// TestEncodeForCacheDoesNotReEncodeSameSourceFormat verifies that
// source images already in the target format pass through unchanged.
// NOTE: Only GIF, AVIF, JXL, and WebP sources pass through; JPEG and PNG
// are always converted to the target format.
func TestEncodeForCacheDoesNotReEncodeSameSourceFormat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source []byte
		ext    string
		format ImageFormat
	}{
		// GIF always passes through
		{"gif_to_webp", validGIFSource(t), ".gif", FormatWebP},
		{"gif_to_avif", validGIFSource(t), ".gif", FormatAVIF},
		{"gif_to_jxl", validGIFSource(t), ".gif", FormatJXL},
		// WebP passes through when no downscale needed
		{"webp_to_webp", validWebPSource(t, 8, 8), ".webp", FormatWebP},
		// AVIF passes through
		{"avif_to_avif", validAVIFSource(t, 8, 8), ".avif", FormatAVIF},
		// JXL passes through
		{"jxl_to_jxl", validJXLSource(t, 8, 8), ".jxl", FormatJXL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, didConvert := encodeForCache(tc.source, tc.format, false, 720, false, nil)
			if didConvert {
				t.Errorf("source already %s: unexpected conversion", tc.ext)
			}
			if !bytes.Equal(got, tc.source) {
				t.Errorf("source already %s: bytes changed during passthrough", tc.ext)
			}
		})
	}
}
