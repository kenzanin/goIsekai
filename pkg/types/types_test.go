package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChapterEpochSeconds(t *testing.T) {
	const in = `{"id":"c1","manga_id":"m1","title":"ch","chapter_num":1,"released_at":1789482933,"url":"u"}`
	var c Chapter
	if err := json.Unmarshal([]byte(in), &c); err != nil {
		t.Fatal(err)
	}
	if !c.ReleasedAt.Equal(time.Unix(1789482933, 0).UTC()) {
		t.Errorf("ReleasedAt = %v, want 2026-09-15T14:35:33Z", c.ReleasedAt)
	}
	if c.ReleasedText != "" {
		t.Errorf("ReleasedText = %q, want empty", c.ReleasedText)
	}
}

func TestChapterRFC3339String(t *testing.T) {
	const in = `{"id":"c1","manga_id":"m1","title":"ch","chapter_num":1,"released_at":"2026-09-15T14:35:33Z","url":"u"}`
	var c Chapter
	if err := json.Unmarshal([]byte(in), &c); err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse(time.RFC3339, "2026-09-15T14:35:33Z")
	if !c.ReleasedAt.Equal(want) {
		t.Errorf("ReleasedAt = %v, want %v", c.ReleasedAt, want)
	}
	if c.ReleasedText != "" {
		t.Errorf("ReleasedText = %q, want empty", c.ReleasedText)
	}
}

func TestChapterProseRetained(t *testing.T) {
	const in = `{"id":"c1","manga_id":"m1","title":"ch","chapter_num":1,"released_at":"recently","url":"u"}`
	var c Chapter
	if err := json.Unmarshal([]byte(in), &c); err != nil {
		t.Fatal(err)
	}
	if c.ReleasedAt != (time.Time{}) {
		t.Errorf("ReleasedAt = %v, want zero time", c.ReleasedAt)
	}
	if c.ReleasedText != "recently" {
		t.Errorf("ReleasedText = %q, want %q", c.ReleasedText, "recently")
	}
}

func TestChapterEpochMillisRejected(t *testing.T) {
	const in = `{"id":"c1","manga_id":"m1","title":"ch","chapter_num":1,"released_at":1789482933123,"url":"u"}`
	var c Chapter
	if err := json.Unmarshal([]byte(in), &c); err != nil {
		t.Fatal(err)
	}
	if c.ReleasedAt != (time.Time{}) {
		t.Errorf("ReleasedAt = %v, want zero time (millis rejected)", c.ReleasedAt)
	}
	if c.ReleasedText != "" {
		t.Errorf("ReleasedText = %q, want empty", c.ReleasedText)
	}
}

func TestChapterNullAndAbsent(t *testing.T) {
	for _, in := range []string{
		`{"id":"c1","manga_id":"m1","title":"ch","chapter_num":1,"released_at":null,"url":"u"}`,
		`{"id":"c1","manga_id":"m1","title":"ch","chapter_num":1,"url":"u"}`,
	} {
		var c Chapter
		if err := json.Unmarshal([]byte(in), &c); err != nil {
			t.Fatalf("in=%s: %v", in, err)
		}
		if c.ReleasedAt != (time.Time{}) {
			t.Errorf("in=%s: ReleasedAt = %v, want zero", in, c.ReleasedAt)
		}
		if c.ReleasedText != "" {
			t.Errorf("in=%s: ReleasedText = %q, want empty", in, c.ReleasedText)
		}
	}
}

func TestChapterFullRoundTrip(t *testing.T) {
	orig := Chapter{
		ID:         "c1",
		MangaID:    "m1",
		Title:      "Chapter 1",
		ChapterNum: 1,
		ReleasedAt: time.Unix(1789482933, 0).UTC(),
		URL:        "https://example.com/ch1",
		Scanlator:  "GroupX",
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Chapter
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string]struct{ got, want string }{
		"ID":        {decoded.ID, orig.ID},
		"MangaID":   {decoded.MangaID, orig.MangaID},
		"Title":     {decoded.Title, orig.Title},
		"URL":       {decoded.URL, orig.URL},
		"Scanlator": {decoded.Scanlator, orig.Scanlator},
	} {
		if pair.got != pair.want {
			t.Errorf("%s = %q, want %q", name, pair.got, pair.want)
		}
	}
	if decoded.ChapterNum != orig.ChapterNum {
		t.Errorf("ChapterNum = %v, want %v", decoded.ChapterNum, orig.ChapterNum)
	}
	if !decoded.ReleasedAt.Equal(orig.ReleasedAt) {
		t.Errorf("ReleasedAt = %v, want %v", decoded.ReleasedAt, orig.ReleasedAt)
	}
}
