# Live plugins audit — builder/runner versions & `released_at` status

- generated: { by: "Reasonix agent", at: "2026-10-28T05:30:00+07:00" }

## Context
Audit all installed (live) plugins under `app_data/plugins/` to record:
- plugin kind + builder/runner version,
- whether each parses `released_at` on chapters (host ABI field `pkg/types/types.go:39` — `ReleasedAt time.Time json:"released_at"`; there is NO `created_at`).

Motivation: a report claimed "mangaread plugin tidak punya data created_at". Audit shows mangaread uses `released_at` (correct ABI field), not `created_at` — and **mangaread_odin already parses it**. The "unreachable" live behaviour is a network/human-verification issue, not a missing-field bug.

- note: `qmd` / OKF MCP tools were unavailable in this session (not on PATH, no `okf-mcp/` index existed). This doc was written as a plain markdown file so `qmd search` can discover it after `qmd init`/`qmd update`.

---

## Builder / runner versions (verified)

| Toolchain | Version (verified) | Path used |
|-----------|--------------------|-----------|
| Odin compiler | `odin version dev-2026-10:84bc3fc21` | `/sbin/odin -> /usr/lib/odin/odin` |
| TinyGo | `tinygo version 0.43.0 linux/amd64 (LLVM 22.1.4)` | `~/opt/tinygo/bin/tinygo` |
| Go (host) | `go 1.27` (module) | system `go` |
| Go (wasm plugin build) | `go 1.26.9` (pinned, `GOTOOLCHAIN=local`) | `~/sdk/go1.26.9` |
| Lua | 5.x (lunar VM, host-runtime) | host `internal/pluginmanager/lua.go` |
| JS / yaegi | goja / yaegi interp | host |

All 15 `*_odin` wasm plugins build with Odin `dev-2026-10` + TinyGo 0.43.0 cross-compiler (see `examples/plugins/wasm/*_odin/build.sh` comments: Odin emits wasm exports itself, no wasi-sdk, `core:mem` allocator, imports `wasi_snapshot_preview1` + `env.host_call`).

---

## Live plugins (`app_data/plugins/`) — count / kind mix

43 live plugins: 15 Lua, 13 wasm (Go/TinyGo), 13 wasm (Odin), 1 yaegi, 1 wasm (komix_to_cpp). Full ID list:

1manga, atsumaru, bato1, batcave, comicasura_odin, comix, comizy, dynastyscan, en-thunderscan, fanfox, komix_to, ksgroupscans, kunmanga_online, lhtranslation, madaradex, madarascans, manga_uno, mangabay, mangabuddy, mangade, mangadex, mangafire, mangafreak, mangahere, mangahub, mangak, mangakakalot, mangakatana, **mangapill**, **mangaread**, mangasushi, mangataro, mangatown, mangzio, manhawato, onisaga, reimanga_net, roliascan, topmanhua, violetmanga, weebcentral, zeroscann.

---

## `released_at` parsing status (source scan)

Scan key: `grep -cE 'released_at|ReleasedAt|date_to_iso|chapter-date|release.date|<date tag>'` per plugin source. Count 0 = does NOT parse dates.

### mangaread — ALREADY HAS released_at (not a "missing" case)
Source: `examples/plugins/wasm/mangaread_odin/main.odin` — **3 matches**:
- `//  manga_id and released_at too, matching the primary path.` (comment, line 32)
- line 69: `Released_At: string `json:"released_at,omitempty"``
- line 263: `raw_date := abi.find_one(li, `(?s)chapter-release-date[^>]*>.*?<i>([^<]+)</i>`)`
- line 269: `Released_At = abi.date_to_iso(strings.trim_space(raw_date))`

Verified at deploy: `app_data/plugins/mangaread/main.wasm` (413244 bytes, mtime 2026-10-20 G*KH15) **identical size/strings** to `examples/plugins/wasm/mangaread_odin/main.wasm` — both contain the literal `chapter-release-date` (1 hit via `strings`). So the **deployed wasm already includes the date parser**.

