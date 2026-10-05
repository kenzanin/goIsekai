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
