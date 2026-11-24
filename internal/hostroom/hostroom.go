// Package hostroom is the bound on sign-ups: the host can hold one more
// computer. Sign-up is open when the free disk, less a reserve, less the room
// the admitted accounts will still take (Plan.Held), is at least one
// computer's footprint. The auth service decides it inside the transaction
// that makes the account; the capacity reading on Pulse shows the same number.
// A missing or stale reading means closed, never open.
//
// What is scarce is the host's disk, for computers. An account is not scarce,
// so no number of accounts is chosen (owner, 2026-11-10 on bounds: learned or
// measured, not chosen).
package hostroom

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The least a plan may say. A reserve under the 60 GiB deploy floor, or a
// footprint under the largest computer measured (drill: 13,297 MB of data
// image plus a 1.5 GB memory snapshot while hibernated,
// docs/problems/computer-disks-hold-self-development-leftovers-2026-11-24.md,
// about 14 GiB, rounded up), would admit past what the host can hold; a typo
// that went under them is refused where the plan is read.
const (
	MinReserveGiB   = 60
	MinFootprintGiB = 15
)

// MaxOwnersAge is how old the owners file may be before sign-up closes. vmctl
// writes it every half minute.
const MaxOwnersAge = 3 * time.Minute

// maxClockSkew is how far into the future a written_at may be.
const maxClockSkew = 30 * time.Second

// Plan is what the bound is made of.
type Plan struct {
	// ReserveBytes is the disk that stays free whatever happens: the deploy
	// floor and the paper's growth.
	ReserveBytes uint64
	// FootprintBytes is the disk one new computer is planned to take.
	FootprintBytes uint64
}

// Valid reports whether the plan can bound anything.
func (p Plan) Valid() bool { return p.FootprintBytes > 0 }

// Places is how many more computers the host can hold: the free disk over the
// reserve, less heldBytes (Held), in whole footprints.
func (p Plan) Places(freeBytes, heldBytes uint64) int {
	if !p.Valid() || freeBytes <= p.ReserveBytes {
		return 0
	}
	room := freeBytes - p.ReserveBytes
	if heldBytes >= room {
		return 0
	}
	return int((room - heldBytes) / p.FootprintBytes)
}

// Held is the disk the accounts already admitted will still take, which the
// free reading does not show yet. accountIDs are the accounts that hold a
// proved passkey. An account with no computer will take a whole footprint. An
// account whose computer exists has taken only what its disk holds now, and
// will take the rest of a footprint as it grows, so the gap is held back
// until it has grown: opening a computer must not make room. A computer
// already past a footprint holds nothing back.
func (p Plan) Held(accountIDs []string, owners map[string]uint64) uint64 {
	seen := make(map[string]bool, len(accountIDs))
	var held uint64
	for _, id := range accountIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		disk, has := owners[id]
		switch {
		case !has:
			held += p.FootprintBytes
		case disk < p.FootprintBytes:
			held += p.FootprintBytes - disk
		}
	}
	return held
}

// Unopened counts the distinct accounts in accountIDs that have no computer.
func Unopened(accountIDs []string, owners map[string]uint64) int {
	seen := make(map[string]bool, len(accountIDs))
	n := 0
	for _, id := range accountIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, has := owners[id]; !has {
			n++
		}
	}
	return n
}

// Owner is an account that has a computer and the disk that computer holds
// now (allocated bytes of its data image).
type Owner struct {
	UserID    string `json:"user_id"`
	DiskBytes uint64 `json:"disk_bytes"`
	// InexactFiles counts the files (and snapshots) of this account's
	// computers that were not measured exactly and so were counted as 0: the
	// room holds their footprint back, and the deployed proof fails while any
	// is nonzero.
	InexactFiles int `json:"inexact_files,omitempty"`
}

type ownersFile struct {
	WrittenAt time.Time `json:"written_at"`
	Owners    []Owner   `json:"owners"`
}

// WriteOwners writes the accounts that have a computer and the disk each
// holds now, atomically.
func WriteOwners(path string, at time.Time, owners []Owner) error {
	raw, err := json.Marshal(ownersFile{WrittenAt: at.UTC(), Owners: owners})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".owners-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ReadOwners reads the accounts that have a computer and the disk each holds.
// A missing, torn, stale or future-dated file is an error: the caller closes
// sign-up.
func ReadOwners(path string, now time.Time, maxAge time.Duration) (map[string]uint64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("computer owners: %w", err)
	}
	var f ownersFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("computer owners: %w", err)
	}
	if f.WrittenAt.IsZero() {
		return nil, errors.New("computer owners: no time written")
	}
	if age := now.Sub(f.WrittenAt); age > maxAge || age < -maxClockSkew {
		return nil, fmt.Errorf("computer owners: written %s ago", age.Round(time.Second))
	}
	out := make(map[string]uint64, len(f.Owners))
	for _, o := range f.Owners {
		out[o.UserID] = o.DiskBytes
	}
	return out, nil
}
