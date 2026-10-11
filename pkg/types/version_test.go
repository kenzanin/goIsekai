package types

import (
	"strings"
	"testing"
)

// CheckVersion is the ABI handshake: a plugin whose declared contract version
// differs from the host must be rejected, or mismatched layouts corrupt calls
// silently. The exact mismatch text surfaces in the plugin-load error.
func TestCheckVersion(t *testing.T) {
	if err := CheckVersion(ContractVersion); err != nil {
		t.Fatalf("matching version must pass: %v", err)
	}
	if err := CheckVersion(ContractVersion + 1); err == nil {
		t.Fatal("newer declared version must fail")
	}
	if err := CheckVersion(ContractVersion - 1); err == nil {
		t.Fatal("older declared version must fail")
	}
	if err := CheckVersion(ContractVersion + 1); err != nil {
		want := "does not match host version"
		if got := err.Error(); !strings.Contains(got, want) {
			t.Fatalf("error %q must mention %q", got, want)
		}
	}
}
