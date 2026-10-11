package database

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

// The plugin response cache is the stale-payload trap: a wrong key or a
// missing expiry check serves one manga's chapters to another, or expired
// JSON forever. These pin the key shape, the TTL gate, and the purge paths.
func TestPluginCacheRoundTripAndKeyIsolation(t *testing.T) {
	db := openTestDB(t)

	if err := db.SetCache("p1", "m1", "GetMangaDetail", `{"title":"A"}`, time.Hour); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	got, err := db.GetCache("p1", "m1", "GetMangaDetail")
	if err != nil {
		t.Fatalf("GetCache: %v", err)
	}
	if got != `{"title":"A"}` {
		t.Fatalf("GetCache = %q, want stored payload", got)
	}

	// Same manga, different function must not collide.
	if _, err := db.GetCache("p1", "m1", "GetChapterList"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-function lookup must miss, got %v", err)
	}
	// Same function, different manga must not collide.
	if err := db.SetCache("p1", "m2", "GetMangaDetail", `{"title":"B"}`, time.Hour); err != nil {
		t.Fatalf("SetCache m2: %v", err)
	}
	if got, _ := db.GetCache("p1", "m2", "GetMangaDetail"); got != `{"title":"B"}` {
		t.Fatalf("m2 payload = %q, want B", got)
	}
	if got, _ := db.GetCache("p1", "m1", "GetMangaDetail"); got != `{"title":"A"}` {
		t.Fatalf("m1 payload disturbed = %q, want A", got)
	}
}

func TestPluginCacheExpiryAndOverwrite(t *testing.T) {
	db := openTestDB(t)

	// Overwrite via INSERT OR REPLACE must update the payload in place.
	if err := db.SetCache("p1", "m1", "Fn", "v1", time.Hour); err != nil {
		t.Fatalf("SetCache v1: %v", err)
	}
	if err := db.SetCache("p1", "m1", "Fn", "v2", time.Hour); err != nil {
		t.Fatalf("SetCache v2: %v", err)
	}
	if got, _ := db.GetCache("p1", "m1", "Fn"); got != "v2" {
		t.Fatalf("overwrite = %q, want v2", got)
	}

	// A zero/negative TTL is already expired on arrival.
	if err := db.SetCache("p1", "m9", "Fn", "stale", 0); err != nil {
		t.Fatalf("SetCache stale: %v", err)
	}
	if _, err := db.GetCache("p1", "m9", "Fn"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired entry must miss, got %v", err)
	}

	// CleanExpired reaps it; CacheCount reflects the live rows only.
	n, err := db.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired: %v", err)
	}
	if n < 1 {
		t.Fatalf("CleanExpired reaped %d, want >=1", n)
	}
	c, err := db.CacheCount()
	if err != nil {
		t.Fatalf("CacheCount: %v", err)
	}
	if c != 1 {
		t.Fatalf("CacheCount = %d, want 1 (only the live v2 row)", c)
	}
}

func TestPluginCacheDeleteScopes(t *testing.T) {
	db := openTestDB(t)

	seeds := [][3]string{
		{"p1", "m1", "Fn"}, {"p1", "m2", "Fn"}, {"p2", "m1", "Fn"},
	}
	for _, s := range seeds {
		if err := db.SetCache(s[0], s[1], s[2], "x", time.Hour); err != nil {
			t.Fatalf("SetCache %v: %v", s, err)
		}
	}

	// Per-manga delete leaves the sibling manga and the sibling plugin alone.
	if err := db.DeleteCache("p1", "m1"); err != nil {
		t.Fatalf("DeleteCache: %v", err)
	}
	if _, err := db.GetCache("p1", "m1", "Fn"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted row must miss, got %v", err)
	}
	if _, err := db.GetCache("p1", "m2", "Fn"); err != nil {
		t.Fatalf("sibling manga must survive: %v", err)
	}

	// Per-plugin delete clears the plugin's rows but not other plugins'.
	if err := db.DeleteCacheByPlugin("p1"); err != nil {
		t.Fatalf("DeleteCacheByPlugin: %v", err)
	}
	if _, err := db.GetCache("p1", "m2", "Fn"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("plugin rows must be gone, got %v", err)
	}
	if _, err := db.GetCache("p2", "m1", "Fn"); err != nil {
		t.Fatalf("other plugin must survive: %v", err)
	}
}
