---
type: Decision
title: substitute_cipher is a hex token on every runtime
description: "crypto.substitute_cipher returns lowercase hex on encrypt and takes hex on decrypt in Lua, JS, yaegi and wasm alike, because raw cipher bytes do not survive the quickjs string boundary; no shipped plugin used it at the time."
tags: [goisekai, abi, crypto, substitute-cipher, hex, quickjs, decision]
generated: { by: agent/cli, at: "2026-11-01T12:45:02Z" }
status: stable
---

## Decision

`pluginutil.SubstituteCipher(data, materialJSON, direction)` hex-encodes its output on `encrypt` and hex-decodes its input on `decrypt`. Plaintext stays text in both directions. This is the same convention `crypto.xor` already used. Every runtime shares the one implementation: Lua, JS, yaegi/go, wasm.

## Why

Cipher output is arbitrary bytes, not valid UTF-8. The JS runtime replaced them with U+FFFD at the string boundary, so decrypt received garbage. A JS-only hex wrapper was tried first and rejected: it left Lua with raw bytes and JS with hex, an ABI split that the parity test could not catch (`helper_parity_test.go` never mentions `substitute_cipher`). The user chose one ABI everywhere.

## Blast radius

A grep of shipped Lua, JS and wasm plugin sources found **no plugin calling this helper**; only host code and tests referenced it, so nothing broke. A non-hex decrypt input returns `substitute_cipher: decrypt input must be hex: ...`. `knowledge/doc_plugin-wasm-helpers.md` carries a two-sentence note. The OpenSpec delta for the engine change did **not** record this ABI change, so a plugin author reading only the spec would not know.

## Caveat

Plaintext that is not valid UTF-8 still cannot round-trip through JS, since decrypt returns it as a string. The JS contract is text in, text out.

# Related Concepts
- [quickjs Go/JS boundary gotchas that broke host natives](quickjs-go-js-boundary-gotchas.md): The string-boundary corruption that forced this decision.
- [Host helper surface registry](host-helper-surface-registry.md): The registry where crypto.substitute_cipher is declared for every runtime.
