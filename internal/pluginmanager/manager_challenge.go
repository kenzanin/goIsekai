package pluginmanager

import (
	"errors"

	"goisekai/internal/hostnet"
)

// parkChallenge records an anti-bot challenge for pluginID until the running
// invoke finishes. Non-challenge errors are ignored: those already reach the
// guest through the helper's own return value, and only a challenge needs the
// verify wizard.
func (m *Manager) parkChallenge(id string, err error) {
	var ce *hostnet.ChallengeError
	if !errors.As(err, &ce) {
		return
	}
	m.challengeMu.Lock()
	defer m.challengeMu.Unlock()
	if m.challenges == nil {
		m.challenges = make(map[string]error)
	}
	m.challenges[id] = ce
}

// clearChallenge drops any challenge left over from an earlier invoke, so a
// transient block cannot fail a later, unrelated call.
func (m *Manager) clearChallenge(id string) {
	m.challengeMu.Lock()
	defer m.challengeMu.Unlock()
	delete(m.challenges, id)
}

// takeChallenge returns and clears the challenge parked for pluginID, or nil.
func (m *Manager) takeChallenge(id string) error {
	m.challengeMu.Lock()
	defer m.challengeMu.Unlock()
	err := m.challenges[id]
	delete(m.challenges, id)
	return err
}
