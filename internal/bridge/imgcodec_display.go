package bridge

import (
	"bytes"
	"image/jpeg"

	"goisekai/internal/logger"
)

// ClientDisplayFormat re-encodes bytes the reader is about to render into a
// format WebKit decodes cheaply.
//
// The L2 cache stores pages in the configured format (AVIF by default), which
// is compact on disk and fast to decode in Chrome/Firefox — but WebKitGTK
// decodes AVIF on the main thread, and the continuous reader stacks every page
// as a lazy <img>. One AVIF decode per page entering the viewport is enough to
// stutter the strip scroll there. JPEG decodes fast in every engine, so the
// /image proxy hands the reader JPEG and keeps AVIF as the on-disk archive.
//
// Only slow-to-decode formats are rewritten: AVIF and JXL. WebP passes through
// (already cheap to decode and the common cache hit), and any encode failure
// falls open and returns the input untouched.
func ClientDisplayFormat(data []byte) []byte {
	if !isAVIF(data) && !isJXL(data) {
		return data
	}
	src, _, err := decodeImage(data)
	if err != nil {
		logger.Debug("display transcode: decode failed, serving original", "error", err)
		return data
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 85}); err != nil {
		logger.Debug("display transcode: jpeg encode failed, serving original", "error", err)
		return data
	}
	return buf.Bytes()
}
