package database

import (
	"sort"
	"strings"
)

// FindPotentialDuplicates groups in-library manga that look like the same
// story.  Two passes feed one result:
//
//   - exact keys: equal normalised titles, alternative titles, descriptions or
//     alternative descriptions.  Title-family keys are plugin-agnostic (the
//     same story is usually listed under the same name everywhere).
//     Description-family keys additionally require the members to come from
//     different plugins — a single source repeating one boilerplate blurb must
//     not mass-collide — and a minimum length to skip generic text.
//   - distinctive wording: titles and synopses that share story-specific
//     vocabulary even when every name differs (see findSimilarSynopses).
//
// Each manga appears in at most one group.  Groups are sorted by member count
// descending.
func (d *DB) FindPotentialDuplicates() ([]DuplicateGroup, error) {
	// 1. Load all in-library manga.
	allManga, err := d.ListLibrary()
	if err != nil {
		return nil, err
	}
	if len(allManga) < 2 {
		return nil, nil
	}

	idIndex := make(map[int64]int, len(allManga))
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
		if idx, ok := idIndex[mangaIDFromRowID(a.MangaRowID)]; ok {
			addTitle(normalizeTitle(a.Value), idx)
		}
	}
	for _, a := range descAlts {
		if idx, ok := idIndex[mangaIDFromRowID(a.MangaRowID)]; ok {
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

	groups = append(groups, findSimilarSynopses(allManga, grouped)...)

	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Members) > len(groups[j].Members)
	})

	return groups, nil
}

// rareTokenMaxDF is the largest number of library rows a word may appear in
// and still count as evidence.  Words in two or three rows are almost always
// names taken from the story itself.
const rareTokenMaxDF = 2

// minRareTokens is how many of those distinctive words two manga must share
// before they are reported as the same story.  Measured on a real 88-title
// library, the one known re-worded pair shares 9 while the closest unrelated
// pair shares 3.
const minRareTokens = 4

// findSimilarSynopses pairs manga whose titles and synopses share at least
// minRareTokens words that no more than rareTokenMaxDF library rows use.
// Because the wording is compared, a duplicate is still found when every
// source spells the name differently.  Generic genre vocabulary ("reincarnated
// another world") is far too common to reach the frequency limit, so it never
// counts as evidence on its own, and a pair must span two sources.  Manga
// already placed in a group are left alone so the exact-key evidence stays
// authoritative.
func findSimilarSynopses(all []Manga, grouped []bool) []DuplicateGroup {
	// Postings are capped at rareTokenMaxDF+1 rows: anything longer can never
	// be evidence, and stopping early keeps a common word from collecting the
	// whole library.
	postings := make(map[string][]int)
	for i, m := range all {
		for tok := range strings.FieldsSeq(normalizeTitle(m.Title + " " + m.Description)) {
			if len(tok) < 2 {
				continue
			}
			if idxs := postings[tok]; len(idxs) <= rareTokenMaxDF {
				// Only add a row once, however often the word repeats.
				if len(idxs) == 0 || idxs[len(idxs)-1] != i {
					postings[tok] = append(idxs, i)
				}
			}
		}
	}

	// counts[a,b] is how many distinctive words the two rows share; token
	// remembers the alphabetically first of them, which names the group.
	counts := make(map[[2]int]int)
	token := make(map[[2]int]string)
	for word, idxs := range postings {
		if len(idxs) > rareTokenMaxDF {
			continue
		}
		for a := range idxs {
			for b := a + 1; b < len(idxs); b++ {
				// One source repeating its own blurb is not a second
				// sighting of the story, the same rule the description keys
				// follow.
				if all[idxs[a]].PluginID == all[idxs[b]].PluginID {
					continue
				}
				pair := [2]int{idxs[a], idxs[b]}
				counts[pair]++
				if cur, ok := token[pair]; !ok || word < cur {
					token[pair] = word
				}
			}
		}
	}

	// Keep the pairs that clear the bar, then join the survivors into groups:
	// three sources of one story produce three separate pairs, not one group.
	neighbours := make(map[int][]int)
	linked := make(map[int]bool)
	for pair, n := range counts {
		if n < minRareTokens || grouped[pair[0]] || grouped[pair[1]] {
			continue
		}
		neighbours[pair[0]] = append(neighbours[pair[0]], pair[1])
		neighbours[pair[1]] = append(neighbours[pair[1]], pair[0])
		linked[pair[0]] = true
		linked[pair[1]] = true
	}

	var groups []DuplicateGroup
	seen := make(map[int]bool, len(linked))
	for i := range all {
		if !linked[i] || seen[i] {
			continue
		}
		queue := []int{i}
		seen[i] = true
		var members []Manga
		key := ""
		for len(queue) > 0 {
			idx := queue[0]
			queue = queue[1:]
			members = append(members, all[idx])
			for _, other := range neighbours[idx] {
				if !seen[other] {
					seen[other] = true
					queue = append(queue, other)
				}
				pair := [2]int{idx, other}
				if idx > other {
					pair = [2]int{other, idx}
				}
				if w := token[pair]; key == "" || w < key {
					key = w
				}
			}
		}
		groups = append(groups, DuplicateGroup{
			Key:     key,
			Title:   members[0].Title,
			Members: members,
		})
	}
	return groups
}
