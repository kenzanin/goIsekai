---
type: Concept
title: Wails desktop client + tray wiring
description: "Wails v2 desktop client (cmd/goisekai-wails, nested module) + tray wiring replacing Qt6. WebKitGTK tag webkit2_41, AVIF->JPEG display transcode, reader strip scroll native, logs server-rendered. Tray config renamed client_qt6->client_wails, green/red emoji status."
tags: [goisekai, wails, desktop, webkit, tray, avif]
generated: { by: agent/mcp, at: "2026-10-26T16:00:08Z" }
status: stable
---

# Wails desktop client + tray wiring

Qt6 client is gitignored/broken; desktop client is now Wails v2 (thin webview over the running server).

## Layout

- cmd/goisekai-wails/ is a nested Go module (own go.mod) so Wails deps never enter server go.mod (server stays CGO-free).
- just build-wails: CGO_ENABLED=1 go build -tags "desktop production webkit2_41". Tags mandatory, plain go build refuses.
- Output bin/goisekai-wails (~5.6MB). Wails v2.12.0 CLI at ~/go/bin/wails.

## WebKitGTK gotchas

1. Tag = webkit2_41 (system has webkit2gtk-4.1; GTK4 = Wails v3).
2. AVIF decode -> scroll-wheel lag. image_format=avif stores AVIF; WebKitGTK decodes on main thread. Fix: ClientDisplayFormat() transcodes AVIF/JXL -> JPEG q85 for display only. Content-Type image/jpeg. Test: imgcodec_display_test.go.
3. Reader strip scroll: reader.js wheel handler does if (stripMode) return; zones pointer-events:none. Any reader.js edit needs just br.
4. Logs: views/logs.lua server-renders lane cards.
5. White-on-white selects: base.lua :root { color-scheme: dark }.

## Tray wiring

- goisekai.ini [tray] client_qt6 renamed to client_wails (accepts both). Value: bin/goisekai-wails.
- Tray open launches clientBin with -server <url>.
- Server status: green/red emoji in Status menu line.

## Library 5x5 grid

- Web grid: grid-cols-5 + sidebar w-56.
- Covers: w-full aspect-[2/3]; pageSize=25.
- "grid shows 1x25" = persisted list view mode (localStorage), not layout bug.

## Deploy

- frontend/lib/* edit -> just br -> just build.
