package database

import "testing"

// longDesc returns a description long enough to clear minDescKeyLen.
func longDesc(s string) string {
	for len(s) < minDescKeyLen+20 {
		s += " world of adventurers and magic."
	}
	return s
}

func TestFindPotentialDuplicatesTitleAndDesc(t *testing.T) {
	db := openTestDB(t)

	insert := func(id, plugin, src, title, desc string, inLib bool) {
		t.Helper()
		m := Manga{ID: id, PluginID: plugin, SourceMangaID: src, Title: title, Description: desc, InLibrary: inLib}
		if err := db.UpsertManga(m); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	// Two titles differ only in punctuation → must group.
	insert("a", "p1", "a", "Solo Leveling", longDesc("solo leveling origin"), true)
	insert("b", "p2", "b", "Solo-Leveling", longDesc("solo leveling adapted"), true)
	// Same description, same plugin, different titles → NOT a description dupe.
	shared := longDesc("identical boilerplate blurb from one site")
	insert("c", "p1", "c", "Title One", shared, true)
	insert("d", "p1", "d", "Title Two", shared, true)
	// Same description across plugins → must group (description dupe).
	cross := longDesc("a hero reborn in another world seeks revenge")
	insert("e", "p3", "e", "The Reborn One", cross, true)
	insert("f", "p4", "f", "Rebirth Chronicles", cross, true)
	// Short description duplicates with DIFFERENT titles → must NOT group
	// (description keys below minDescKeyLen are ignored).
	short := "Read online free"
	insert("g", "p5", "g", "Gamma", short, true)
	insert("h", "p6", "h", "Delta", short, true)
	// Not in library → excluded entirely.
	insert("i", "p7", "i", "Solo Leveling", longDesc("hidden"), false)

	groups, err := db.FindPotentialDuplicates()
	if err != nil {
		t.Fatalf("FindPotentialDuplicates: %v", err)
	}

	// pairGrouped reports whether both IDs appear together in one group.
	pairGrouped := func(a, b string) bool {
		for _, g := range groups {
			hasA, hasB := false, false
			for _, m := range g.Members {
				if m.ID == a {
					hasA = true
				}
				if m.ID == b {
					hasB = true
				}
			}
			if hasA && hasB {
				return true
			}
		}
		return false
	}

	want := []struct{ a, b string }{
		{"a", "b"}, // title dupe across plugins
		{"e", "f"}, // description dupe across plugins
	}
	for _, w := range want {
		if !pairGrouped(w.a, w.b) {
			t.Errorf("expected duplicate pair (%s,%s) to be grouped", w.a, w.b)
		}
	}
	// c & d share description but same plugin → must NOT be a dupe.
	if pairGrouped("c", "d") {
		t.Errorf("same-plugin description must not group (c,d)")
	}
	// g & h share a short description but different titles → must NOT group.
	if pairGrouped("g", "h") {
		t.Errorf("short description must not group (g,h)")
	}
	// a & b share a description prefix but are not identical → no match.
	if pairGrouped("a", "c") || pairGrouped("b", "d") {
		t.Errorf("distinct descriptions must not group")
	}
	// i is not in library → no pair with it.
	for _, g := range groups {
		for _, m := range g.Members {
			if m.ID == "i" {
				t.Errorf("out-of-library manga must not appear in groups")
			}
		}
	}
}
