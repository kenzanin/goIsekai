package bridge

import "testing"

func TestWithReferer(t *testing.T) {
	t.Parallel()
	t.Run("sets referer preserving others", func(t *testing.T) {
		got := withReferer(map[string]string{"Accept": "image/*"}, "https://mangak.io")
		if got["Referer"] != "https://mangak.io" {
			t.Fatalf("Referer = %q", got["Referer"])
		}
		if got["Accept"] != "image/*" {
			t.Fatalf("Accept lost: %q", got["Accept"])
		}
	})
	t.Run("empty ref no-op", func(t *testing.T) {
		in := map[string]string{"Accept": "image/*"}
		got := withReferer(in, "")
		if len(got) != 1 {
			t.Fatalf("headers mutated: %v", got)
		}
	})
	t.Run("does not mutate input map", func(t *testing.T) {
		in := map[string]string{"Accept": "image/*"}
		_ = withReferer(in, "https://mangak.io")
		if _, ok := in["Referer"]; ok {
			t.Fatal("input map was mutated")
		}
	})
}

func TestRefererCandidates(t *testing.T) {
	t.Parallel()
	t.Run("origin first site fallback", func(t *testing.T) {
		got := refererCandidates("https://uploads.mangadex.org/covers/x.jpg", "https://mangadex.org")
		want := []string{"https://uploads.mangadex.org/", "https://mangadex.org/", "https://mangadex.org", ""}
		if len(got) != len(want) {
			t.Fatalf("candidates = %v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("candidate %d = %q, want %q (all: %v)", i, got[i], want[i], got)
			}
		}
	})
	t.Run("site offered with and without trailing slash", func(t *testing.T) {
		// 2xstorage.com / zinmanga1.com gate on the slash form; the bare origin
		// stays as a fallback for hosts that gate the other way.
		got := refererCandidates("https://cdn.zinmanga1.com//zin/a/1.png", "https://www.kunmanga.online")
		if len(got) < 2 || got[1] != "https://www.kunmanga.online/" || got[2] != "https://www.kunmanga.online" {
			t.Fatalf("candidates = %v", got)
		}
	})
	t.Run("dedupes identical origin and site", func(t *testing.T) {
		got := refererCandidates("https://x.io/a.jpg", "https://x.io/")
		if len(got) != 2 || got[0] != "https://x.io/" || got[1] != "" {
			t.Fatalf("candidates = %v", got)
		}
	})
	t.Run("unparseable url falls back to site", func(t *testing.T) {
		got := refererCandidates("::bad", "https://site.io")
		if len(got) != 3 || got[0] != "https://site.io/" || got[1] != "https://site.io" || got[2] != "" {
			t.Fatalf("candidates = %v", got)
		}
	})
}

func TestMemorizedReferer(t *testing.T) {
	t.Parallel()
	s := &AppService{}
	if _, ok := s.memorizedReferer("h"); ok {
		t.Fatal("expected miss on empty memo")
	}
	s.memorizeReferer("h", "https://site.io/")
	ref, ok := s.memorizedReferer("h")
	if !ok || ref != "https://site.io/" {
		t.Fatalf("memo = %q, %v", ref, ok)
	}
}
