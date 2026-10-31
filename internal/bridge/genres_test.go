package bridge

import (
	"reflect"
	"testing"
)

func TestGenreIndexNormalizesToCanonical(t *testing.T) {
	idx := newGenreIndex(CanonicalGenres)

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "separator and case variants collapse onto the canonical name",
			in:   []string{"scifi", "SCI FI", "Sci_Fi"},
			want: []string{"Sci-Fi"},
		},
		{
			name: "canonical spelling resolves without an alias entry",
			in:   []string{"Sci-Fi", "Horror"},
			want: []string{"Sci-Fi", "Horror"},
		},
		{
			name: "punctuation and case are ignored when matching",
			in:   []string{"SCI_FI", "slice-of-life"},
			want: []string{"Sci-Fi", "Slice of Life"},
		},
		{
			name: "unknown names pass through unchanged",
			in:   []string{"Doujinshi", "Horror Psychological"},
			want: []string{"Doujinshi", "Horror Psychological"},
		},
		{
			name: "order holds and duplicates are dropped",
			in:   []string{"Action", "scifi", "Action", "Sci-Fi"},
			want: []string{"Action", "Sci-Fi"},
		},
		{
			name: "blank and whitespace-only entries are dropped",
			in:   []string{"", "  ", "Action"},
			want: []string{"Action"},
		},
		{
			name: "surrounding whitespace is trimmed",
			in:   []string{"  Action  "},
			want: []string{"Action"},
		},
		{
			name: "empty input is returned as-is",
			in:   nil,
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := idx.normalize(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestGenreKeyCollapsesSeparators(t *testing.T) {
	same := []string{"Sci-Fi", "sci fi", "SCI_FI", "scifi", "Sci.Fi"}
	keys := make(map[string]bool, len(same))
	for _, s := range same {
		keys[genreKey(s)] = true
	}
	if len(keys) != 1 {
		t.Errorf("expected every spelling to collapse to one key, got %d: %v", len(keys), keys)
	}
	if genreKey("Slice of Life") != genreKey("slice-of-life") {
		t.Error("space and hyphen should collapse the same way")
	}
}
