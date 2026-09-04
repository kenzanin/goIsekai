## Tasks

### 1. Keyboard event handler
- [x] 1.1 Add `document.addEventListener('keydown', handleKeyboard)` in reader.js
- [x] 1.2 Map ArrowLeft → prevPage(), ArrowRight/Space → nextPage(), Escape → navigate to detail
- [x] 1.3 Skip shortcuts when input/textarea is focused
- [x] 1.4 Prevent default on Space to avoid scroll

### 2. Chapter boundary crossing
- [x] 2.1 Ensure nextPage() on last page triggers switchChapter(next)
- [x] 2.2 Ensure prevPage() on first page triggers switchChapter(prev)

### 3. Optional shortcut hint
- [x] 3.1 Add small shortcut hint overlay or tooltip to reader.jet

### 4. Verify
- [x] 4.1 `go build ./...` passes
- [x] 4.2 Live smoke: test all shortcuts in reader
- [x] 4.3 Verify shortcuts don't fire when search input is focused
