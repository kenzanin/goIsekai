// Package workers provides bounded, typed worker lanes for the app's
// concurrent work: interactive UI, plugin fetch, image fetch, maintenance.
//
// Lanes are independent pools (design D1): a full image queue can never
// block the interactive lane. Jobs are typed (design D2) and carry a
// context-aware Future result; the context controls waiting AND cancellation
// of the job while it is still queued.
package workers

import (
	"errors"
)

var (
	// ErrBusy is returned when a background lane rejects a job within its
	// short enqueue timeout (callers map it to 503).
	ErrBusy = errors.New("workers: queue full")
	// ErrClosed is returned after Shutdown.
	ErrClosed = errors.New("workers: pool closed")
	// ErrEnqueueTimeout is returned by the interactive lane when its ctx
	// deadline hits before the queue frees up (callers map it to 503).
	ErrEnqueueTimeout = errors.New("workers: enqueue timed out")
)

// ErrPermanent marks errors that must not be retried (rate limit exhausted,
// permanent upstream 404...). Wrap with Permanent().
type ErrPermanent struct{ Err error }

func (e *ErrPermanent) Error() string { return e.Err.Error() }

func (e *ErrPermanent) Unwrap() error { return e.Err }

// Permanent wraps err so the retry logic stops retrying it.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &ErrPermanent{Err: err}
}
