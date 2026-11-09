//go:build linux

package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPauseDrain = 10 * time.Minute
	defaultPauseTTL   = 10 * time.Minute
)

// pauseFileSync writes data to path atomically: tmp, fsync, rename, fsync dir.
func pauseFileSync(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDirPath(filepath.Dir(path))
}

func syncDirPath(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// writePauseRecord writes the record into base (a capsule state directory).
func writePauseRecord(base string, rec PauseRecord) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return pauseFileSync(filepath.Join(base, PauseRecordName), raw)
}

// readPauseRecord reads and validates the record in base.
func readPauseRecord(base string) (PauseRecord, error) {
	raw, err := os.ReadFile(filepath.Join(base, PauseRecordName))
	if err != nil {
		return PauseRecord{}, err
	}
	var rec PauseRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return PauseRecord{}, fmt.Errorf("pause record: %w", err)
	}
	if rec.Version != PauseRecordVersion || rec.CapsuleID == "" || rec.AssignmentID == "" {
		return PauseRecord{}, fmt.Errorf("pause record: invalid")
	}
	return rec, nil
}

// digestUpper is the upper's own digest and counts: the work tree the
// capsule wrote, over path, type, mode, size and content hash (not mtime).
// walkUpperdir skips upper/computer and the release scaffold, so the digest
// proves the work tree, not those (design, "Digest limits").
func digestUpper(ctx context.Context, upperDir string) (digest string, files int, bytes int64, err error) {
	manifests, err := walkUpperdir(ctx, upperDir)
	if err != nil {
		return "", 0, 0, err
	}
	h := sha256.New()
	for _, m := range manifests {
		fmt.Fprintf(h, "%s\x00%s\x00%o\x00%d\x00%s\x00%t\n", m.Path, m.Type, m.Mode, m.Size, m.Hash, m.Opaque)
		if m.Type == "file" {
			files++
			bytes += m.Size
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), files, bytes, nil
}

func (e *Executor) pausedSpareRoot() string { return filepath.Join(e.stateDir, PausedSpareDir) }

// PauseCapsule pauses one live capsule: admission block, drain, freeze, the
// claude-token scan, the upper digest, then the durable record, whose
// presence is the commit (Decision 4). Before the record a failure unwinds
// to an ordinary live capsule. A capsule already paused returns its record
// (idempotent); one still pausing is ErrPauseInProgress (single-flight).
func (e *Executor) PauseCapsule(ctx context.Context, req PauseRequest) (_ PauseRecord, retErr error) {
	e.mu.RLock()
	caps := e.capsules[req.CapsuleID]
	e.mu.RUnlock()
	if caps == nil {
		return PauseRecord{}, fmt.Errorf("%w: capsule %s not found", ErrPauseRefused, req.CapsuleID)
	}
	if caps.paused.Load() {
		if rec, err := readPauseRecord(filepath.Join(e.stateDir, req.CapsuleID)); err == nil {
			return rec, nil
		}
		return PauseRecord{}, fmt.Errorf("%w: paused capsule %s has no readable record", ErrPauseRefused, req.CapsuleID)
	}
	if caps.Spec.VerifierBundleDir != "" || strings.TrimSpace(caps.Spec.VerifierBinding) != "" {
		return PauseRecord{}, fmt.Errorf("%w: a verifier-slot capsule does not pause", ErrPauseRefused)
	}
	if !caps.pauseBlock.CompareAndSwap(false, true) {
		return PauseRecord{}, ErrPauseInProgress
	}
	base := filepath.Join(e.stateDir, req.CapsuleID)
	froze := false
	defer func() {
		if retErr == nil {
			return
		}
		// Unwind: the record is the commit, so remove any partial one, thaw
		// and lift the admission block; the capsule is ordinary again.
		_ = os.Remove(filepath.Join(base, PauseRecordName))
		_ = os.Remove(filepath.Join(base, PauseRecordName+".tmp"))
		if froze {
			thawCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			_ = caps.Thaw(thawCtx)
			cancel()
		}
		caps.pauseBlock.Store(false)
	}()
	drain := req.DrainTimeout
	if drain <= 0 {
		drain = defaultPauseDrain
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = defaultPauseTTL
	}
	started := time.Now()
	drainCtx, cancel := context.WithTimeout(ctx, drain)
	err := caps.Quiesce(drainCtx)
	cancel()
	if err != nil {
		return PauseRecord{}, fmt.Errorf("drain and freeze: %w", err)
	}
	froze = true
	frozeAt := time.Now()
	if len(caps.claudeToken) > 0 {
		changes, err := caps.Diff(ctx)
		if err != nil {
			return PauseRecord{}, fmt.Errorf("%w: claude token scan: %v", ErrPauseRefused, err)
		}
		if err := e.refuseClaudeTokenInChanges(ctx, caps, changes); err != nil {
			return PauseRecord{}, fmt.Errorf("%w: %v", ErrPauseRefused, err)
		}
	}
	digestStarted := time.Now()
	digest, files, bytes, err := digestUpper(ctx, caps.UpperDir)
	if err != nil {
		return PauseRecord{}, fmt.Errorf("digest upper: %w", err)
	}
	spec := caps.Spec
	spec.InputsDir, spec.CarriedPatchDir, spec.VerifierBundleDir, spec.SeedUpper = "", "", "", ""
	now := time.Now().UTC()
	rec := PauseRecord{
		Version: PauseRecordVersion, AssignmentID: req.AssignmentID, Attempt: req.Attempt, CapsuleID: req.CapsuleID,
		RunID: caps.OwnerRunID, SubjectDigest: req.SubjectDigest, Spec: spec,
		UpperDigest: digest, UpperFiles: files, UpperBytes: bytes, ReleaseCommit: req.ReleaseCommit,
		PausedAt: now, ExpiresAt: now.Add(ttl),
		DrainMS: frozeAt.Sub(started).Milliseconds(), FreezeMS: digestStarted.Sub(frozeAt).Milliseconds(), DigestMS: time.Since(digestStarted).Milliseconds(),
	}
	// A stale consumed record of an earlier resume goes with the next pause.
	_ = os.Remove(filepath.Join(base, PauseConsumedName))
	if err := writePauseRecord(base, rec); err != nil {
		return PauseRecord{}, fmt.Errorf("write pause record: %w", err)
	}
	caps.paused.Store(true)
	return rec, nil
}

// UnpauseCapsule thaws a paused (or pausing) capsule, removes its record and
// lifts the admission block. It is idempotent.
func (e *Executor) UnpauseCapsule(ctx context.Context, capsuleID string) error {
	e.mu.RLock()
	caps := e.capsules[capsuleID]
	e.mu.RUnlock()
	if caps == nil {
		return nil // destroyed meanwhile: nothing to unpause
	}
	base := filepath.Join(e.stateDir, capsuleID)
	// Record first: thawing under a surviving record would let a crash read
	// a running capsule as paused. A removal that fails keeps everything.
	if err := os.Remove(filepath.Join(base, PauseRecordName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pause record: %w", err)
	}
	if err := syncDirPath(base); err != nil {
		return fmt.Errorf("sync pause record removal: %w", err)
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	if state == StateFrozen {
		if err := caps.Thaw(ctx); err != nil {
			return err
		}
	}
	caps.paused.Store(false)
	caps.pauseBlock.Store(false)
	return nil
}

// AckPause marks the record acked (the phase-2 ack: the restart marker is
// written next, under the caller's per-capsule mutex). It refuses a record
// that is missing or past its pre-ack expiry.
func (e *Executor) AckPause(capsuleID string, now time.Time) error {
	base := filepath.Join(e.stateDir, capsuleID)
	rec, err := readPauseRecord(base)
	if err != nil {
		return fmt.Errorf("ack pause: %w", err)
	}
	if rec.Acked {
		return nil
	}
	if !now.Before(rec.ExpiresAt) {
		return fmt.Errorf("ack pause: record expired at %s", rec.ExpiresAt.Format(time.RFC3339))
	}
	rec.Acked, rec.AckedAt = true, now.UTC()
	return writePauseRecord(base, rec)
}

// IsPaused reports whether this process holds the capsule paused.
func (e *Executor) IsPaused(capsuleID string) bool {
	e.mu.RLock()
	caps := e.capsules[capsuleID]
	e.mu.RUnlock()
	return caps != nil && caps.paused.Load()
}

// LookupPause finds a capsule's pause record: in this process's own
// directory (live, or residual when the process has no such capsule), or in
// the area a boot spared it into. A record that cannot be read or parsed is
// an Err: the caller reads it as live, never paused.
func (e *Executor) LookupPause(capsuleID string) PauseLookup {
	if !validCapsuleResidueIdentity(capsuleID) {
		return PauseLookup{Err: fmt.Errorf("pause lookup identity is invalid")}
	}
	own := filepath.Join(e.stateDir, capsuleID)
	spare := filepath.Join(e.pausedSpareRoot(), capsuleID)
	e.mu.RLock()
	caps := e.capsules[capsuleID]
	e.mu.RUnlock()
	ownWhere := PauseResidual
	if caps != nil {
		ownWhere = PauseLive
	}
	for _, c := range []struct {
		dir   string
		where PauseWhere
	}{{own, ownWhere}, {spare, PauseSpared}} {
		rec, err := readPauseRecord(c.dir)
		if err == nil {
			return PauseLookup{Where: c.where, Record: rec}
		}
		if !errors.Is(err, os.ErrNotExist) {
			return PauseLookup{Err: err}
		}
	}
	for _, dir := range []string{own, spare} {
		raw, err := os.ReadFile(filepath.Join(dir, PauseConsumedName))
		if err != nil {
			continue
		}
		var rec PauseRecord
		if json.Unmarshal(raw, &rec) != nil {
			return PauseLookup{Err: fmt.Errorf("pause consumed record unreadable")}
		}
		return PauseLookup{Where: PauseConsumed, Record: rec}
	}
	return PauseLookup{}
}

// SparedPausedIDs lists the capsule ids whose directories this boot spared.
func (e *Executor) SparedPausedIDs() []string {
	entries, err := os.ReadDir(e.pausedSpareRoot())
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, entry.Name())
		}
	}
	return out
}

