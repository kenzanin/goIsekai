## Tasks

### 1. Keyboard event handler
- [ ] 1.1 Add `document.addEventListener('keydown', handleKeyboard)` in reader.js
- [ ] 1.2 Map ArrowLeft → prevPage(), ArrowRight/Space → nextPage(), Escape → navigate to detail
- [ ] 1.3 Skip shortcuts when input/textarea is focused
- [ ] 1.4 Prevent default on Space to avoid scroll

### 2. Chapter boundary crossing
- [ ] 2.1 Ensure nextPage() on last page triggers switchChapter(next)
- [ ] 2.2 Ensure prevPage() on first page triggers switchChapter(prev)

### 3. Optional shortcut hint
- [ ] 3.1 Add small shortcut hint overlay or tooltip to reader.jet

### 4. Verify
- [ ] 4.1 `go build ./...` passes
- [ ] 4.2 Live smoke: test all shortcuts in reader
- [ ] 4.3 Verify shortcuts don't fire when search input is focused
