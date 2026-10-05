package templates

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"goisekai/internal/logger"

	lua "github.com/mmcdole/lunar"
)

// openSource serves `require` from the in-memory source cache. It is the point
// of the sources map: with it, rendering a view that requires several partials
// costs no filesystem access.
//
// fs.ErrNotExist for an unknown name lets lunar keep walking its remaining
// package.path candidates, exactly as FSLoader would.
func (e *LuaEngine) openSource(_ context.Context, name string) (io.ReadCloser, error) {
	// lunar resolves require through package.path, so it asks for
	// "partials/nav.lua" while the caches are keyed without the extension.
	key := strings.TrimSuffix(name, ".lua")
	e.mu.RLock()
	src, ok := e.sources[key]
	e.mu.RUnlock()
	if !ok {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(src)), nil
}

// entry is one recompiled template waiting to be published.
type entry struct {
	name  string
	src   string
	hash  [32]byte
	proto *lua.Prototype
}

// refresh recompiles the templates whose content changed since the last check.
// Only called when hot reload is enabled.
//
// One walk per refresh, not per render: the hash is compared first, so an
// unedited tree costs a walk and no compilation. A template that fails to
// compile keeps its previous entry and the failure is logged, so one bad edit
// cannot take every page down.
func (e *LuaEngine) refresh() error {
	var (
		fresh []entry
		broke []error
	)

	walkErr := fs.WalkDir(e.templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".lua") {
			return err
		}
		src, rerr := fs.ReadFile(e.templatesFS, path)
		if rerr != nil {
			broke = append(broke, fmt.Errorf("read %s: %w", path, rerr))
			return nil
		}
		name := strings.TrimSuffix(path, ".lua")
		hash := sha256.Sum256(src)
		if e.hashFor(name) == hash {
			return nil
		}
		proto, cerr := lua.Compile(path, string(src))
		if cerr != nil {
			broke = append(broke, fmt.Errorf("compile %s: %w", path, cerr))
			return nil
		}
		fresh = append(fresh, entry{name: name, src: string(src), hash: hash, proto: proto})
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	for _, e := range broke {
		logger.Warn("template hot reload: keeping previous version", "error", e)
	}
	e.publish(fresh)
	return nil
}

// publish swaps in the recompiled entries under a single write lock, so a
// concurrent render sees either the whole old set or the whole new one.
func (e *LuaEngine) publish(entries []entry) {
	if len(entries) == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, en := range entries {
		e.protos[en.name] = en.proto
		e.sources[en.name] = en.src
		e.hashes[en.name] = en.hash
	}
}

// hashFor returns the content hash recorded for name.
func (e *LuaEngine) hashFor(name string) [32]byte {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.hashes[name]
}
