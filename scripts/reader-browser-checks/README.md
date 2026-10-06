# Reader browser checks

Two real-browser checks for the reader's read-ahead behaviour. They exist because
the repo has no JS test runner and `cmd/goisekai/frontend/lib/reader.js` is a
self-wiring IIFE that exposes nothing, so a Go or source-reading test can only
pin shape, never behaviour. These drive headless Chrome over CDP using node's
built-in `WebSocket` — **zero dependencies**, nothing to install.

They cover tasks 4.3-4.5 of the `harden-runtime-contention-and-cancel` change,
which were previously shape-pinned only.

## Why they are deterministic

Both use the CDP `Fetch` domain to control timing, so nothing races:

- page 1 (the display page) is fulfilled instantly with a 1x1 PNG, so the
  reader always paints without touching the network
- read-ahead images (page >= 2) are **held open**, so read-ahead is genuinely
  in flight when the script acts
- in `abort-on-chapter-switch.mjs` only, a chapter switch's `reader-data` is
  served from a canned payload — but the **first** request is passed through to
  the real server, otherwise the reader would boot on the canned chapter and
  every later observation would describe the wrong chapter

## Running them

Start a server on a port with a real plugin and cached chapter pages, then:

```sh
# One-time: a browser to drive. Leave it running; the scripts close only the
# page target they create, not the browser.
google-chrome --headless=new --remote-debugging-port=9333 \
  --user-data-dir=/tmp/chrome-checks --no-sandbox --disable-gpu about:blank &

WS=$(curl -s http://127.0.0.1:9333/json/version | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["webSocketDebuggerUrl"])')

M=after-being-reborn-i-became-the-strongest-to-save-everyone
BASE=http://127.0.0.1:8090 \
READER="$BASE/view/read/1manga/$M/$M:chapter-22" \
CANNED_DATA="$(cat canned-next-chapter.json)" \
  node scripts/reader-browser-checks/abort-on-chapter-switch.mjs "$WS"
```

`CANNED_DATA` is a `reader-data` payload for the *next* chapter with a
**different page count** than the current one, so a completed switch is visible
in the page counter instead of being indistinguishable from no switch at all.

`read-ahead-failure-isolated.mjs` needs no canned payload:

```sh
BASE=http://127.0.0.1:8090 READER="$BASE/view/read/..." \
  node scripts/reader-browser-checks/read-ahead-failure-isolated.mjs "$WS"
```

## What each one asserts

`abort-on-chapter-switch.mjs` (task 4.3)

- read-ahead images were in flight when the switch happened
- every one of them was aborted (`Network.loadingFailed` with `canceled`)
- the switch completed — page counter moved
- no reader-visible error afterwards

`read-ahead-failure-isolated.mjs` (task 4.5)

- at least 3 read-ahead requests genuinely failed (502)
- the chapter on screen still loaded: spinner hidden, canvas painted, counter
  correct, error panel not shown

## Negative checks

Both scripts are verified to fail when the behaviour is removed, so a green run
means something:

- remove `abortAllWarm()` from `commitChapter` in `reader.js`, regenerate
  `reader.js.br` with `just br`, and `abort-on-chapter-switch.mjs` reports
  `abortedReadAheadAtClick: 0` and exits 1
- run `read-ahead-failure-isolated.mjs` with `FAIL_DISPLAY=1` and it reports
  `canvasPainted: false` and exits 1

Note the `.br` sibling: `brHandler` serves `reader.js.br` whenever it exists, so
editing `reader.js` without `just br` ships the old code and the check silently
tests stale behaviour.

## Task 4.4 is not here on purpose

"Abandoned read-ahead is distinguishable from a real failure" is a
**server-side** classification in `internal/bridge/image_fetch.go`, decided by
`ctx.Err()`. The browser cannot observe it, so it is checked with `curl` against
a running server instead — see `docs/reader-read-ahead-verification.md` for the
recorded output.