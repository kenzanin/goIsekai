package pluginmanager

import (
	"context"
	"fmt"

	"github.com/goccy/go-json"
	"runtime/debug"
)

// callYaegi invokes an ABI function on a Yaegi plugin.
func callYaegi(m *Manager, p *loadedPlugin, fnName, inputJSON string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	yp := p.yaegi
	if yp == nil {
		return "", fmt.Errorf("yaegi plugin %s: not loaded", p.id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
	defer cancel()

	out, err := yp.callWithTimeout(ctx, fnName, inputJSON)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("yaegi plugin %s %s: timed out", p.id, fnName)
		}
		return "", fmt.Errorf("yaegi plugin %s %s: %w", p.id, fnName, err)
	}

	if out == "" {
		return "", fmt.Errorf("yaegi plugin %s %s: empty result", p.id, fnName)
	}

	var probe any
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		return "", fmt.Errorf("yaegi plugin %s %s: result is not JSON: %w", p.id, fnName, err)
	}

	return out, nil
}

// callWithTimeout invokes the load-time wrapper for fnName in the Yaegi
// interpreter. The wrapper returns a single string; plugin errors (and
// timeouts, via EvalWithContext) surface as Eval errors.
func (yp *yaegiPlugin) callWithTimeout(ctx context.Context, fnName, arg string) (out string, err error) {
	// Recover from panics in interpreted code.
	defer func() {
		if r := recover(); r != nil {
			out = ""
			err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()

	result, err := yp.i.EvalWithContext(ctx, fmt.Sprintf("gskCall_%s(%q)", fnName, arg))
	if err != nil {
		return "", err
	}

	val := result.Interface()
	switch t := val.(type) {
	case string:
		return t, nil
	case []any:
		// Defensive: a multi-value surface would mean the wrapper is missing.
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s, nil
			}
		}
		return "", fmt.Errorf("yaegi %s: unexpected return %T", fnName, val)
	default:
		return "", fmt.Errorf("yaegi %s: unexpected return type %T", fnName, val)
	}
}
