package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The apply journal is the authority for whether a release was installed
// (docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md,
// "Journal-first settlement"). Two facts make it decidable from outside the
// updater:
//   - Apply holds the key's lock from before it reads the journal until it
//     returns, so a holder of the lock knows no Apply for the key is running;
//   - a settle that finds nothing installed writes a terminal `refused`
//     journal (the fence) under that lock, so a later Apply for the key (a
//     request queued behind another, or one whose client gave up) returns the
//     fence and never stages or swaps.

// applyKeyLockPath is the per-key lock file beside the journal.
func applyKeyLockPath(root, idempotencyKey string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(root)), "operations", safeName(idempotencyKey)+".lock")
}

// lockApplyKey takes the key's lock, waiting when wait is set. acquired=false
// (with a nil error) means an Apply for the key holds it.
func lockApplyKey(root, idempotencyKey string, wait bool) (unlock func(), acquired bool, err error) {
	path := applyKeyLockPath(root, idempotencyKey)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	release, acquired, err := lockFile(f, wait)
	if err != nil || !acquired {
		f.Close()
		return nil, false, err
	}
	return func() { release(); f.Close() }, true, nil
}

// FenceState is what the journal of one apply key proves.
type FenceState string

const (
	// FenceInFlight: an Apply for the key is running now.
	FenceInFlight FenceState = "in_flight"
	// FenceNothingInstalled: no Apply is running and none changed `current`:
	// there was no journal, or a dead one that never swapped (both are now
	// fenced), or a terminal outcome other than applied.
	FenceNothingInstalled FenceState = "nothing_installed"
	// FenceApplied: the journal is terminal applied: the release is installed.
	FenceApplied FenceState = "applied"
	// FenceInterrupted: a dead journal past the pointer swap. `current` may
	// name the new release; only the updater's resume (Apply again) settles it.
	FenceInterrupted FenceState = "interrupted"
)

// FenceResult answers FenceApply. Fenced reports that this call wrote the
// fence (there was no terminal journal before it). Outcome is the journal as
// it now stands.
type FenceResult struct {
	State   FenceState
	Fenced  bool
	Outcome JournalOutcome
}

// FenceApply decides, without waiting, what the apply for idempotencyKey did,
// and fences it when it installed nothing and has no terminal journal. reason
// is the fence's failure text.
func FenceApply(root, idempotencyKey, reason string) (FenceResult, error) {
	unlock, acquired, err := lockApplyKey(root, idempotencyKey, false)
	if err != nil {
		return FenceResult{}, fmt.Errorf("updater: apply key lock: %w", err)
	}
	if !acquired {
		return FenceResult{State: FenceInFlight}, nil
	}
	defer unlock()
	journalPath := filepath.Join(filepath.Clean(strings.TrimSpace(root)), "operations", safeName(idempotencyKey)+".json")
	journal, found, err := readJournal(journalPath)
	if err != nil {
		return FenceResult{}, err
	}
	switch {
	case found && journal.Result.Outcome == "applied":
		return FenceResult{State: FenceApplied, Outcome: outcomeOf(journal)}, nil
	case found && journal.Result.Outcome != "":
		return FenceResult{State: FenceNothingInstalled, Outcome: outcomeOf(journal)}, nil
	case found && journal.Phase != "prepared":
		// pointer_swapped, restart_requested, recovering: `current` moved.
		return FenceResult{State: FenceInterrupted, Outcome: outcomeOf(journal)}, nil
	}
	// No journal, or a dead `prepared` one: the release may be staged, but
	// `current` never moved. Fence it.
	if strings.TrimSpace(reason) == "" {
		return FenceResult{}, errors.New("updater: a fence needs a reason")
	}
	fence := operationJournal{
		RequestCommitment: journal.RequestCommitment, Phase: "fenced",
		PriorReleaseDigest: journal.PriorReleaseDigest, PriorReleaseTarget: journal.PriorReleaseTarget,
		TargetReleaseDigest: journal.TargetReleaseDigest, StartedAt: journal.StartedAt,
		Failure: reason,
		Result:  ApplyResult{ReleaseDigest: journal.TargetReleaseDigest, Outcome: "refused"},
	}
	if fence.StartedAt.IsZero() {
		fence.StartedAt = time.Now().UTC().Truncate(time.Microsecond)
	}
	if err := writeJournal(journalPath, fence); err != nil {
		return FenceResult{}, err
	}
	return FenceResult{State: FenceNothingInstalled, Fenced: true, Outcome: outcomeOf(fence)}, nil
}

func outcomeOf(journal operationJournal) JournalOutcome {
	return JournalOutcome{Result: journal.Result, Failure: journal.Failure, Terminal: journal.Result.Outcome != ""}
}