// PausedSeed is a validated, unmounted pause directory ready for a seeded
// spawn.
type PausedSeed struct {
	Record   PauseRecord
	UpperDir string
	Dir      string
}

// PrepareResume is the first half of Decision 8's resume. It finds the
// pause directory (spared at this boot, or the same boot's own after a
// process-only restart), tears down live residue (kill the cgroup, wait
// until empty, delete it, unmount the merged root), moves a same-boot
// directory into the spare area, validates the record against want and the
// upper's digest and counts after teardown, and takes the record (renames
// it to pause.consumed.json inside the spare directory, so a second attempt
// cannot resume). Any error leaves the directory for DiscardPause.
func (e *Executor) PrepareResume(ctx context.Context, want PauseRecord) (PausedSeed, error) {
	id := want.CapsuleID
	if !validCapsuleResidueIdentity(id) {
		return PausedSeed{}, fmt.Errorf("resume identity is invalid")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, live := e.capsules[id]; live {
		return PausedSeed{}, fmt.Errorf("resume: capsule %s is live in this process", id)
	}
	own := filepath.Join(e.stateDir, id)
	spare := filepath.Join(e.pausedSpareRoot(), id)
	// Residue first: a process-only restart keeps the boot id, so the old
	// merged mount may still be mounted and its cgroup frozen with tasks.
	if err := cleanupOrphanedCapsuleCgroup(id); err != nil {
		return PausedSeed{}, fmt.Errorf("resume: tear down residual cgroup: %w", err)
	}
	if _, err := os.Lstat(own); err == nil {
		root := filepath.Join(own, "root")
		mounted, err := mountedAtPath(root)
		if err != nil {
			return PausedSeed{}, err
		}
		if mounted {
			if err := unmountCapsuleRoot(root); err != nil {
				return PausedSeed{}, fmt.Errorf("resume: unmount residual root: %w", err)
			}
			if still, err := mountedAtPath(root); err != nil || still {
				return PausedSeed{}, fmt.Errorf("resume: residual root stayed mounted (%v)", err)
			}
		}
		if _, err := os.Lstat(spare); err == nil {
			return PausedSeed{}, fmt.Errorf("resume: both %s and the spared copy exist", own)
		}
		if err := os.MkdirAll(e.pausedSpareRoot(), 0o700); err != nil {
			return PausedSeed{}, err
		}
		if err := os.Rename(own, spare); err != nil {
			return PausedSeed{}, fmt.Errorf("resume: move the pause directory aside: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return PausedSeed{}, err
	}
	rec, err := readPauseRecord(spare)
	if err != nil {
		return PausedSeed{}, fmt.Errorf("resume: %w", err)
	}
	if rec.CapsuleID != id || rec.AssignmentID != want.AssignmentID || rec.Attempt != want.Attempt || rec.SubjectDigest != want.SubjectDigest {
		return PausedSeed{}, fmt.Errorf("resume: the pause record does not match the binding")
	}
	if !rec.Acked {
		return PausedSeed{}, fmt.Errorf("resume: the pause was never acknowledged for a restart")
	}
	upper := filepath.Join(spare, "upper")
	digest, files, bytes, err := digestUpper(ctx, upper)
	if err != nil {
		return PausedSeed{}, fmt.Errorf("resume: digest upper: %w", err)
	}
	if digest != rec.UpperDigest || files != rec.UpperFiles || bytes != rec.UpperBytes {
		return PausedSeed{}, fmt.Errorf("resume: the upper does not match the pause record (digest, counts)")
	}
	if err := os.Rename(filepath.Join(spare, PauseRecordName), filepath.Join(spare, PauseConsumedName)); err != nil {
		return PausedSeed{}, fmt.Errorf("resume: take the pause record: %w", err)
	}
	_ = syncDirPath(spare)
	return PausedSeed{Record: rec, UpperDir: upper, Dir: spare}, nil
}

// CompleteResume finishes a seeded spawn: the rest of the old directory is
// deleted and the consumed record is left in the new directory, where a
// second resume finds it.
func (e *Executor) CompleteResume(seed PausedSeed) error {
	raw, err := json.Marshal(seed.Record)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := removePrivateTree(seed.Dir); err != nil {
		return fmt.Errorf("resume: delete the old pause directory: %w", err)
	}
	return pauseFileSync(filepath.Join(e.stateDir, seed.Record.CapsuleID, PauseConsumedName), raw)
}

// DiscardPause deletes every trace of a capsule's pause that is not a live
// capsule of this process: the residual cgroup (killed), the mount, the
// directory of this boot and the spared copy. It is the crash path and the
// close-on-any-resume-error path (Decisions 7 and 8).
func (e *Executor) DiscardPause(ctx context.Context, capsuleID string) error {
	if !validCapsuleResidueIdentity(capsuleID) {
		return fmt.Errorf("discard pause identity is invalid")
	}
	if err := e.CleanupOrphanedCapsule(ctx, capsuleID); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := removePrivateTree(filepath.Join(e.pausedSpareRoot(), capsuleID)); err != nil {
		return fmt.Errorf("discard spared pause directory: %w", err)
	}
	return nil
}

// DeleteSparedPauses deletes every spared pause directory whose capsule id is
// not in keep: after the boot reconcile, a spared dir that was not resumed
// (no open assignment, or a close) goes (failure 25).
func (e *Executor) DeleteSparedPauses(keep map[string]bool) error {
	var errs error
	for _, id := range e.SparedPausedIDs() {
		if keep[id] {
			continue
		}
		e.mu.Lock()
		err := removePrivateTree(filepath.Join(e.pausedSpareRoot(), id))
		e.mu.Unlock()
		errs = errors.Join(errs, err)
	}
	return errs
}

// seedCapsuleUpper renames a paused capsule's preserved upper into the new
// capsule's upper (same persistent filesystem, no copy). Only the executor's
// own spare area may seed, and only that capsule's own upper: nothing else
// of the old directory comes along.
func seedCapsuleUpper(stateDir string, spec SpawnSpec, upper string) error {
	seed := filepath.Clean(spec.SeedUpper)
	if filepath.Dir(seed) != filepath.Join(stateDir, PausedSpareDir, spec.CapsuleID) || filepath.Base(seed) != "upper" {
		return fmt.Errorf("capsule seed upper %q is not a paused capsule's upper", spec.SeedUpper)
	}
	if err := os.Rename(seed, upper); err != nil {
		return fmt.Errorf("capsule seed upper: %w", err)
	}
	return nil
}
