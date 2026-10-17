package pluginmanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/dop251/goja"

	"goisekai/internal/logger"
)

// callJS invokes a JS function on a goja-based plugin. The ABI contract is
// identical to Lua: the function receives a single JSON string argument and
// must return a JSON string (or an object that goja will serialize).
func callJS(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	if _, ok := jsFnNames[fnName]; !ok {
		return "", fmt.Errorf("js plugin %s: no js mapping for %s", p.id, fnName)
	}
	jsName, ok := resolveJSFn(p.id, p.js, fnName)
	if !ok {
		return "", fmt.Errorf("js plugin %s: %s is not a function", p.id, jsFnNames[fnName])
	}
	fn := p.js.Get(jsName)
	if fn == nil || goja.IsUndefined(fn) {
		return "", fmt.Errorf("js plugin %s: %s is not a function", p.id, jsName)
	}

	budget := p.invokeDeadline()
	start := time.Now()
	logger.Debug("plugin invoke start", "plugin", p.id, "fn", fnName, "budget", budget.String())
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	// Timeout via Interrupt. A stale interrupt on an idle VM persists into the
	// next call, so always ClearInterrupt after the call completes. The interrupt
	// aborts JS between statements; ctx carries the same deadline and tears down
	// host calls (an HTTP fetch, a CDP solve) already blocking inside a native,
	// which goja cannot interrupt. The interrupt must not watch ctx.Done():
	// cancel() runs on return, after ClearInterrupt, so a select on Done() can
	// fire late and leave a stale interrupt for the next call.
	stop := make(chan struct{})
	go func() {
		select {
		case <-time.After(p.invokeDeadline()):
			p.js.Interrupt("timeout")
		case <-stop:
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.curCtx = ctx
	defer func() { p.curCtx = nil }()

	callable, ok := goja.AssertFunction(fn)
	if !ok {
		close(stop)
		return "", fmt.Errorf("js plugin %s %s: not callable", p.id, jsName)
	}

	v, err := callable(goja.Undefined(), p.js.ToValue(inputJSON))
	close(stop)
	p.js.ClearInterrupt()
	if err != nil {
		logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("js plugin %s %s: %w", p.id, jsName, err)
	}
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return "", fmt.Errorf("js plugin %s %s: returned nil", p.id, jsName)
	}
	// If it's already a string, return directly.
	if s, ok := v.Export().(string); ok {
		logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "bytes", len(s))
		return s, nil
	}
	// Otherwise marshal to JSON.
	raw, err := json.Marshal(v.Export())
	if err != nil {
		logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("js plugin %s %s: marshal result: %w", p.id, jsName, err)
	}
	logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "bytes", len(raw))
	return string(raw), nil
}

// newJSConsole builds the `console` global as a plain JS object with the
// lowercase names a JS author expects. Handing goja a Go struct instead leaves
// it mapping methods by their exported name, so only `console.Log` would
// resolve while the familiar `console.log(...)` would not exist at all.
// Arguments are joined with a space, the way console.log formats them.
func newJSConsole(id string) map[string]any {
	emit := func(level string, args []goja.Value) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a.Export())
		}
		msg := strings.Join(parts, " ")
		switch level {
		case "debug":
			logger.Debug("plugin="+id, "msg", msg)
		case "warn":
			logger.Warn("plugin="+id, "msg", msg)
		case "error":
			logger.Error("plugin="+id, "msg", msg)
		default:
			logger.Info("plugin="+id, "msg", msg)
		}
	}
	fn := func(level string) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			emit(level, call.Arguments)
			return goja.Undefined()
		}
	}
	return map[string]any{
		"log":   fn("info"),
		"info":  fn("info"),
		"warn":  fn("warn"),
		"error": fn("error"),
		"debug": fn("debug"),
	}
}
