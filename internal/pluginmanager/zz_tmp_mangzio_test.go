package pluginmanager

import (
	"testing"

	"goisekai/internal/hostnet"
)

// Temporary diagnostic: does the real mangzio plugin emit released_at?
func TestTmpMangzioChapters(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), "/home/kenzanin/tmp/goIsekai/app_data/plugins")
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	chs, err := mgr.GetChapterList("mangzio", "gaikotsu-kishi-sama-tadaima-isekai-e-odekakechuu")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	t.Logf("count=%d", len(chs))
	for i, c := range chs {
		if i >= 3 {
			break
		}
		t.Logf("ch[%d] id=%s released_at=%v text=%q", i, c.ID, c.ReleasedAt, c.ReleasedText)
	}
}
