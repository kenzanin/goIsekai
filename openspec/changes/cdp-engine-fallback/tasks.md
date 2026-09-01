## 1. Config: CDP engine settings

- [x] 1.1 Add `cdp_engine` (`off`|`lightpanda`|`chrome`, default `off`) and `cdp_path` to `internal/config/config.go` (ini keys + CLI flags, flag overrides ini, mirroring host/port)
- [x] 1.2 Wire the new settings into the hostnet proxy constructor / options so the fallback path can read them

## 2. CDP engine driver

- [x] 2.1 Add `github.com/chromedp/chromedp` to go.mod (verify CGO_ENABLED=0 build still passes)
- [x] 2.2 Create `internal/hostnet/cdp.go`: launch browser at `cdp_path`, navigate to a URL, wait for challenge solve (bounded timeout), harvest cookies via CDP, close the process
- [x] 2.3 Return harvested cookies + browser User-Agent to the caller for handoff

## 3. Fallback wiring in request flow

- [x] 3.1 In `internal/hostnet/request.go`, when a challenge response is detected and the engine is enabled, invoke the CDP solve, hand cookies/UA to `SetVerifyCookies`, and retry the original request once
- [x] 3.2 Ensure the `off` and solve-failure paths still surface the existing `ChallengeError` unchanged
- [x] 3.3 Add per-solve timeout config (default 30s) and process cleanup on timeout

## 4. Plugin needs_js hint

- [x] 4.1 Add optional `needs_js` to the plugin Init metadata contract (WASM + Lua parity) and thread it into the proxy so requests skip the fast path when true and an engine is configured
- [x] 4.2 When `needs_js` is true but the engine is `off`, fall back to the fast path (no change in behavior)

## 5. Tests & verification

- [x] 5.1 Unit-test the challenge-detection → solve → retry decision (with a fake engine) and the `off`/failure degradation paths
- [x] 5.2 Unit-test `needs_js` routing (hint present/absent, engine on/off)
- [x] 5.3 Run full `go build ./...` + `go test ./internal/...` + gofmt/vet; confirm CGO_ENABLED=0 binary
