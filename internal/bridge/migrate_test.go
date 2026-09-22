package bridge

import (
	"testing"

	"goisekai/internal/database"
)

func TestNormalizeTitleExport(t *testing.T) {
	if got := database.NormalizeTitle("  Solo   Leveling!! "); got != "solo leveling" {
		t.Fatalf("NormalizeTitle = %q, want %q", got, "solo leveling")
	}
}

func TestAutoSelectCandidate(t *testing.T) {
	cands := []MigrationCandidate{
		{PluginID: "a", Title: "Solo Leveling", IsExactMatch: true},
		{PluginID: "b", Title: "Solo Leveling Ragnarok", IsExactMatch: false},
	}
	if sel := AutoSelectCandidate(cands); sel == nil || sel.PluginID != "a" {
		t.Fatalf("expected auto-select a, got %v", sel)
	}
	if sel := AutoSelectCandidate([]MigrationCandidate{
		{PluginID: "a", Title: "Foo", IsExactMatch: false},
	}); sel != nil {
		t.Fatalf("expected nil for zero exact, got %v", sel)
	}
	if sel := AutoSelectCandidate([]MigrationCandidate{
		{PluginID: "a", Title: "Solo Leveling", IsExactMatch: true},
		{PluginID: "b", Title: "Solo Leveling", IsExactMatch: true},
	}); sel != nil {
		t.Fatalf("expected nil for several exact, got %v", sel)
	}
}
