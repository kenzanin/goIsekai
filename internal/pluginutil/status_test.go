package pluginutil

import "testing"

func TestDefaultStatusMap(t *testing.T) {
	m := DefaultStatusMap()
	if len(m) == 0 {
		t.Fatal("default map is empty")
	}
	// Check known entries exist
	if m["releasing"] != "Ongoing" {
		t.Errorf("releasing -> %q, want Ongoing", m["releasing"])
	}
	if m["completed"] != "Completed" {
		t.Errorf("completed -> %q, want Completed", m["completed"])
	}
	if m["dropped"] != "Dropped" {
		t.Errorf("dropped -> %q, want Dropped", m["dropped"])
	}
}

func TestNormalizeStatus(t *testing.T) {
	m := DefaultStatusMap()

	tests := []struct {
		raw    string
		expect string
	}{
		{"releasing", "Ongoing"},
		{"Releasing", "Ongoing"}, // case-insensitive
		{"RELEASING", "Ongoing"}, // case-insensitive
		{"publishing", "Ongoing"},
		{"completed", "Completed"},
		{"HIATUS", "Hiatus"},
		{"dropped", "Dropped"},
		{"upcoming", "Upcoming"},
		{"on-going", "Ongoing"},              // site-specific
		{"unknown_status", "unknown_status"}, // passthrough
		{"", ""},                             // empty passthrough
	}
	for _, tt := range tests {
		got := NormalizeStatus(m, tt.raw)
		if got != tt.expect {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.raw, got, tt.expect)
		}
	}
}

func TestNormalizeStatusNilMap(t *testing.T) {
	// nil map should fall back to DefaultStatusMap
	got := NormalizeStatus(nil, "releasing")
	if got != "Ongoing" {
		t.Errorf("nil map -> %q, want Ongoing", got)
	}
}

func TestNormalizeStatusCustomMap(t *testing.T) {
	custom := map[string]string{
		"serializing": "Ongoing",
		"fin":         "Completed",
	}
	if got := NormalizeStatus(custom, "serializing"); got != "Ongoing" {
		t.Errorf("custom serializing -> %q, want Ongoing", got)
	}
	// Unknown should passthrough
	if got := NormalizeStatus(custom, "releasing"); got != "releasing" {
		t.Errorf("custom unknown releasing -> %q, want passthrough", got)
	}
}

func TestNormalizeStatusIdenticalAcrossCases(t *testing.T) {
	m := DefaultStatusMap()
	// All case variants of the same key should produce the same canonical result
	variants := map[string]string{
		"releasing": "Ongoing",
		"Releasing": "Ongoing",
		"RELEASING": "Ongoing",
		"ReLeAsInG": "Ongoing",
		"on-going":  "Ongoing",
		"On-GOING":  "Ongoing",
		"completed": "Completed",
		"COMPLETED": "Completed",
		"hiatus":    "Hiatus",
		"HIATUS":    "Hiatus",
		"dropped":   "Dropped",
		"DROPPED":   "Dropped",
		"upcoming":  "Upcoming",
		"UPCOMING":  "Upcoming",
	}
	for raw, want := range variants {
		got := NormalizeStatus(m, raw)
		if got != want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeStatusPassthrough(t *testing.T) {
	m := DefaultStatusMap()
	// Values not in the map should be returned unchanged
	for _, raw := range []string{"reading", "plan_to_read", "rereading", "CUSTOM"} {
		got := NormalizeStatus(m, raw)
		if got != raw {
			t.Errorf("NormalizeStatus(%q) = %q, want passthrough", raw, got)
		}
	}
}
