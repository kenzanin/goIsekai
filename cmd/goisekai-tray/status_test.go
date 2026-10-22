package main

import (
	"bytes"
	"image/png"
	"testing"
)

func TestStatusIconDiffers(t *testing.T) {
	up, down := statusIcon(true), statusIcon(false)
	if len(up) == 0 || len(down) == 0 {
		t.Fatal("statusIcon returned empty bytes")
	}
	if string(up) == string(down) {
		t.Fatal("up and down icons are identical")
	}
	// systray hands the bytes straight to the desktop, so they must decode as
	// a real PNG and be the advertised size.
	img, err := png.Decode(bytes.NewReader(up))
	if err != nil {
		t.Fatalf("green icon is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 22 || b.Dy() != 22 {
		t.Fatalf("icon is %v, want 22x22", b)
	}
}

func TestHealthyRejectsDeadServer(t *testing.T) {
	// Port 1 is not listening in any test environment.
	if healthy("http://127.0.0.1:1/") {
		t.Fatal("healthy() reported up for a dead port")
	}
}
