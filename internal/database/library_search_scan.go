package database

import "strings"

// SearchLibrarySubstring returns in-library manga whose title, alt titles,
// main description or alt descriptions contain q as a case-insensitive
// substring (LIKE wildcards in q are escaped so it matches literally). It
// complements SearchLibraryFTS: prefix-token matching misses mid-token
// partials like "olo" in "Solo", and descriptions are not in the FTS index.
// The library is a few hundred rows, so a scan is fine — no FTS schema change.
func (d *DB) SearchLibrarySubstring(q string) ([]CandidateRow, error) {
	t := strings.TrimSpace(q)
	if t == "" {
		return nil, nil
	}
	pat := "%" + escapeLikePattern(t) + "%"
	rows, err := d.db.Query(`SELECT CAST(m.id AS TEXT), m.plugin_id, m.source_manga_id, m.title, m.description
		FROM mangas m WHERE m.in_library = 1
		AND (lower(m.title) LIKE lower(?) ESCAPE '\'
			OR lower(m.description) LIKE lower(?) ESCAPE '\'
			OR EXISTS (SELECT 1 FROM alt_titles a
				WHERE CAST(a.manga_row_id AS TEXT) = CAST(m.id AS TEXT)
				AND lower(a.title) LIKE lower(?) ESCAPE '\')
			OR EXISTS (SELECT 1 FROM alt_descriptions d
				WHERE CAST(d.manga_row_id AS TEXT) = CAST(m.id AS TEXT)
				AND lower(d.description) LIKE lower(?) ESCAPE '\'))`,
		pat, pat, pat, pat)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CandidateRow
	for rows.Next() {
		var c CandidateRow
		if err := rows.Scan(&c.MangaRowID, &c.PluginID, &c.SourceMangaID, &c.Title, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// escapeLikePattern escapes LIKE wildcards (and the escape char itself) so a
// substring scan matches q literally.
func escapeLikePattern(q string) string {
	r := strings.ReplaceAll(q, `\`, `\\`)
	r = strings.ReplaceAll(r, `%`, `\%`)
	return strings.ReplaceAll(r, `_`, `\_`)
}
