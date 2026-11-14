package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// What an apply installed is decided by one rule
// (docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md,
// "Journal-first settlement"):
//   - Apply holds the key's lock from before it reads the journal until it
//     returns, so a holder of the lock knows no Apply for the key is running;
//   - a terminal journal is the authority;
//   - a dead journal (nobody holds the lock and it is missing, non-terminal or
//     corrupt) is arbitrated on the `current` pointer, the ground truth of what
//     is installed: the journal's prior release (or no journal at all) is
//     nothing installed, its target release is installed, anything else is
//     unknown. No apply input (bundle, pinned release) is needed.
// A settle that acts on a dead journal fences it (a terminal `refused`
// journal) so a later Apply for the key, queued or resumed, returns the fence
// and never stages, swaps or restores.

// keyLockMarker is written by every updater of this build when it starts: the
// daemon takes the per-key lock. Without it (an older daemon in the base
// image) a free lock proves nothing, and a missing or non-terminal journal
// cannot be called dead.
const keyLockMarker = ".key-lock-v1"

func operationsDir(root string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(root)), "operations")
}

func writeKeyLockMarker(root string) error {
	return os.WriteFile(filepath.Join(operationsDir(root), keyLockMarker), []byte("apply takes the per-key lock\n"), 0o600)
}

