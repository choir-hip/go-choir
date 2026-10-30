// Package storeschema is the single authority for the guest persistent
// store's schema epoch. internal/store writes the receipt in the sidecar
// directory beside the SQLite state file ("<base>.texture", kept from the
// Dolt era so the updater's path and the floor survive the migration) on Open; the updater reads it pre-mutation for the S2-d
// state-compat gate; the builder stamps it on release receipts. It is a
// leaf package so the updater daemon and host builder binaries need not
// link the embedded-Dolt engine.
package storeschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Version is the persistent-store schema epoch of this build. It MUST be
// bumped whenever the runtime/texture/object-graph schema changes in a way
// that is not freely backward compatible for older binaries — new required
// columns, changed semantics, changed event encoding. A release declares
// the epoch it was built against (store_schema_version) and the oldest
// epoch it can migrate from (min_store_schema_version); the updater refuses
// before mutation when the persisted epoch falls outside that window. That
// is the pre-mutation fence for the failure the vm-3dc68688 crash-loop
// exposed — an August binary exec'ing against an October store.
//
// Epochs:
//   - 1: the unified Dolt workspace.
//   - 2: choir_events, run telemetry in its own table
//     (docs/design/choir-event-table-f-c-2026-10-22.md, deploy 1). An older
//     binary ignores ProjectionOp.Table and calls the table unknown. The
//     epoch fences only the updater's Apply of a release that declares an
//     epoch: Apply refuses an epoch-1 release on an epoch-2 store. A CI
//     revert (deploy-refresh), a platform-follow offer that declares no epoch
//     and the updater's own restorePrior after a failed health probe reach
//     the older binary unfenced; its Open rewrites the receipt to 1. That
//     binary runs, but checkpoint publication is refused until a forward
//     deploy or the design's rollback recipe. The minimum epoch a release
//     migrates from stays 1: deploy 1 migrates an epoch-1 store by creating
//     the empty table.
//   - 3: the stream_seq lease
//     (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md). An
//     object of kind choir.stream_seq_lease on the tape holds the highest
//     stream_seq the computer may hand out. An older binary ignores the row
//     and seeds by its own bounded read; it can allocate a number above the
//     lease and rewrites the receipt to 2. The receipt is therefore read by
//     Open before it is rewritten: a prior receipt below 3 (or absent on a
//     workspace that holds rows) writes the lease untrust marker first
//     (LeaseUntrustedFile), and a marked workspace seeds by the exact read of
//     the event history once. The epoch fences only the updater's Apply of a
//     release that declares one: Apply refuses an epoch-2 release on an
//     epoch-3 store. The epoch-3 release keeps min_store_schema_version at 2
//     or below.
const Version uint64 = 3

// File is the receipt filename inside the store's sidecar directory
// (deriveTextureWorkspacePath(StorePath)).
const File = "store-schema.json"

// Name is the receipt schema contract tag.
const Name = "choir-store-schema-v1"

// Receipt is the on-disk state-compat receipt. It lives inside the
// workspace so a quarantined/replaced store carries its own epoch and a
// fresh workspace starts absent (callers decide whether absent means
// unproven or epoch 0).
type Receipt struct {
	Schema  string `json:"schema"`
	Version uint64 `json:"version"`
}

// Write records the running build's store schema epoch inside
// workspacePath, atomically. It never lowers a newer receipt: a store a
// later binary opened keeps that binary's epoch, so the updater's check
// still sees it (review finding A on F-c deploy 1). A binary that predates
// this rule still lowers it; the floor (floor.go) is the fence for that.
func Write(workspacePath string) error {
	if existing, found, err := Read(filepath.Join(workspacePath, File)); err == nil && found && existing.Version > Version {
		return nil
	}
	raw, err := json.Marshal(Receipt{Schema: Name, Version: Version})
	if err != nil {
		return err
	}
	temporary := filepath.Join(workspacePath, ".store-schema.tmp")
	if err := os.WriteFile(temporary, raw, 0o644); err != nil {
		return fmt.Errorf("runtime store: write schema receipt: %w", err)
	}
	if err := os.Rename(temporary, filepath.Join(workspacePath, File)); err != nil {
		return fmt.Errorf("runtime store: publish schema receipt: %w", err)
	}
	return nil
}

// Read loads a persisted receipt. found=false means none exists — the store
// predates the contract or the workspace is fresh.
func Read(path string) (Receipt, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Receipt{}, false, nil
		}
		return Receipt{}, false, fmt.Errorf("read store schema receipt: %w", err)
	}
	var receipt Receipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, false, fmt.Errorf("decode store schema receipt: %w", err)
	}
	return receipt, true, nil
}
