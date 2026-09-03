package bridge

import (
	"archive/zip"
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
	defer r.Close()
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

func TestReadCachedImageDiskOnly(t *testing.T) {
	s := newTestServiceWithCache(t)
	url := serveImage(t, "image/png", validPNG(t))
	if _, err := s.GetImage("p", url, nil, "m", "c"); err != nil {
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
