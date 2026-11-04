package hostnet

import (
	"context"
	"strings"

	"goisekai/pkg/types"
)

// SetHTTPProfiles stores a plugin-declared ordered profile ladder. Only
// recognized profile names (or "stdlib") are kept. An empty (or all-filtered)
// list clears any previously stored hints so a manifest that drops
// http_profiles resets the plugin to auto-mode on reload.
func (p *Proxy) SetHTTPProfiles(pluginID string, names []string) {
	var filtered []string
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] || !isKnownProfile(n) {
			continue
		}
		seen[n] = true
		filtered = append(filtered, n)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(filtered) == 0 {
		delete(p.hints, pluginID)
		return
	}
	p.hints[pluginID] = filtered
}

// SetUserAgents stores a plugin-declared ordered User-Agent ladder. Empty
// entries are dropped and the literal "default" is kept as a marker that
// resolves to the host's configured User-Agent at send time. An empty (or
// all-filtered) list clears any previously stored ladder so a manifest that
// drops user_agents resets the plugin to the host default on reload.
func (p *Proxy) SetUserAgents(pluginID string, list []string) {
	var filtered []string
	seen := map[string]bool{}
	for _, u := range list {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		filtered = append(filtered, u)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(filtered) == 0 {
		delete(p.uaLists, pluginID)
		// A manifest that drops user_agents resets the identity: a remembered
		// pin for the old list is stale and would keep a blocked entry active.
		delete(p.uaPins, pluginID)
		return
	}
	p.uaLists[pluginID] = filtered
}

// uaList returns the plugin's declared User-Agent ladder, empty when the
// plugin declares none.
func (p *Proxy) uaList(pluginID string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.uaLists[pluginID]
}

// uaPin returns the remembered winning User-Agent entry for pluginID, or "".
// A pin of "default" is not resolved here — the resolution to the host's
// configured value happens at send time so a config reload of the host UA
// still applies.
func (p *Proxy) uaPin(pluginID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.uaPins[pluginID]
}

// setUaPin remembers the first User-Agent entry that cleared a block for
// pluginID. In-memory only (decision 2): no DB persistence; the upgrade
// path is a plugins.ua_pin column wired like http_profile.
func (p *Proxy) setUaPin(pluginID, ua string) {
	p.mu.Lock()
	p.uaPins[pluginID] = ua
	p.mu.Unlock()
}

// clearUaPin drops the remembered User-Agent entry for pluginID.
func (p *Proxy) clearUaPin(pluginID string) {
	p.mu.Lock()
	delete(p.uaPins, pluginID)
	p.mu.Unlock()
}

// resolveUserAgent returns the User-Agent value to send for a request by
// pluginID that already carries reqUA (a per-request "User-Agent" header, ""
// when the plugin did not set one). Precedence: verify-seeded override,
// per-request header, the plugin's remembered (pin) or first declared entry,
// host default. The literal "default" entry resolves to the host's
// configured value. One helper shared by the tls-client and stdlib request
// paths so the precedence cannot drift between them.
func (p *Proxy) resolveUserAgent(pluginID, reqUA string) string {
	if ua := p.uaOverride(pluginID); ua != "" {
		return ua
	}
	if reqUA != "" {
		return reqUA
	}
	p.mu.Lock()
	entries := p.uaLists[pluginID]
	pin := p.uaPins[pluginID]
	ua, _ := p.defaultHeaders["User-Agent"]
	p.mu.Unlock()
	if pin != "" {
		entries = append([]string{pin}, entries...)
	}
	if len(entries) == 0 {
		return ua
	}
	if entries[0] == "default" {
		return ua
	}
	return entries[0]
}

// uaCandidates returns the ordered User-Agent values to walk on the current
// profile when the plugin's current identity is blocked: the declared ladder
// entries after the pinned one, with "default" resolved to the host value.
// A plugin that declared no ladder yields a single entry (the host default),
// so the TLS-profile ladder keeps its current walk shape.
func (p *Proxy) uaCandidates(pluginID string) []string {
	p.mu.Lock()
	entries := p.uaLists[pluginID]
	pin := p.uaPins[pluginID]
	ua, _ := p.defaultHeaders["User-Agent"]
	p.mu.Unlock()
	if len(entries) == 0 {
		return []string{ua}
	}
	var rest []string
	for _, e := range entries {
		if e != pin {
			rest = append(rest, e)
		}
	}
	if len(rest) == 0 {
		rest = []string{entries[0]}
	}
	for i, e := range rest {
		if e == "default" {
			rest[i] = ua
		}
	}
	return rest
}

// SetPinnedProfiles bulk-loads persisted pins at startup.
func (p *Proxy) SetPinnedProfiles(m map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, prof := range m {
		if isKnownProfile(prof) {
			p.pins[id] = prof
		}
	}
}

// SetPersistPin registers a callback invoked whenever a plugin's pin changes.
func (p *Proxy) SetPersistPin(f func(string, string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.persistPin = f
}

// pin returns the currently pinned profile name for pluginID, or "".
func (p *Proxy) pin(pluginID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pins[pluginID]
}

// setPin sets the in-memory pin and persists it (if the callback is set).
func (p *Proxy) setPin(pluginID, profile string) {
	p.mu.Lock()
	p.pins[pluginID] = profile
	cb := p.persistPin
	p.mu.Unlock()
	if cb != nil {
		cb(pluginID, profile)
	}
}

// clearPin removes the in-memory pin and persists the empty string. It also
// drops the plugin's remembered User-Agent pin: resetting a plugin's profile
// from the plugins page re-learns the whole identity, so a stale UA pin
// would keep the block on the next request.
func (p *Proxy) clearPin(pluginID string) {
	p.setPin(pluginID, "")
	p.clearUaPin(pluginID)
}

// ClearPin is the exported wrapper for clearPin, used by the action handler.
func (p *Proxy) ClearPin(pluginID string) {
	p.clearPin(pluginID)
}

// PinnedProfile returns the plugin's pinned profile name, or "" when unpinned.
func (p *Proxy) PinnedProfile(pluginID string) string {
	return p.pin(pluginID)
}

// TestProfile runs a single GET against url using an explicit TLS profile (or
// "stdlib"), returning the resulting HTTP status. It does NOT mutate the
// plugin's pin; used by the plugins-page "Test profile" button.
func (p *Proxy) TestProfile(pluginID, profileName, url string) (int, error) {
	req := types.HTTPRequest{Method: "GET", URL: url}
	if profileName == stdlibProfileName {
		resp, err := p.doRequestStd(context.Background(), pluginID, req)
		if err != nil {
			return 0, err
		}
		return resp.Status, nil
	}
	resp, err := p.doRequestProfile(context.Background(), pluginID, profileName, req)
	if err != nil {
		return 0, err
	}
	return resp.Status, nil
}
