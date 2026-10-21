---
id: native-zig-build
title: Native .so plugins build with zig c++ against static musl - a self-contained .so with zero DT_NEEDED entries
type: finding
tags: [goisekai, plugins, native, zig, c++, build, toolchain, static-link, musl]
---

# Native .so plugins now build with zig c++

Request: *"check plugin native .so, coba compile dengan zig c++ biar bisa
dibuat static build"*.

## What changed

`tools/build-native.sh` is the new single entry point: it builds any native
plugin with **zig c++** and falls back to **g++** when zig is missing. The
three plugin `build.sh` files (`roliascan`, `madarascans`, `dummy`) are now
thin wrappers over it, so the toolchain choice lives in one place - the same
shape `tools/build-wasm.sh` already has for TinyGo.

## Size, dependencies, toolchain

| plugin | g++ 16.2.1 | zig 0.18.0-dev.253 | zig deps |
|---|---|---|---|
| `roliascan` | 137,560 B | **290,416 B** | `libc.so.6` only |
| `madarascans` | 273,816 B | **572,688 B** | `libc.so.6` only |
| `dummy` (fixture) | 79,920 B | **256,768 B** | `libc.so.6` only |

g++ links `libstdc++.so.6` + `libgcc_s.so.1` + `libm.so.6`; zig links its own
libc++ and libgcc **statically**, so the artifact depends on `libc.so.6` only.
That is the portability win: a host without a matching libstdc++ can still
load the plugin. The cost is roughly 2x the file size.

Build time, same source, same machine:

| | cold | warm |
|---|---|---|
| g++ | 5.9 s | 5.9 s |
| zig | 38 s | **74-87 ms** (its cache) |

## Full static .so: not reachable, but "no shared dependency" IS

`zig c++ -static -shared` does not produce a shared object - it emits an **ar
archive** (`file` says "current ar archive"), which `dlopen` cannot load. A
`.so` needs PIC relocations in the first place, so a fully static *executable*
is contradictory.

**Correction (later session):** a fully static *executable* is out of reach, but
a **self-contained shared object** - a `.so` with **zero `DT_NEEDED` entries** -
is not, and is what the project now builds. See
[static-musl-so.md](static-musl-so.md) for the musl recipe and its traps.

For comparison, `g++ -static-libstdc++ -static-libgcc` also reaches
`libc.so.6` only, at 490,304 B for roliascan - so g++ can do the same job, it
just is not the default.

## The trap: -fvisibility=hidden is per-plugin, not global

The first zig build of the `dummy` fixture failed the native ABI tests with
`symbol plugin_free missing`. The cause was **my flag choice, not zig**:
`dummy/build.sh` deliberately does *not* pass `-fvisibility=hidden` (its
`extern "C"` block must stay exported as written), while `roliascan` and
`madarascans` do pass it and mark their entry points with the `EXPORT` macro
from `util.h`.

So the visibility flag is part of each plugin's contract, and the shared tool
takes it as an argument instead of deciding it. With the same flags as
`dummy/build.sh`, the zig build passes **5/5** native ABI tests
(`TestNativePluginRegistersLazilyWithMetadata`,
`TestNativePluginCallCrossesABI`, `TestNativePluginReachesHostHelpers`,
`TestNativePluginInstallAndHotLoad`,
`TestNativePluginSurvivesInPlaceOverwriteOfItsSo`).

## Behaviour is identical, proven by A/B against the live site

roliascan, g++ build vs zig build, same running host, same query:

| endpoint | result |
|---|---|
| `search?q=one+piece` | **byte-identical** (677 B) |
| `detail/10724` | **byte-identical** (1,089 B) |
| `chapters/10724` | 500 rows, identical after normalising `released_at` |

`released_at` differed by 30 s between the two runs - and it also differs
between **two consecutive fetches of the same g++ build** (0/500 rows
identical on that second comparison), because the site stamps the chapter
upload time per response. It is site-side drift, not a build difference.

The zig-built plugin was then deployed to `app_data` and the host loaded it
live: `plugin loaded (lazy) id=roliascan kind=go version=1`, search returned
"One Piece" (id 10724) and "The Perks of Being a Villainess" (id 160380).

`madarascans` returns `[]` for the same query under **both** builds - that is
the site, not the toolchain.

## The toolchain is visible to the user

`built_with` in each `plugin.json` is stamped by the build, and the plugins
page renders it as a badge (`internal/templates/partials/plugin_cards.lua`).
It now reads `zig 0.18.0-dev.253+16d31ceed` instead of `g++ 16.2.1`, so a
user can tell which compiler produced the artifact without stat-ing bytes.

## Gate

`go test ./internal/pluginmanager` green with the zig-built fixture,
`git diff --check` / `gofmt` / `go vet` clean, and `bash -n` clean on all
four shell scripts.
