---
type: finding
title: Chapter lists came out oldest-first for weebcentral and comizy
description: "Chapter lists came out oldest-first for weebcentral and comizy - two plugins reversed the site's newest-first fragment."
tags: [goisekai, plugins, chapters, abi, weebcentral, comizy, odin, bug, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:47:02Z" }
status: stable
---

# Chapter lists came out oldest-first for weebcentral and comizy

## The symptom was two symptoms

Both a library manga and a non-library one showed the same series in opposite orders:

| manga | path | order |
|---|---|---|
| in the library | DB cache, ORDER BY chapter_num DESC, id DESC | newest-first |
| not in the library | the plugin's own response, live | oldest-first |

## Root cause: two plugins broke the ABI convention

The repo convention is newest-first. 15 plugins say so in their own comments. Two plugins reversed the site's newest-first fragment and shipped ascending:

| plugin | what it did | site order |
|---|---|---|
| weebcentral | reversed the /full-chapter-list fragment | newest-first |
| comizy | reversed the /titles/{id}/chapters list | newest-first |

An audit of all 34 installed plugins against the live sandbox endpoint found these two and no others; 30 return newest-first. (Three plugins errored for unrelated reasons: anisascans, comix, onisaga.)

## The fix

Delete the reversal in all four sources. The sites already answer newest-first.

### 1. chapter_num where it was missing

weebcentral sent no chapter_num; comizy sent chapter_num=0 for every row. Both now send the real number. Progress is keyed off the chapter's source id (read_history joins chapters.id), so a real number detaches nothing.

comizy_chapter_num folds the hyphen into the decimal dot form, so Chapter 12-2 reads as 12.2. Measured over 355 comizy chapter rows: 330 plain, 25 split, largest tail 5, at most 2 halves per base.

### 2. Traps

- plugin_cache is not invalidated by a plugin change. GetChapterList responses live 168h. Must purge: DELETE FROM plugin_cache WHERE plugin_id IN ('weebcentral','comizy');
- The reader's page is paginated; chapter doc only carries max_page, so the list must come from the full-chapter-list fragment.
- text.chapter_num is a leading-number reader, wrong for Chapter 12-2.
- Odin port uses a proc alias: comizy_chapter_num :: proc (title: string) -> f64 { … } calling into abi.

## Verified live

After swap (same plugin id and plugin.json name): weebcentral 75 chapters newest-first, chapter_num unique 75/75; comizy 14 chapters newest-first, unique 14/14. After re-sync DB holds real numbers including split chapters.

## Regression test

internal/pluginmanager/weebcentral_chapter_order_test.go: TestLuaWeebcentralChaptersStayNewestFirst (offline), TestWasmWeebcentralChaptersStayNewestFirst / TestWasmComizyChaptersStayNewestFirst (live).
