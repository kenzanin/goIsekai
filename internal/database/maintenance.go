package database

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// BackupTo writes a consistent snapshot of the database into dir using
// SQLite's VACUUM INTO (safe while the live DB is in WAL mode and in use),
// then prunes older backups beyond keep. Returns the backup file path.
func (d *DB) BackupTo(dir string, keep int) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Milliseconds in the name: VACUUM INTO fails if the output exists, and
	// two backups can legitimately land in the same second (startup + test).
	name := filepath.Join(dir, "goisekai-"+time.Now().Format("2006-01-02_150405.000")+".db")
	// VACUUM INTO produces a defragmented, fully-checkpointed copy.
	if _, err := d.db.Exec("VACUUM INTO ?", name); err != nil {
		return "", fmt.Errorf("vacuum into %s: %w", name, err)
	}
	d.pruneBackups(dir, keep)
	return name, nil
}

// pruneBackups deletes the oldest backup files, keeping only the newest keep.
func (d *DB) pruneBackups(dir string, keep int) {
	if keep <= 0 {
		keep = 1
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	// Names sort chronologically because of the timestamp layout.
	slices.Sort(files)
	for i := 0; i < len(files)-keep; i++ {
		_ = os.Remove(files[i])
	}
}

// PruneOrphans deletes rows that reference data which no longer exists, plus
// the storage the purge policy no longer requires: chapters of non-library
// manga, duplicate read-history rows (newest kept), and cached pages of
// finished chapters. It returns a human-readable summary of what was removed.
func (d *DB) PruneOrphans() (string, error) {
	var b strings.Builder
	run := func(label, query string) {
		res, err := d.db.Exec(query)
		if err != nil {
			fmt.Fprintf(&b, "%s: error %v; ", label, err)
			return
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			fmt.Fprintf(&b, "%s=%d ", label, n)
		}
	}

	// Order matters: children first so we never delete a parent still referenced.
	run("orphan_chapter_pages", `DELETE FROM chapter_pages WHERE chapter_id NOT IN (SELECT id FROM chapters)`)
	run("orphan_read_history", `DELETE FROM read_history WHERE chapter_id NOT IN (SELECT id FROM chapters)`)
	run("orphan_chapters", `DELETE FROM chapters WHERE manga_id NOT IN (SELECT id FROM mangas)`)

	// Same purge policy the storage migration applies once: only library manga
	// keep chapters, a chapter keeps only its newest history row, and pages of
	// finished chapters are dropped (the reader re-fetches them on the next open).
	run("non_library_chapters", `DELETE FROM chapters WHERE manga_id IN (SELECT id FROM mangas WHERE in_library = 0)`)
	run("read_chapter_pages", `DELETE FROM chapter_pages WHERE chapter_id IN (SELECT id FROM chapters WHERE is_read = 1)`)
	run("duplicate_read_history", `DELETE FROM read_history WHERE id NOT IN (SELECT MAX(id) FROM read_history GROUP BY chapter_id)`)

	run("orphan_alt_titles", `DELETE FROM alt_titles WHERE manga_row_id NOT IN (SELECT id FROM mangas)`)
	run("orphan_alt_covers", `DELETE FROM alt_covers WHERE manga_row_id NOT IN (SELECT id FROM mangas)`)
	run("non_library_no_chapters", `DELETE FROM mangas WHERE in_library = 0 AND id NOT IN (SELECT DISTINCT manga_id FROM chapters)`)
	run("orphan_fts", `DELETE FROM library_fts WHERE CAST(manga_row_id AS TEXT) NOT IN (SELECT CAST(id AS TEXT) FROM mangas)`)

	if b.Len() == 0 {
		return "clean", nil
	}
	return strings.TrimSpace(b.String()), nil
}

// PurgeRemovedPlugin deletes every row belonging to one plugin: its manga
// (chapters, pages, history and enrichment data follow), its response cache,
// its verify cookies, its FTS documents and finally the plugins row itself.
// It is meant for plugins the user has removed from disk — a removed plugin
// normally only flips to is_active=0 because reading history still names it.
// Returns a human-readable summary of what was removed.
func (d *DB) PurgeRemovedPlugin(pluginID string) (string, error) {
	if pluginID == "" {
		return "", fmt.Errorf("purge: empty plugin id")
	}
	var b strings.Builder
	run := func(label, query string, args ...any) {
		res, err := d.db.Exec(query, args...)
		if err != nil {
			fmt.Fprintf(&b, "%s: error %v; ", label, err)
			return
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			fmt.Fprintf(&b, "%s=%d ", label, n)
		}
	}

	// Children before parents: manga-linked rows go first so nothing is left
	// pointing at a manga that is already gone.
	// plugin_id binds as TEXT against a TEXT column, so the FTS delete matches
	// (an integer bind against these columns silently matches zero rows).
	run("fts", `DELETE FROM library_fts WHERE plugin_id = ?`, pluginID)
	run("alt_titles", `DELETE FROM alt_titles WHERE manga_row_id IN (SELECT id FROM mangas WHERE plugin_id = ?)`, pluginID)
	run("alt_covers", `DELETE FROM alt_covers WHERE manga_row_id IN (SELECT id FROM mangas WHERE plugin_id = ?)`, pluginID)
	// mangas cascades chapters -> chapter_pages/read_history plus the
	// alt_descriptions/manga_categories/manga_related tables that carry an
	// ON DELETE CASCADE to it.
	run("mangas", `DELETE FROM mangas WHERE plugin_id = ?`, pluginID)
	run("plugin_cache", `DELETE FROM plugin_cache WHERE plugin_id = ?`, pluginID)
	run("plugin_verify", `DELETE FROM plugin_verify WHERE plugin_id = ?`, pluginID)
	run("plugin", `DELETE FROM plugins WHERE id = ?`, pluginID)

	if b.Len() == 0 {
		return "clean", nil
	}
	return strings.TrimSpace(b.String()), nil
}

// Vacuum reclaims the pages freed by the purge deletes. It cannot run inside a
// transaction and holds the write lock for the duration, so callers should run
// it once at the end of a maintenance action rather than per plugin.
func (d *DB) Vacuum() error {
	_, err := d.db.Exec("VACUUM")
	return err
}
