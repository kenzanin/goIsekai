# harimanga — Madara Theme Plugin

**Status: NOT installable from this host. Code is complete and loads; the site is unreachable.**

The plugin parses fine and every ABI call is implemented, but `harimanga.cc`
sits behind a Cloudflare managed challenge that this host cannot pass. Nothing
here is a half-finished implementation — it is blocked upstream.

## Why it is blocked

`harimanga.cc` answers every request with a Cloudflare interstitial. The
challenge never completes, and it does not even issue a cookie:

| Attempt | Result |
|---|---|
| plain `curl` | `403 "Just a moment..."` |
| `curl` with browser UA | `403` |
| `obscura fetch` | `Just a moment...` |
| `obscura --stealth fetch` | `Just a moment...` |
| `obscura --stealth scrape --timeout 110` | `Just a moment...` in **1318ms** |
| goIsekai + Chrome CDP solver | cookies harvested, retry still `403` |
| goIsekai + `obscura serve --stealth` | solver timeout, then lightpanda timeout |
| real Chrome, 45s wall clock | `Just a moment...`, **zero cookies** |

Two things rule out a fingerprint problem:

1. `obscura scrape` returned in 1318ms despite a 110s timeout — the engine does
   not wait for the challenge at all, so the timeout is irrelevant.
2. Real Chrome ran for 45 real seconds and produced **no cookies whatsoever**, not
   even `cf_clearance`. A fingerprint problem would still run the challenge and
   still issue cookies. Zero cookies means the request is rejected before the
   challenge is served — an IP reputation block.

`--stealth` is therefore not the answer, and neither is headless-vs-headful or a
longer timeout.

## Why the site is in this state

The Mihon/Tachiyomi extension repo once shipped a `harimanga` extension. It was a
plain `Madara` subclass with **no anti-bot handling whatsoever** — no cookies, no
JS solving, no special headers:

```kotlin
class Harimanga : Madara("Harimanga", "https://harimanga.me", "en") {
    override val client = super.client.newBuilder().rateLimit(3).build()
    override val useLoadMoreRequest = LoadMoreStrategy.Never
}
```

It was deleted on 2026-06-13 in commit `ffca4b1de` ("Remove Harimanga,
ManhwaClan, Kun Manga", #16667), alongside two other sites, with no reason given
in the commit body.

The domain moved:

```
https://harimanga.me/  →  200 "Account Suspended"   (789 bytes, Hostinger)
https://harimanga.cc/  →  403 "Just a moment..."     (5.4 KB, Cloudflare)
```

So the sequence was: the extension worked → `.me` was suspended → the operator
moved to `.cc` behind Cloudflare → Mihon deleted the extension instead of fixing
it. This plugin is byte-for-byte the same Madara approach Mihon shipped, which
means Mihon never had a way through either.

## What would make it work

Only a different egress IP. `harimanga.cc` is the successor of a domain that was
already suspended once, so treat it as unstable.

The plumbing already exists and is not the blocker: `obscura` accepts
`--proxy <PROXY>`, and goIsekai has its own proxy settings under `[network]`.
Point either at a residential proxy (a datacenter VPN will be detected the same
way) and the plugin should work as-is. No code change is needed for that, which
is why no proxy plumbing was added here — it would be dead code.

## How the plugin works

Standard WordPress + Madara, the same shape as `anisascans` and `mangaread`:

- **Search**: GET `/?s=<query>&post_type=wp-manga` → `.c-tabs-item__content`
  cards, title from `.post-title h3 a`, cover from `.summary_image img`.
- **Detail**: GET `/manga/<slug>/` → `.post-content_item` label/value rows.
- **Chapters**: POST `/manga/<slug>/ajax/chapters/` with an empty body and
  `X-Requested-With: XMLHttpRequest` → `li.wp-manga-chapter` rows, newest-first,
  full list in one response.
- **Pages**: GET `/manga/<slug>/chapter-<n>/` → `img.wp-manga-chapter-img`, with
  the real URL in `data-src` because images are lazy-loaded.

`get_genres` scrapes the site's own navigation instead of hardcoding a list, so it
self-corrects when the theme reorganises.

### Unverified detail

The genre archive path is assumed to be `/manga-genre/<slug>/`, copied from
`anisascans`. It could not be confirmed against `harimanga.cc` because the site
never served a page. Search and genre browsing are the only paths that differ
from `anisascans`; detail, chapters and pages are the shared Madara contract.

`host.text.chapter_num` must keep matching decimal chapter numbers (`[\d.]+`, not
`\d+`), so sub-chapters such as `7.1` are not dropped.

## Do not install it as-is

Installing this would put a plugin in the UI that loads and then fails every
call. That is the same shape as the retired `kaliscan` entry. Leave it here as
reference until an egress route exists.
