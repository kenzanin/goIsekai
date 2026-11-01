# Tasks

## 1. ABI: route the release date by type

- [ ] 1.1 `pkg/types/types.go`: add `ReleasedText string` tagged `json:"-"`; give `Chapter` a custom `UnmarshalJSON` that reads `released_at` as `json.RawMessage` and dispatches — number → epoch seconds, RFC 3339 string → `time.Time`, any other string → `ReleasedText`; treat a number outside `1970-01-01`..`2100-01-01` as no date and log a warning
- [ ] 1.2 `pkg/types`: tests for epoch seconds, RFC 3339, prose retained, an epoch-millisecond number rejected, and a fully populated chapter round-trip (this one guards the shadow struct against dropping a field)
- [ ] 1.3 Verify: `go test ./pkg/types/ -count=1`

## 2. Persistence

- [ ] 2.1 `internal/database/schema.go`: add `released_text TEXT` at migration index 32, with the same idempotent shape as the neighbouring column migrations
- [ ] 2.2 Regenerate the jet model under `internal/database/.gen/` rather than editing it
- [ ] 2.3 `internal/database`: carry `ReleasedText` on the model, in the upsert column list and in `chapterReleasedAt`'s sibling so a chapter stores either a moment or a wording
- [ ] 2.4 Tests: a wording round-trips; `released_at` stays NULL when only a wording is stored; both stay empty when the source reports nothing
- [ ] 2.5 Verify: `go test ./internal/database/ -count=1`

## 3. Propagation and render

- [ ] 3.1 `internal/bridge`: carry `ReleasedText` where chapters are rebuilt from cache (`service.go`, `library_fallback.go`), and through `migrate.go`'s insert
- [ ] 3.2 `internal/templates/partials/detail_chapter_row.lua`: when a chapter has no moment but has a wording, expose the wording through the string branch the row already handles
- [ ] 3.3 Tests: a cached chapter with a wording renders it; a chapter with a moment still renders the formatted date
- [ ] 3.4 Verify: `go test ./internal/bridge/ ./internal/httpserver/ -count=1`

## 4. Gate, restart, smoke-test

- [ ] 4.1 `just check && just build` green; restart per the deploy recipe and confirm the root page returns HTTP 200
- [ ] 4.2 Smoke-test a manga detail page: a chapter with an epoch/ISO date shows the formatted date, and the chapter row still renders when a chapter carries no date at all
- [ ] 4.3 Confirm no existing row was rewritten: chapters stored before the change keep their `released_at` and have an empty `released_text`
