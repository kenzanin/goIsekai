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

// luaFnNames is the source-plugin ABI: the snake_case globals a plugin that
// scrapes a site defines. The ABI is normalised — the Lua name is the host name
// in snake_case — so Search is `search` and GetMangaDetail is
// `get_manga_detail`. One map drives both the load-time verification and the
// call-time dispatch.
var luaFnNames = map[string]string{
	types.SearchFunc:         "search",
	types.GetMangaDetailFunc: "get_manga_detail",
	types.GetChapterListFunc: "get_chapter_list",
	types.GetPageListFunc:    "get_page_list",
	types.GetGenresFunc:      "get_genres",
}

// luaInfoFnNames is the enrichment-script ABI. An info script serves metadata
// only — it never scrapes a site and is never offered as a manga source — so
// its entry points are disjoint from a source plugin's. Keeping them in a
// separate map is what lets a source plugin be verified without ever
// mentioning enrichment.
var luaInfoFnNames = map[string]string{
	types.GetEnrichmentFunc: "get_enrichment",
}

// luaLegacyNames are the globals shipped before the ABI names were unified.
// A plugin that defines one still loads and runs, with a deprecation warning,
// so publishing the normalised ABI breaks nothing. Drop an entry once no
// shipped plugin uses it.
var luaLegacyNames = map[string]string{
	types.SearchFunc:        "search_manga",
	types.GetEnrichmentFunc: "getEnrichment",
}

// luaCanonicalName returns the current global name for a host ABI function,
// looking in the source ABI and then the enrichment ABI.
func luaCanonicalName(fnName string) (string, bool) {
	if n, ok := luaFnNames[fnName]; ok {
		return n, true
	}
	if n, ok := luaInfoFnNames[fnName]; ok {
		return n, true
	}
	return "", false
}

// luaABINames lists the global names to try for a host ABI function, the
// canonical name first and any legacy alias second.
func luaABINames(fnName string) []string {
	names := make([]string, 0, 2)
	if n, ok := luaCanonicalName(fnName); ok {
		names = append(names, n)
	}
	if n, ok := luaLegacyNames[fnName]; ok {
		names = append(names, n)
	}
	return names
}

// resolveLuaFn returns the first name for fnName that is a function in state,
// preferring the canonical name. Callers hold the per-plugin lock.
func resolveLuaFn(id string, state *lua.State, fnName string) (string, bool) {
	canonical, _ := luaCanonicalName(fnName)
	for _, name := range luaABINames(fnName) {
		fn, err := state.RawGlobal(name)
		if err != nil || fn.Kind() != lua.FunctionKind {
			continue
		}
		if name != canonical {
			logger.Warn("plugin uses a legacy ABI name", "plugin", id, "fn", fnName, "legacy", name, "use", canonical)
		}
		return name, true
	}
	return "", false
}

// callLua invokes a Lua ABI function with a JSON string argument under timeout,
// and returns the result as a JSON string.
func callLua(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	if _, ok := luaCanonicalName(fnName); !ok {
		return "", fmt.Errorf("lua plugin %s: no lua mapping for %s", p.id, fnName)
	}

	// Lunar State is not goroutine-safe; serialize per-plugin access.
	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.lunar

	luaName, ok := resolveLuaFn(p.id, state, fnName)
	if !ok {
		return "", fmt.Errorf("lua plugin %s: %s is not a function", p.id, fnName)
	}
	fnVal, err := state.RawGlobal(luaName)
	if err != nil {
		return "", fmt.Errorf("lua plugin %s %s: read %s: %w", p.id, fnName, luaName, err)
	}

	// Set a wall-clock deadline on the State's context (a plugin's declared
	// timeout may extend the default for round-trip-heavy calls).
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
