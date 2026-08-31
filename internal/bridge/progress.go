package bridge

import (
	"fmt"

	"goisekai/internal/database"
)

// RecordRead appends a read-history entry for a chapter's current page,
// addressed by source identifiers.
func (s *AppService) RecordRead(pluginID, mangaID, chapterID string, pageNum int) error {
	if err := s.db.RecordRead(chapterRowID(pluginID, mangaID, chapterID), pageNum); err != nil {
		return fmt.Errorf("bridge: record read: %w", err)
	}
	return nil
}

// SetChapterProgress records a chapter's last page read and marks it read,
// addressed by source identifiers.
func (s *AppService) SetChapterProgress(pluginID, mangaID, chapterID string, lastPage int) error {
	if err := s.db.SetChapterProgress(chapterRowID(pluginID, mangaID, chapterID), lastPage); err != nil {
		return fmt.Errorf("bridge: set chapter progress: %w", err)
	}
	return nil
}

// MarkChapterRead marks a single chapter as read, addressed by source ids.
func (s *AppService) MarkChapterRead(pluginID, mangaID, chapterID string) error {
	if err := s.db.MarkChapterRead(chapterRowID(pluginID, mangaID, chapterID)); err != nil {
		return fmt.Errorf("bridge: mark chapter read: %w", err)
	}
	return nil
}

// MarkChapterReadRange marks every chapter from fromChapterID up to (and
// including) toChapterID as read, in chapter_num order (order-independent).
func (s *AppService) MarkChapterReadRange(pluginID, mangaID, fromChapterID, toChapterID string) error {
	if err := s.db.MarkChapterReadRange(mangaRowID(pluginID, mangaID), fromChapterID, toChapterID); err != nil {
		return fmt.Errorf("bridge: mark chapter read range: %w", err)
	}
	return nil
}

// SetChapterTotalPages stores a chapter's page count so progress badges can
// render "N/M". Best-effort: callers may ignore the error.
func (s *AppService) SetChapterTotalPages(pluginID, mangaID, chapterID string, total int) error {
	return s.db.SetChapterTotalPages(chapterRowID(pluginID, mangaID, chapterID), total)
}

// GetChapterProgresses returns read progress keyed by source chapter id.
func (s *AppService) GetChapterProgresses(pluginID, mangaID string) (map[string]database.ChapterProgress, error) {
	rows, err := s.db.GetChapterProgressForManga(mangaRowID(pluginID, mangaID))
	if err != nil {
		return nil, err
	}
	out := make(map[string]database.ChapterProgress, len(rows))
	for _, r := range rows {
		out[r.SourceChapterID] = r
	}
	return out, nil
}
