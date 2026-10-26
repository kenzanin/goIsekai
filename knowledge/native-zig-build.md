---
type: Concept
title: Native .so plugins build with zig c++ against static musl
description: "Native .so plugins now build with zig c++ against static musl for zero DT_NEEDED entry, with portability + build-cache trade-offs."
tags: [goisekai, plugins, native, zig, c++, build, toolchain, static-link, musl, finding, Concept]
generated: { by: agent/mcp, at: "2026-10-26T15:44:18Z" }
status: stable
---

# Native .so plugins build with zig c++ against static musl - a self-contained .so with zero DT_NEEDED entries

Request: "check plugin native .so, coba compile dengan zig c++ biar bisa dibuat static build".

## What changed

tools/build-native.sh is the new single entry point: builds any native plugin with zig c++ and falls back to g++ when zig is missing. The three plugin build.sh files (roliascan, madarascans, dummy) are now thin wrappers over it, so the toolchain choice lives in one place.

## Size, dependencies, toolchain

| plugin | g++ 16.2.1 | zig 0.18.0-dev.253 | zig deps |
|---|---|---|---|
| roliascan | 137,560 B | 290,416 B | libc.so.6 only |
| madarascans | 273,816 B | 572,688 B | libc.so.6 only |
| dummy (fixture) | 79,920 B | 256,768 B | libc.so.6 only |

g++ links libstdc++.so.6 + libgcc_s.so.1 + libm.so.6; zig links its own libc++ and libgcc statically, so the artifact depends on libc.so.6 only — the portability win at roughly 2x the file size.

Build time: g++ 5.9s; zig 38s cold / 74-87ms warm (cache).

## Full static .so: not reachable, but "no shared dependency" IS

zig c++ -static -shared emits an ar archive (dlopen cannot load it); a fully static executable is contradictory with PIC relocations. A self-contained shared object with zero DT_NEEDED entries is reachable — see static-musl-so.md. g++ -static-libstdc++ -static-libgcc also reaches libc.so.6 only.

## The trap: -fvisibility=hidden is per-plugin, not global

dummy/build.sh deliberately does NOT pass -fvisibility=hidden (extern "C" must stay exported); roliascan and madarascans do and mark entry points with the EXPORT macro. So the visibility flag is each plugin's contract; the shared tool takes it as an argument. With dummy/build.sh flags, zig build passes 5/5 native ABI tests.

## Behaviour identical — A/B vs live site

roliascan g++ vs zig, same host, same query: search?q=one+piece byte-identical (677 B); detail/10724 byte-identical (1,089 B); chapters/10724 500 rows identical after normalizing released_at (differs by 30s site-side per fetch, not toolchain). Deployed live; loaded as kind=go.

## Toolchain visible to user

built_with in plugin.json is stamped by the build; plugins page renders it. Now reads zig 0.18.0-dev.253+16d31ceed instead of g++ 16.2.1.

## Gate

go test ./internal/pluginmanager green with zig-built fixture; git diff --check / gofmt / go vet clean; bash -n clean on all four shell scripts.
