package pluginmanager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-json"
)

// callGo invokes an ABI function on a native plugin. The C call itself cannot
// be interrupted — there is no VM to trap — so the deadline below bounds only
// the host helpers the plugin calls back into (an HTTP fetch, a CDP solve).
// Without it those would run on context.Background() and outlive the invoke.
func callGo(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), p.invokeDeadline())
	defer cancel()
	// Published for the host_call bridge, which runs on this goroutine while mu
	// is held and reads curCtx directly (locking here would deadlock).
	p.curCtx = ctx
	defer func() { p.curCtx = nil }()

	sym, ok := p.goFns[fnName]
	if !ok {
		return "", fmt.Errorf("go plugin %s: no symbol for %s", p.id, fnName)
	}
	fn, ok := sym.(*goFn)
	if !ok {
		return "", fmt.Errorf("go plugin %s %s: bad symbol type", p.id, fnName)
	}

	// purego converts the Go string to a NUL-terminated C string for the call
	// and keeps it alive across it, so the input needs no manual buffer.
	out := fn.call(inputJSON)
	if out == nil {
		return "", fmt.Errorf("go plugin %s %s: returned a null result", p.id, fnName)
	}
	res := cString(out)
	fn.free(out)

	if strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("go plugin %s %s: empty result", p.id, fnName)
	}
	var probe any
	if err := json.Unmarshal([]byte(res), &probe); err != nil {
		return "", fmt.Errorf("go plugin %s %s: result is not JSON: %w", p.id, fnName, err)
	}
	// A plugin reports failure through the shared error envelope.
	if obj, ok := probe.(map[string]any); ok {
		if msg, ok := obj["error"].(string); ok && len(obj) == 1 {
			if reason := PluginError(msg); reason != "" {
				return "", fmt.Errorf("go plugin %s %s: %s", p.id, fnName, reason)
			}
			return "", fmt.Errorf("go plugin %s %s: plugin reported an error", p.id, fnName)
		}
	}
	return res, nil
}

// discoverGo registers *.so files as native plugins, in the same two shapes
// every other runtime uses: a folder plugin (<dir>/<id>/main.so, so plugin.json
// sits beside it) and a bare <dir>/<id>.so.
func (m *Manager) discoverGo() error {
	globs := []struct {
		pattern string
		folder  bool
	}{
		{filepath.Join(m.pluginsDir, "*", "main.so"), true},
		{filepath.Join(m.pluginsDir, "*.so"), false},
	}
	for _, g := range globs {
		matches, err := filepath.Glob(g.pattern)
		if err != nil {
			return err
		}
		for _, path := range matches {
			id := strings.TrimSuffix(filepath.Base(path), ".so")
			dir := m.pluginsDir
			if g.folder {
				id = filepath.Base(filepath.Dir(path))
				dir = filepath.Dir(path)
			}
			if _, dup := m.plugins[id]; dup {
				continue
			}
			if !m.registerFolderPlugin(id, dir, "go") {
				continue
			}
			m.plugins[id].wasmPath = path
		}
	}
	return nil
}
