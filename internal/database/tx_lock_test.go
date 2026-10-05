package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Task 1.1: the connection string carries immediate transaction locking, which
// is what makes a contending writer wait at BEGIN instead of failing at its
// first write.
func TestDSNCarriesImmediateTxLock(t *testing.T) {
	got := dsn("/tmp/x.db")
	if !strings.Contains(got, "_txlock=immediate") {
		t.Fatalf("dsn() = %q, want _txlock=immediate", got)
	}
	// The pre-existing pragmas must survive the edit.
	for _, want := range []string{"_journal_mode=WAL", "_busy_timeout=5000", "_foreign_keys=1"} {
		if !strings.Contains(got, want) {
			t.Errorf("dsn() = %q, lost %s", got, want)
		}
	}
}

// Task 1.2: a transaction from Begin() holds the write lock before it issues
// any write. Two handles on one file: while the first transaction is open and
// silent, the second cannot take the lock. Under deferred locking the second
// Begin would succeed, because a deferred transaction takes no lock until it
// writes.
func TestBeginTakesWriteLockBeforeFirstWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()

	// A second handle, so the two transactions do not share a connection.
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	tx, err := a.Begin()
	if err != nil {
		t.Fatal(err)
	}
	// Note: no write has been issued on tx yet.
	if _, err := tx.Exec(`INSERT INTO plugins (id, name, version, wasm_path, is_active) VALUES ('p', 'P', '1', '/tmp', 1)`); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// b must not be able to acquire the write lock now. A short timeout keeps
	// the test fast; success here is the failure this task guards against.
	_, err = sqlBeginWithTimeout(t, b, 300*time.Millisecond)
	if err == nil {
		t.Fatal("second handle acquired the write lock while an uncommitted write transaction was open;" +
			" transactions are deferred, not immediate")
	}
	if !isBusyError(err) {
		t.Fatalf("second Begin error = %v, want a busy/locked error", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
}

// Task 1.4: once the first transaction commits, the waiting writer proceeds
// rather than failing. busy_timeout absorbs the wait.
func TestContendingWriterSucceedsAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	first, err := a.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec(`INSERT INTO plugins (id, name, version, wasm_path, is_active) VALUES ('p1', 'P1', '1', '/tmp', 1)`); err != nil {
		t.Fatalf("first write: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		tx, err := b.Begin()
		if err != nil {
			done <- err
			return
		}
		_, err = tx.Exec(`INSERT INTO plugins (id, name, version, wasm_path, is_active) VALUES ('p2', 'P2', '1', '/tmp', 1)`)
		if err == nil {
			err = tx.Commit()
		}
		done <- err
	}()

	time.Sleep(200 * time.Millisecond)
	if err := first.Commit(); err != nil {
		t.Fatalf("first commit: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("contending writer failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("contending writer never completed")
	}

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM plugins`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("plugins rows = %d, want 2 (both writers committed)", n)
	}
}

// Task 1.5: a writer that cannot take the lock within busy_timeout returns an
// error and leaves nothing behind.
func TestTimedOutBeginLeavesNoRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	hold, err := a.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hold.Exec(`INSERT INTO plugins (id, name, version, wasm_path, is_active) VALUES ('holder', 'H', '1', '/tmp', 1)`); err != nil {
		t.Fatalf("holder write: %v", err)
	}

	// b has the app default busy_timeout of 5s, so this wait must fail.
	if _, err := b.Begin(); err == nil {
		t.Fatal("Begin succeeded while the write lock was held past busy_timeout")
	} else if !isBusyError(err) {
		t.Fatalf("Begin error = %v, want a busy/locked error", err)
	}

	if err := hold.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM plugins`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("plugins rows = %d, want 0: the timed-out transaction left rows behind", n)
	}
}

// sqlBeginWithTimeout opens a transaction on d after temporarily lowering its
// busy_timeout, so contention tests do not wait out the 5-second application
// default. The timeout is always restored before returning.
func sqlBeginWithTimeout(t *testing.T, d *DB, timeout time.Duration) (*sql.Tx, error) {
	t.Helper()
	// PRAGMA does not accept bind parameters, so the value is interpolated.
	set := func(ms int) {
		q := fmt.Sprintf(`PRAGMA busy_timeout = %d`, ms)
		if _, err := d.db.Exec(q); err != nil {
			t.Fatalf("busy_timeout=%d: %v", ms, err)
		}
	}
	set(int(timeout.Milliseconds()))
	defer set(defaultBusyTimeoutMS)
	return d.db.Begin()
}

// isBusyError reports whether err is SQLite's busy/locked condition.
func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "busy")
}
