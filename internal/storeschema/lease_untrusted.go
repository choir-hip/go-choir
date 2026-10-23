package storeschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The stream_seq lease untrust marker
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 3).
//
// The lease on the tape is trusted only when nothing in this workspace's
// history can have outrun it. The receipt says which binary last opened the
// store, and the marker says a workspace was swapped in (a base install, a
// rebase, a rematerialize, a replaced workspace) or that a prior receipt was
// below the epoch that writes leases. A marked workspace seeds by the exact
// read of the event history, once, and the seeder clears the marker after its
// lease lands.
//
// Every writer of a workspace that becomes a live store path writes the marker
// into the incoming workspace, durably, before the swap. This is a leaf
// package so the projection-base publisher and installer, the updater and the
// host builder can write it without linking the embedded Dolt engine.

// LeaseUntrustedFile is the marker's filename, beside the receipt in the
// workspace root.
const LeaseUntrustedFile = "stream-seq-lease-untrusted.json"

// LeaseUntrustedName is the marker body's schema contract tag.
const LeaseUntrustedName = "choir-stream-seq-lease-untrusted-v1"

// leaseUntrustedBody is the marker's content.
type leaseUntrustedBody struct {
	Schema string `json:"schema"`
	Reason string `json:"reason"`
}

// LeaseUntrustedBody is the marker's bytes for a reason, for a caller that
// carries it as a tar entry rather than a file (the projection-base
// publisher). It is the same JSON WriteLeaseUntrusted writes.
func LeaseUntrustedBody(reason string) []byte {
	raw, err := json.Marshal(leaseUntrustedBody{Schema: LeaseUntrustedName, Reason: reason})
	if err != nil {
		// Two strings cannot fail to marshal.
		panic(fmt.Sprintf("storeschema: marshal lease untrust marker: %v", err))
	}
	return raw
}

// WriteLeaseUntrusted writes the marker into workspacePath durably: the bytes
// are written to a temporary file in the same directory, synced, renamed over
// the marker and the directory synced. Writing it again is idempotent. An
// error means the marker may not be durable, and the caller must not proceed
// with the swap (or the store open) the marker guards.
func WriteLeaseUntrusted(workspacePath, reason string) error {
	if info, err := os.Stat(workspacePath); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("%s is not a directory", workspacePath)
		}
		return fmt.Errorf("runtime store: write lease untrust marker: %w", err)
	}
	temporary, err := os.CreateTemp(workspacePath, ".stream-seq-lease-untrusted-*.tmp")
	if err != nil {
		return fmt.Errorf("runtime store: write lease untrust marker: %w", err)
	}
	temporaryPath := temporary.Name()
	published := false
	defer func() {
		if !published {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(LeaseUntrustedBody(reason)); err != nil {
		return fmt.Errorf("runtime store: write lease untrust marker: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("runtime store: sync lease untrust marker: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("runtime store: close lease untrust marker: %w", err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(workspacePath, LeaseUntrustedFile)); err != nil {
		return fmt.Errorf("runtime store: publish lease untrust marker: %w", err)
	}
	published = true
	if err := syncDirectory(workspacePath); err != nil {
		return fmt.Errorf("runtime store: sync lease untrust marker directory: %w", err)
	}
	return nil
}

// LeaseUntrusted reports whether the workspace carries the marker. Anything
// that is not a clean absence reads as present: a directory, an empty file or
// a corrupt file where the marker belongs closes the lease, never opens it. An
// error is returned only when presence cannot be decided at all (a stat error
// other than not-exist), and the caller must treat that as untrusted too.
func LeaseUntrusted(workspacePath string) (bool, error) {
	_, err := os.Lstat(filepath.Join(workspacePath, LeaseUntrustedFile))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return true, fmt.Errorf("runtime store: stat lease untrust marker: %w", err)
	}
}

// LeaseUntrustedReason returns the reason the marker was written with. A
// marker that cannot be read or decoded is present with the reason "unreadable".
func LeaseUntrustedReason(workspacePath string) (reason string, present bool, err error) {
	raw, readErr := os.ReadFile(filepath.Join(workspacePath, LeaseUntrustedFile))
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return "", false, nil
		}
		return "unreadable", true, nil
	}
	var body leaseUntrustedBody
	if json.Unmarshal(raw, &body) != nil || body.Schema != LeaseUntrustedName || body.Reason == "" {
		return "unreadable", true, nil
	}
	return body.Reason, true, nil
}

// ClearLeaseUntrusted removes the marker after a seeder's exact-basis lease has
// landed, and syncs the directory. A marker that is not there is not an error.
func ClearLeaseUntrusted(workspacePath string) error {
	if err := os.Remove(filepath.Join(workspacePath, LeaseUntrustedFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("runtime store: clear lease untrust marker: %w", err)
	}
	if err := syncDirectory(workspacePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("runtime store: sync lease untrust marker directory: %w", err)
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
