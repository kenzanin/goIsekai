package database

import (
	"sort"
	"strings"
	"unicode"
)

// DuplicateGroup is a set of ≥2 in-library manga that share a normalised
// title or alternative title.  The UI consumes these to flag likely dupes.
type DuplicateGroup struct {
	Key     string  // normalised title that was matched
	Title   string  // most readable original title (main title preferred)
	Members []Manga // the manga in this group (≥2)
}

// normalizeTitle lowercases, trims, strips non-alphanumeric characters
// (keeping letters, digits, and spaces), and collapses whitespace runs.
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace { // non-alnum → treat as space separator
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// FindPotentialDuplicates groups in-library manga by normalised title or
// alternative title.  Manga that share at least one normalised key form a
// DuplicateGroup; each manga appears in at most one group.  Groups are
// sorted by member count descending.
func (d *DB) FindPotentialDuplicates() ([]DuplicateGroup, error) {
	// 1. Load all in-library manga.
	allManga, err := d.ListLibrary()
	if err != nil {
		return nil, err
	}
	if len(allManga) < 2 {
		return nil, nil
	}

	idIndex := make(map[string]int, len(allManga))
	for i, m := range allManga {
		idIndex[m.ID] = i
	}

	// 2. Bulk-load all alt_titles for in-library manga (one query, no N+1).
	type altRow struct {
		MangaRowID string
		Title      string
	}
	rows, err := d.db.Query(`SELECT at.manga_row_id, at.title
		FROM alt_titles at
		JOIN mangas m ON m.id = at.manga_row_id
		WHERE m.in_library = 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var alts []altRow
	for rows.Next() {
		var a altRow
		if err := rows.Scan(&a.MangaRowID, &a.Title); err != nil {
			return nil, err
		}
		alts = append(alts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 3. Build normalised key → manga-index set.
	keyToMembers := make(map[string]map[int]struct{})

	addKey := func(key string, idx int) {
		if key == "" {
			return
		}
		if keyToMembers[key] == nil {
			keyToMembers[key] = make(map[int]struct{})
		}
		keyToMembers[key][idx] = struct{}{}
	}

	for i, m := range allManga {
		addKey(normalizeTitle(m.Title), i)
	}
	for _, a := range alts {
		idx, ok := idIndex[a.MangaRowID]
		if !ok {
			continue
		}
		addKey(normalizeTitle(a.Title), idx)
	}

	// 4. Extract groups: keys that bind ≥2 distinct manga.
	//    Each manga lands in at most one group (first match wins).
	grouped := make([]bool, len(allManga))
	var groups []DuplicateGroup

	// Sort keys so grouping is deterministic.
	keys := make([]string, 0, len(keyToMembers))
	for key := range keyToMembers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		memberSet := keyToMembers[key]
		if len(memberSet) < 2 {
			continue
		}
		var members []Manga
		var title string
		for idx := range memberSet {
			if grouped[idx] {
				continue
			}
			grouped[idx] = true
			members = append(members, allManga[idx])
			if title == "" {
				title = allManga[idx].Title // prefer main title
			}
		}
		if len(members) < 2 {
			continue
		}
		groups = append(groups, DuplicateGroup{
			Key:     key,
			Title:   title,
			Members: members,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Members) > len(groups[j].Members)
	})

	return groups, nil
}
