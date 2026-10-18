package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// mangadexPlaceholderSHA256 is the SHA256 of the deterministic placeholder
// that uploads.mangadex.org serves (HTTP 200, 59480 bytes, 600x642) when a
// foreign Referer is sent. The placeholder is a valid JPEG, so it passes
// validateImageFull; this hash check catches it specifically.
const mangadexPlaceholderSHA256 = "699b7af33aa5af1ee858b31c2434fabf8bc128b520cda11561b6358096efc1da"

// isPlaceholderImage reports whether data is the known MangaDex placeholder.
// The placeholder is served with HTTP 200 and is a valid JPEG, so normal
// image validation cannot reject it. We match by SHA256 hash.
func isPlaceholderImage(data []byte) bool {
	if len(data) != 59480 {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == mangadexPlaceholderSHA256
}
