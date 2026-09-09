package hostnet

import "goisekai/pkg/types"

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

// clearPin removes the in-memory pin and persists the empty string.
func (p *Proxy) clearPin(pluginID string) {
	p.setPin(pluginID, "")
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
		resp, err := p.doRequestStd(pluginID, req)
		if err != nil {
			return 0, err
		}
		return resp.Status, nil
	}
	resp, err := p.doRequestProfile(pluginID, profileName, req)
	if err != nil {
		return 0, err
	}
	return resp.Status, nil
}
