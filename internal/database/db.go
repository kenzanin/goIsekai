package database

import (
	"database/sql"
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

// Open opens the SQLite database at path, enables foreign keys so cascade
// deletes work, and applies pending migrations.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_foreign_keys=1&_journal_mode=WAL&_busy_timeout=5000")
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
