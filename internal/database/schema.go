package database

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
}
