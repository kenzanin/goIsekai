package bridge

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// GetImage probes the cache twice: before joining the singleflight, and again
// once it owns the entry. These cover the probe itself, which the second call
// depends on - it must be re-runnable, must promote a disk hit into L1, and must
// treat an undecodable file as a miss while removing it.
func TestCachedImagePromotesDiskHitAndRejectsGarbage(t *testing.T) {
	s := newTestServiceWithCache(t)
	url := "http://cdn.example.com/probe/1.png"
	base := s.diskCachePath("plugin-x", "m1", "c1", url)
	if base == "" {
		t.Fatal("no disk cache path for the probe")
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, ok := s.cachedImage("plugin-x", "m1", "c1", url); ok {
		t.Fatal("an empty cache must report a miss")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	payload := buf.Bytes()
	if err := os.WriteFile(base+".img", payload, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok := s.cachedImage("plugin-x", "m1", "c1", url)
	if !ok {
		t.Fatal("a valid cached file must be a hit")
	}
	if len(got) != len(payload) {
		t.Fatalf("cache returned %d bytes, want %d", len(got), len(payload))
	}
	// A second probe must still hit, and must be served from L1 even with the
	// file removed - that is what makes the probe cheap enough to run twice.
	if err := os.Remove(base + ".img"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := s.cachedImage("plugin-x", "m1", "c1", url); !ok {
		t.Error("the probe is not re-runnable: L1 promotion did not stick")
	}

	// Undecodable bytes are a miss, and the stale file is cleaned up.
	s2 := newTestServiceWithCache(t)
	bad := s2.diskCachePath("plugin-x", "m1", "c1", "http://cdn.example.com/probe/2.png")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(bad+".img", []byte("definitely not an image"), 0o644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	if _, ok := s2.cachedImage("plugin-x", "m1", "c1", "http://cdn.example.com/probe/2.png"); ok {
		t.Error("undecodable cached bytes must be treated as a miss")
	}
	if _, err := os.Stat(bad + ".img"); !os.IsNotExist(err) {
		t.Errorf("the stale file should have been removed, stat err = %v", err)
	}
}
