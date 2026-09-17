// Regex helpers shared by the host runtimes. Lua has no regex engine of its
// own — its string library speaks patterns, not regular expressions — so the
// matching runs here, once, on a Go engine rather than once per plugin in Lua
// pattern syntax. coregex is an O(n) engine, so a plugin pattern cannot make a
// scrape path hang on a pathological backup (the reason to prefer it over a
// backtracking engine).
package pluginutil

import (
	"sync"

	"github.com/coregx/coregex"
)

// regexCacheMax bounds the compiled-pattern cache. Plugins call these helpers
// with a fixed set of literal patterns, so a small cache hits on nearly every
// call; the bound keeps a plugin that builds patterns in a loop from growing
// the map without limit.
const regexCacheMax = 512

var (
	regexMu    sync.RWMutex
	regexCache = make(map[string]*coregex.Regexp, regexCacheMax)
)

// regexCompile returns the compiled pattern, reusing an earlier compile of the
// same text. Compiled patterns are safe for concurrent use, and the read lock
// keeps the common (already-cached) path off the write lock.
func regexCompile(pattern string) (*coregex.Regexp, error) {
	regexMu.RLock()
	re, ok := regexCache[pattern]
	regexMu.RUnlock()
	if ok {
		return re, nil
	}
	re, err := coregex.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexMu.Lock()
	if len(regexCache) < regexCacheMax {
		regexCache[pattern] = re
	}
	regexMu.Unlock()
	return re, nil
}

// RegexFind returns the captures of the first match the way Lua's string.match
// does: the capture groups when the pattern has any, the whole match when it
// has none, and nil when nothing matches. A pattern that cannot be compiled is
// an error, so a mistyped pattern is distinguishable from a pattern that simply
// did not match.
func RegexFind(subject, pattern string) ([]string, error) {
	re, err := regexCompile(pattern)
	if err != nil {
		return nil, err
	}
	m := re.FindStringSubmatch(subject)
	if m == nil {
		return nil, nil
	}
	if re.NumSubexp() == 0 {
		return m[:1], nil
	}
	return m[1:], nil
}

// RegexMatch reports whether pattern matches anywhere in subject.
func RegexMatch(subject, pattern string) (bool, error) {
	re, err := regexCompile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(subject), nil
}

// RegexFindAll returns every match in document order as a row of capture
// values, the shape Lua's string.gmatch walks: a row holds the capture groups
// when the pattern has any, the whole match when it has none. Every row of one
// pattern therefore has the same length, so a one-value row is a plain string.
func RegexFindAll(subject, pattern string) ([][]string, error) {
	re, err := regexCompile(pattern)
	if err != nil {
		return nil, err
	}
	whole := re.NumSubexp() == 0
	matches := re.FindAllStringSubmatch(subject, -1)
	rows := make([][]string, 0, len(matches))
	for _, m := range matches {
		if whole {
			rows = append(rows, m[:1])
			continue
		}
		rows = append(rows, m[1:])
	}
	return rows, nil
}

// RegexReplace replaces every match with repl. Capture references are written
// in Go's syntax ($1, ${name}), not Lua's percent syntax.
func RegexReplace(subject, pattern, repl string) (string, error) {
	re, err := regexCompile(pattern)
	if err != nil {
		return "", err
	}
	return re.ReplaceAllString(subject, repl), nil
}
