# Reader read-ahead verification

Recorded evidence for tasks 4.3-4.5 of the `harden-runtime-contention-and-cancel`
change. Those tasks were previously marked done on the strength of a source-reading
test only, which can pin shape but not behaviour. This is the real check.

Everything below was run against an isolated instance on port 8090 with its own
`data_dir`, a copy of the production SQLite so cached chapter pages were available,
and headless Chrome over CDP. The production instance on 8080 was not touched.

## Why a browser was necessary

- the repo has no JS test runner (no `package.json`, no test script) and only two
  static JS files
- `cmd/goisekai/frontend/lib/reader.js` is a self-wiring IIFE: it fetches its own
  data, binds its own listeners, and exposes nothing on `window`
- the abort is an `AbortController` on a `fetch` — invisible to a Go test, because
  the browser owns the cancellation

So the check drives a real browser. Scripts live in
`scripts/reader-browser-checks/` and need no dependencies (node's built-in
`WebSocket` speaks CDP directly).

## 4.3 — read-ahead is cancelled on chapter switch

Chapter 22 loaded from the real server (18 pages), so the reader booted on the
genuine chapter and its read-ahead covered pages 2, 3 and 4. A second chapter's
read-ahead was also in flight from the spill rule. Those images were held open by
the CDP `Fetch` domain; the switch was then triggered through the real UI control
`#btn-next-ch`.

```
read-ahead in flight: 5
  -the-strongest-to-save-everyone/22/2.jpg
  -the-strongest-to-save-everyone/22/3.jpg
  -the-strongest-to-save-everyone/22/4.jpg
  -the-strongest-to-save-everyone/23/2.jpg
  -the-strongest-to-save-everyone/23/3.jpg

clicked next chapter: ok

abortedReadAheadAtClick: 5
abortedTotal:           5
errorTextsSeen:         ["net::ERR_ABORTED"]
readerVisibleError:     false
readerErrorOverlay:     none
pageCounter:            "1 / 5"
verdict:                PASS
```

All five failures report `canceled: true` and each was one of the requests held at
the moment of the click. The counter moved from `1 / 18` to `1 / 5`, so the switch
completed rather than stalling, and no error was shown.

**Negative check.** With `abortAllWarm()` removed from `commitChapter` (and
`reader.js.br` regenerated):

```
abortedReadAheadAtClick: 0
abortedTotal:           0
pageCounter:            "1 / 5"
verdict:                FAIL
```

So the check measures the abort rather than passing incidentally.

### Two instrumentation traps hit along the way

Both are recorded because they would silently produce a false PASS:

- `Fetch.requestPaused` and `Network.loadingFailed` do **not** share a `requestId`
  namespace in Chrome. Matching aborts by request id reported 0 even while 5 aborts
  were observed; matching on the upstream page URL fixed it.
- fulfilling `/api/reader-data` for the **initial** page load boots the reader on
  the canned chapter, so every later observation describes the wrong chapter. The
  first request is now passed through to the real server.

## 4.5 — a genuine read-ahead failure surfaces without blocking the chapter

Every read-ahead image was fulfilled with `502`; the display page was fulfilled
with a 1x1 PNG.

```
readAheadRequestsFailed: 5
pageCounter:             "1 / 18"
spinnerDisplay:          "none"
canvasPainted:           true
canvasSize:              "780x493"
errorPanelDisplay:       "none"
failedTextShown:         false
verdict:                 PASS
```

The chapter on screen finished loading and painted while every read-ahead request
failed.

`drawPage` builds a detached `new Image()` and paints it to `#page-canvas`, so
there is no `<img>` in the DOM to inspect. The honest signals are that the spinner
is hidden — which only happens from `im.onload` — and that the canvas has painted
pixels.

**Negative check.** `FAIL_DISPLAY=1` also fails the display page:

```
canvasPainted: false
verdict:       FAIL
```

## 4.4 — abandoned is distinguishable from failed

This one is server-side and cannot be observed from a browser: the classification
lives in `internal/bridge/image_fetch.go` and is decided by `ctx.Err()`. It was
checked against the running server with `curl`, which reproduces the same signal
(a client that goes away mid-request).

Abandoned — client disconnects while the upstream is still hanging:

```
$ curl --max-time 3 ".../image?pluginID=1manga&url=http%3A%2F%2F10.255.255.1%2Fblackhole.jpg&prio=high"
  http=000 total=3.001073s

server log: 2 x "image fetch abandoned"
            0 x "image fetch failed"
```

Genuine upstream failure — a real status from upstream:

```
$ curl --max-time 90 ".../image?pluginID=1manga&url=http%3A%2F%2F127.0.0.1%3A8090%2Fnope.jpg&prio=high"
  http=502 total=12.506479s

server log: 3 x "image fetch retrying"
            1 x "image bad status"
            0 x "image fetch abandoned"
```

The two are cleanly separated. A client that leaves is logged as abandoned and
never as a failure; an upstream error is retried while it is still cheap to do so
and then reported as a bad status.

That retry behaviour is also why the earlier image-latency fix is shaped the way it
is: the retry ladder is bounded by elapsed time, so a fast failure still gets its
three attempts while a slow one stops after ~35s instead of holding the reader for
two minutes.

## Reproducing

See `scripts/reader-browser-checks/README.md`. The short version: start a server
with a cached chapter, start headless Chrome with `--remote-debugging-port`, pass
the browser WebSocket URL as the first argument.

If you edit `reader.js` for any of this, run `just br` first — `brHandler` serves
`reader.js.br` whenever it exists, so a stale sibling makes the check test old code
while appearing to pass.