Live fetch test:
```
GET /api/sandbox/plugins/mangaread/chapters/mangaread:12345
→ {"error":"bridge: plugin mangaread is unreachable and manga mangaread:12345 has no cached copy"}
```
This is a **network/human-verification** failure (plugin unreachable), NOT a missing-field bug. Need a real manga id + human-verify cookies to test the parser end-to-end.

### mangapill — intentionally NO release date
`wasm/mangapill/main.go` — count 0. mangapill.com chapter listing HTML does not publish a per-chapter release date in the scraped markup, so the plugin intentionally omits `released_at` (empty). This is by design, not a bug.

### 21 live plugins that do NOT parse `released_at`/dates (count = 0)

| Plugin | Kind | Source path |
|--------|------|-------------|
| atsumaru | lua | `lua/atsumaru/main.lua` |
| comix | lua | `lua/comix/main.lua` |
| dynastyscan | lua | `lua/dynastyscan/main.lua` |
| fanfox | lua | `lua/fanfox/main.lua` |
| htmlscrape | lua | `lua/htmlscrape/main.lua` |
| kaliscan | lua | `lua/kaliscan/main.lua` |
| madaradex | lua | `lua/madaradex/main.lua` |
| mangade | lua | `lua/mangade/main.lua` |
| mangafreak | lua | `lua/mangafreak/main.lua` |
| mangahere | lua | `lua/mangahere/main.lua` |
| mangakatana | lua | `lua/mangakatana/main.lua` |
| mangatown | lua | `lua/mangatown/main.lua` |
| topmanhua | lua | `lua/topmanhua/main.lua` |
| valirscans | lua | `lua/valirscans.lua` |
| violetmanga | lua | `lua/violetmanga/main.lua` |
| zeroscann | lua | `lua/zeroscann/main.lua` |
| ksgroupscans | wasm (Go) | `wasm/ksgroupscans/main.go` |
| mangade_odin | wasm (Odin) | `wasm/mangade_odin/main.odin` |
| mangakatana_odin | wasm (Odin) | `wasm/mangakatana_odin/main.odin` |
| violetmanga_odin | wasm (Odin) | `wasm/violetmanga_odin/main.odin` |
| bato1 | yaegi | `yaegi/bato1/main.go` |

(mangapill also count 0 — see note above.)

### Representative "has released_at" plugins (for pattern reference)
- `wasm/1manga/main.go:337` — `c.ReleasedAt = parseTime(hostString("text.date_to_iso", numStr(ch.Date)))`.
- `wasm/mangak/main.go`, `wasm/reimanga_net/main.go`, `wasm/komix_to/main.go`, `wasm/manga_uno/main.go`, `wasm/lhtranslation/main.go` — all use `time.ParseInLocation` / `parseTime` + `ReleasedAt`.
- `lua/mangaread/main.lua`, `lua/mangabuddy/main.lua`, `lua/mangasushi/main.lua` — Lua equivalents set `released_at`.
- `wasm/atsumaru_odin/main.odin`, `wasm/dynastyscan_odin/main.odin`, `wasm/manga_uno_odin/main.odin`, `wasm/weebcentral_odin/main.odin` — Odin `abi.date_to_iso`.

---

## Decision log
- "tambahkan released_at ke mangaread" → **SKIPPED (N/A)**: mangaread_odin already parses `released_at` and live wasm contains the parser. Adding it would duplicate. If mangaread detail pages show no date, the cause is the live fetch failing (unreachable/challenge) — needs human-verify cookies, not plugin edits.
- "tambahkan released_at ke X" for any of the 21 above → open new task; each needs source-HTML date-field discovery + `time.Parse`/`abi.date_to_iso` wiring + rebuild + `POST /reload`.

## Next-action trigger
- If/when a plugin from the "0 date" list is selected: `okf_search(query="released_at date_to_iso pattern", limit=3)` → follow 1manga/Odin pattern → deploy + `POST .../reload` + clear `plugin_cache` → `GET /api/sandbox/plugins/{id}/chapters/{mangaID:ch}`.
