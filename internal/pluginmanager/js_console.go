package pluginmanager

import (
	"context"
	"fmt"
	"time"

	"goisekai/internal/logger"
	"modernc.org/quickjs"
)

// callJS invokes a JS function on a quickjs-based plugin. The ABI contract is
// identical to Lua: the function receives a single JSON string argument and
// must return a JSON string (or an object that quickjs will serialize).
func callJS(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	if _, ok := jsFnNames[fnName]; !ok {
		return "", fmt.Errorf("js plugin %s: no js mapping for %s", p.id, fnName)
	}
	jsName, ok := resolveJSFn(p.js, p.id, fnName)
	if !ok {
		return "", fmt.Errorf("js plugin %s: %s is not a function", p.id, jsFnNames[fnName])
	}

	budget := p.invokeDeadline()
	start := time.Now()
	logger.Debug("plugin invoke start", "plugin", p.id, "fn", fnName, "budget", budget.String())
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	// The eval timeout bounds the JS evaluation itself. The interrupt handler
	// is re-armed on every Eval/Call by quickjs, so a per-call SetEvalTimeout
	// here is sufficient: when the budget expires, the next JS checkpoint
	// aborts the evaluation with "InternalError: interrupted".
	if err := p.js.SetEvalTimeout(budget); err != nil {
		return "", fmt.Errorf("js plugin %s %s: %w", p.id, fnName, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.curCtx = ctx
	defer func() { p.curCtx = nil }()

	// Call the global function by name; quickjs evaluates the name reference
	// and invokes it with the single JSON string argument.
	v, err := p.js.CallValue(jsName, inputJSON)
	elapsed := time.Since(start)
	if err != nil {
		logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", elapsed.String(), "error", err)
		// quickjs reports its Interrupt as "InternalError: interrupted", which
		// says nothing about why. When the invoke deadline is what fired, label
		// it a timeout; the original error stays in the chain. Any other JS
		// exception passes through unchanged. The elapsed check covers the race
		// where quickjs's interrupt fires before the ctx timer has marked Err.
		if ctx.Err() == context.DeadlineExceeded || elapsed >= budget {
			return "", fmt.Errorf("js plugin %s %s: timeout after %s: %w", p.id, jsName, budget, err)
		}
		return "", fmt.Errorf("js plugin %s %s: %w", p.id, jsName, err)
	}
	if v.IsUndefined() {
		return "", fmt.Errorf("js plugin %s %s: returned nil", p.id, fnName)
	}

	// Convert the result to a Go value.
	out, err := v.Any()
	if err != nil {
		return "", fmt.Errorf("js plugin %s %s: convert result: %w", p.id, jsName, err)
	}

	// If it's already a string, return directly.
	if s, ok := out.(string); ok {
		logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", elapsed.String(), "bytes", len(s))
		return s, nil
	}

	// Otherwise marshal to JSON.
	switch r := out.(type) {
	case *quickjs.Object:
		raw, err := jsonMarshalAny(r)
		if err != nil {
			logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", elapsed.String(), "error", err)
			return "", fmt.Errorf("js plugin %s %s: marshal result: %w", p.id, jsName, err)
		}
		logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", elapsed.String(), "bytes", len(raw))
		return string(raw), nil
	default:
		raw, err := jsonMarshalAny(out)
		if err != nil {
			logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", elapsed.String(), "error", err)
			return "", fmt.Errorf("js plugin %s %s: marshal result: %w", p.id, jsName, err)
		}
		logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", elapsed.String(), "bytes", len(raw))
		return string(raw), nil
	}
}

// jsonMarshalAny is a thin wrapper to keep the call site readable.
func jsonMarshalAny(v any) ([]byte, error) {
	switch r := v.(type) {
	case *quickjs.Object:
		// Object implements MarshalJSON.
		type jsonMarshallable interface {
			MarshalJSON() ([]byte, error)
		}
		//nolint:staticcheck // Object implements json.Marshaler
		if jm, ok := any(r).(jsonMarshallable); ok {
			return jm.MarshalJSON()
		}
	}
	// Fallback: use encoding/json for Go primitives.
	return jsonMarshal(v)
}

// ponytail: keep the encoding/json import here; jsonMarshal is defined in
// js_json.go as a plain json.Marshal wrapper. Upgrade path: drop jsonMarshal
// and use json.Marshal directly when the indirection is no longer needed.
func jsonMarshal(v any) ([]byte, error) {
	// Reuse the existing jsonEncodeValue path: it already handles nil,
	// primitives, maps, slices and *quickjs.Object via JSON.stringify.
	out, err := jsonEncodeValue(v)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}
