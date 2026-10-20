---
id: chapter-order-newest-first-abi
title: Chapter lists came out oldest-first for weebcentral and comizy - two plugins reversed the site's newest-first fragment
type: finding
tags: [goisekai, plugins, chapters, abi, weebcentral, comizy, odin, bug]
---

# Chapter lists came out oldest-first for weebcentral and comizy

Reported as "plugin weebcentral index manga chapter nya kebalik" - the chapter
list on a manga detail page read oldest-first, and the order seemed to change
depending on whether the manga was in the library.

## The symptom was two symptoms

Both a library manga and a non-library one showed the same series in opposite
orders:

| manga | path that served the list | order |
|---|---|---|
| in the library | DB cache, `ORDER BY chapter_num DESC, id DESC` | newest-first |
| not in the library | the plugin's own response, live | **oldest-first** |

So the same series read 74, 73, 72 … in one place and 1, 2, 3 … in the other.

## Root cause: two plugins broke the ABI convention

The repo's convention is **newest-first**, and the host depends on it:

- `internal/database/chapters_query.go` - the cached list is sorted
  `ORDER BY chapter_num DESC, id DESC`;
- `internal/httpserver/reader.go` - `chapterNeighbors` walks the list assuming
  newest-first, so the older chapter (prev) sits at `i+1`;
- `internal/httpserver/views_manga_continue.go` - `computeContinue` keeps the
  **last** unread seen, which is the lowest number in a newest-first list.

Fifteen plugins say so in their own comments (`atsumaru`: *"The ABI requires
newest-first. The site returns ascending order, which would make 'continue
reading' resume from the oldest chapter."*; `mangadex`: *"ABI contract:
chapters newest-first"*).

**Two plugins reversed the site's newest-first fragment** and shipped the
result as ascending:

| plugin | what it did | site order |
|---|---|---|
| `weebcentral` | reversed the `/full-chapter-list` fragment | newest-first |
| `comizy` | reversed the `/titles/{id}/chapters` list | newest-first |

Both carried a comment asserting the opposite of the convention — *"the reader
wants ascending"* / *"the ABI wants ascending"*. The reader wants newest-first:
that is exactly what makes "continue reading" resume from the newest chapter.

An audit of all 34 installed plugins against the live sandbox endpoint found
these two and no others; 30 return newest-first. (Three plugins errored for
unrelated reasons: `anisascans` an encoding fault, `comix` and `onisaga` HTTP
500.)

## The fix

Delete the reversal in all four sources — `examples/plugins/lua/weebcentral/main.lua`,
`examples/plugins/wasm/weebcentral_odin/main.odin`,
`examples/plugins/lua/comizy/main.lua`,
`examples/plugins/wasm/comizy_odin/main.odin`. The sites already answer in the
ABI order, so the list is delivered as it comes.

Two things had to be added at the same time, or the cached path would have
started flipping instead:

### 1. `chapter_num` where it was missing

The cached list sorts by `chapter_num`, so a plugin that ships no number leaves
the order to the `id` tie-break — which is *reverse insertion order*. `weebcentral`
sent no `chapter_num` at all; `comizy` sent `chapter_num = 0` for every row.
Both now send the real number.

The `comizy` Odin header justified the zeros with a claim that is **false**:
*"reading progress is keyed off chapter_num, so inventing a number here would
silently detach the reader"*. Progress is keyed off the chapter's **source id**
(`GetChapterProgressForManga` joins `read_history` on `chapters.id`), so a real
number detaches nothing.

### 2. comizy's split-chapter labels

The site labels a split chapter `Chapter 12-2` and a decimal one `Chapter 12.1`.
`text.chapter_num` reads the **leading number only**, so both halves of a split
chapter collapsed onto 12 — six rows collided in one manga, and their order
fell back to the id tie-break. `comizy_chapter_num` folds the hyphen into the
decimal the dot form already uses, so `Chapter 12-2` reads as 12.2.

Measured over the 355 comizy chapter rows: 330 plain, 25 split, largest tail
5, at most 2 halves per base, and no base using both the dot and the hyphen
form — so the fold is unambiguous.

## Traps hit on the way

- **`plugin_cache` is not invalidated by a plugin change.** `GetChapterList`
  responses live 168 h under `plugin_id + manga_id + function_name`, so the
  stale ascending JSON kept being served after the fix. It has to be purged:
  `DELETE FROM plugin_cache WHERE plugin_id IN ('weebcentral','comizy');`
- **The reader's page is paginated** and its chapter doc only carries
  `max_page`; the list had to come from the full-chapter-list fragment.
- **`text.chapter_num` is a leading-number reader, not a label parser.** It is
  the right helper for `Chapter 74` and the wrong one for `Chapter 12-2`.
- The Odin port has no `using import`, but a plain procedure alias works:
  `comizy_chapter_num :: proc (title: string) -> f64 { … }` calling into `abi`.

## Verified live

After the swap (same plugin id and `plugin.json` name, so library and progress
survive):

- `weebcentral` — 75 chapters, `Chapter 74` first down to `Chapter 1`,
  `chapter_num` unique 75/75;
- `comizy` — 14 chapters, `Chapter 14` first down to `Chapter 1`, unique 14/14;
- after a re-sync the DB holds real numbers, split chapters included:
  `14.2 (Chapter 14-2)`, `14.1 (14-1)`, `13.2`, `13.1`, `12.2`, `12.1`,
  `11.1 (Chapter 11.1)`, `10.0 (Chapter 10)`;
- the rendered detail pages are newest-first on **both** paths: the library
  manga shows `Ch. 74, 73, 72 … 25` and the non-library manga (the live plugin
  path that used to show `Ch. 1, 2, 3, 4`) shows `Ch. 86, 85, 84 … 37`.

## Regression test

`internal/pluginmanager/weebcentral_chapter_order_test.go`:

- `TestLuaWeebcentralChaptersStayNewestFirst` — offline, a 3-row
  `/full-chapter-list` fragment served newest-first; asserts the delivered
  order, the numbers and the ids;
- `TestWasmWeebcentralChaptersStayNewestFirst` / `TestWasmComizyChaptersStayNewestFirst`
  — live (`GOISEKAI_LIVE=1`), assert descending order, a non-zero number and
  no duplicate numbers (the latter is what catches a broken split-chapter
  fold).
