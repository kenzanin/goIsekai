package bridge

// MarkChapterRead marks a single chapter as read, addressed by source ids.
func (s *AppService) MarkChapterRead(pluginID, mangaID, chapterID string) error {
	return s.withChapter(pluginID, mangaID, chapterID, "mark chapter read", func(chapterIntID int64) error {
		return s.db.MarkChapterRead(chapterIntID)
	})
}

// ToggleChapterRead toggles the read status of a single chapter, addressed by source ids.
func (s *AppService) ToggleChapterRead(pluginID, mangaID, chapterID string) error {
	return s.withChapter(pluginID, mangaID, chapterID, "toggle chapter read", func(chapterIntID int64) error {
		return s.db.ToggleChapterRead(chapterIntID)
	})
}

// SetChaptersRead marks (read=true) or unmarks (read=false) the given chapters,
// addressed by source ids.
func (s *AppService) SetChaptersRead(pluginID, mangaID string, chapterIDs []string, read bool) error {
	return s.withManga(pluginID, mangaID, "set chapters read", func(mangaIntID int64) error {
		return s.db.SetChaptersRead(mangaIntID, chapterIDs, read)
	})
}

// SetChaptersUpTo marks (or unmarks) every chapter up to the highest of the
// given chapters.
func (s *AppService) SetChaptersUpTo(pluginID, mangaID string, chapterIDs []string, read bool) error {
	return s.setChaptersTo(pluginID, mangaID, chapterIDs, read, false)
}

// SetChaptersDownTo marks (or unmarks) every chapter from the lowest of the given
// chapters onward.
func (s *AppService) SetChaptersDownTo(pluginID, mangaID string, chapterIDs []string, read bool) error {
	return s.setChaptersTo(pluginID, mangaID, chapterIDs, read, true)
}

func (s *AppService) setChaptersTo(pluginID, mangaID string, chapterIDs []string, read, down bool) error {
	dir, set := "up to", s.db.SetChaptersUpTo
	if down {
		dir, set = "down to", s.db.SetChaptersDownTo
	}
	return s.withManga(pluginID, mangaID, "set chapters "+dir, func(mangaIntID int64) error {
		return set(mangaIntID, chapterIDs, read)
	})
}

// SetMangaChaptersRead marks (or unmarks) every chapter of a manga.
func (s *AppService) SetMangaChaptersRead(pluginID, mangaID string, read bool) error {
	return s.withManga(pluginID, mangaID, "set manga chapters read", func(mangaIntID int64) error {
		return s.db.SetMangaChaptersRead(mangaIntID, read)
	})
}

// SetChapterTotalPages stores a chapter's page count so progress badges can
// render "N/M". Best-effort: callers may ignore the error.
func (s *AppService) SetChapterTotalPages(pluginID, mangaID, chapterID string, total int) error {
	return s.withChapter(pluginID, mangaID, chapterID, "set chapter total pages", func(chapterIntID int64) error {
		return s.db.SetChapterTotalPages(chapterIntID, total)
	})
}

// ResetChapterProgress clears a single chapter's read progress, addressed by
// source identifiers.
func (s *AppService) ResetChapterProgress(pluginID, mangaID, chapterID string) error {
	return s.withChapter(pluginID, mangaID, chapterID, "reset chapter progress", func(chapterIntID int64) error {
		return s.db.ResetChapterProgress(chapterIntID)
	})
}

// ToggleChapterSkip toggles the skip flag for a chapter.
func (s *AppService) ToggleChapterSkip(pluginID, mangaID, chapterID string) error {
	return s.withChapter(pluginID, mangaID, chapterID, "toggle chapter skip", func(chapterIntID int64) error {
		return s.db.ToggleChapterSkip(chapterIntID)
	})
}
