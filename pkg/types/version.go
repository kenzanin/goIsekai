package types

import "fmt"

// CheckVersion returns an error when a plugin declares an ABI contract version
// different from the host's ContractVersion. The host calls this after reading
// a plugin's exported contract_version symbol.
func CheckVersion(declared int32) error {
	if declared != ContractVersion {
		return fmt.Errorf("plugin ABI contract version %d does not match host version %d", declared, ContractVersion)
	}
	return nil
}
