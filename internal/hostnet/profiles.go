package hostnet

import (
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	// stdlibProfileName is the sentinel pin that routes via doRequestStd.
	stdlibProfileName = "stdlib"

	// defaultProfileName is the initial TLS profile before any rotation.
	defaultProfileName = "chrome_146"
)

// profileByName maps a lowercase profile name to its tls-client constant.
func profileByName(name string) (profiles.ClientProfile, bool) {
	switch name {
	case "chrome_146":
		return profiles.Chrome_146, true
	case "chrome_120":
		return profiles.Chrome_120, true
	case "chrome_124":
		return profiles.Chrome_124, true
	case "chrome_133":
		return profiles.Chrome_133, true
	case "chrome_131":
		return profiles.Chrome_131, true
	case "firefox_133":
		return profiles.Firefox_133, true
	case "firefox_148":
		return profiles.Firefox_148, true
	case "firefox_117":
		return profiles.Firefox_117, true
	case "safari_16_0":
		return profiles.Safari_16_0, true
	case "opera_90":
		return profiles.Opera_90, true
	case "brave_146":
		return profiles.Brave_146, true
	default:
		return profiles.ClientProfile{}, false
	}
}

// isKnownProfile reports whether name is a valid tls-client profile or "stdlib".
func isKnownProfile(name string) bool {
	if name == stdlibProfileName {
		return true
	}
	_, ok := profileByName(name)
	return ok
}

// defaultLadder returns the global fallback rotation ladder tried after
// plugin-declared hints are exhausted. "stdlib" is always last.
func defaultLadder() []string {
	return []string{
		"chrome_146",
		"firefox_133",
		"safari_16_0",
		"chrome_120",
		"firefox_117",
		"opera_90",
		"brave_146",
		stdlibProfileName,
	}
}

// AvailableProfiles returns every selectable profile name (known tls profiles
// plus "stdlib") in ladder order, for the plugins-page profile UI.
func (p *Proxy) AvailableProfiles() []string {
	return defaultLadder()
}
