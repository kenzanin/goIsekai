---
type: Fact
title: quickjs Go/JS boundary gotchas that broke host natives
description: "How values really cross the modernc.org/quickjs boundary in goisekai: return Undefined{} not UndefinedValue, numbers arrive as int or float64, objects and function args are JSON snapshots, Go strings must be valid UTF-8, plus the fix each one needed."
tags: [goisekai, quickjs, pluginmanager, gotcha, host-natives, abi]
generated: { by: agent/cli, at: "2026-11-01T12:45:01Z" }
status: stable
---

Each item below caused a real test failure during the goja to quickjs swap. Read this before adding or porting a JS host native.

## Returning a value

A host function must return `quickjs.Undefined{}` on success, **not** `quickjs.UndefinedValue`. `UndefinedValue` is a `Value` with no VM attached; converting it hits a nil dereference that quickjs re-raises through `Eval`, so the stack points into quickjs, not at the native. Error-path returns are fine because the error is thrown before the value is converted. Fixed in `registerJSConsole` and `registerJSLog`.

## Arguments arriving in a host function

| JS value | Go type |
|---|---|
| string | `string` |
| number in int32 range (including `7.0`) | `int` |
| any other number | `float64` (2^53-1 stays exact) |
| number nested in array/object | `float64` after `Into` |
| bool | `bool` |
| null | `nil` |
| undefined | `quickjs.Undefined{}` |
| array / object | `*quickjs.Object`, a JSON snapshot (use `Into(&x)`) |
| function | JSON snapshot only, **never callable** |

Consequences and fixes:

- A wrapper that only did `args[0].(string)` silently turned numbers into `""`. `jsArgString` now coerces string, int, int64, float64 and bool, and every `jsStr*` and `jsFloat1` wrapper shares it. Null, undefined and objects still give `""`.
- JSON objects keep **insertion order** in the snapshot, so `json.encode` output ordering differed from the Lua path. `jsJSONEncode` decodes through `Into(&any)` so the encoder sorts keys.
- A JS callback cannot be handed to Go. `host.regex.replace` with a function replacer therefore runs in a small JS wrapper (`jsRegexReplaceWrap`) over a native `split` returning `[gap, captures, gap, ..., gap]`, built on the shared `pluginutil.RegexSplitMatches` walker that Lua also uses.

## Strings carrying bytes

A Go string holding non-UTF-8 bytes is corrupted to U+FFFD when it crosses into a JS string. Anything binary must travel as hex or base64. See the cipher decision linked below.

## Calling convention for registration

`RegisterHostFunc` can only define a function on the global object, and `SetProperty`'s `any` path marshals through JSON, which a function cannot survive. So natives register under a scratch global name and are then moved onto the target object by `setFn` / `SetPropertyValue`. A typed `RegisterFunc` route was ruled out because its `(T, error)` return becomes a JS array instead of a thrown error.

## Lifecycle

`VM.Close` is not safe to run twice; `jsEngine.close` nils `p.js` afterwards. The per-VM html handle registry must be released before the VM closes.

# Related Concepts
- [JS plugins run on modernc.org/quickjs (CGO-free), goja dropped](plugin-js-engine-quickjs-swap.md): The decision these gotchas came out of, including the invoke budget and the registry leak fix.
- [substitute_cipher is a hex token on every runtime](substitute-cipher-hex-abi.md): Why binary data had to move to hex.
