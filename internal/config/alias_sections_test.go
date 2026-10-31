package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLoadIgnoresLegacyAliasSections pins that a goisekai.ini written by an
// older version still loads: the host owns both vocabularies, so [genre] and
// [status] lines are parsed and dropped rather than rejected as unknown.
func TestLoadIgnoresLegacyAliasSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goisekai.ini")
	ini := `[app]
data_dir = ` + dir + `

[genre]
Sci-Fi = scifi, sci-fi

[status]
Hiatus = hiatus, uncertain
`
	if err := os.WriteFile(path, []byte(ini), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("a config carrying legacy alias sections must still load: %v", err)
	}
	if c.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", c.DataDir, dir)
	}
}

// TestSaveWritesNoAliasSections pins that neither vocabulary is written: a
// generated config must not carry a section plugins could mistake for a
// supported override.
func TestSaveWritesNoAliasSections(t *testing.T) {
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
	for _, section := range []string{"[genre]", "[status]"} {
		if strings.Contains(string(raw), section) {
			t.Errorf("saved config still carries a %s section:\n%s", section, raw)
		}
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
