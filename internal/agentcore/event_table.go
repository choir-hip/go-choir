package agentcore

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/eventtablegrant"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// store.event_table at the runtime seam (run telemetry in its own table,
// deploy 2: docs/design/choir-event-table-f-c-2026-10-22.md 4.2). The store's
// writer stamps the ten batched telemetry kinds for choir_events only when
// both keys hold:
//
//   - the host named this computer: vmctl's CHOIR_EVENT_TABLE_COMPUTERS
//     writes a grant onto the root-only credential disk at cold boot
//     (internal/vmmanager/event_table_grant.go). Read once per boot; the disk
//     changes only at a cold boot;
//   - the owner set store.event_table: an owner act through
//     /api/computer-settings, stored on the tape. Applied at once and read
//     again at every boot. Not on choir.ProductAPI's allowlist, and refused
//     to any caller inside the guest (server.AuthenticatedOwner), so no desk,
//     model or guest process sets it.
//
// The setting off is the live rollback: new events go to og_objects, and the
// events already in choir_events stay there and stay readable (the readers
// read both tables). Nothing is moved back.

// defaultEventTableCredentialDir is where the guest mounts the credential
// disk (nix/autoputer-vm.nix, /run/choir-bootstrap).
const defaultEventTableCredentialDir = "/run/choir-bootstrap"

// EventTableStatus is the switch as this computer applies it, for
// /api/runtime/observability.
type EventTableStatus struct {
	HostGranted bool   `json:"host_granted"`
	Setting     bool   `json:"setting"`
	Writing     bool   `json:"writing"`
	Reason      string `json:"reason"`
	// StoreFloor is the lowest store schema epoch that may run on this
	// store (0: none). Once the writer has been on it is 2: no binary below
	// deploy 1 may boot here (design 4.5). Read before any rollback.
	StoreFloor uint64 `json:"store_floor,omitempty"`
}

func (rt *Runtime) eventTableCredentialDir() string {
	if dir := strings.TrimSpace(rt.eventTableGrantDir); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("CHOIR_EVENT_TABLE_CREDENTIAL_DIR")); dir != "" {
		return dir
	}
	return defaultEventTableCredentialDir
}

// configureEventTable applies the switch at boot, before the first event of
// the process: the grant from the credential disk and the owner's stored
// setting.
func (rt *Runtime) configureEventTable(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	grant := eventtablegrant.ReadGrant(rt.eventTableCredentialDir(), rt.cfg.ComputerID)
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	setting := rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingEventTable)
	rt.eventTableMu.Lock()
	defer rt.eventTableMu.Unlock()
	rt.eventTableGrant = &grant
	rt.setEventTableLocked(setting)
}

// applyEventTableSetting is the owner's setting change at runtime.
func (rt *Runtime) applyEventTableSetting(value bool) {
	if rt == nil || rt.store == nil {
		return
	}
	rt.eventTableMu.Lock()
	defer rt.eventTableMu.Unlock()
	if rt.eventTableGrant == nil {
		grant := eventtablegrant.ReadGrant(rt.eventTableCredentialDir(), rt.cfg.ComputerID)
		rt.eventTableGrant = &grant
	}
	rt.setEventTableLocked(value)
}

func (rt *Runtime) setEventTableLocked(setting bool) {
	grant := *rt.eventTableGrant
	st := EventTableStatus{HostGranted: grant.Granted, Setting: setting}
	switch {
	case !setting:
		st.Reason = "store.event_table is off"
	case !grant.Granted:
		st.Reason = "store.event_table is set, but " + grant.Reason
	case !rt.store.ProjectionTapeBound():
		st.Reason = "store.event_table is set, but the projection tape is not bound"
	default:
		st.Writing = true
		st.Reason = "batched telemetry is written to choir_events"
	}
	if err := rt.store.SetEventTableWrites(st.Writing); err != nil {
		// The floor could not be written: the writer stays off (fail closed).
		st.Writing = false
		st.Reason = "store.event_table is set, but " + err.Error()
	}
	if st.Setting || st.HostGranted {
		log.Printf("runtime: event table: %s (host grant: %s)", st.Reason, grant.Reason)
	}
	rt.eventTableState = st
}

// eventTableStatus returns the switch as last applied, with the store floor
// read now.
func (rt *Runtime) eventTableStatus() EventTableStatus {
	rt.eventTableMu.Lock()
	st := rt.eventTableState
	rt.eventTableMu.Unlock()
	if rt.store != nil {
		// The store's own switch is the truth: Reopen turns it off when it
		// cannot floor an adopted workspace, which this copy never hears of
		// (bunch 148, Devin 2b).
		st.Writing = st.Writing && rt.store.EventTableWritesOn()
		floor, err := rt.store.StoreFloor()
		if err != nil {
			st.Reason += "; the store floor is unreadable: " + err.Error()
		}
		st.StoreFloor = floor
	}
	return st
}
