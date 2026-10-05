package database

import (
	"database/sql"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

type Manga struct {
	ID            int64
	PluginID      string
	SourceMangaID string
	Title         string
	CoverURL      string
	Description   string
	Status        string
	CoverDim      int64
	InLibrary     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Chapter struct {
	ID              int64
	MangaID         int64
	SourceChapterID string
	Title           string
	ChapterNum      float64
	VolumeNum       float64
	IsRead          bool
	LastPageRead    int
	TotalPages      int
	DownloadStatus  string
	FetchedAt       time.Time
}

// ChapterProgress is the per-chapter read progress surfaced to the UI.
type ChapterProgress struct {
	SourceChapterID string
	LastPageRead    int
	TotalPages      int
	IsRead          bool // manually marked read (mark-read actions)
	IsSkipped       bool // user opted to skip this chapter
	Done            bool // IsRead OR fully read (LastPageRead >= TotalPages > 0)
	CachedPages     int  // page files present in the disk cache (populated by the bridge layer)
}

type Plugin struct {
	ID         string
	Name       string
	Version    string
	WasmPath   string
	IsActive   bool
	IconURL    string
	ThumbRatio float64
}

const (
	DownloadNotDownloaded = "NOT_DOWNLOADED"
	DownloadDownloading   = "DOWNLOADING"
	DownloadDownloaded    = "DOWNLOADED"
)

type DB struct{ db *sql.DB }

// defaultBusyTimeoutMS is busy_timeout for a freshly opened handle, in
// milliseconds. dsn() sets it; tests that need a shorter wait override it per
// connection and restore it afterwards.
const defaultBusyTimeoutMS = 5000

// dsn builds the SQLite connection string.
//
// _txlock=immediate makes every transaction take the write lock at BEGIN
// instead of at its first write. Without it a deferred transaction that reads
// before writing can collide with a concurrent writer and fail with
// SQLITE_BUSY *after* doing work; with it the loser waits at BEGIN and
// busy_timeout retries it. The driver applies this per connection, so it also
// covers the paths that call d.db.Begin() directly.
//
// It applies to read-only transactions too. If a read-heavy path ever shows up
// taking a write lock it does not need, open it with an explicit
// "BEGIN; DEFERRED" on a dedicated handle rather than removing this flag —
// per-transaction locking is not selectable on a pooled handle.
func dsn(path string) string {
	return path + "?_foreign_keys=1&_journal_mode=WAL" +
		"&_busy_timeout=" + strconv.Itoa(defaultBusyTimeoutMS) +
		"&_txlock=immediate" +
		"&_pragma=cache_size=-64000&_pragma=mmap_size=268435456&_pragma=synchronous=1"
}

// Open opens the SQLite database at path, enables foreign keys so cascade
// deletes work, and applies pending migrations.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	// modernc.org/sqlite is pure Go; ping ensures the file is usable.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	d := &DB{db: db}
	if err := d.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the underlying database handle.
func (d *DB) Close() error { return d.db.Close() }

// Begin starts a write transaction. The lock is taken here, at BEGIN, not at
// the first write — see dsn. Errors if the write lock is held past
// busy_timeout.
func (d *DB) Begin() (*sql.Tx, error) { return d.db.Begin() }

// Exec executes a statement against the underlying database.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.db.Exec(query, args...)
}
