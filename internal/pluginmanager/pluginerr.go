package pluginmanager

import "strings"

// Plugin error codes. A plugin reports failure as (nil, code) — or
// (nil, "code: detail") when the code takes context — and the host owns the
// wording. Plugins used to invent their own sentences, which produced eleven
// different phrasings of the same four failures and leaked internal detail
// (key ids, upstream paths) into the UI.
//
// A code the host does not know falls back to the plugin's own text, so a
// third-party or not-yet-updated plugin keeps working.
const (
	// ErrNoPages: the chapter page loaded but carried no reader images. The site
	// is serving a broken chapter, as opposed to being unreachable.
	ErrNoPages = "no_pages"

	// ErrUpstreamUnavailable: the site could not be reached at all (dead domain,
	// blocked IP, DNS failure).
	ErrUpstreamUnavailable = "upstream_unavailable"

	// ErrUpstreamAuthFailed: the site answered but withheld what the plugin needs
	// (missing session key or cookie).
	ErrUpstreamAuthFailed = "upstream_auth_failed"

	// ErrDecryptKeyMismatch: the page list was encrypted under a key this session
	// was never served, so a re-fetch is the only way forward.
	ErrDecryptKeyMismatch = "decrypt_key_mismatch"

	// ErrDecryptFailed: decryption ran but the payload was not a page list.
	ErrDecryptFailed = "decrypt_failed"

	// ErrEnvelopeUnrecognised: the upstream returned a shape the plugin does not
	// understand, which usually means the site changed its response format.
	ErrEnvelopeUnrecognised = "envelope_unrecognised"
)

// pluginErrorText maps a code to the message shown to the reader. The wording
// lives here so every plugin reports the same failure identically.
var pluginErrorText = map[string]string{
	ErrNoPages:              "this chapter has no images on the site",
	ErrUpstreamUnavailable:  "the source site could not be reached",
	ErrUpstreamAuthFailed:   "the source site refused the request (auth or block)",
	ErrDecryptKeyMismatch:   "the page list is encrypted under an unknown key; try again in a new session",
	ErrDecryptFailed:        "the page list could not be decrypted",
	ErrEnvelopeUnrecognised: "the source site changed its response format",
}

// PluginError renders a plugin's second return value as user-facing text. An
// empty value yields "", a known code becomes the host's wording (with any
// detail in parentheses), and anything else is passed through untouched so a
// plugin that has not migrated to codes keeps its message.
func PluginError(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	code, detail, hasDetail := strings.Cut(reason, ":")
	text, known := pluginErrorText[code]
	if !known {
		return reason
	}
	if hasDetail {
		if d := strings.TrimSpace(detail); d != "" {
			return text + " (" + d + ")"
		}
	}
	return text
}
