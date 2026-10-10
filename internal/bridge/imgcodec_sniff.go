package bridge

import (
	"bytes"
	"image"

	"github.com/anthonynsimon/bild/transform"
)

// isTargetFormat reports whether data is already encoded in format, so a
// conversion that would change nothing can be skipped.
func isTargetFormat(data []byte, format ImageFormat) bool {
	switch format {
	case FormatAVIF:
		return isAVIF(data)
	case FormatJXL:
		return isJXL(data)
	case FormatWebP:
		return isWebP(data)
	default:
		return false
	}
}

// fitWithin scales the image down so neither side exceeds maxDim, keeping the
// aspect ratio and never upscaling — the bild equivalent of imaging.Fit.
func fitWithin(src image.Image, maxDim int) image.Image {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}
	if w > h {
		h = max(1, h*maxDim/w) // truncation, imaging.Fit-compatible
		w = maxDim
	} else {
		w = max(1, w*maxDim/h)
		h = maxDim
	}
	return transform.Resize(src, w, h, transform.Lanczos)
}

// needsDownscale reports whether either side exceeds a positive cap.
func needsDownscale(w, h, maxDim int) bool {
	return maxDim > 0 && (w > maxDim || h > maxDim)
}

// decodeImage decodes any format the process registered a decoder for. The webp
// and jxl packages register themselves with image.RegisterFormat, so a source
// in either reaches fitWithin like any other format.
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

// isJXL matches both JPEG XL signatures: the bare codestream and the container.
func isJXL(data []byte) bool {
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0x0a {
		return true
	}
	return bytes.HasPrefix(data, []byte("\x00\x00\x00\x0cJXL \x0d\x0a\x87\x0a"))
}
