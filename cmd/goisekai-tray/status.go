package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"time"
)

// statusIcon draws a filled dot. Generating it beats shipping two more PNGs
// next to the real app icon: the shape is a circle, and the colour is the only
// thing that varies.
func statusIcon(up bool) []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill := color.RGBA{R: 0xe5, G: 0x39, B: 0x35, A: 0xff} // red
	if up {
		fill = color.RGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff} // green
	}
	const cx, cy, r = 10.5, 10.5, 9.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			// Skip the corners outside the circle so the dot reads as round on
			// panels that do not darken transparent pixels.
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, fill)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// healthy reports whether the server answers its library page. Any answer
// counts as up: a non-200 still means the process is listening, and that is
// what the dot is for.
func healthy(url string) bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}
