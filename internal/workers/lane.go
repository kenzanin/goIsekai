// Package workers provides bounded, typed worker lanes for the app's
// concurrent work: interactive UI, plugin fetch, image fetch, maintenance.
//
// Lanes are independent pools (design D1): a full image queue can never
// block the interactive lane. Jobs are typed (design D2) and carry a
// context-aware Future result; the context controls waiting AND cancellation
// of the job while it is still queued.
package workers

// Lane identifies which pool a job runs on.
type Lane int

const (
	LaneInteractive Lane = iota
	LaneFetch
	LaneImage
	LaneMaintenance
	LaneCDP // anti-bot challenge solving (Phase 4)
)

// Priority orders the two image sub-queues (high drained before low).
type Priority int

const (
	PriorityLow Priority = iota
	PriorityHigh
)

// JobStatus is the observable lifecycle state (design D6).
type JobStatus int

const (
	StatusQueued JobStatus = iota
	StatusRunning
	StatusDone
	StatusFailed
	StatusDead
)
