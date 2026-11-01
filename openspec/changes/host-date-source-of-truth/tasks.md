# Tasks

## 1. ABI: route the release date by type

- [x] 1.1 `pkg/types/types.go`: add `ReleasedText string` tagged `json:"-"`; give `Chapter` a custom `UnmarshalJSON` that reads `released_at` as `json.RawMessage` and dispatches — number → epoch seconds, RFC 3339 string → `time.Time`, any other string → `ReleasedText`; treat a number outside `1970-01-01`..`2100-01-01` as no date and log a warning
- [x] 1.2 `pkg/types`: tests for epoch seconds, RFC 3339, prose retained, an epoch-millisecond number rejected, and a fully populated chapter round-trip (this one guards the shadow struct against dropping a field)
- [x] 1.3 Verify: `go test ./pkg/types/ -count=1`

## 2. Persistence

- [x] 2.1 `internal/database/schema.go`: add `released_text TEXT` at migration index 32, with the same idempotent shape as the neighbouring column migrations
- [x] 2.2 Regenerate the jet model under `internal/database/.gen/` rather than editing it
- [x] 2.3 `internal/database`: carry `ReleasedText` on the model, in the upsert column list and in `chapterReleasedAt`'s sibling so a chapter stores either a moment or a wording
- [x] 2.4 Tests: a wording round-trips; `released_at` stays NULL when only a wording is stored; both stay empty when the source reports nothing
- [x] 2.5 Verify: `go test ./internal/database/ -count=1`

## 3. Propagation and render

- [x] 3.1 `internal/bridge`: carry `ReleasedText` where chapters are rebuilt from cache (`service.go`, `library_fallback.go`), and through `migrate.go`'s insert
- [x] 3.2 `internal/templates/partials/detail_chapter_row.lua`: when a chapter has no moment but has a wording, expose the wording through the string branch the row already handles
- [x] 3.3 Tests: a cached chapter with a wording renders it; a chapter with a moment still renders the formatted date
- [x] 3.4 Verify: `go test ./internal/bridge/ ./internal/httpserver/ -count=1`

## 4. Gate, restart, smoke-test

- [x] 4.1 `just check && just build` green; restart per the deploy recipe and confirm the root page returns HTTP 200
- [x] 4.2 Smoke-test a manga detail page: a chapter with an epoch/ISO date shows the formatted date, and the chapter row still renders when a chapter carries no date at all
- [x] 4.3 Confirm no existing row was rewritten: chapters stored before the change keep their `released_at` and have an empty `released_text`
