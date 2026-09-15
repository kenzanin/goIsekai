package database

// altTitlesMigration is the index in the migrations slice that is handled by
// migrateAltTitles() (Go-side JSON→table copy, DROP COLUMN, FTS5) rather than
// a plain DDL Exec.
const altTitlesMigration = 9

// skipColumnMigration is the index for the is_skipped ALTER TABLE migration.
// It's special-cased because test databases may already have the column from
// the CREATE TABLE DDL, making a plain ALTER TABLE fail with "duplicate column".
const skipColumnMigration = 18

// migrations is an ordered list of DDL statements applied in sequence.
// Version is tracked via PRAGMA user_version; migrations[i] is applied when
// user_version < len(migrations) so partial upgrades resume correctly.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS mangas (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL,
    source_manga_id TEXT NOT NULL,
    title TEXT NOT NULL,
    cover_url TEXT,
    description TEXT,
    status TEXT,
    in_library INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plugin_id, source_manga_id)
);`,
	`CREATE TABLE IF NOT EXISTS chapters (
    id TEXT PRIMARY KEY,
    manga_id TEXT NOT NULL,
    source_chapter_id TEXT NOT NULL,
    title TEXT NOT NULL,
    chapter_num REAL NOT NULL,
    volume_num REAL,
    is_read INTEGER DEFAULT 0,
    is_skipped INTEGER DEFAULT 0,
    last_page_read INTEGER DEFAULT 0,
    download_status TEXT DEFAULT 'NOT_DOWNLOADED',
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(manga_id) REFERENCES mangas(id) ON DELETE CASCADE
);`,
	`CREATE TABLE IF NOT EXISTS read_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_id TEXT NOT NULL,
    page_num INTEGER NOT NULL,
    read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
);`,
	`CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    wasm_path TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    icon_url TEXT
);`,
	`ALTER TABLE chapters ADD COLUMN total_pages INTEGER DEFAULT 0;`,
	`CREATE TABLE IF NOT EXISTS plugin_verify (
    plugin_id TEXT PRIMARY KEY,
    verify_url TEXT NOT NULL DEFAULT '',
    cookies TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0
);`,
	`ALTER TABLE plugins ADD COLUMN thumb_ratio REAL DEFAULT 0;`,
	`ALTER TABLE mangas ADD COLUMN new_since TIMESTAMP;`,
	`ALTER TABLE mangas ADD COLUMN alt_titles TEXT DEFAULT '';`,
	// Index 9: alt_titles migration — handled by migrateAltTitles() in db.go
	// (CREATE TABLE alt_titles, JSON→table copy, DROP COLUMN, CREATE FTS5, backfill).
	`/* alt_titles: see migrateAltTitles */`,
	// Index 10: user-chosen main title lock — UpsertManga must not overwrite it.
	`ALTER TABLE mangas ADD COLUMN custom_title INTEGER NOT NULL DEFAULT 0;`,
	// Index 11: per-plugin pinned TLS profile (winner of the WAF rotation
	// ladder). Empty string = not yet pinned (auto-ladder on next block).
	`ALTER TABLE plugins ADD COLUMN http_profile TEXT DEFAULT '';`,
	// Index 12: per-chapter cached page list (JSON array of {index,url}) so a
	// fully-fetched chapter can be read offline without a plugin round-trip.
	`CREATE TABLE IF NOT EXISTS chapter_pages (
    chapter_id TEXT PRIMARY KEY,
    pages TEXT NOT NULL,
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
);`,
	// Index 13: alternative descriptions (mirror of alt_titles). Origin
	// summary lives in mangas.description and is never stored here.
	`CREATE TABLE IF NOT EXISTS alt_descriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    manga_row_id TEXT NOT NULL REFERENCES mangas(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    source TEXT NOT NULL,
    UNIQUE(manga_row_id, description)
);`,
	// Index 14: user-chosen main summary lock — UpsertManga must not overwrite it.
	`ALTER TABLE mangas ADD COLUMN custom_description INTEGER NOT NULL DEFAULT 0;`,
	// Index 15: enrichment categories/tags per manga.
	`CREATE TABLE IF NOT EXISTS manga_categories (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    manga_row_id TEXT NOT NULL REFERENCES mangas(id) ON DELETE CASCADE,
	    category TEXT NOT NULL,
	    source TEXT NOT NULL DEFAULT 'mangadex',
	    UNIQUE(manga_row_id, category)
	);`,
	// Index 16: related/recommended manga per manga.
	`CREATE TABLE IF NOT EXISTS manga_related (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    manga_row_id TEXT NOT NULL REFERENCES mangas(id) ON DELETE CASCADE,
	    title TEXT NOT NULL,
	    url TEXT,
	    source TEXT NOT NULL DEFAULT 'mangadex',
	    UNIQUE(manga_row_id, title)
	);`,
	// Index 17: user override for manga genres (JSON text array); NULL means
	// "use the plugin-supplied list". ponytail: when genre overrides grow
	// into per-source tracking, replace the JSON column with a table.
	`ALTER TABLE mangas ADD COLUMN genres TEXT;`,
	// Index 18: skip flag for chapters — skipped chapters are excluded from
	// "continue reading" navigation and reader auto-advance.
	`ALTER TABLE chapters ADD COLUMN is_skipped INTEGER DEFAULT 0;`,
	// Index 19: user toggle to dim the manga cover image (persisted per manga).
	`ALTER TABLE mangas ADD COLUMN cover_dim INTEGER DEFAULT 0;`,
	// Index 20: plugin response cache — stores JSON responses keyed by plugin,
	// manga, and function. ponytail: when the cache grows to millions of entries,
	// add a shard index on (plugin_id, cached_at) for the cleanup scan.
	`CREATE TABLE IF NOT EXISTS plugin_cache (
		id TEXT PRIMARY KEY,
		plugin_id TEXT NOT NULL,
		manga_id TEXT NOT NULL,
		function_name TEXT NOT NULL,
		response TEXT NOT NULL,
		cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		UNIQUE(plugin_id, manga_id, function_name)
	);`,
	// Index 21: author captured by an enrichment provider (plugins do not
	// always supply one). Empty string means "not fetched yet".
	`ALTER TABLE mangas ADD COLUMN author TEXT NOT NULL DEFAULT '';`,
}
