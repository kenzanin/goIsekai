package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAliasesFromIni(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goisekai.ini")
	ini := `[app]
data_dir = ` + dir + `

[genre]
Sci-Fi = scifi, sci-fi
Doujinshi = doujin, dj

[status]
Hiatus = hiatus, uncertain
`
	if err := os.WriteFile(path, []byte(ini), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		got, want []string
	}{
		{"Sci-Fi", c.GenreAlias["Sci-Fi"], []string{"scifi", "sci-fi"}},
		{"Doujinshi", c.GenreAlias["Doujinshi"], []string{"doujin", "dj"}},
		{"Hiatus", c.StatusAlias["Hiatus"], []string{"hiatus", "uncertain"}},
	} {
		if len(tc.got) != len(tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
			continue
		}
		for i := range tc.got {
			if tc.got[i] != tc.want[i] {
				t.Errorf("%s[%d] = %q, want %q", tc.name, i, tc.got[i], tc.want[i])
			}
		}
	}

	// A name the file did not mention keeps its built-in entry.
	if _, ok := c.GenreAlias["Action"]; !ok {
		t.Error("Action default should still be present")
	}
	// A file line replaces the built-in variants instead of appending to them.
	if got := c.StatusAlias["Hiatus"]; len(got) != 2 {
		t.Errorf("Hiatus = %v, want exactly the 2 variants from the file", got)
	}
}

func TestSaveRoundTripsAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goisekai.ini")
	c := Default()
	c.DataDir = dir
	c.GenreAlias["Sci-Fi"] = []string{"scifi"}
	c.StatusAlias["Hiatus"] = []string{"uncertain"}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if g := got.GenreAlias["Sci-Fi"]; len(g) != 1 || g[0] != "scifi" {
		t.Errorf("Sci-Fi = %v, want [scifi]", g)
	}
	if s := got.StatusAlias["Hiatus"]; len(s) != 1 || s[0] != "uncertain" {
		t.Errorf("Hiatus = %v, want [uncertain]", s)
	}
}
