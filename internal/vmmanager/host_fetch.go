package vmmanager

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
)

// Host fetch route, host side (SDC NET-01; internal/hostfetch). A granted
// computer's public web fetches make their network hop through the
// gateway's /provider/v1/fetch instead of the guest's own egress. The grant
// is a file vmctl writes on the computer's root-only credential disk at
// cold boot. Off while the list is empty (nix/node-b.nix, go-choir-vmctl
// Environment):
//
//	CHOIR_HOST_FETCH_COMPUTERS  ComputerIDs and/or class:test, comma or
//	                            space separated
//
// A snapshot resume keeps the old credential disk, so a changed list
// reaches a computer at its next cold boot. Rollback: empty the list and
// cold-boot the computer; it fetches by itself again.
const hostFetchComputersEnv = "CHOIR_HOST_FETCH_COMPUTERS"

// HostFetchConfig is the host switch.
type HostFetchConfig struct {
	Computers map[string]bool
	ClassTest bool
}

func loadHostFetchConfig(getenv func(string) string) HostFetchConfig {
	cfg := HostFetchConfig{Computers: map[string]bool{}}
	for _, entry := range strings.FieldsFunc(getenv(hostFetchComputersEnv), func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if entry == hostfetch.ClassTestToken {
			cfg.ClassTest = true
			continue
		}
		cfg.Computers[entry] = true
	}
	return cfg
}

// hostFetchMaterial is the grant a computer's credential disk carries, or
// nothing for a computer the switch does not name.
func (c HostFetchConfig) hostFetchMaterial(computerID, warmnessClass string, now time.Time) (map[string][]byte, string) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, ""
	}
	basis := ""
	switch {
	case c.Computers[computerID]:
		basis = hostfetch.BasisComputer
	case c.ClassTest && strings.TrimSpace(warmnessClass) == "test":
		basis = hostfetch.BasisClassTest
	default:
		return nil, ""
	}
	raw, _ := json.Marshal(hostfetch.Grant{ComputerID: computerID, Status: hostfetch.GrantGranted, Basis: basis, GrantedAt: now.UTC().Format(time.RFC3339)})
	return map[string][]byte{hostfetch.GrantFile: append(raw, '\n')}, "granted (" + basis + ")"
}
