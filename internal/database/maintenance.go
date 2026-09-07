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

// PruneOrphans deletes rows that reference data which no longer exists and
// non-library manga with no chapters (abandoned detail views / search cache).
// It returns a human-readable summary of what was removed.
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
	run("orphan_alt_titles", `DELETE FROM alt_titles WHERE manga_row_id NOT IN (SELECT id FROM mangas)`)
	run("non_library_no_chapters", `DELETE FROM mangas WHERE in_library = 0 AND id NOT IN (SELECT DISTINCT manga_id FROM chapters)`)

	if b.Len() == 0 {
		return "clean", nil
	}
	return strings.TrimSpace(b.String()), nil
}
