package bridge

import (
	"path/filepath"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// newStatusService returns a service whose plugin is registered but inactive, so
// reads come from SQLite without a live plugin call.
func newStatusService(t *testing.T) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "status.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RegisterPlugin(database.Plugin{ID: "st", Name: "st", IsActive: false}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
	t.Cleanup(s.Shutdown)
	return s
}

// The write boundary is the host's last line of defence: a plugin that reports a
// site-specific word must not get that word into mangas.status. Only the
// canonical vocabulary is stored, and a non-canonical value is dropped rather
// than echoed back or invented.
func TestPersistNormalizesStatusAtWriteBoundary(t *testing.T) {
	tests := []struct {
		name   string
		plugin string
		expect string
	}{
		{"canonical passes", "Completed", "Completed"},
		{"canonical case folded", "ongoing", "Ongoing"},
		{"upcoming accepted", "Upcoming", "Upcoming"},
		{"cancelled accepted", "Cancelled", "Cancelled"},
		{"site word dropped", "publishing", ""},
		{"site word finished dropped", "finished", ""},
		{"site word on hiatus dropped", "on hiatus", ""},
		{"retired label dropped", "Dropped", ""},
		{"garbage dropped", "unknown_status", ""},
		{"empty stays empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStatusService(t)
			sourceID := "src-" + tt.name

			if err := s.persistMangaDetails("st", types.Manga{
				ID:     sourceID,
				Title:  "Status Fixture",
				Status: tt.plugin,
			}, nil); err != nil {
				t.Fatalf("persistMangaDetails: %v", err)
			}

			got, _, err := s.CachedMangaAndChapters("st", sourceID)
			if err != nil {
				t.Fatalf("CachedMangaAndChapters: %v", err)
			}
			if got.Status != tt.expect {
				t.Errorf("stored status for plugin value %q = %q, want %q",
					tt.plugin, got.Status, tt.expect)
			}
		})
	}
}
