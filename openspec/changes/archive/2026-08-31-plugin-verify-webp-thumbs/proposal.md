# Plugin Verify (v1) + WebP Cache + Per-Plugin Thumbnails

## Why

Three user-reported gaps after the HTTP-server migration:

1. **Human verification for protected sites**: Cloudflare-protected sources return challenge pages (403 + "Just a moment") that the tls-client fingerprint alone cannot pass. Users need a manual one-time verification flow whose session cookies are then reused by the host's per-plugin HTTP client.
2. **Disk cache size**: cached page images are stored as-is (progressive JPEGs, ~1 MB each). Converting to WebP at encode time shrinks the cache 30-60%.
3. **Thumbnail aspect ratio**: search/library cards use a fixed `h-64` slot with `object-cover`, cropping MangaDex covers (256×364). The plugin knows its source's cover ratio; the host should honor it.

## What Changes

- New `plugin_verify` DB table storing pasted verification cookies + optional browser User-Agent per plugin; ABI additions let plugins declare `verify_url` and `needs_human_verify`; the host injects those cookies (and UA override) into the per-plugin tls-client before requests; challenge responses surface a "needs verification" banner in search/detail views.
- `image.go` disk-cache write path converts non-animated, non-WebP images to WebP (quality 85) via `gen2brain/webp` (pure Go, CGO-free) before writing; `Content-Type: image/webp` on serve; existing cache entries left as-is.
- Plugins declare an optional `thumb_ratio` (w/h) in their Init response; stored in the `plugins` table; library/search cards use `aspect-ratio` styling instead of fixed height.
- `examples/mangadex-plugin` emits `thumb_ratio: 0.703` (256/364) and verify metadata.

## Capabilities

- plugin-runtime
