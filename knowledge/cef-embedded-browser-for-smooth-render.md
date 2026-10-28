---
type: decision
title: CEF embedded browser for smooth UI rendering + CF bypass option
description: "CEF app using host's shared libcef.so (281MB) — primarily to fix image scroll jank in Wails/WebKit reader view. Secondary: use CEF for HTTP fetch to bypass CF via real browser traffic (not tunnel/tls-client)."
tags: [cef, libcef, rendering, scroll, image, wails, webkit, cloudflare, embedded-browser]
generated: { by: agent/mcp, at: "2026-10-28T13:24:00Z" }
status: stable
---

# CEF Embedded Browser — Smooth UI Rendering (Primary) + CF Bypass (Secondary)

## Context

- **Primary problem**: Wails desktop client (WebKitGTK) punya scroll jank/lag saat render gambar beruntun di reader view — ini masalah rendering, bukan Cloudflare.
- **Secondary context**: tls-client v1.15.1 + Chrome_146 profiles terjebak CF bot protection di beberapa situs. CDP challenge solving (obscura/lightpanda) time out 120s tanpa clear.

## Decision

Build standalone CEF app at `tmp/ui-cef/goisekai_app/`:
- Link ke **shared** `libcef.so` di `/usr/lib/cef/libcef.so` (bukan static)
- Default buka `http://localhost:8080` (goIsekai web UI)
- `--url=` CLI override support
- Compile `libcef_dll_wrapper` dari `/usr/src/cef/libcef_dll/`
- C++20 + GCC 16 flags (`-fpermissive`)

## Purpose 1: UI Rendering (primary)

CEF/Chromium memberi smooth rendering untuk image-heavy reader view — mengganti WebKit yang janky.

## Purpose 2: HTTP Fetch via CEF (secondary/optional)

Jika butuh fetch image dari situs dengan proteksi CF:
- Pakai CEF/Chrome headless → traffic tampak sebagai browser asli
- Bukan pakai cloudflared tunnel (menggunakan service orang)
- Bukan pakai tls-client (terjebak CF challenge)

## Build

```bash
cd tmp/ui-cef/goisekai_app/build && cmake -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Release ..
make -j$(nproc)
```

## Status: implemented + verified

- Binary: build/bin/goisekai_cef (2.4M)
- Links to libcef.so from /usr/lib/cef

# Related Concepts
- [Wails desktop client + tray wiring](wails-desktop-client-and-tray.md): CEF approach replaces Wails/WebKit for smooth reader view rendering where WebKit shows scroll jank on image-heavy content.
