package vmmanager

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/eventtablegrant"
)

// The store.event_table selector, host side (run telemetry in its own
// table, deploy 2: docs/design/choir-event-table-f-c-2026-10-22.md 4.2). The
// guest's writer stamps telemetry ops for choir_events only on a computer
// named here and with the owner's setting on. Off while the list is empty
// (nix/node-b.nix, go-choir-vmctl Environment):
//
//	CHOIR_EVENT_TABLE_COMPUTERS  ComputerIDs, comma or space separated
//
// ComputerIDs only: no class:test and no "*". Once a computer stamps an op,
// deploy 1 is its rollback floor, so each computer is a platform decision
// made by name. A snapshot resume keeps the old credential disk, so a
// change reaches a computer at its next cold boot. Removing a computer stops
// new stamps after its next cold boot; its choir_events rows stay and stay
// readable. The live rollback is the owner's setting off.
const eventTableComputersEnv = "CHOIR_EVENT_TABLE_COMPUTERS"

// EventTableConfig is the host switch.
type EventTableConfig struct {
	Computers map[string]bool
}

func loadEventTableConfig(getenv func(string) string) EventTableConfig {
	cfg := EventTableConfig{Computers: map[string]bool{}}
	for _, entry := range parseComputerSelector(getenv(eventTableComputersEnv)) {
		if strings.Contains(entry, ":") || strings.Contains(entry, "*") {
			continue // a class or wildcard token; computers are named
		}
		cfg.Computers[entry] = true
	}
	return cfg
}

// eventTableMaterial is the grant a computer's credential disk carries, or
// nothing for a computer the switch does not name.
func (c EventTableConfig) eventTableMaterial(computerID, _ string, now time.Time) (map[string][]byte, string) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || !c.Computers[computerID] {
		return nil, ""
	}
	raw, _ := json.Marshal(eventtablegrant.Grant{
		ComputerID: computerID, Status: eventtablegrant.GrantGranted,
		Basis: eventtablegrant.BasisComputer, GrantedAt: now.UTC().Format(time.RFC3339),
	})
	return map[string][]byte{eventtablegrant.GrantFile: append(raw, '\n')}, "granted (" + eventtablegrant.BasisComputer + ")"
}
