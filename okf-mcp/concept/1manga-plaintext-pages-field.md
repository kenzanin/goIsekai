# 1manga: pages field arrives as plaintext JSON, not enc:v1 (2026-10-24)

## Situation

Reader showed an empty/error chapter for 1manga `the-king-of-cave-will-live-a-paradise-life:chapter-23.1`, but the same chapter on the website loaded fine (`1manga.har`: `api.mghcdn.com/graphql` → `pages:"{\"p\":\".../23.1/\",\"i\":[\"1a.jpg\",\"2b.jpg\",...]}"`).

## Root cause

The 1manga plugin only knew one authoritative pages shape: `enc:v1:<keyId>:<iv>:<tag>:<ciphertext>` (AES-256-GCM blob, key from `GET /api/chapter-crypto`). When that prefix check missed, the code fell through to the CDN **probe** (`1.jpg`, `1.jpeg`, …), which 404s for this manga because its pages are letter-suffixed (`1a.jpg`, `2b.jpg`, …). The reader then rendered an empty chapter with no stated cause.

The API had quietly changed: today it answers `chapter.pages` as **plaintext JSON** `{"p":"<prefix>/","i":["1a.jpg",...]}` for this content — the encrypted path was skipped, the probe was the wrong tool, and the failure was silent.

## Fix

`examples/plugins/lua/1manga/main.lua` (`get_page_list`) now handles both authoritative shapes:

- `enc:v1:` → decrypt via `encrypted_page_list` (unchanged), fail loudly with the reason on failure (do **not** probe).
- plaintext JSON string → `host.json.decode`, require `p` (string) + `i` (table), build `IMG_CDN .. p .. name`.
- only when the field is missing/unparseable does the old probe run — and the probe now **fails loudly** (`nil, "page_list_failed"`) instead of emitting a silently-empty list, consistent with the "an empty chapter with a stated cause beats a wrong chapter that reads as complete" rule that already applied to the encrypted path.

Synced to `app_data/plugins/1manga/main.lua`, reloaded via `POST /api/sandbox/plugins/1manga/reload` (no restart needed), verified live: 17 pages for chapter-23.1.

## Verified

- `GET /api/sandbox/plugins/1manga/pages/the-king-of-cave-will-live-a-paradise-life:chapter-23.1` → 17 entries, first `1a.jpg`, last `17d.jpg` (matches HAR file names).
- `GET /api/reader-data/...` → `pages: 17`; reader canvas renders.
- Chapter 22 (numeric `1.jpg…31.jpg`) still resolves through the same plaintext path.
- luacheck: 0 errors (pre-existing 35 style warnings); `luac -p` clean.

## Notes

- Commit: `345ceee fix(1manga): support plaintext JSON pages field in get_page_list` (not pushed).
- `enc:v1:` stays in the code: other MangaHub chapters still encrypt the list; the two shapes coexist per-chapter.
- Lesson generalises: for any site plugin, "unknown pages payload → probe" is only safe when the probe's assumption (contiguous numeric names) is actually checkable cheaply; otherwise fail loud.
