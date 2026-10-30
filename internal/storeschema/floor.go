package storeschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The store floor (docs/archive/design/choir-event-table-f-c-2026-10-22.md 4.5;
// review finding A on deploy 1). The epoch receipt says which binary opened
// the store last, and every older binary's Open rewrites it. The floor says
// the lowest epoch that may run on the store at all, and only binaries that
// know it write it: a binary writes it before it first writes data an older
// binary would mishandle (F-c deploy 2: before the first op stamped for
// choir_events). No binary lowers it, and binaries below it never read or
// touch it. The updater refuses, before mutation, both Apply and its
// automatic recovery (restorePrior) to a release below the floor, or to one
// that declares no epoch; a binary refuses to open a store whose floor is
// above its own epoch.
//
// A restore or rebuild that swaps in a workspace without the file drops it
// with the workspace; Open writes it again when the store holds data that
// needs it.

// FloorFile is the floor's filename, beside the receipt in the workspace.
const FloorFile = "store-floor.json"

// FloorName is the floor's schema contract tag.
const FloorName = "choir-store-floor-v1"

// Floor is the on-disk floor.
type Floor struct {
	Schema  string `json:"schema"`
	Version uint64 `json:"version"`
	Reason  string `json:"reason"`
}

// FloorPath is the floor beside a receipt path (the updater's view).
func FloorPath(receiptPath string) string {
	return filepath.Join(filepath.Dir(receiptPath), FloorFile)
}

// ReadFloor loads the floor at path. found=false means none. A floor that
// exists but cannot be decoded is an error, never "no floor".
func ReadFloor(path string) (Floor, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Floor{}, false, nil
		}
		return Floor{}, false, fmt.Errorf("read store floor: %w", err)
	}
	var floor Floor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&floor); err != nil {
		return Floor{}, false, fmt.Errorf("decode store floor: %w", err)
	}
	if floor.Schema != FloorName || floor.Version == 0 {
		return Floor{}, false, fmt.Errorf("store floor: unrecognised floor %q version %d", floor.Schema, floor.Version)
	}
	return floor, true, nil
}

// WriteFloor raises the floor in workspacePath to version, durably (written,
// synced, renamed, directory synced). It never lowers a floor, and refuses
// to replace one it cannot read.
func WriteFloor(workspacePath string, version uint64, reason string) error {
	path := filepath.Join(workspacePath, FloorFile)
	existing, found, err := ReadFloor(path)
	if err != nil {
		return err
	}
	if found && existing.Version >= version {
		return nil
	}
	raw, err := json.Marshal(Floor{Schema: FloorName, Version: version, Reason: reason})
	if err != nil {
		return err
	}
	temporary := filepath.Join(workspacePath, ".store-floor.tmp")
	f, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("write store floor: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return fmt.Errorf("write store floor: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync store floor: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close store floor: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("publish store floor: %w", err)
	}
	if dir, err := os.Open(workspacePath); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// CheckFloor refuses a store whose floor is above this build's epoch, or
// whose floor cannot be read.
func CheckFloor(workspacePath string) error {
	floor, found, err := ReadFloor(filepath.Join(workspacePath, FloorFile))
	if err != nil {
		return fmt.Errorf("runtime store: %w (refusing to open: the floor cannot be proven)", err)
	}
	if found && floor.Version > Version {
		return fmt.Errorf("runtime store: store floor %d is above this build's epoch %d (%s); refusing to open", floor.Version, Version, floor.Reason)
	}
	return nil
}
