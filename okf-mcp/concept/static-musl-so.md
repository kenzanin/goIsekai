---
id: static-musl-so
title: Native .so plugins are self-contained - zig c++ against static musl gives a shared object with zero DT_NEEDED entries
type: finding
tags: [goisekai, plugins, native, zig, c++, build, toolchain, static-link, musl, dlopen, purego]
---

# Native .so plugins: a self-contained shared object

Request: *"jangan pakai gnu libc bro untuk zig nya gunakan musl libc check
lagi berapa size nya?"* - a follow-up to [native-zig-build.md](native-zig-build.md).

## What changed

`tools/build-native.sh` now has **three tiers**, best first:

| tier | toolchain | `DT_NEEDED` of the result |
|---|---|---|
| 1 | zig c++ + **static musl** | **none** |
| 2 | zig c++ (glibc) | `libc.so.6`, `ld-linux-x86-64.so.2` |
| 3 | g++ | `libstdc++.so.6`, `libgcc_s.so.1`, `libm.so.6`, `libc.so.6` |

Tier 1 is the interesting one and is now the default. The `.so` carries its
own libc, so the host needs **no libc at all** - neither glibc nor musl.

## Size

| plugin | g++ 16.2.1 | zig glibc | **zig + static musl** |
|---|---|---|---|
| `roliascan` | 137,560 B | 290,416 B | **376,264 B** |
| `madarascans` | 273,816 B | 572,688 B | **696,424 B** |
| `dummy` (fixture) | 79,920 B | 256,768 B | **328,048 B** |

~30 % bigger than the glibc build, ~2.7x g++. That is the price of carrying
a libc inside the file.

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

## Why the HOST's musl and not zig's own

zig's bundled musl `libc.a` is **not PIC** - linking it into a `.so` fails
with `relocation R_X86_64_32S cannot be used against local symbol`
(`__utc`, `strerror.o`). The distro's musl (`musl-tools` / `musl-dev`) ships
a PIC `libc.a` and works. zig's own musl is fine for *executables* only.

## Traps found the hard way

1. **`-I/usr/include` breaks libc++.** It puts glibc's `<string.h>` ahead of
   libc++'s own, and `<cstring>`/`<cwchar>`/`<cstdio>` fail with *"tried
   including <wchar.h> but didn't find libc++'s <wchar.h> header"*. Use
   `-idirafter /usr/include` instead - RapidJSON is found, libc++ keeps
   priority.

2. **A version script (`--version-script`) segfaults.** Limiting the exports
   to the 7 ABI names (343,096 B instead of 376,264 B) looked attractive, but
   the plugin then crashed with `SIGSEGV` on the first call, inside
   `purego.RegisterFunc`. The symbol hiding also hides the `.so`'s own
   `malloc`, so it calls musl's uninitialised allocator.

3. **`-Bsymbolic` segfaults too**, for the same reason: it forces the `.so`
   to bind `malloc`/`free` to its own musl copies, and musl's `__init_libc`
   never runs for a `dlopen`'d `.so`. The plain build is safe because
   `RTLD_LOCAL` keeps the `.so`'s definitions out of the global scope, so
   `malloc` still binds to the host's libc.

So: **do not** add `-Bsymbolic` or a version script to this build. The `.so`
legitimately exports ~80 musl/libc++ symbols; with `RTLD_LOCAL` they cannot
interpose the host.

4. **A dynamically linked musl `.so` is useless on a glibc host.** Building
   against musl's `libc.so` gives `NEEDED: libc.so`, and the host's
   `/usr/lib/libc.so` is a *linker script*, not an ELF: `dlopen` fails with
   `invalid ELF header`. Bundling musl's `libc.so` next to the plugin and
   setting `-rpath $ORIGIN` does load, but then two libcs live in one process
   and it segfaults. Only the **static** musl `.so` works.

## Proof

- `readelf -d` on all three plugins: **no `DT_NEEDED` entry at all**.
- `nm -D --defined-only`: the 7 ABI names are present
  (`Search`, `GetMangaDetail`, `GetChapterList`, `GetPageList`, `GetGenres`,
  `plugin_init`, `plugin_free`).
- Native ABI tests: **5/5 PASS**
  (`TestNativePluginRegistersLazilyWithMetadata`,
  `TestNativePluginCallCrossesABI`, `TestNativePluginReachesHostHelpers`,
  `TestNativePluginInstallAndHotLoad`,
  `TestNativePluginSurvivesInPlaceOverwriteOfItsSo`).
- Live on the running host (Go + purego `dlopen`, `RTLD_LOCAL`):
  `plugin loaded (lazy) id=roliascan kind=go version=1`;
  `search?q=one+piece` 200 (677 B), `detail/10724` 200 (1,089 B),
  `chapters/10724` 200 (90,743 B, 500 rows) - same byte sizes as the glibc
  build.
- `pages/10724` returns 500 *"the source site could not be reached"* - that
  is a **wrong id**, not a build problem: 10724 is a manga id, the chapter id
  is 332027.

## Note

musl's version comes from the loader, not from `musl-gcc --version` - that
one prints the **GCC** version (`gcc (GCC) 16.2.1`), which made the
`built_with` stamp lie. The stamp now reads `zig c++ musl 1.2.6`, read from
`/usr/lib/ld-musl-*.so.1`.
