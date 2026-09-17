package database

import "strings"

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
