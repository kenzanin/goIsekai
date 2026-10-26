---
type: finding
title: Native .so plugins are self-contained with zero DT_NEEDED entries
description: Native .so plugins are self-contained - zig c++ against static musl gives a shared object with zero DT_NEEDED entries.
tags: [goisekai, plugins, native, zig, c++, build, toolchain, static-link, musl, dlopen, purego, finding]
generated: { by: agent/mcp, at: "2026-10-26T15:47:00Z" }
status: stable
---

## What changed

tools/build-native.sh now has three tiers, best first:

- tier 1 — zig c++ + static musl — zero DT_NEEDED entries
- tier 2 — zig c++ (glibc) — libc.so.6, ld-linux-x86-64.so.2
- tier 3 — g++ — libstdc++.so.6, libgcc_s.so.1, libm.so.6, libc.so.6

## Size

| plugin | g++ | zig glibc | zig + static musl |
|---|---|---|---|
| roliascan | 137,560 | 290,416 | 376,264 |
| madarascans | 273,816 | 572,688 | 696,424 |
| dummy | 79,920 | 256,768 | 328,048 |

~30% bigger than glibc build, ~2.7x g++. The .so carries its own libc, so the host needs no libc at all.

## The recipe

```sh
zig c++ -target x86_64-linux-musl -idirafter /usr/include \
    -std=c++17 -O2 -fPIC -ffunction-sections -fdata-sections \
    -c *.cpp -o main.o
zig c++ -target x86_64-linux-musl -shared -fPIC \
    -Wl,--gc-sections -Wl,-Bstatic /usr/lib/musl/lib/libc.a -Wl,-Bdynamic \
    main.o -o main.so
strip main.so
```

## Why the HOSTs musl and not zigs own

zigs bundled musl libc.a is NOT PIC - linking it into a .so fails with relocation R_X86_64_32S cannot be used against local symbol. The distros musl (musl-tools / musl-dev) ships a PIC libc.a and works. zigs own musl is fine for executables only.

## Traps

1. -I/usr/include breaks libc++ (glibc headers win). Use -idirafter.
2. version script (--version-script) segfaults: hides malloc so it calls musls uninitialised allocator.
3. -Bsymbolic segfaults: forces .so to bind malloc/free to its own musl copies; musls __init_libc never runs for dlopen'd .so. RTLD_LOCAL keeps definitions out of global scope so malloc still binds to hosts libc.
4. dynamically linked musl .so is useless on glibc host: NEEDED libc.so, host libc.so is a linker script not ELF. Only static musl .so works.

## Proof

- readelf -d: no DT_NEEDED on all plugins
- nm -D: 7 ABI names present
- Native ABI tests: 5/5 PASS
- Live: plugin loaded kind=go; search 200 (677B), detail 200 (1,089B), chapters 200 (90,743B, 500 rows)
- pages/10724 returns 500 (wrong id: 10724 is manga id, chapter id is 332027)

## built_with stamp

Now reads zig c++ musl 1.2.6, from /usr/lib/ld-musl-*.so.1 (gcc version print lied).
