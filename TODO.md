# goIsekai — Remaining Work

## 🟢 Nice to Have

### Reader.js error handling
- No try-catch on critical paths (fetch reader-data, canvas draw). Blank canvas on error with no user feedback.

### Request logging middleware
- No logging for slow requests, error rates, or traffic patterns.

### Remove duplicate pagination
- Pagination appears above AND below manga grid. Top one is redundant on page 1.

## ✅ Completed (from prior sessions)
- [x] SPA genre form submit fix
- [x] Reset enrichment fix
- [x] Detail page normalization (cover, genres, author, synopsis)
- [x] Enrichment panel (alt titles, summaries, genres, related)
- [x] FetchEnrichment stores titles + summaries
- [x] Author label prefix
- [x] Mark read/unread works
- [x] Skip chapter feature (DB schema, migration, handler, route, UI, computeContinue)
- [x] AGENTS.md updated with tool preferences
- [x] Fix "Show more" genre toggle (moved JS to alpine-components.js)
- [x] Skip chapter tests (db + handler)
