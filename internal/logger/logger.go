// Package logger provides a small multi-level logger built on the standard
// library's log/slog. Call Init once at startup, then use Debug/Info/Warn.
package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// levels maps the string names accepted by configuration to slog levels.
// "warn" is accepted as an alias for "warning".
var levels = map[string]slog.Level{
	"debug":   slog.LevelDebug,
	"info":    slog.LevelInfo,
	"warning": slog.LevelWarn,
	"warn":    slog.LevelWarn,
}

// ParseLevel converts a case-insensitive, whitespace-trimmed level name into a
// slog.Level. Unknown names return an error.
func ParseLevel(s string) (slog.Level, error) {
	l, ok := levels[strings.ToLower(strings.TrimSpace(s))]
	if !ok {
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
	return l, nil
}

// Init parses levelStr and installs a process-wide default logger that writes
// text records to stderr at that level. It returns the parse error when
// levelStr is invalid.
func Init(levelStr string) error {
	l, err := ParseLevel(levelStr)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
	return nil
}

// Debug logs at the debug level via the process-wide default logger.
func Debug(msg string, args ...any) { slog.Debug(msg, args...) }

// Info logs at the info level via the process-wide default logger.
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Warn logs at the warning level via the process-wide default logger.
func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

// Error logs at the error level via the process-wide default logger.
func Error(msg string, args ...any) { slog.Error(msg, args...) }
