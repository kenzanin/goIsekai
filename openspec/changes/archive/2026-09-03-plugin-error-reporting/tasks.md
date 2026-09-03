## Tasks

### 1. Add error tracking to bridge/plugin layer
- [ ] 1.1 Add `LastError string`, `ErrorCount int`, `LastSuccess time.Time` to plugin metadata in manager
- [ ] 1.2 Update on every ABI call: success → reset error count + update LastSuccess; failure → increment ErrorCount + set LastError

### 2. Surface errors in views
- [ ] 2.1 Create `plugin-error.jet` partial with error message + retry button
- [ ] 2.2 Include partial in search.jet, detail.jet when plugin error is present
- [ ] 2.3 Add consistent ChallengeError banner handling to all views

### 3. Plugin health indicator
- [ ] 3.1 Pass health data (LastSuccess, ErrorCount) to plugins.jet template
- [ ] 3.2 Add health display to each plugin card

### 4. Verify
- [ ] 4.1 `go build ./...` passes
- [ ] 4.2 Live smoke: trigger plugin error, verify error message + retry button shown
- [ ] 4.3 Live smoke: check plugins page shows health indicators