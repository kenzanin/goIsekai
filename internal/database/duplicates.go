package database

import (
	"sort"
)

// DuplicateGroup is a set of ≥2 in-library manga that share a normalised
// title or alternative title (or, when from different plugins, a normalised
// description or alternative description).  The UI consumes these to flag
// likely dupes.
type DuplicateGroup struct {
	Key     string  // normalised key that was matched
	Title   string  // most readable original title (main title preferred)
	Members []Manga // the manga in this group (≥2)
}

// minDescKeyLen is the minimum length (in runes) of a normalised description
// before it takes part in duplicate matching.  Short blurbs ("read online
// free", SEO placeholder text) are too generic to be evidence on their own.
const minDescKeyLen = 100

// FindPotentialDuplicates groups in-library manga by normalised title,
// alternative title, description, or alternative description.  Title-family
// keys are plugin-agnostic (the same story is usually listed under the same
// name everywhere).  Description-family keys additionally require the members
// to come from different plugins — a single source repeating one boilerplate
// blurb must not mass-collide — and a minimum length to skip generic text.
// Manga that share at least one key form a DuplicateGroup; each manga appears
// in at most one group.  Groups are sorted by member count descending.
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

	// 2. Bulk-load all alt_titles and alt_descriptions for in-library manga
	//    (two queries, no N+1).
	type altRow struct {
		MangaRowID string
		Value      string
	}

	loadAlts := func(table, column string) ([]altRow, error) {
		rows, err := d.db.Query(`SELECT at.manga_row_id, at.` + column + `
			FROM ` + table + ` at
			JOIN mangas m ON m.id = at.manga_row_id
			WHERE m.in_library = 1`)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		var alts []altRow
		for rows.Next() {
			var a altRow
			if err := rows.Scan(&a.MangaRowID, &a.Value); err != nil {
				return nil, err
			}
			alts = append(alts, a)
		}
		return alts, rows.Err()
	}

	titleAlts, err := loadAlts("alt_titles", "title")
	if err != nil {
		return nil, err
	}
	descAlts, err := loadAlts("alt_descriptions", "description")
	if err != nil {
		return nil, err
	}

	// 3. Build normalised key → manga-index set.  Title keys bind any two
	//    manga; description keys bind only across different plugins.
	titleKeys := make(map[string]map[int]struct{})
	descKeys := make(map[string]map[int]struct{})

	addTitle := func(key string, idx int) {
		if key == "" {
			return
		}
		if titleKeys[key] == nil {
			titleKeys[key] = make(map[int]struct{})
		}
		titleKeys[key][idx] = struct{}{}
	}
	addDesc := func(key string, idx int) {
		if len([]rune(key)) < minDescKeyLen {
			return
		}
		if descKeys[key] == nil {
			descKeys[key] = make(map[int]struct{})
		}
		descKeys[key][idx] = struct{}{}
	}

	for i, m := range allManga {
		addTitle(normalizeTitle(m.Title), i)
		addDesc(normalizeTitle(m.Description), i)
	}
	for _, a := range titleAlts {
		if idx, ok := idIndex[a.MangaRowID]; ok {
			addTitle(normalizeTitle(a.Value), idx)
		}
	}
	for _, a := range descAlts {
		if idx, ok := idIndex[a.MangaRowID]; ok {
			addDesc(normalizeTitle(a.Value), idx)
		}
	}

	// 4. Extract groups: keys that bind ≥2 distinct manga.  Description keys
	//    additionally require the members to span ≥2 plugins.
	//    Each manga lands in at most one group (first match wins).
	grouped := make([]bool, len(allManga))
	var groups []DuplicateGroup

	// Sort keys so grouping is deterministic.
	keys := make([]string, 0, len(titleKeys)+len(descKeys))
	for key := range titleKeys {
		keys = append(keys, key)
	}
	for key := range descKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	membersOf := func(key string) map[int]struct{} {
		if m, ok := titleKeys[key]; ok {
			return m
		}
		return descKeys[key]
	}

	for _, key := range keys {
		memberSet := membersOf(key)
		if len(memberSet) < 2 {
			continue
		}
		if _, isDesc := descKeys[key]; isDesc && !spansPlugins(memberSet, allManga) {
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
