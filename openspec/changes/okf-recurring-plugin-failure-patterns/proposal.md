# Proposal

## Why

The OKF knowledge bundle shows the same ~6 plugin failure modes resurfacing across
multiple sessions and plugins (regex shape, released_at zero-time, Lua→RE2 regex,
stale plugin_cache, wasm toolchain drift, sandbox decodeString). Each time they are
re-diagnosed from scratch, wasting effort and letting silent plugin failures reach
the UI as empty lists. This change captures them as a single quick-reference concept
so a plugin author — human or agent — can pattern-match a symptom to its known cause
and fix immediately.

## What Changes

- Add one new OKF concept: `okf-mcp/concept/plugin-failure-patterns.md` — a
  symptom→root-cause→fix reference for the recurring plugin failure modes.
- Each pattern carries the detection signal (why it is silent), the fix, and the
  OKF/source references to drill into.
- Wire the concept into `okf-mcp/index.md` and rebuild the qmd BM25 index.
- Documentation/knowledge only — no code or behavior change.

## Capabilities

### New Capabilities

None. This is a pure documentation change (a knowledge-bundle concept), so no
spec-level behavior changes. The change declares `skip_specs: true`.

### Modified Capabilities

(none)

## Impact

- Affected code: `okf-mcp/concept/`, `okf-mcp/index.md`, qmd index (regenerable).
- No runtime, API, database, or dependency change.
- Audience: anyone porting or debugging a goIsekai plugin.
