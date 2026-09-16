package bridge

import (
	"bytes"
	"image"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
)

// ImageFormat selects the on-disk encoding used by the L2 image cache. It is
// read from the [app] section of goisekai.ini (image_format) and fixed at
// startup, because changing it invalidates every cached file.
type ImageFormat string

const (
	FormatWebP ImageFormat = "webp"
	FormatAVIF ImageFormat = "avif"
	// FormatOriginal keeps whatever the source sent (gifs, already-small
	// jpegs, undecodable bytes) with no conversion at all.
	FormatOriginal ImageFormat = "original"
)

// discFormatExtension maps an ImageFormat to the file extension its encoded
// bytes are stored under. Both encode to x/image decoders that sniff magic
// bytes, so the extension only matters for on-disk identification.
func (f ImageFormat) extension() string {
	switch f {
	case FormatAVIF:
		return ".avif"
	case FormatOriginal:
		return ".img"
	default:
		return ".webp"
	}
}

// FormatExtension is the exported form used by cache readers that live
// outside the codec path.
func (f ImageFormat) FormatExtension() string { return f.extension() }

// encodeForCache converts data to the configured format, downscaling covers
// (cover is non-empty) so neither side exceeds maxDim. It returns the bytes to
// store and whether they differ from the input.
//
// Fail-open by design: gif input, undecodable bytes, and encode errors all keep
// the original bytes, because a cache that silently drops images is worse than
// one that stores a few large files. A malformed maxDim is ignored the same way.
func encodeForCache(data []byte, format ImageFormat, cover bool, maxDim int) ([]byte, bool) {
	if format == FormatOriginal || bytes.HasPrefix(data, []byte("GIF8")) {
		return data, false
	}
	// Already in the target format headroom check: re-encoding AVIF/WebP loses
	// quality for no size win, so bytes that need no downscale pass straight
	// through. That covers every page image and every cover already under the
	// cap, which is the common case.
	if isAVIF(data) {
		return data, false
	}
	if isWebP(data) {
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		if err == nil && !needsDownscale(cfg.Width, cfg.Height, maxDim) {
			return data, false
		}
	}

	src, _, err := decodeImage(data)
	if err != nil {
		return data, false
	}
	if b := src.Bounds(); cover && needsDownscale(b.Dx(), b.Dy(), maxDim) {
		src = imaging.Fit(src, maxDim, maxDim, imaging.Lanczos)
	}

	var buf bytes.Buffer
	switch format {
	case FormatAVIF:
		// Speed 6 is libavif's balanced preset; covers are small enough that
		// the encode stays well under the fetch it is replacing.
		err = avif.Encode(&buf, src, avif.Options{Quality: 60, Speed: 6})
	default:
		err = webp.Encode(&buf, src, webp.Options{Quality: 85})
	}
	if err != nil {
		return data, false
	}
	return buf.Bytes(), true
}

// needsDownscale reports whether either side exceeds a positive cap.
func needsDownscale(w, h, maxDim int) bool {
	return maxDim > 0 && (w > maxDim || h > maxDim)
}

// decodeImage decodes any format the process registered a decoder for. The
// webp package registers itself with image.RegisterFormat, so a webp source
// reaches imaging.Fit like any other format.
func decodeImage(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

// isAVIF matches an ISO base media file with a brand of avif/avis.
func isAVIF(data []byte) bool {
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return false
	}
	brand := data[8:12]
	return bytes.Equal(brand, []byte("avif")) || bytes.Equal(brand, []byte("avis"))
}
