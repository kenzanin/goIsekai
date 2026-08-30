// Package logger provides a small multi-level logger built on the standard
// library's log/slog. Call Init once at startup, then use Debug/Info/Warn.
// All records are also appended to an in-memory ring buffer (GetLines) so the
// frontend Log page can display Go + webview logs without touching the terminal.
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// ring capacity for in-memory log retention.
const bufSize = 2000

var (
	mu    sync.Mutex
	lines = make([]string, 0, bufSize)
)

// appendLine stores one formatted log line in the ring buffer, evicting the
// oldest when full. Thread-safe; called by the capture handler on every record.
func appendLine(line string) {
	mu.Lock()
	defer mu.Unlock()
	lines = append(lines, line)
	if len(lines) > bufSize {
		lines = lines[len(lines)-bufSize:]
	}
}

// GetLines returns a copy of the buffered log lines (oldest first). The returned
// slice is safe to hold; it does not alias the internal buffer.
func GetLines() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

// Clear empties the in-memory log buffer.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	lines = lines[:0]
}

// captureHandler writes each record to stderr (normal) AND to the ring buffer.
// It forwards Enabled to the underlying handler so level filtering still applies.
type captureHandler struct {
	inner slog.Handler
}

func (h captureHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h captureHandler) Handle(ctx context.Context, r slog.Record) error {
	appendLine(formatRecord(r))
	return h.inner.Handle(ctx, r)
}

func (h captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return captureHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h captureHandler) WithGroup(name string) slog.Handler {
	return captureHandler{inner: h.inner.WithGroup(name)}
}

// formatRecord renders a record to a single human-readable line, e.g.
// "2026/08/30 12:00:00 INFO msg key=value".
func formatRecord(r slog.Record) string {
	ts := r.Time.Format("15:04:05")
	var b strings.Builder
	b.WriteString(ts)
	b.WriteByte(' ')
	b.WriteString(strings.ToUpper(r.Level.String()))
	b.WriteByte(' ')
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(fmt.Sprint(a.Value.Any()))
		return true
	})
	return b.String()
}

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
// text records to stderr at that level and captures a copy into the in-memory
// ring buffer (see GetLines). It returns the parse error when levelStr invalid.
func Init(levelStr string) error {
	l, err := ParseLevel(levelStr)
	if err != nil {
		return err
	}
	base := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})
	slog.SetDefault(slog.New(captureHandler{inner: base}))
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

// Fatal logs at the error level and exits with status 1.
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
