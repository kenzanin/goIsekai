package pluginmanager

import "testing"

// TestPluginErrorCodes pins the contract that lets plugins report failure
// without owning the wording. Every code must render to distinct text (two
// codes sharing a message would make the UI undiagnosable), and unknown input
// must pass through so plugins that have not migrated keep working.
func TestPluginErrorCodes(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{ErrNoPages, "this chapter has no images on the site"},
		{ErrUpstreamUnavailable, "the source site could not be reached"},
		{ErrDecryptKeyMismatch, "the page list is encrypted under an unknown key; try again in a new session"},
		{ErrEnvelopeUnrecognised, "the source site changed its response format"},
		// Detail is appended so a plugin can name the offending chapter.
		{ErrNoPages + ": chapter 12", "this chapter has no images on the site (chapter 12)"},
		// A bare code with whitespace-padded detail collapses to no parens.
		{ErrDecryptFailed + ":   ", "the page list could not be decrypted"},
		// Unknown input is the plugin's own message, untouched.
		{"some bespoke message", "some bespoke message"},
		// A message that merely contains a colon is not mistaken for a code.
		{"rate limited: slow down", "rate limited: slow down"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := PluginError(tc.reason); got != tc.want {
			t.Errorf("PluginError(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

// TestPluginErrorCodesAreDistinct guards against two codes rendering the same
// text, which would leave the reader unable to tell them apart.
func TestPluginErrorCodesAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for code := range pluginErrorText {
		text := PluginError(code)
		if other, dup := seen[text]; dup {
			t.Errorf("codes %q and %q render the same text %q", other, code, text)
		}
		seen[text] = code
	}
	if len(seen) != len(pluginErrorText) {
		t.Fatalf("rendered %d distinct messages for %d codes", len(seen), len(pluginErrorText))
	}
}

// TestEveryExportedCodeHasText keeps the constant list and the wording table in
// step: a code with no message would fall through to the raw code string.
func TestEveryExportedCodeHasText(t *testing.T) {
	codes := []string{
		ErrNoPages, ErrUpstreamUnavailable, ErrUpstreamAuthFailed,
		ErrDecryptKeyMismatch, ErrDecryptFailed, ErrEnvelopeUnrecognised,
	}
	for _, code := range codes {
		if _, ok := pluginErrorText[code]; !ok {
			t.Errorf("code %q has no entry in pluginErrorText", code)
		}
	}
}
