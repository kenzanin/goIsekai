package bridge

import "testing"

func TestWithDefaultReferer(t *testing.T) {
	t.Parallel()
	t.Run("fills missing referer", func(t *testing.T) {
		got := withDefaultReferer(map[string]string{"Accept": "image/*"}, "https://mangak.io")
		if got["Referer"] != "https://mangak.io" {
			t.Fatalf("Referer = %q", got["Referer"])
		}
		if got["Accept"] != "image/*" {
			t.Fatalf("Accept lost: %q", got["Accept"])
		}
	})
	t.Run("explicit referer wins", func(t *testing.T) {
		got := withDefaultReferer(map[string]string{"Referer": "https://other/"}, "https://mangak.io")
		if got["Referer"] != "https://other/" {
			t.Fatalf("Referer = %q", got["Referer"])
		}
	})
	t.Run("case-insensitive explicit wins", func(t *testing.T) {
		got := withDefaultReferer(map[string]string{"referer": "https://other/"}, "https://mangak.io")
		if got["referer"] != "https://other/" {
			t.Fatalf("referer = %q", got["referer"])
		}
	})
	t.Run("empty siteURL no-op", func(t *testing.T) {
		in := map[string]string{"Accept": "image/*"}
		got := withDefaultReferer(in, "")
		if len(got) != 1 {
			t.Fatalf("headers mutated: %v", got)
		}
	})
	t.Run("does not mutate input map", func(t *testing.T) {
		in := map[string]string{"Accept": "image/*"}
		_ = withDefaultReferer(in, "https://mangak.io")
		if _, ok := in["Referer"]; ok {
			t.Fatal("input map was mutated")
		}
	})
}
