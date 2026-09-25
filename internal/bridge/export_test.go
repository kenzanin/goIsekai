package bridge

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"goisekai/pkg/types"
)

func TestCompleteCSVRoundTrip(t *testing.T) {
	s := newTestServiceWithCache(t)
	pages := []types.Page{
		{URL: "http://cdn.example.com/a/1.png?acc=1"},
		{URL: "http://cdn.example.com/a/2.png?acc=2"},
		{URL: "http://cdn.example.com/a/3.png"},
	}

	if err := s.writeCompleteCSV("p", "m", "c", pages); err != nil {
		t.Fatalf("writeCompleteCSV: %v", err)
	}
	got := s.readCompleteCSV("p", "m", "c")
	if len(got) != 3 || got[0] != pages[0].URL || got[1] != pages[1].URL || got[2] != pages[2].URL {
		t.Fatalf("readCompleteCSV = %v, want %v", got, []string{pages[0].URL, pages[1].URL, pages[2].URL})
	}

	// complete.csv must not count as a cached page.
	if n := s.countCachedPages("p", "m", "c"); n != 0 {
		t.Errorf("countCachedPages counted complete.csv: got %d, want 0", n)
	}
}

func TestImageExtAVIF(t *testing.T) {
	// AVIF files are ISOBMFF: 'ftyp' box at offset 4-7, 'avif' brand at offset 8-11.
	avif := []byte{
		0x00, 0x00, 0x00, 0x20, // box size
		0x66, 0x74, 0x79, 0x70, // 'ftyp'
		0x61, 0x76, 0x69, 0x66, // 'avif' brand
		0x00, 0x00, 0x00, 0x00, // minor version
		0x61, 0x76, 0x69, 0x66, // 'avif' compatible brand
		0x6D, 0x69, 0x66, 0x31, // 'mif1'
		0x00, 0x00, 0x00, 0x00, // filler
	}
	if got := imageExt(avif); got != ".avif" {
		t.Errorf("imageExt(AVIF) = %q, want .avif", got)
	}
}

func TestImageExtJXL(t *testing.T) {
	container := []byte{0x00, 0x00, 0x00, 0x0c, 'J', 'X', 'L', ' ', 0x0d, 0x0a, 0x87, 0x0a}
	if got := imageExt(container); got != ".jxl" {
		t.Errorf("imageExt(JXL container) = %q, want .jxl", got)
	}
	bare := []byte{0xFF, 0x0A, 0x00, 0x00}
	if got := imageExt(bare); got != ".jxl" {
		t.Errorf("imageExt(JXL bare) = %q, want .jxl", got)
	}
}

func TestImageContentType(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"jxl container", []byte{0x00, 0x00, 0x00, 0x0c, 'J', 'X', 'L', ' ', 0x0d, 0x0a, 0x87, 0x0a}, "image/jxl"},
		{"jxl bare", []byte{0xFF, 0x0A}, "image/jxl"},
		{"avif", []byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f'}, "image/avif"},
		{"png", []byte("\x89PNG\r\n\x1a\n"), "image/png"},
	}
	for _, tt := range tests {
		if got := ImageContentType(tt.data); got != tt.want {
			t.Errorf("%s: ImageContentType = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestZipImagesOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.cbz")
	n, err := zipImages(path, [][]byte{
		[]byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"),
		validPNG(t),
		validPNG(t),
	})
	if err != nil {
		t.Fatalf("zipImages: %v", err)
	}
	if n != 3 {
		t.Fatalf("zipImages wrote %d entries, want 3", n)
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open cbz: %v", err)
	}
	defer func() { _ = r.Close() }()
	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	want := []string{"0001.gif", "0002.png", "0003.png"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, names[i], want[i])
		}
	}
}

// TestGetImageReturnsConvertedBytes pins the cold-fetch contract: the bytes
// handed back are the converted ones stored in the cache, not the raw source.
// Serving the source made a first view and its second render two different
// images (raw JPEG vs enhanced AVIF on disk), which exported as .jpg CBZs.
func TestGetImageReturnsConvertedBytes(t *testing.T) {
	url := serveImage(t, "image/png", validPNG(t))
	s := newTestServiceWithFormat(t, FormatWebP)

	got, err := s.GetImage("p", url, nil, "m", "c", PrioLow)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if !isWebP(got) {
		t.Errorf("cold GetImage returned %s bytes, want converted webp", imageExt(got))
	}

	// L1 hit must serve the same converted bytes as the cold fetch.
	got2, err := s.GetImage("p", url, nil, "m", "c", PrioLow)
	if err != nil {
		t.Fatalf("GetImage (L1): %v", err)
	}
	if !bytes.Equal(got, got2) {
		t.Error("L1 hit returned different bytes than the cold fetch")
	}
	_ = os.RemoveAll(s.chapterCacheDir("p", "m", "c"))
}

func TestReadCachedImageDiskOnly(t *testing.T) {
	s := newTestServiceWithCache(t)
	url := serveImage(t, "image/png", validPNG(t))
	if _, err := s.GetImage("p", url, nil, "m", "c", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	data, ok := s.readCachedImage("p", "m", "c", url)
	if !ok || len(data) == 0 {
		t.Fatalf("readCachedImage: ok=%v len=%d, want cached bytes", ok, len(data))
	}

	// A never-fetched URL must miss (no network fallback in readCachedImage).
	if _, ok := s.readCachedImage("p", "m", "c", "http://cdn.example.com/missing.png"); ok {
		t.Fatal("readCachedImage hit for a URL never fetched")
	}
	_ = os.RemoveAll(s.chapterCacheDir("p", "m", "c"))
}

// TestReadCachedImageAVIF verifies readCachedImage finds .avif cache
// files (the enhanced format) and not only .webp/.img.
func TestReadCachedImageAVIF(t *testing.T) {
	s := newTestServiceWithFormat(t, FormatAVIF)
	url := serveImage(t, "image/png", validPNG(t))
	if _, err := s.GetImage("p", url, nil, "m", "c", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	// readCachedImage must find the .avif file.
	data, ok := s.readCachedImage("p", "m", "c", url)
	if !ok || len(data) == 0 {
		t.Fatalf("readCachedImage(AVIF): ok=%v len=%d, want cached .avif bytes", ok, len(data))
	}
	_ = os.RemoveAll(s.chapterCacheDir("p", "m", "c"))
}

// TestReadCachedImageJXL verifies readCachedImage finds .jxl cache files.
func TestReadCachedImageJXL(t *testing.T) {
	s := newTestServiceWithFormat(t, FormatJXL)
	url := serveImage(t, "image/png", validPNG(t))
	if _, err := s.GetImage("p", url, nil, "m", "c", PrioLow); err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	data, ok := s.readCachedImage("p", "m", "c", url)
	if !ok || len(data) == 0 {
		t.Fatalf("readCachedImage(JXL): ok=%v len=%d, want cached .jxl bytes", ok, len(data))
	}
	if !isJXL(data) {
		t.Fatal("cached bytes are not JXL")
	}
	_ = os.RemoveAll(s.chapterCacheDir("p", "m", "c"))
}
