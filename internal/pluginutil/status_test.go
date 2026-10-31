package pluginutil

import "testing"

func TestStatuses(t *testing.T) {
	got := Statuses()
	want := []string{"Ongoing", "Completed", "Hiatus", "Cancelled", "Upcoming"}
	if len(got) != len(want) {
		t.Fatalf("Statuses() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Statuses() = %v, want %v", got, want)
		}
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		expect string
	}{
		// Every canonical label round-trips.
		{"ongoing", "Ongoing", "Ongoing"},
		{"completed", "Completed", "Completed"},
		{"hiatus", "Hiatus", "Hiatus"},
		{"cancelled", "Cancelled", "Cancelled"},
		{"upcoming", "Upcoming", "Upcoming"},

		// Case-insensitive on both sides.
		{"upper", "ONGOING", "Ongoing"},
		{"mixed", "CoMpLeTeD", "Completed"},
		{"lower", "upcoming", "Upcoming"},

		// Site-specific words are not canonical any more: the plugin maps them.
		{"site publishing", "publishing", ""},
		{"site finished", "finished", ""},
		{"site releasing", "releasing", ""},
		{"site on hiatus", "on hiatus", ""},
		{"site discontinued", "discontinued", ""},
		{"site not yet published", "not yet published", ""},

		// Retired label.
		{"dropped retired", "Dropped", ""},
		{"dropped lower", "dropped", ""},

		// Near-misses must not manufacture a status via substring matching.
		{"prefix", "Ongoing (12 vol)", ""},
		{"table-ish input", `{"publishing":"Ongoing"}`, ""},

		{"empty", "", ""},
		{"garbage", "unknown_status", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeStatus(tt.raw); got != tt.expect {
				t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.raw, got, tt.expect)
			}
		})
	}
}

func TestNormalizeStatusIsIdempotent(t *testing.T) {
	for _, status := range Statuses() {
		if got := NormalizeStatus(NormalizeStatus(status)); got != status {
			t.Errorf("NormalizeStatus(NormalizeStatus(%q)) = %q, want %q", status, got, status)
		}
	}
}
