package bridge

import (
	"bytes"
	"image"

	"github.com/gen2brain/webp"
)

// validateImageFast performs cheap header-only validation. GIFs pass via magic
// prefix (matching the passthrough path); everything else is checked with
// image.DecodeConfig which reads only the image header (no pixel decode).
func validateImageFast(data []byte) bool {
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("GIF8")) {
		return true
	}
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	return err == nil
}

// validateImageFull performs a full decode: GIF/PNG magic trusted, RIFF/WebP
// via webp.Decode, everything else via image.Decode. Used at trust boundaries
// (network fetch) to reject corrupt data before caching.
func validateImageFull(data []byte) bool {
	if len(data) >= 4 && (bytes.HasPrefix(data, []byte("GIF8")) || bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))) {
		return true
	}
	if bytes.HasPrefix(data, []byte("RIFF")) {
		_, err := webp.Decode(bytes.NewReader(data))
		return err == nil
	}
	_, _, err := image.Decode(bytes.NewReader(data))
	return err == nil
}
