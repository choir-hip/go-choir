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
// cold boot. The switch is CHOIR_HOST_FETCH_VMS (hostfetch.EnvVMs): VM IDs,
// because the gateway, which reads the same list from its own environment
// and refuses every other caller, knows a guest only by its VM ID (the
// credential identity). Off while empty (nix/node-b.nix, one binding for
// both units).
//
// A snapshot resume keeps the old credential disk, so a changed list
// reaches a computer at its next cold boot. Rollback: empty the list,
// deploy, cold-boot the computer; it fetches by itself again.
const hostFetchVMsEnv = hostfetch.EnvVMs

// HostFetchConfig is the host switch.
type HostFetchConfig struct {
	VMs map[string]bool
}

func loadHostFetchConfig(getenv func(string) string) HostFetchConfig {
	return HostFetchConfig{VMs: hostfetch.ParseVMList(getenv(hostFetchVMsEnv))}
}

// hostFetchMaterial is the grant a computer's credential disk carries, or
// nothing for a VM the switch does not name.
func (c HostFetchConfig) hostFetchMaterial(computerID, vmID string, now time.Time) (map[string][]byte, string) {
	computerID, vmID = strings.TrimSpace(computerID), strings.TrimSpace(vmID)
	if computerID == "" || vmID == "" || !c.VMs[vmID] {
		return nil, ""
	}
	raw, _ := json.Marshal(hostfetch.Grant{ComputerID: computerID, Status: hostfetch.GrantGranted, Basis: hostfetch.BasisVM, GrantedAt: now.UTC().Format(time.RFC3339)})
	return map[string][]byte{hostfetch.GrantFile: append(raw, '\n')}, "granted (vm)"
}