// lockApplyKey takes the key's lock, waiting when wait is set. acquired=false
// (with a nil error) means an Apply for the key holds it. Lock files are kept:
// removing one while another process waits on it would split the lock.
func lockApplyKey(root, idempotencyKey string, wait bool) (unlock func(), acquired bool, err error) {
	dir := operationsDir(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(dir, safeName(idempotencyKey)+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
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

// ApplyVerdict is what the journal of one apply key, arbitrated on `current`
// when it is dead, proves.
type ApplyVerdict string

const (
	// VerdictInFlight: an Apply for the key is running now.
	VerdictInFlight ApplyVerdict = "in_flight"
	// VerdictLockUnproven: the journal is missing or non-terminal and the
	// daemon predates the per-key lock, so a running Apply cannot be ruled out.
	VerdictLockUnproven ApplyVerdict = "lock_unproven"
	// VerdictApplied: the journal is terminal applied (signed receipts).
	VerdictApplied ApplyVerdict = "applied"
	// VerdictFailed: the journal is terminal with any other outcome.
	VerdictFailed ApplyVerdict = "failed"
	// VerdictNothingInstalled: dead, and `current` is the journal's prior
	// release (or there is no journal).
	VerdictNothingInstalled ApplyVerdict = "nothing_installed"
	// VerdictInstalled: dead, and `current` is the journal's target release.
	VerdictInstalled ApplyVerdict = "installed"
	// VerdictUnknown: dead, and `current` is neither (or the journal is
	// corrupt, or `current` is unreadable).
	VerdictUnknown ApplyVerdict = "unknown"
)

// ApplyKeyHold is a held apply key and its verdict. While it is held no Apply
// for the key can start. Release it; Fence it after acting on a dead verdict.
type ApplyKeyHold struct {
	Verdict ApplyVerdict
	// Outcome is a terminal journal's outcome.
	Outcome JournalOutcome
	// Prior, Target and Current are the release digests a dead journal was
	// arbitrated on ("" when unknown).
	Prior, Target, Current string
	// Cause says why a dead verdict is what it is.
	Cause string

	root, key string
	unlock    func()
	journal   operationJournal
}

// NoApplyKey is the hold of an operation that has no apply: nothing installed.
func NoApplyKey() *ApplyKeyHold {
	return &ApplyKeyHold{Verdict: VerdictNothingInstalled, Cause: "the operation has no apply"}
}

// Dead reports a verdict reached by arbitration, which Fence makes durable.
func (h *ApplyKeyHold) Dead() bool {
	switch h.Verdict {
	case VerdictNothingInstalled, VerdictInstalled, VerdictUnknown:
		return h.root != ""
	}
	return false
}

// Release lets the key go. It is idempotent.
func (h *ApplyKeyHold) Release() {
	if h != nil && h.unlock != nil {
		h.unlock()
		h.unlock = nil
	}
}

// Fence writes a terminal `refused` journal for a dead verdict, keeping the
// journal's prior and target, so no later Apply for the key acts. It is a
// no-op for any other verdict or a released hold.
func (h *ApplyKeyHold) Fence(reason string) error {
	if h == nil || !h.Dead() || h.unlock == nil {
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("updater: a fence needs a reason")
	}
	phase := "fenced"
	if h.Verdict != VerdictNothingInstalled {
		phase = "arbitrated"
	}
	fence := operationJournal{
		RequestCommitment: h.journal.RequestCommitment, Phase: phase,
		PriorReleaseDigest: h.journal.PriorReleaseDigest, PriorReleaseTarget: h.journal.PriorReleaseTarget,
		TargetReleaseDigest: h.journal.TargetReleaseDigest, StartedAt: h.journal.StartedAt,
		Verdict: string(h.Verdict),
		Failure: reason + " (" + string(h.Verdict) + ": " + h.Cause + ")",
		Result:  ApplyResult{ReleaseDigest: h.journal.TargetReleaseDigest, PriorReleaseDigest: h.journal.PriorReleaseDigest, Outcome: "refused"},
	}
	if fence.StartedAt.IsZero() {
		fence.StartedAt = time.Now().UTC().Truncate(time.Microsecond)
	}
	return writeJournal(filepath.Join(operationsDir(h.root), safeName(h.key)+".json"), fence)
}

// HoldApplyKey takes the key's lock without waiting and returns the verdict.
// An in-flight or lock-unproven verdict holds nothing.
func HoldApplyKey(root, idempotencyKey string) (*ApplyKeyHold, error) {
	unlock, acquired, err := lockApplyKey(root, idempotencyKey, false)
	if err != nil {
		return nil, fmt.Errorf("updater: apply key lock: %w", err)
	}
	if !acquired {
		return &ApplyKeyHold{Verdict: VerdictInFlight}, nil
	}
	hold := &ApplyKeyHold{root: root, key: idempotencyKey, unlock: unlock}
	raw, readErr := os.ReadFile(filepath.Join(operationsDir(root), safeName(idempotencyKey)+".json"))
	found := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		hold.Release()
		return nil, readErr
	}
	corrupt := false
	if found {
		if err := json.Unmarshal(raw, &hold.journal); err != nil {
			corrupt, hold.journal = true, operationJournal{}
		} else if hold.journal.Result.Outcome != "" {
			hold.Outcome = outcomeOf(hold.journal)
			hold.Verdict = VerdictFailed
			if hold.journal.Result.Outcome == "applied" {
				hold.Verdict = VerdictApplied
			}
			return hold, nil
		}
	}
	if _, err := os.Stat(filepath.Join(operationsDir(root), keyLockMarker)); err != nil {
		hold.Release()
		return &ApplyKeyHold{Verdict: VerdictLockUnproven, Cause: "the updater daemon predates the per-key lock"}, nil
	}
	current, _, currentErr := currentReleaseOf(root)
	hold.Current, hold.Prior, hold.Target = current, hold.journal.PriorReleaseDigest, hold.journal.TargetReleaseDigest
	switch {
	case !found:
		hold.Verdict, hold.Cause = VerdictNothingInstalled, "no journal: the updater never started"
	case corrupt:
		hold.Verdict, hold.Cause = VerdictUnknown, "the journal is corrupt"
	case currentErr != nil:
		hold.Verdict, hold.Cause = VerdictUnknown, "the current release pointer is unreadable: "+currentErr.Error()
	case current == hold.Prior || hold.Prior == hold.Target:
		hold.Verdict, hold.Cause = VerdictNothingInstalled, "current is the journal's prior release "+short(current)
	case current == hold.Target:
		hold.Verdict, hold.Cause = VerdictInstalled, "current is the journal's target release "+short(current)
	default:
		hold.Verdict, hold.Cause = VerdictUnknown, fmt.Sprintf("current %s is neither the journal's prior %s nor its target %s", short(current), short(hold.Prior), short(hold.Target))
	}
	return hold, nil
}

func short(digest string) string {
	if digest == "" {
		return "(none)"
	}
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func outcomeOf(journal operationJournal) JournalOutcome {
	return JournalOutcome{Result: journal.Result, Failure: journal.Failure, Terminal: journal.Result.Outcome != ""}
}
