package bridge

import (
	"goisekai/internal/logger"
)

// GetLogs returns the buffered log lines (Go logger + forwarded [ui] webview
// messages), oldest first. The frontend Log page polls this to render live logs.
func (s *AppService) GetLogs() []string {
	return logger.GetLines()
}

// ClearLogs empties the in-memory log buffer.
func (s *AppService) ClearLogs() {
	logger.Clear()
}
