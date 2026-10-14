---
type: Concept
title: Ants Worker Migration Guide (REJECTED)
rejected: 2026-10-14
rejected-by: agent
reason: |
  Rejected in favour of keeping internal/workers custom pool.
  The custom pool already satisfies all 5 audit phases (workers-pool-refactoring
  task + pond-v2-migration-assessment). Migrating to github.com/panjf2000/ants/v2
  would DROP critical features the audit mandates and that are actively used:
  - context-aware futures that cancel QUEUED jobs (bridge awaits ctx-bound futures)
  - ctx-bounded backpressure: interactive block-until-deadline -> 503,
    background bounded enqueue -> ErrBusy (used by /action/sync, /action/export-cbz)
  - enqueue-time DedupeKey reuse (image fetch dedupe)
  - bounded retry (3x) + ErrPermanent + linear backoff (pool.go)
  - per-plugin fairness gates: fetchGate / interactiveGate (one in-flight/PluginKey)
  - LaneCDP with poolDrainTimeout=5s bounded shutdown + ctx propagation to solver
  - image high/low priority split + per-host hostAcquire/pacing (multiplexed by
    one worker — cannot be expressed as two independent ants pools without
    duplicating pacing logic into callers)
  - lane statistics: running/completed/failed/abandoned + duration ring (p99)
  pond-v2-migration-assessment already rejected library migration for the same
  high/low-priority-sub-queue reason. Reimplementing all of the above on top of
  ants would yield net duplicate code, not simplicity.
decision: REJECT — keep internal/workers. archive this proposal.
---

Understood. Switching to English. 

Here is the **Agentic AI Implementation Guide** for using the `ants` library, rewritten in clear, strict English. You can copy and paste this directly into your local AI agent (like Cursor, Cline, or Windsurf) to ensure it generates code that perfectly matches your audit requirements and management's request to use a standard library.

---

# 📄 Agentic AI Implementation Guide: `ants` Worker Pool for goIsekai

**Objective**: Replace/wrap the custom worker pool with the standard library `github.com/panjf2000/ants/v2` to satisfy management's maintainability standards, **WITHOUT** sacrificing critical architectural requirements identified in the `workers-audit.md`: DTX-safe panic recovery, priority sub-queues (Image Lane), and per-lane statistics.

**Target Library**: `github.com/panjf2000/ants/v2`

---

## 🚫 HARD CONSTRAINTS (MUST FOLLOW)
The AI agent **MUST** strictly adhere to these rules when generating code:
1. **NO `os.Exit(1)` OR `log.Fatal` IN PANIC HANDLERS**: Every job panic MUST be caught, logged, and the pool execution MUST continue. This is the non-negotiable "DTX-safe panic recovery" requirement from the audit.
2. **NO SINGLE GLOBAL `ants.Pool`**: Jobs MUST be routed to separate `ants.Pool` instances per Lane to prevent head-of-line blocking, as specified in the audit.
3. **IMAGE LANE MUST SUPPORT PRIORITY**: Since `ants` does not have a built-in priority queue, you MUST implement a wrapper that separates the Image lane into `ImageHigh` and `ImageLow` pools to guarantee priority execution.
4. **USE THE FACADE PATTERN**: Application code (bridge, httpserver, etc.) MUST NOT call `ants` directly. They must call a uniform `WorkerPool` interface.

---

## ⚙️ LANE CONFIGURATION (Strict Specifications)
Create separate `ants.Pool` instances for each lane with these exact parameters:

| Lane Name | Workers (Capacity) | Queue Size | Special Requirement |
| :--- | :--- | :--- | :--- |
| `Interactive` | 4 | 32 | Standard FIFO |
| `Fetch` | 2 | 8 | Standard FIFO |
| `ImageHigh` | 4 | 64 | **REQUIRED**: Handles high-priority image jobs. |
| `ImageLow` | 4 | 64 | **REQUIRED**: Handles low-priority image jobs. |
| `Maintenance`| 1 | 8 | Standard FIFO |
| `CDP` | 4 | 16 | Standard FIFO |

