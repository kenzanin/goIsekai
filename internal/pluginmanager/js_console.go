package pluginmanager

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dop251/goja"

	"goisekai/internal/logger"
)

// callJS invokes a JS function on a goja-based plugin. The ABI contract is
// identical to Lua: the function receives a single JSON string argument and
// must return a JSON string (or an object that goja will serialize).
func callJS(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	jsName, ok := jsFnNames[fnName]
	if !ok {
		return "", fmt.Errorf("js plugin %s: no js mapping for %s", p.id, fnName)
	}
	fn := p.js.Get(jsName)
	if fn == nil || goja.IsUndefined(fn) {
		return "", fmt.Errorf("js plugin %s: %s is not a function", p.id, jsName)
	}

	// Timeout via Interrupt. A stale interrupt on an idle VM persists into the
	// next call, so always ClearInterrupt after the call completes.
	stop := make(chan struct{})
	go func() {
		select {
		case <-time.After(invokeTimeout):
			p.js.Interrupt("timeout")
		case <-stop:
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	callable, ok := goja.AssertFunction(fn)
	if !ok {
		close(stop)
		return "", fmt.Errorf("js plugin %s %s: not callable", p.id, jsName)
	}

	v, err := callable(goja.Undefined(), p.js.ToValue(inputJSON))
	close(stop)
	p.js.ClearInterrupt()
	if err != nil {
		return "", fmt.Errorf("js plugin %s %s: %w", p.id, jsName, err)
	}
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return "", fmt.Errorf("js plugin %s %s: returned nil", p.id, jsName)
	}
	// If it's already a string, return directly.
	if s, ok := v.Export().(string); ok {
		return s, nil
	}
	// Otherwise marshal to JSON.
	raw, err := json.Marshal(v.Export())
	if err != nil {
		return "", fmt.Errorf("js plugin %s %s: marshal result: %w", p.id, jsName, err)
	}
	return string(raw), nil
}

// jsConsole wraps fmt.Printf-style console output for JS plugins.
type jsConsole struct{ id string }

func newJSConsole(id string) *jsConsole { return &jsConsole{id: id} }

func (c *jsConsole) Log(args ...goja.Value)   { c.log("info", args...) }
func (c *jsConsole) Info(args ...goja.Value)  { c.log("info", args...) }
func (c *jsConsole) Warn(args ...goja.Value)  { c.log("warn", args...) }
func (c *jsConsole) Error(args ...goja.Value) { c.log("error", args...) }
func (c *jsConsole) Debug(args ...goja.Value) { c.log("debug", args...) }

func (c *jsConsole) log(level string, args ...goja.Value) {
	parts := make([]any, len(args))
	for i, a := range args {
		parts[i] = a.Export()
	}
	switch level {
	case "debug":
		logger.Debug("plugin="+c.id, "msg", fmt.Sprint(parts...))
	case "info":
		logger.Info("plugin="+c.id, "msg", fmt.Sprint(parts...))
	case "warn":
		logger.Warn("plugin="+c.id, "msg", fmt.Sprint(parts...))
	case "error":
		logger.Error("plugin="+c.id, "msg", fmt.Sprint(parts...))
	}
}
