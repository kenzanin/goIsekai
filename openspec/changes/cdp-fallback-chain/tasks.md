## Tasks

### 1. Challenge detection
- [ ] 1.1 Add `isChallengeResponse(status int, body []byte) bool` to `internal/hostnet/`
- [ ] 1.2 Define challenge marker list (CF, Turnstile, generic "Just a moment")
- [ ] 1.3 Unit tests for challenge detection

### 2. CDP fallback loop
- [ ] 2.1 Add fallback wrapper in bridge request path: detect → CDP solve → cookie inject → retry
- [ ] 2.2 Cookie extraction from CDP browser session
- [ ] 2.3 Cookie injection into tls-client jar for target domain
- [ ] 2.4 Max 1 retry guard to prevent loops

### 3. Configuration
- [ ] 3.1 Add `cdp_fallback` config option (auto/manual/off)
- [ ] 3.2 Wire config to fallback logic
- [ ] 3.3 Settings page UI for fallback mode

### 4. Verify
- [ ] 4.1 `go build ./... && go vet ./...` passes
- [ ] 4.2 `go test ./internal/hostnet/` passes
- [ ] 4.3 Live smoke: test against a challenge-blocked site with CDP enabled
