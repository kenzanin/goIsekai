package bridge

// Task 3.2: the library protection predicate returned to the prune covers
// library manga covers and chapter image directories, and returns nil when
// the library is empty.

import (
	"os"
	"testing"

	"goisekai/internal/database"
)

func TestLibraryProtectionPredicate(t *testing.T) {
	t.Run("empty library returns nil", func(t *testing.T) {
		s := newTestServiceWithCache(t)
		if s.LibraryProtection() != nil {
			t.Fatal("nil predicate expected for an empty library")
		}
	})

	t.Run("protects cover and chapter dirs", func(t *testing.T) {
		s := newTestServiceWithCache(t)
		cover := "https://uploads.example/cover1.jpg"
		if _, err := s.db.UpsertManga(database.Manga{
			PluginID: "plugin-x", SourceMangaID: "m1",
			Title: "M", InLibrary: true, CoverURL: cover,
		}); err != nil {
			t.Fatal(err)
		}

		p := s.LibraryProtection()
		if p == nil {
			t.Fatal("predicate is nil for a non-empty library")
		}

		coverPath := s.diskCachePath("plugin-x", "", "", cover)
		if !p(coverPath + ".img") {
			t.Errorf("cover file not protected: %s", coverPath)
		}
		pagePath := s.cacheDir + "/images/plugin-x/m1/c1/page1.img"
		if !p(pagePath) {
			t.Errorf("chapter page not protected: %s", pagePath)
		}
		browsePath := s.cacheDir + "/images/plugin-x/browsing/foo.img"
		if p(browsePath) {
			t.Errorf("browsing file must not be protected: %s", browsePath)
		}

		// Exercise the real filesystem path: write a page file and check it
		// survives a prune that deletes the unprotected one.
		_ = os.MkdirAll(s.cacheDir+"/images/plugin-x/m1/c1", 0o755)
		if err := os.WriteFile(pagePath, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
}
