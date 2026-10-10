package bridge

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jxl"
	"github.com/gen2brain/webp"
)

// ImageFormat selects the on-disk encoding used by the L2 image cache. It is
// read from the [app] section of goisekai.ini (image_format) and fixed at
// startup, because changing it invalidates every cached file.
type ImageFormat string

const (
	FormatWebP ImageFormat = "webp"
	FormatAVIF ImageFormat = "avif"
	// FormatJXL buys fidelity per byte rather than raw size: on real manga
	// pages it encodes larger than AVIF at this quality. The cache is served
	// straight to the reader's <img>, and JPEG XL is still behind a flag in
	// Chrome and Firefox, so this stays a deliberate opt-in, not the default.
	FormatJXL ImageFormat = "jxl"
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
	case FormatJXL:
		return ".jxl"
	case FormatOriginal:
		return ".img"
	default:
		return ".webp"
	}
}

// FormatExtension is the exported form used by cache readers that live
// outside the codec path.
func (f ImageFormat) FormatExtension() string { return f.extension() }

// ImageContentType returns the MIME type for cached image bytes. It is
// http.DetectContentType plus the formats Go's sniffing table does not know:
// both JPEG XL signatures and AVIF's ftyp box fall through to
// application/octet-stream there, which would serve a reader page as a
// binary download instead of an image.
func ImageContentType(data []byte) string {
	switch {
	case isJXL(data):
		return "image/jxl"
	case isAVIF(data):
		return "image/avif"
	default:
		return http.DetectContentType(data)
	}
}

// encodeStats reports where a cache write spent its time, for the caller's
// debug log. Both fields are zero when the work they time did not happen.
type encodeStats struct {
	// enhance is the cleanup pipeline alone (0 when it was skipped).
	enhance time.Duration
	// encode is everything else: decode, cover downscale, and the re-encode.
	encode time.Duration
	// resized reports that a cover was fitted under maxDim. Covers only;
	// page images are never resized.
	resized bool
	// resizeFrom/resizeTo are the pixel dimensions before and after the fit.
	resizeFrom [2]int
	resizeTo   [2]int
}

// encodeForCache converts data to the configured format, downscaling covers
// (cover is non-empty) so neither side exceeds maxDim, and, when enhance is set,
// rewriting greyscale pages through enhanceScan first. It returns the bytes to
// store and whether they differ from the input.
//
// stats may be nil; when it is not, it is filled with the time each stage took.
//
// Fail-open by design: gif input, undecodable bytes, and encode errors all keep
// the original bytes, because a cache that silently drops images is worse than
// one that stores a few large files. A malformed maxDim is ignored the same way.
func encodeForCache(data []byte, format ImageFormat, cover bool, maxDim int, enhance bool, stats *encodeStats) ([]byte, bool) {
	started := time.Now()
	if format == FormatOriginal || bytes.HasPrefix(data, []byte("GIF8")) {
		return data, false
	}
	// Already in the target format: re-encoding AVIF/WebP/JXL loses quality for
	// no size win, so bytes that need no pixel change pass straight through.
	// That covers every page image and every cover already under the cap, which
	// is the common case.
	//
	// enhance is excluded on purpose: it rewrites greyscale pages, so those must
	// fall through to the pipeline below even when they are already WebP.
	if !enhance && (isAVIF(data) || isJXL(data)) {
		return data, false
	}
	if !enhance && isWebP(data) {
		// maxDim is the cover cap and covers are the only thing downscaled, so a
		// page image never needs the round trip however tall it is. Without this
		// a 1500x2125 page was decoded and re-encoded to a larger file for no
		// reason at all.
		if !cover {
			return data, false
		}
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		if err == nil && !needsDownscale(cfg.Width, cfg.Height, maxDim) {
			return data, false
		}
	}

	src, _, err := decodeImage(data)
	if err != nil {
		return data, false
	}
	changed := false
	if b := src.Bounds(); cover && needsDownscale(b.Dx(), b.Dy(), maxDim) {
		src = fitWithin(src, maxDim)
		changed = true
		if stats != nil {
			stats.resized = true
			stats.resizeFrom = [2]int{b.Dx(), b.Dy()}
			stats.resizeTo = [2]int{src.Bounds().Dx(), src.Bounds().Dy()}
		}
	}
	// Colour pages are never enhanced. Deciding here, before the pipeline runs,
	// means the bytes cannot be touched twice: the original is what gets encoded.
	if enhance && !isColourPage(src) {
		mark := time.Now()
		src = enhanceScan(src)
		changed = true
		if stats != nil {
			stats.enhance = time.Since(mark)
		}
	}
	// Nothing about the pixels changed and the bytes are already in the target
	// format, so re-encoding would only add a generation of loss - in practice
	// it made files larger: a 1500x2125 WebP page came back 29% bigger. Decoding
	// was unavoidable to reach this point under enhance, but encoding is the
	// expensive half and it is the one that gets skipped.
	if !changed && isTargetFormat(data, format) {
		return data, false
	}

	var buf bytes.Buffer
	switch format {
	case FormatAVIF:
		// Speed 6 is libavif's balanced preset; covers are small enough that
		// the encode stays well under the fetch it is replacing.
		err = avif.Encode(&buf, src, avif.Options{Quality: 60, Speed: 6})
	case FormatJXL:
		// Effort 4 of 7: the top of the range spends several times the CPU on
		// a wider block search for a few percent, which a cache write cannot
		// afford on a page-sized image.
		err = jxl.Encode(&buf, src, jxl.EncodeOptions{Quality: 85, Effort: 4})
	default:
		err = webp.Encode(&buf, src, webp.Options{Quality: 85})
	}
	if err != nil {
		return data, false
	}
	if stats != nil {
		stats.encode = time.Since(started) - stats.enhance
	}
	return buf.Bytes(), true
}
