## Context
Plugin errors are logged but not surfaced to the UI. Users see empty results with no indication of failure.

## Goals
1. Every plugin error visible in the UI with message and retry
2. Challenge detection consistent across all views
3. Plugin health visible on plugins page

## Decisions

### D1: Error state in bridge layer
Add `LastError` and `ErrorCount` fields to the plugin metadata tracked by the bridge. Updated on every call (success resets, failure increments).

### D2: Template error partial
A shared `plugin-error.jet` partial included in search, detail, and library views. Receives plugin ID + error message.

### D3: Health tracking in-memory only
Plugin health (last success, error count) is tracked in-memory in the plugin manager. Not persisted to DB — resets on restart. Sufficient for debugging active issues.

## Risks
- Error count unbounded — cap at 999 display
- Health tracking adds minimal overhead per call (one time.Time + one int update)