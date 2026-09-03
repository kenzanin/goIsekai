## Tasks

### 1. Reader error handling
- [ ] 1.1 Wrap all fetch calls in reader.js with try-catch
- [ ] 1.2 Add error overlay div to reader.jet template (hidden by default)
- [ ] 1.3 Show overlay on error with message and retry button
- [ ] 1.4 Hide overlay on successful data load

### 2. HTMX error toast
- [ ] 2.1 Add toast element to base layout template
- [ ] 2.2 Register htmx:responseError handler in app.js or base layout script
- [ ] 2.3 Auto-dismiss toast after 5 seconds

### 3. Verify
- [ ] 3.1 `go build ./...` passes
- [ ] 3.2 Live smoke: break reader-data endpoint, verify error overlay appears
- [ ] 3.3 Live smoke: trigger HTMX error, verify toast appears
