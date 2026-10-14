---
status: draft
type: concept
category: architecture
---

# Pond v2 Migration Assessment

## Summary

Pond v2 is a modern Go worker pool library with bounded/unbounded queues, task groups, subpools, and dynamic resizing. It does NOT have an explicit "QueueGroup" concept, but offers equivalent functionality via `TaskGroup` and `Subpool`.

## Key Findings

### Pond v2 API

| Component | Purpose |
|---|---|
| `Pool` | Main pool for fire-and-forget or error-returning tasks |
| `ResultPool` | Type-safe pool for tasks returning typed results |
| `TaskGroup` | Coordinate related tasks, wait for all/first error |
| `Subpool` | Hierarchical concurrency with own limits |

### QueueGroup Replacement

Pond v2 does NOT have a "QueueGroup" feature, but offers:

1. **TaskGroup** - logical grouping for batch completion
2. **Subpool** - separate concurrency limits from parent pool

Each subpool acts like a "lane" with its own dedicated worker capacity.

### Migration Steps

1. Add `github.com/alitto/pond/v2` dependency
2. Replace `pool.Submit()` / `pool.SubmitErr()` / `resultPool.Submit()`
3. Use `pool.NewGroup()` for task grouping
4. Use `pond.WithQueueSize()`, `pond.WithNonBlocking()` for config
5. `pool.StopAndWait()` for graceful shutdown

### Comparison to Current Pool

| Feature | Current goIsekai Pool | Pond v2 |
|---|---|---|
| Multiple queues | 4 explicit lanes | Subpools (shared workers) |
| Per-lane sizing | Fixed workers per lane | Dynamic with subpool limits |
| Queue size | Fixed per lane | `WithQueueSize()` option |
| Non-blocking | Custom logic | `TrySubmit()` / `WithNonBlocking()` |

## Decision

**Chosen approach: Pond v2 Subpools** (Option C)

| Option | Verdict | Reason |
|---|---|---|
| A: Pond per lane (4 ponds) | Rejected | Unnecessary complexity |
| B: Keep current pool, add pond for new jobs | Rejected | Mixed implementations |
| C: Pond subpools for lanes | **Accepted** | Single parent pool + 4 subpools = per-lane isolation + shared workers |

## Design Spec

**Architecture**: Single parent pool + 4 subpools (one per lane).

| Lane | Pond Pattern | Workers | Queue |
|---|---|---|---|
| Interactive | `Subpool` | 4 | 32 |
| Fetch | `Subpool` | 2 | 8 |
| Image | `Subpool` | 8 | 64 |
| Maintenance | `Subpool` | 1 | 8 |

- Parent pool workers = 15 (sum of all subpool workers)
- Subpools get isolated concurrency limits
- Workers are shared across subpools dynamically

## Implementation Plan

1. **Phase 1**: Add Pond v2 dependency
2. **Phase 2**: Create parent pool + 4 subpools
3. **Phase 3**: Replace `pool.Enqueue()` calls with subpool Submit
4. **Phase 4**: Move non-pool jobs (5 items) to pool
5. **Phase 5**: Add CDP-solve pool
6. **Phase 6**: Refactor duplicate worker sizing

## References

- https://github.com/alitto/pond
- DeepWiki assessment completed