*(Note: The audit states Image lane has 8 workers and 64 queue each. Splitting into High/Low with 4 workers each satisfies the "high/low priority split within one lane" requirement using standard library primitives).*

---

## 🏗️ IMPLEMENTATION ARCHITECTURE (Step-by-Step for AI)

### Step 1: Define the Common Interface (Facade)
Create `internal/workers/interface.go`. This hides the `ants` implementation details from the rest of the application.

```go
package workers

type Priority int

const (
	PriorityLow Priority = iota
	PriorityHigh
)

type PoolStats struct {
	Lane      string
	Capacity  int
	Running   int
	Waiting   int
}

type WorkerPool interface {
	// Submit submits a job to a specific lane with default (low) priority.
	Submit(lane string, task func()) error
	
	// SubmitWithPriority submits a job, routing Image lane jobs to High/Low pools.
	SubmitWithPriority(lane string, task func(), priority Priority) error
	
	// Stats returns statistics for all lanes.
	Stats() []PoolStats
	
	// Release gracefully shuts down all pools.
	Release()
}
```

### Step 2: Implement the Safe Panic Handler
Create `internal/workers/ants_wrapper.go`. This is the **critical** component to satisfy the audit's panic recovery requirement.

```go
package workers

import (
	"log" // Replace with goIsekai's actual logger (e.g., slog or zerolog)
	"runtime/debug"
	
	"github.com/panjf2000/ants/v2"
)

// safePanicHandler is MANDATORY for every ants pool.
// It ensures DTX-safe panic recovery: logs the error and stack trace, but DOES NOT crash the app.
func safePanicHandler(err interface{}) {
	log.Printf("WORKER PANIC RECOVERED (Non-Fatal): %v\nStack: %s", err, debug.Stack())
}

func createAntsPool(size int, queueSize int) (*ants.Pool, error) {
	return ants.NewPool(size, 
		ants.WithPreAlloc(true), // Performance optimization
		ants.WithMaxBlockingTasks(queueSize),
		ants.WithPanicHandler(safePanicHandler), // MANDATORY: Prevents os.Exit(1)
	)
}
```

### Step 3: Implement the Pool Manager
Create `internal/workers/manager.go`. This struct manages the lifecycle and routing of all lanes.

