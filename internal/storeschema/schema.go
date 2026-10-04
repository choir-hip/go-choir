// Package storeschema is the single authority for the guest persistent
// store's schema epoch. internal/store writes the receipt inside the Dolt
// workspace on Open; the updater reads it pre-mutation for the S2-d
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
const Version uint64 = 1

// File is the receipt filename inside the unified Dolt workspace
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
// workspacePath, atomically.
func Write(workspacePath string) error {
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
