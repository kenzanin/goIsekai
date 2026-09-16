package bridge

import "fmt"

// MarkChapterRead marks a single chapter as read, addressed by source ids.
func (s *AppService) MarkChapterRead(pluginID, mangaID, chapterID string) error {
	chapterIntID, err := s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
	if err != nil {
		return fmt.Errorf("bridge: resolve chapter: %w", err)
	}
	if err := s.db.MarkChapterRead(chapterIntID); err != nil {
		return fmt.Errorf("bridge: mark chapter read: %w", err)
	}
	return nil
}

// SetChaptersRead marks (read=true) or unmarks (read=false) the given chapters,
// addressed by source ids.
func (s *AppService) SetChaptersRead(pluginID, mangaID string, chapterIDs []string, read bool) error {
	mangaIntID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	if err := s.db.SetChaptersRead(mangaIntID, chapterIDs, read); err != nil {
		return fmt.Errorf("bridge: set chapters read: %w", err)
	}
	return nil
}

// SetChaptersUpTo marks (or unmarks) every chapter up to the highest of the
// given chapters.
func (s *AppService) SetChaptersUpTo(pluginID, mangaID string, chapterIDs []string, read bool) error {
	mangaIntID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	if err := s.db.SetChaptersUpTo(mangaIntID, chapterIDs, read); err != nil {
		return fmt.Errorf("bridge: set chapters up to: %w", err)
	}
	return nil
}

// SetMangaChaptersRead marks (or unmarks) every chapter of a manga.
func (s *AppService) SetMangaChaptersRead(pluginID, mangaID string, read bool) error {
	mangaIntID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	if err := s.db.SetMangaChaptersRead(mangaIntID, read); err != nil {
		return fmt.Errorf("bridge: set manga chapters read: %w", err)
	}
	return nil
}

// SetChapterTotalPages stores a chapter's page count so progress badges can
// render "N/M". Best-effort: callers may ignore the error.
func (s *AppService) SetChapterTotalPages(pluginID, mangaID, chapterID string, total int) error {
	chapterIntID, err := s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
	if err != nil {
		return fmt.Errorf("bridge: resolve chapter: %w", err)
	}
	return s.db.SetChapterTotalPages(chapterIntID, total)
}

// ResetChapterProgress clears a single chapter's read progress, addressed by
// source identifiers.
func (s *AppService) ResetChapterProgress(pluginID, mangaID, chapterID string) error {
	chapterIntID, err := s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
	if err != nil {
		return fmt.Errorf("bridge: resolve chapter: %w", err)
	}
	if err := s.db.ResetChapterProgress(chapterIntID); err != nil {
		return fmt.Errorf("bridge: reset chapter progress: %w", err)
	}
	return nil
}

// ToggleChapterSkip toggles the skip flag for a chapter.
func (s *AppService) ToggleChapterSkip(pluginID, mangaID, chapterID string) error {
	chapterIntID, err := s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
	if err != nil {
		return fmt.Errorf("bridge: resolve chapter: %w", err)
	}
	if err := s.db.ToggleChapterSkip(chapterIntID); err != nil {
		return fmt.Errorf("bridge: toggle chapter skip: %w", err)
	}
	return nil
}