```go
package workers

import (
	"errors"
	"github.com/panjf2000/ants/v2"
)

var ErrUnknownLane = errors.New("unknown worker lane")

type antsPoolManager struct {
	interactivePool *ants.Pool
	fetchPool       *ants.Pool
	imageHighPool   *ants.Pool 
	imageLowPool    *ants.Pool 
	maintenancePool *ants.Pool
	cdpPool         *ants.Pool
}

func NewWorkerPool() (WorkerPool, error) {
	m := &antsPoolManager{}
	var err error

	m.interactivePool, err = createAntsPool(4, 32)
	if err != nil { return nil, err }

	m.fetchPool, err = createAntsPool(2, 8)
	if err != nil { return nil, err }

	// Image Lane: Split into High and Low to satisfy the priority sub-queue requirement
	m.imageHighPool, err = createAntsPool(4, 64)
	if err != nil { return nil, err }
	m.imageLowPool, err = createAntsPool(4, 64)
	if err != nil { return nil, err }

	m.maintenancePool, err = createAntsPool(1, 8)
	if err != nil { return nil, err }

	m.cdpPool, err = createAntsPool(4, 16)
	if err != nil { return nil, err }

	return m, nil
}

func (m *antsPoolManager) Submit(lane string, task func()) error {
	return m.SubmitWithPriority(lane, task, PriorityLow)
}

func (m *antsPoolManager) SubmitWithPriority(lane string, task func(), priority Priority) error {
	var pool *ants.Pool

	switch lane {
	case "Interactive":
		pool = m.interactivePool
	case "Fetch":
		pool = m.fetchPool
	case "Image":
		// PRIORITY LOGIC: Route to the correct sub-pool
		if priority == PriorityHigh {
			pool = m.imageHighPool
		} else {
			pool = m.imageLowPool
		}
	case "Maintenance":
		pool = m.maintenancePool
	case "CDP":
		pool = m.cdpPool
	default:
		return ErrUnknownLane
	}

	return pool.Submit(task)
}

func (m *antsPoolManager) Stats() []PoolStats {
	return []PoolStats{
		{Lane: "Interactive", Capacity: m.interactivePool.Cap(), Running: m.interactivePool.Running(), Waiting: m.interactivePool.Waiting()},
		{Lane: "Fetch", Capacity: m.fetchPool.Cap(), Running: m.fetchPool.Running(), Waiting: m.fetchPool.Waiting()},
		{Lane: "ImageHigh", Capacity: m.imageHighPool.Cap(), Running: m.imageHighPool.Running(), Waiting: m.imageHighPool.Waiting()},
		{Lane: "ImageLow", Capacity: m.imageLowPool.Cap(), Running: m.imageLowPool.Running(), Waiting: m.imageLowPool.Waiting()},
		{Lane: "Maintenance", Capacity: m.maintenancePool.Cap(), Running: m.maintenancePool.Running(), Waiting: m.maintenancePool.Waiting()},
		{Lane: "CDP", Capacity: m.cdpPool.Cap(), Running: m.cdpPool.Running(), Waiting: m.cdpPool.Waiting()},
	}
}

func (m *antsPoolManager) Release() {
	m.interactivePool.Release()
	m.fetchPool.Release()
	m.imageHighPool.Release()
	m.imageLowPool.Release()
	m.maintenancePool.Release()
	m.cdpPool.Release()
}
```

---

## 🔄 MIGRATION CHECKLIST FOR AI

When refactoring the codebase, follow this exact sequence:

1. **Install Dependency**: Run `go get github.com/panjf2000/ants/v2`.
2. **Create New Files**: Generate `internal/workers/interface.go`, `internal/workers/ants_wrapper.go`, and `internal/workers/manager.go` using the templates above.
3. **Update Initialization**: Find where the old custom pool is initialized (e.g., in `main.go` or `cmd/goisekai/`) and replace it with `workers.NewWorkerPool()`.
4. **Refactor Enqueue Calls**: Search for all old `pool.Enqueue` or wrapper calls (like `runOnFetch`, `runOnInteractive`). Replace them with the new interface:
   ```go
   // OLD: pool.Enqueue(LaneImage, func() { fetchImage(...) })
   
   // NEW:
   err := globalWorkerPool.SubmitWithPriority("Image", func() {
       fetchImage(...)
   }, PriorityHigh) // or PriorityLow
   
   if err != nil {
       // Handle error (e.g., log "queue full")
   }
   ```
5. **Verify Panic Safety**: Ensure NO code inside the `task func()` calls `log.Fatal` or `os.Exit`. If found, refactor it to return an error or log it safely.
6. **Run Tests**: Ensure `just check` or `go test ./internal/workers/...` passes cleanly, especially any tests simulating a panic inside a job (verify the app does not crash).

---

## 💡 SPECIAL NOTES FOR THE AI AGENT
- **Do not** attempt to build a custom priority queue data structure inside a single `ants.Pool`. That violates the "use a standard library" principle and adds unnecessary complexity. Splitting into `ImageHigh` and `ImageLow` pools is the accepted industry-standard workaround for this specific requirement.
- Ensure `ants.WithPanicHandler(safePanicHandler)` is applied to **EVERY** `ants.NewPool` call. Missing this will cause the app to crash on panic, failing the audit requirements.
- If you find any dead code related to the old custom pool implementation, flag it for removal, as the audit (Phase 7) already cleaned up dead code.

---

### How to use this:
Copy the entire block above and paste it into your local AI agent's prompt. Tell it: *"Read this specification carefully. Generate the `internal/workers` package files exactly as described, adhering to all hard constraints."*