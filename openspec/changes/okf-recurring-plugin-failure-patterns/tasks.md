# Tasks — okf-recurring-plugin-failure-patterns

## 1. Create the concept

- [x] 1.1 Write `okf-mcp/concept/plugin-failure-patterns.md` with full frontmatter
      (`id`, `title`, `description`, `type: Concept`, `tags`, `generated.at`).
- [x] 1.2 Add the six patterns per design.md (symptom / silent-failure signal /
      root cause / fix / references), plus the cross-cutting "fails silently" note.
- [x] 1.3 Document the fixed find_all row shape in `three-lua-plugins-converted-to-tinygo-wasm`
      (mangabuddy pages gAll fix) and `plugin-failure-patterns` Pattern 1 (wasmRegexRow).

## 2. Wire into the bundle

- [ ] 2.1 Add the concept entry to `okf-mcp/index.md`.
- [ ] 2.2 Re-run `qmd update -c okf-mcp` so BM25 can find it.
- [ ] 2.3 Validate the bundle (frontmatter/description/tags/index/dangling-refs all 0).

## 3. Finish

- [ ] 3.1 Set `skip_specs: true` in the change's `.openspec.yaml` (docs-only).
- [ ] 3.2 `openspec validate` the change; commit + push.
