package bridge

import (
	"bytes"
	"image/jpeg"
	"testing"
)

// ClientDisplayFormat must hand the reader JPEG for a slow-to-decode cache
// format (AVIF) and pass already-cheap bytes through untouched.
func TestClientDisplayFormat(t *testing.T) {
	avifBytes, converted := encodeForCache(validJPEG(t, 400, 600), FormatAVIF, false, 0, false, nil)
	if !converted || !isAVIF(avifBytes) {
		t.Fatalf("fixture: expected converted avif, got converted=%v ext=%s", converted, imageExt(avifBytes))
	}

	got := ClientDisplayFormat(avifBytes)
	if !bytes.HasPrefix(got, []byte("\xff\xd8")) {
		t.Fatalf("AVIF input served as %s, want JPEG (ffd8 prefix)", imageExt(got))
	}
	if _, err := jpeg.DecodeConfig(bytes.NewReader(got)); err != nil {
		t.Fatalf("transcoded bytes not decodable as jpeg: %v", err)
	}

	// Cheap formats pass through: no pointless re-encode.
	jpegIn := validJPEG(t, 100, 100)
	if out := ClientDisplayFormat(jpegIn); !bytes.Equal(out, jpegIn) {
		t.Error("JPEG input was re-encoded, want passthrough")
	}
	webpBytes, _ := encodeForCache(validJPEG(t, 100, 100), FormatWebP, false, 0, false, nil)
	if out := ClientDisplayFormat(webpBytes); !bytes.Equal(out, webpBytes) {
		t.Error("WebP input was re-encoded, want passthrough")
	}
}
