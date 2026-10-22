// Package eventtablegrant is the host half of store.event_table (run
// telemetry in its own table, docs/design/choir-event-table-f-c-2026-10-22.md
// 4.2). vmctl writes the grant onto a named computer's root-only credential
// disk at cold boot (internal/vmmanager/event_table_grant.go); the guest
// runtime reads it at boot (internal/agentcore/event_table.go). The writer
// stamps ops for choir_events only when the grant is here and the owner's
// setting is on: once an op is stamped deploy 1 is that computer's rollback
// floor, so the platform names the computer, never the guest. No grant, no
// change: events stay in og_objects.
package eventtablegrant

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	// GrantFile is the grant's name on the credential disk.
	GrantFile = "event-table.json"
	// GrantGranted is the only status that grants.
	GrantGranted = "granted"
	// BasisComputer is the only basis: the host names ComputerIDs.
	BasisComputer = "computer"
)

// Grant is the grant record.
type Grant struct {
	ComputerID string `json:"computer_id"`
	Status     string `json:"status"`
	Basis      string `json:"basis"`
	GrantedAt  string `json:"granted_at"`
}

// Status is the grant as this computer reads it.
type Status struct {
	Granted    bool   `json:"granted"`
	Reason     string `json:"reason"`
	ComputerID string `json:"computer_id,omitempty"`
}

// ReadGrant reads the grant from the credential directory for computerID.
// Anything but a well-formed granted record naming this computer is off.
// Reasons never quote file contents.
func ReadGrant(dir, computerID string) Status {
	dir, computerID = strings.TrimSpace(dir), strings.TrimSpace(computerID)
	if dir == "" {
		return Status{Reason: "no credential directory is configured"}
	}
	if computerID == "" {
		return Status{Reason: "this computer's id is unknown"}
	}
	raw, err := os.ReadFile(filepath.Join(dir, GrantFile))
	if errors.Is(err, os.ErrNotExist) {
		return Status{Reason: "this computer is not granted the event table by the host (no grant on the credential disk)"}
	}
	if err != nil {
		return Status{Reason: "the event table grant is unreadable"}
	}
	var g Grant
	if err := json.Unmarshal(raw, &g); err != nil {
		return Status{Reason: "the event table grant is malformed"}
	}
	if g.Status != GrantGranted {
		return Status{Reason: "the host withheld the event table"}
	}
	if strings.TrimSpace(g.ComputerID) != computerID {
		return Status{Reason: "the event table grant names another computer"}
	}
	return Status{Granted: true, ComputerID: computerID, Reason: "granted (" + BasisComputer + ")"}
}
