package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	// The [status] section above is deliberately still present: an INI written by an
	// older version must load without error, and the section is simply ignored.
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
	if got := c.GenreAlias["Sci-Fi"]; len(got) != 2 {
		t.Errorf("Sci-Fi = %v, want exactly the 2 variants from the file", got)
	}
}

func TestSaveRoundTripsAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goisekai.ini")
	c := Default()
	c.DataDir = dir
	c.GenreAlias["Sci-Fi"] = []string{"scifi"}
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
}

// TestSaveWritesNoStatusSection pins the removal of the [status] alias
// vocabulary: the host owns the canonical status list, so saving a config must
// not write a section plugins could mistake for a supported override.
func TestSaveWritesNoStatusSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goisekai.ini")
	c := Default()
	c.DataDir = filepath.Dir(path)
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "[status]") {
		t.Errorf("saved config still carries a [status] section:\n%s", raw)
	}
	// The genre section is untouched by this change.
	if !strings.Contains(string(raw), "[genre]") {
		t.Error("saved config lost its [genre] section")
	}
}

// TestSaveRoundTripsEveryField pins the app's generation contract: a
// goisekai.ini written by Save() must carry every config option filled with a
// value, so nothing silently disappears on a load round-trip. It walks the
// Config struct, which means a new field fails the test until it is written
// and parsed.
func TestSaveRoundTripsEveryField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goisekai.ini")
	c := Default()
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want, have := reflect.ValueOf(*c), reflect.ValueOf(*got)
	typ := want.Type()
	for i := 0; i < want.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if a, b := want.Field(i).Interface(), have.Field(i).Interface(); !reflect.DeepEqual(a, b) {
			t.Errorf("%s = %v after round-trip, want %v", field.Name, b, a)
		}
	}
}
