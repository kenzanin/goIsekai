package pluginmanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// luaFnNames maps host ABI function names to the snake_case globals a Lua
// plugin defines (the Lua-facing ABI; design.md). One map drives both the
// load-time verification and the call-time dispatch.
var luaFnNames = map[string]string{
	types.SearchFunc:         "search_manga",
	types.GetMangaDetailFunc: "get_manga_detail",
	types.GetChapterListFunc: "get_chapter_list",
	types.GetPageListFunc:    "get_page_list",
	types.GetEnrichmentFunc:  "getEnrichment",
	types.GetGenresFunc:      "get_genres",
}

// callLua invokes a Lua ABI function with a JSON string argument under timeout,
// and returns the result as a JSON string.
func callLua(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	luaName, ok := luaFnNames[fnName]
	if !ok {
		return "", fmt.Errorf("lua plugin %s: no lua mapping for %s", p.id, fnName)
	}

	// Lunar State is not goroutine-safe; serialize per-plugin access.
	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.lunar

	fnVal, err := state.RawGlobal(luaName)
	if err != nil || fnVal.Kind() != lua.FunctionKind {
		return "", fmt.Errorf("lua plugin %s: %s is not a function", p.id, fnName)
	}

	// Set a wall-clock deadline on the State's context (PLUGIN.timeout may
	// extend the default for round-trip-heavy calls).
	budget := p.invokeDeadline()
	start := time.Now()
	logger.Debug("plugin invoke start", "plugin", p.id, "fn", fnName, "budget", budget.String())
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	if err := state.SetContext(ctx); err != nil {
		return "", fmt.Errorf("lua plugin %s %s: set context: %w", p.id, fnName, err)
	}
	defer func() { _ = state.RemoveContext() }()

	vals, err := state.Call(fnVal, lua.String(inputJSON))
	if err != nil {
		logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", err)
		// The http natives throw the ErrChallenge marker when a dead anti-bot
		// session aborts the call — map it back to the typed error the views
		// check for, so the human-verify wizard re-opens.
		if ce, ok := challengeFromVM(err.Error()); ok {
			return "", ce
		}
		return "", fmt.Errorf("lua plugin %s %s: %w", p.id, fnName, err)
	}
	if len(vals) == 0 {
		logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", "returned no values")
		return "", fmt.Errorf("lua plugin %s %s: returned no values", p.id, fnName)
	}
	res := vals[0]
	// Plugins signal failure the Lua way: return nil, "<code>" (optionally
	// "<code>: <detail>"). The host owns the wording — see pluginerr.go — so
	// the same failure reads identically no matter which plugin raised it.
	pluginReason := func() string {
		if len(vals) < 2 || vals[1].IsNil() {
			return ""
		}
		s, ok := vals[1].AsString()
		if !ok {
			return ""
		}
		return PluginError(s)
	}
	switch {
	case res.IsNil():
		if reason := pluginReason(); reason != "" {
			// A plugin that propagates the native's challenge text verbatim
			// (nil, err) lands here — same typed-error mapping as above.
			if ce, ok := challengeFromVM(reason); ok {
				logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", reason)
				return "", ce
			}
			logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", reason)
			return "", fmt.Errorf("lua plugin %s %s: %s", p.id, fnName, reason)
		}
		logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", "returned nil")
		return "", fmt.Errorf("lua plugin %s %s: returned nil", p.id, fnName)
	case res.Kind() == lua.BoolKind:
		return "", fmt.Errorf("lua plugin %s %s: returned bool, want string or table", p.id, fnName)
	case res.Kind() == lua.TableKind:
		tbl, _ := res.AsTable()
		jsonBytes, err := lunarTableToJSON(state, tbl)
		if err != nil {
			logger.Debug("plugin invoke fail", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "error", err)
			return "", fmt.Errorf("lua plugin %s %s: encode result table: %w", p.id, fnName, err)
		}
		logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "bytes", len(jsonBytes))
		return string(jsonBytes), nil
	default:
		s, _ := res.AsString()
		logger.Debug("plugin invoke done", "plugin", p.id, "fn", fnName, "elapsed", time.Since(start).String(), "bytes", len(s))
		return s, nil
	}
}

// challengeFromVM maps the marker thrown by the http natives (or propagated by
// the plugin as its second return) back to a typed *hostnet.ChallengeError.
// The marker is hostnet.ErrChallenge.Error() optionally followed by ": <url>".
func challengeFromVM(msg string) (*hostnet.ChallengeError, bool) {
	i := strings.Index(msg, hostnet.ErrChallenge.Error())
	if i < 0 {
		return nil, false
	}
	rest := msg[i+len(hostnet.ErrChallenge.Error()):]
	url := strings.TrimSpace(strings.TrimPrefix(rest, ":"))
	return &hostnet.ChallengeError{VerifyURL: url}, true
}
