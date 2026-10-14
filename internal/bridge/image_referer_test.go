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
		if len(got) != 3 || got[0] != "https://uploads.mangadex.org/" || got[1] != "https://mangadex.org" || got[2] != "" {
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
		if len(got) != 2 || got[0] != "https://site.io" || got[1] != "" {
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
