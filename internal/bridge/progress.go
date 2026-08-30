package bridge

import (
	"fmt"
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
