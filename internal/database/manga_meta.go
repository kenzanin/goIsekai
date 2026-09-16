package database

import (
	"github.com/goccy/go-json"
)

// SetMangaGenres stores a user-defined genre override as a JSON text array.
// Pass nil to clear the override (return to plugin-supplied genres).
func (d *DB) SetMangaGenres(mangaID int64, genres []string) error {
	var payload *string
	if genres != nil {
		raw, err := json.Marshal(genres)
		if err != nil {
			return err
		}
		s := string(raw)
		payload = &s
	}
	_, err := d.db.Exec(`UPDATE mangas SET genres = ? WHERE id = ?`, payload, mangaID)
	return err
}

// SetMangaCoverDim sets the cover dim overlay flag (0 = off, 1 = on).
func (d *DB) SetMangaCoverDim(mangaID int64, dim int64) error {
	_, err := d.db.Exec(`UPDATE mangas SET cover_dim = ? WHERE id = ?`, dim, mangaID)
	return err
}

// SetMangaAuthor stores the author captured by an enrichment provider.
// An empty author clears the value.
func (d *DB) SetMangaAuthor(mangaID int64, author string) error {
	_, err := d.db.Exec(`UPDATE mangas SET author = ? WHERE id = ?`, author, mangaID)
	return err
}

// GetMangaAuthor returns the stored enrichment author for a manga.
// Returns ("", false) when nothing was fetched yet.
func (d *DB) GetMangaAuthor(mangaID int64) (string, bool, error) {
	var author string
	err := d.db.QueryRow(`SELECT author FROM mangas WHERE id = ?`, mangaID).Scan(&author)
	if err != nil {
		return "", false, err
	}
	if author == "" {
		return "", false, nil
	}
	return author, true, nil
}

// GetMangaCoverDim returns the current cover_dim flag for a manga.
// Returns (0, false) when no override exists.
func (d *DB) GetMangaCoverDim(mangaID int64) (int64, bool, error) {
	var dim *int64
	err := d.db.QueryRow(`SELECT cover_dim FROM mangas WHERE id = ?`, mangaID).Scan(&dim)
	if err != nil {
		return 0, false, err
	}
	if dim == nil {
		return 0, false, nil
	}
	return *dim, true, nil
}

// GetMangaGenres returns the stored genre override for a manga.
// Returns the genre list and true on success; returns (nil, false) when no override exists.
func (d *DB) GetMangaGenres(mangaID int64) ([]string, bool, error) {
	var genresJSON *string
	err := d.db.QueryRow(`SELECT genres FROM mangas WHERE id = ?`, mangaID).Scan(&genresJSON)
	if err != nil {
		return nil, false, err
	}
	if genresJSON == nil || *genresJSON == "" {
		return nil, false, nil
	}
	var genres []string
	if err := json.Unmarshal([]byte(*genresJSON), &genres); err != nil {
		return nil, false, err
	}
	return genres, true, nil
}
