package version

import "testing"

func TestStringHasLeadingV(t *testing.T) {
	got := String()
	if got == "" {
		t.Fatal("String() is empty")
	}
	if got[0] != 'v' {
		t.Errorf("String() = %q, want a leading \"v\"", got)
	}
	if got != "v"+Version {
		t.Errorf("String() = %q, want \"v\"+Version = %q", got, "v"+Version)
	}
}

// A release build stamps Version via -ldflags, so it must stay a var and must
// not be empty. A blank version would render as "v" in the About page.
func TestVersionIsNotBlank(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty; the About page would show a bare \"v\"")
	}
}

// `git describe --tags` already prefixes "v", and -ldflags stamps whatever it
// returns. Without normalisation that rendered as "vv0.1.0".
func TestStringNormalisesTheStampedForm(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	for _, tc := range []struct{ stamped, want string }{
		{"0.1.0", "v0.1.0"},
		{"v0.1.0", "v0.1.0"},
		{"v1.2.3-4-gabc1234", "v1.2.3-4-gabc1234"},
		{"1.2.3", "v1.2.3"},
	} {
		Version = tc.stamped
		if got := String(); got != tc.want {
			t.Errorf("Version=%q: String() = %q, want %q", tc.stamped, got, tc.want)
		}
	}
}
