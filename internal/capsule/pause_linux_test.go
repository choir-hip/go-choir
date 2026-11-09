//go:build linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Sandbox pause and resume (docs/design/sandbox-pause-resume-2026-11-09.md):
// the capsule-side failure modes, written first.
//
// Ways this can fail:
//   - the pause record commits before the capsule is quiet (a cell still
//     running, an op admitted during the drain);
//   - a drain that times out, a token hit or a failed write leaves the
//     capsule frozen or blocked (mode 13, unwind);
//   - a boot spares a directory whose record is unacknowledged, mismatched
//     or from a restore-reason marker (modes 2, 20);
//   - the resume adopts instead of seeding, or seeds from outside the spare
//     area (mode 18 class);
//   - the resume proceeds over a tampered or truncated upper (mode 7);
//   - a process-only restart's residue (merged mount, frozen cgroup) is not
//     torn down before the move (modes 12, 17);
//   - a paused capsule can be thawed, extracted or inspected behind the
//     pause's back (mode 26);
//   - a second resume takes the record twice (mode 9).

func newPauseExecutor(t *testing.T) (*Executor, string) {
	t.Helper()
	state := t.TempDir()
	return &Executor{
		stateDir: state, capsules: map[string]*Capsule{}, capabilities: map[capKey]*Capability{},
		controlHandles: map[capKey]string{}, revokedCaps: map[string]bool{},
	}, state
}

// pausableCapsule registers a live capsule with a real upper directory.
func pausableCapsule(t *testing.T, e *Executor, id string) (*Capsule, *testCapsuleCgroup) {
	t.Helper()
	base := filepath.Join(e.stateDir, id)
	upper := filepath.Join(base, "upper")
	if err := os.MkdirAll(filepath.Join(upper, "workspace", "platform"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upper, "workspace", "platform", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cg := &testCapsuleCgroup{}
	caps := &Capsule{ID: id, State: StateActive, UpperDir: upper, MergedDir: filepath.Join(base, "root"), Cgroup: cg, OwnerRunID: "run:" + id,
		Spec: SpawnSpec{CapsuleID: id, OwnerRunID: "run:" + id, InputsDir: "/tmp/staged", CarriedPatchDir: "/tmp/carried"}}
	e.mu.Lock()
	e.capsules[id] = caps
	e.mu.Unlock()
	return caps, cg
}

func pauseReq(id string) PauseRequest {
	return PauseRequest{CapsuleID: id, AssignmentID: "assignment-" + id, Attempt: 1, SubjectDigest: "sha256:subject", ReleaseCommit: "old-release",
		DrainTimeout: 200 * time.Millisecond, TTL: time.Minute}
}

func TestPauseCapsuleCommitsRecordAfterFreezeAndRefusesOps(t *testing.T) {
	e, state := newPauseExecutor(t)
	caps, cg := pausableCapsule(t, e, "capsule-a")
	rec, err := e.PauseCapsule(context.Background(), pauseReq("capsule-a"))
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !cg.frozen || caps.State != StateFrozen || !e.IsPaused("capsule-a") {
		t.Fatalf("not frozen and paused: frozen=%v state=%s", cg.frozen, caps.State)
	}
	if rec.UpperFiles != 1 || rec.UpperBytes != int64(len("package a\n")) || !strings.HasPrefix(rec.UpperDigest, "sha256:") {
		t.Fatalf("digest and counts = %+v", rec)
	}
	if rec.Spec.InputsDir != "" || rec.Spec.CarriedPatchDir != "" {
		t.Fatalf("transient staging directories were recorded: %+v", rec.Spec)
	}
	onDisk, err := readPauseRecord(filepath.Join(state, "capsule-a"))
	if err != nil || onDisk.UpperDigest != rec.UpperDigest || onDisk.ReleaseCommit != "old-release" {
		t.Fatalf("record on disk = %+v, %v", onDisk, err)
	}
	// Admission stays closed with the pause's own message.
	if err := caps.acquireOp(); err == nil || !strings.Contains(err.Error(), PauseBlockMessage) {
		t.Fatalf("an op was admitted or refused with another message: %v", err)
	}
	// A repeat pause is idempotent and returns the record.
	again, err := e.PauseCapsule(context.Background(), pauseReq("capsule-a"))
	if err != nil || again.UpperDigest != rec.UpperDigest {
		t.Fatalf("repeat pause = %+v, %v", again, err)
	}
}

// The drain waits for the in-flight op and times out without a record: the
// capsule is ordinary again (failure 1 and the unwind).
func TestPauseCapsuleDrainTimeoutUnwinds(t *testing.T) {
	e, state := newPauseExecutor(t)
	caps, cg := pausableCapsule(t, e, "capsule-drain")
	caps.inflightMu.Lock()
	caps.inflightOps = 1
	caps.inflightMu.Unlock()
	_, err := e.PauseCapsule(context.Background(), pauseReq("capsule-drain"))
	if err == nil {
		t.Fatal("a pause committed with an op in flight")
	}
	if cg.frozen || caps.State != StateActive || caps.pauseBlock.Load() || e.IsPaused("capsule-drain") {
		t.Fatalf("not unwound: frozen=%v state=%s block=%v", cg.frozen, caps.State, caps.pauseBlock.Load())
	}
	if _, err := os.Stat(filepath.Join(state, "capsule-drain", PauseRecordName)); !os.IsNotExist(err) {
		t.Fatalf("a record exists after an abandoned pause: %v", err)
	}
	if err := caps.acquireOp(); err != nil {
		t.Fatalf("ops stay blocked after the abandon: %v", err)
	}
	caps.releaseOp()
}

func TestPauseCapsuleSingleFlight(t *testing.T) {
	e, _ := newPauseExecutor(t)
	caps, _ := pausableCapsule(t, e, "capsule-sf")
	caps.pauseBlock.Store(true) // a pause is in progress
	if _, err := e.PauseCapsule(context.Background(), pauseReq("capsule-sf")); !errors.Is(err, ErrPauseInProgress) {
		t.Fatalf("second pause = %v, want ErrPauseInProgress", err)
	}
}

// Failure 13: a claude token in the upper abandons the pause and thaws.
func TestPauseCapsuleClaudeTokenInUpperAbandons(t *testing.T) {
	e, state := newPauseExecutor(t)
	caps, cg := pausableCapsule(t, e, "capsule-tok")
	caps.claudeToken = []byte("relay-secret-0123456789abcdef")
	caps.MergedDir = ""
	if err := os.WriteFile(filepath.Join(caps.UpperDir, "workspace", "platform", "leak.txt"), []byte("x relay-secret-0123456789abcdef x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := e.PauseCapsule(context.Background(), pauseReq("capsule-tok"))
	if !errors.Is(err, ErrPauseRefused) {
		t.Fatalf("pause with a token in the upper = %v, want ErrPauseRefused", err)
	}
	if cg.frozen || caps.State != StateActive || caps.pauseBlock.Load() {
		t.Fatalf("not thawed after the abandon: frozen=%v state=%s", cg.frozen, caps.State)
	}
	if _, err := os.Stat(filepath.Join(state, "capsule-tok", PauseRecordName)); !os.IsNotExist(err) {
		t.Fatalf("record after an abandoned pause: %v", err)
	}
}

func TestPauseCapsuleRefusesVerifierSlot(t *testing.T) {
	e, _ := newPauseExecutor(t)
	caps, _ := pausableCapsule(t, e, "capsule-ver")
	caps.Spec.VerifierBundleDir = "/bundle"
	if _, err := e.PauseCapsule(context.Background(), pauseReq("capsule-ver")); !errors.Is(err, ErrPauseRefused) {
		t.Fatalf("verifier-slot pause = %v, want ErrPauseRefused", err)
	}
}

// Failure 3/4: an unpause thaws, removes the record and lifts the block.
func TestUnpauseThawsRemovesRecordAndLiftsBlock(t *testing.T) {
	e, state := newPauseExecutor(t)
	caps, cg := pausableCapsule(t, e, "capsule-un")
	if _, err := e.PauseCapsule(context.Background(), pauseReq("capsule-un")); err != nil {
		t.Fatal(err)
	}
	if err := e.UnpauseCapsule(context.Background(), "capsule-un"); err != nil {
		t.Fatalf("unpause: %v", err)
	}
	if cg.frozen || caps.State != StateActive || e.IsPaused("capsule-un") || caps.pauseBlock.Load() {
		t.Fatalf("not live after unpause: frozen=%v state=%s", cg.frozen, caps.State)
	}
	if _, err := os.Stat(filepath.Join(state, "capsule-un", PauseRecordName)); !os.IsNotExist(err) {
		t.Fatalf("record survives the unpause: %v", err)
	}
	if err := e.UnpauseCapsule(context.Background(), "capsule-un"); err != nil {
		t.Fatalf("unpause is not idempotent: %v", err)
	}
	if err := caps.acquireOp(); err != nil {
		t.Fatalf("op refused after the unpause: %v", err)
	}
	caps.releaseOp()
}

// Failure 21: the ack and the pre-ack expiry. An expired record cannot be
// acked; an acked record stays acked.
func TestAckPauseRefusesExpiredAndPersistsAck(t *testing.T) {
	e, state := newPauseExecutor(t)
	pausableCapsule(t, e, "capsule-ack")
	rec, err := e.PauseCapsule(context.Background(), pauseReq("capsule-ack"))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AckPause("capsule-ack", rec.ExpiresAt.Add(time.Second)); err == nil {
		t.Fatal("an expired record was acked")
	}
	if err := e.AckPause("capsule-ack", rec.PausedAt.Add(time.Second)); err != nil {
		t.Fatalf("ack: %v", err)
	}
	onDisk, err := readPauseRecord(filepath.Join(state, "capsule-ack"))
	if err != nil || !onDisk.Acked {
		t.Fatalf("ack not on disk: %+v, %v", onDisk, err)
	}
	if err := e.AckPause("capsule-ack", rec.ExpiresAt.Add(time.Hour)); err != nil {
		t.Fatalf("an acked record must not expire: %v", err)
	}
}

// Failure 26 (and the r3 paused flag): thaw, extract and inspect refuse.
func TestPausedCapsuleRefusesThawExtractAndInspect(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := newPauseExecutor(t)
	e.publicKey = publicKey
	pausableCapsule(t, e, "capsule-ref")
	capability := &Capability{
		CapabilityID: "cap-ref", Handle: "grant-ref", CapsuleID: "capsule-ref", TargetCapsule: "capsule-ref",
		AgentRunID: "run-ref", AgentRole: RoleEngineering, Verbs: RoleVerbSets()[RoleEngineering], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	e.capabilities[capKey{AgentRunID: "run-ref", Handle: "grant-ref"}] = capability
	if _, err := e.PauseCapsule(context.Background(), pauseReq("capsule-ref")); err != nil {
		t.Fatal(err)
	}
	if err := e.ThawGranted(context.Background(), "run-ref", "grant-ref"); !errors.Is(err, ErrCapsulePaused) {
		t.Fatalf("thaw of a paused capsule = %v", err)
	}
	if _, err := e.ExtractGranted(context.Background(), "run-ref", "grant-ref"); !errors.Is(err, ErrCapsulePaused) {
		t.Fatalf("extract of a paused capsule = %v", err)
	}
	if _, err := e.InspectGrantedWorktree(context.Background(), "run-ref", "grant-ref"); !errors.Is(err, ErrCapsulePaused) {
		t.Fatalf("inspect of a paused capsule = %v", err)
	}
	// The block holds during the pause too, before the record.
	e.capsules["capsule-ref"].paused.Store(false)
	e.capsules["capsule-ref"].pauseBlock.Store(true)
	if err := e.ThawGranted(context.Background(), "run-ref", "grant-ref"); !errors.Is(err, ErrCapsulePaused) {
		t.Fatalf("thaw during the pause = %v", err)
	}
}

func TestLookupPauseFailsClosedOnAnUnreadableRecord(t *testing.T) {
	e, state := newPauseExecutor(t)
	pausableCapsule(t, e, "capsule-bad")
	if got := e.LookupPause("capsule-bad"); got.Where != PauseNone || got.Err != nil {
		t.Fatalf("no record = %+v", got)
	}
	if err := os.WriteFile(filepath.Join(state, "capsule-bad", PauseRecordName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := e.LookupPause("capsule-bad"); got.Err == nil || got.Where != PauseNone {
		t.Fatalf("an unreadable record = %+v, want Err and no Where", got)
	}
	if got := e.LookupPause("../escape"); got.Err == nil {
		t.Fatal("a path-like capsule id was looked up")
	}
}

// ---- boot spare (failures 2, 20, 25)

func writeSparable(t *testing.T, root, boot, id string, rec PauseRecord) {
	t.Helper()
	base := filepath.Join(root, boot, id)
	if err := os.MkdirAll(filepath.Join(base, "upper", "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "upper", "workspace", "f"), []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePauseRecord(base, rec); err != nil {
		t.Fatal(err)
	}
}

func goodRecord(id string) PauseRecord {
	return PauseRecord{Version: PauseRecordVersion, AssignmentID: "assignment-" + id, Attempt: 1, CapsuleID: id, SubjectDigest: "sha256:subject", Acked: true,
		ExpiresAt: time.Now().Add(time.Hour)}
}

func TestBootSparesOnlyAckedValidRecordsWhenAsked(t *testing.T) {
	root := t.TempDir()
	writeSparable(t, root, "boot-old", "capsule-ok", goodRecord("capsule-ok"))
	unacked := goodRecord("capsule-unacked")
	unacked.Acked = false
	writeSparable(t, root, "boot-old", "capsule-unacked", unacked)
	mismatched := goodRecord("capsule-other")
	writeSparable(t, root, "boot-old", "capsule-mismatch", mismatched)
	if err := os.MkdirAll(filepath.Join(root, "boot-old", "capsule-plain", "upper"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := bootScopedStateDir(root, "now", func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root, "boot-now") {
		t.Fatalf("dir = %s", got)
	}
	if _, err := os.Stat(filepath.Join(got, PausedSpareDir, "capsule-ok", "upper", "workspace", "f")); err != nil {
		t.Fatalf("an acked valid record was not spared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got, PausedSpareDir, "capsule-ok", PauseRecordName)); err != nil {
		t.Fatalf("the record did not move with the directory: %v", err)
	}
	for _, id := range []string{"capsule-unacked", "capsule-mismatch", "capsule-plain"} {
		if _, err := os.Stat(filepath.Join(got, PausedSpareDir, id)); !os.IsNotExist(err) {
			t.Fatalf("%s was spared: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "boot-old")); !os.IsNotExist(err) {
		t.Fatalf("the earlier boot survived: %v", err)
	}
}

// Failure 2 and 20: without a reason-aware yes (a crash, a restore marker)
// everything is swept as before.
func TestBootSparesNothingWithoutAPlatformUpdateMarker(t *testing.T) {
	for name, spare := range map[string]func() bool{"crash (no check)": nil, "restore or stale marker": func() bool { return false }} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeSparable(t, root, "boot-old", "capsule-ok", goodRecord("capsule-ok"))
			got, err := bootScopedStateDir(root, "now", spare)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(got, PausedSpareDir)); !os.IsNotExist(err) {
				t.Fatalf("a paused directory was spared: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "boot-old")); !os.IsNotExist(err) {
				t.Fatalf("the earlier boot survived: %v", err)
			}
		})
	}
}

// ---- seeded resume (failures 6, 7, 9, 12, 17, 18, 25)

// sparedExecutor models a planned boot: the old directory was spared into the
// new boot's .paused area.
func sparedExecutor(t *testing.T, id string) (*Executor, PauseRecord, string) {
	t.Helper()
	root := t.TempDir()
	rec := goodRecord(id)
	writeSparable(t, root, "boot-old", id, rec)
	// The record must describe the upper as it is.
	digest, files, bytes, err := digestUpper(context.Background(), filepath.Join(root, "boot-old", id, "upper"))
	if err != nil {
		t.Fatal(err)
	}
	rec.UpperDigest, rec.UpperFiles, rec.UpperBytes = digest, files, bytes
	if err := writePauseRecord(filepath.Join(root, "boot-old", id), rec); err != nil {
		t.Fatal(err)
	}
	state, err := bootScopedStateDir(root, "now", func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	e := &Executor{stateDir: state, capsules: map[string]*Capsule{}, capabilities: map[capKey]*Capability{}, controlHandles: map[capKey]string{}, revokedCaps: map[string]bool{}}
	return e, rec, state
}

func wantFor(rec PauseRecord) PauseRecord {
	return PauseRecord{CapsuleID: rec.CapsuleID, AssignmentID: rec.AssignmentID, Attempt: rec.Attempt, SubjectDigest: rec.SubjectDigest}
}

func TestPrepareResumeValidatesTakesTheRecordAndSeeds(t *testing.T) {
	e, rec, state := sparedExecutor(t, "capsule-r")
	if look := e.LookupPause("capsule-r"); look.Where != PauseSpared {
		t.Fatalf("lookup = %+v, want spared", look)
	}
	seed, err := e.PrepareResume(context.Background(), wantFor(rec))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if seed.UpperDir != filepath.Join(state, PausedSpareDir, "capsule-r", "upper") {
		t.Fatalf("seed upper = %s", seed.UpperDir)
	}
	// The record is taken (renamed) before the spawn: a second resume cannot
	// take it again (failure 9).
	if _, err := os.Stat(filepath.Join(seed.Dir, PauseRecordName)); !os.IsNotExist(err) {
		t.Fatalf("the record was not taken: %v", err)
	}
	if _, err := e.PrepareResume(context.Background(), wantFor(rec)); err == nil {
		t.Fatal("a second resume took the same record")
	}
	// The seed is the only thing Spawn takes: it renames the upper into the
	// new capsule's upper (same filesystem), and nothing else.
	base := filepath.Join(state, "capsule-r")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := SpawnSpec{CapsuleID: "capsule-r", SeedUpper: seed.UpperDir}
	if err := seedCapsuleUpper(state, spec, filepath.Join(base, "upper")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(base, "upper", "workspace", "f")); err != nil || string(b) != "work" {
		t.Fatalf("the seeded upper lost its work: %q, %v", b, err)
	}
	if err := e.CompleteResume(seed); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := os.Stat(seed.Dir); !os.IsNotExist(err) {
		t.Fatalf("the old directory survives the resume: %v", err)
	}
	if look := e.LookupPause("capsule-r"); look.Where != PauseConsumed {
		t.Fatalf("after the resume lookup = %+v, want consumed (failure 9)", look)
	}
}

func TestSeedUpperOnlyFromTheSpareAreaOfTheSameCapsule(t *testing.T) {
	state := t.TempDir()
	for name, seed := range map[string]string{
		"outside the state":       filepath.Join(t.TempDir(), "upper"),
		"another capsule's upper": filepath.Join(state, PausedSpareDir, "capsule-other", "upper"),
		"not the upper":           filepath.Join(state, PausedSpareDir, "capsule-x", "work"),
		"a live capsule's upper":  filepath.Join(state, "capsule-x", "upper"),
	} {
		if err := seedCapsuleUpper(state, SpawnSpec{CapsuleID: "capsule-x", SeedUpper: seed}, filepath.Join(state, "dest")); err == nil {
			t.Errorf("%s: the seed was accepted", name)
		}
	}
}

// Failure 7: a mismatched binding or a changed upper closes (the error).
func TestPrepareResumeRefusesMismatchedRecordOrTamperedUpper(t *testing.T) {
	for name, mutate := range map[string]func(t *testing.T, e *Executor, rec *PauseRecord, state string){
		"assignment id": func(t *testing.T, e *Executor, rec *PauseRecord, state string) { rec.AssignmentID = "assignment-wrong" },
		"attempt":       func(t *testing.T, e *Executor, rec *PauseRecord, state string) { rec.Attempt = 2 },
		"subject":       func(t *testing.T, e *Executor, rec *PauseRecord, state string) { rec.SubjectDigest = "sha256:other" },
		"upper content": func(t *testing.T, e *Executor, rec *PauseRecord, state string) {
			if err := os.WriteFile(filepath.Join(state, PausedSpareDir, rec.CapsuleID, "upper", "workspace", "f"), []byte("tampered"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"upper count": func(t *testing.T, e *Executor, rec *PauseRecord, state string) {
			if err := os.WriteFile(filepath.Join(state, PausedSpareDir, rec.CapsuleID, "upper", "workspace", "extra"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			e, rec, state := sparedExecutor(t, "capsule-m")
			want := wantFor(rec)
			mutate(t, e, &want, state)
			if _, err := e.PrepareResume(context.Background(), want); err == nil {
				t.Fatal("the resume went ahead over a mismatch")
			}
			// The failure leaves the directory for DiscardPause (failure 25).
			if err := e.DiscardPause(context.Background(), "capsule-m"); err != nil {
				t.Fatalf("discard: %v", err)
			}
			if look := e.LookupPause("capsule-m"); look.Where != PauseNone {
				t.Fatalf("after the discard lookup = %+v", look)
			}
			if _, err := os.Stat(filepath.Join(state, PausedSpareDir, "capsule-m")); !os.IsNotExist(err) {
				t.Fatalf("the spared directory survives the discard: %v", err)
			}
		})
	}
}

func TestPrepareResumeRefusesAnUnackedRecord(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "boot-now")
	rec := goodRecord("capsule-un")
	rec.Acked = false
	writeSparable(t, root, "boot-now/"+PausedSpareDir, "capsule-un", rec)
	e := &Executor{stateDir: state, capsules: map[string]*Capsule{}}
	if _, err := e.PrepareResume(context.Background(), wantFor(rec)); err == nil {
		t.Fatal("an unacknowledged pause was resumed")
	}
}

// Failures 12 and 17: a process-only restart keeps the boot id, so the old
// directory is still in the current boot dir. The resume moves it into the
// spare area (after tearing down residue) before the seeded spawn.
func TestPrepareResumeMovesASameBootDirectoryAside(t *testing.T) {
	state := t.TempDir()
	rec := goodRecord("capsule-same")
	writeSparable(t, state, ".", "capsule-same", rec)
	digest, files, bytes, err := digestUpper(context.Background(), filepath.Join(state, "capsule-same", "upper"))
	if err != nil {
		t.Fatal(err)
	}
	rec.UpperDigest, rec.UpperFiles, rec.UpperBytes = digest, files, bytes
	if err := writePauseRecord(filepath.Join(state, "capsule-same"), rec); err != nil {
		t.Fatal(err)
	}
	e := &Executor{stateDir: state, capsules: map[string]*Capsule{}}
	if look := e.LookupPause("capsule-same"); look.Where != PauseResidual {
		t.Fatalf("lookup = %+v, want residual (no capsule in this process)", look)
	}
	seed, err := e.PrepareResume(context.Background(), wantFor(rec))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "capsule-same")); !os.IsNotExist(err) {
		t.Fatalf("the live-path directory is still there, so Spawn would refuse it as quarantined: %v", err)
	}
	if _, err := os.Stat(filepath.Join(seed.UpperDir, "workspace", "f")); err != nil {
		t.Fatalf("the upper did not move with the directory: %v", err)
	}
}

// A live capsule of this process is never the object of a resume.
func TestPrepareResumeRefusesALiveCapsule(t *testing.T) {
	e, state := newPauseExecutor(t)
	pausableCapsule(t, e, "capsule-live")
	if _, err := e.PrepareResume(context.Background(), PauseRecord{CapsuleID: "capsule-live", AssignmentID: "a"}); err == nil {
		t.Fatal("resume of a live capsule")
	}
	if _, err := os.Stat(filepath.Join(state, "capsule-live")); err != nil {
		t.Fatalf("the live capsule's directory was touched: %v", err)
	}
}

// Failure 25: after the reconcile every spared directory that was not resumed
// is deleted.
func TestDeleteSparedPausesKeepsOnlyTheResumed(t *testing.T) {
	root := t.TempDir()
	writeSparable(t, root, "boot-old", "capsule-keep", goodRecord("capsule-keep"))
	writeSparable(t, root, "boot-old", "capsule-drop", goodRecord("capsule-drop"))
	state, err := bootScopedStateDir(root, "now", func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	e := &Executor{stateDir: state, capsules: map[string]*Capsule{}}
	if got := e.SparedPausedIDs(); len(got) != 2 {
		t.Fatalf("spared = %v", got)
	}
	if err := e.DeleteSparedPauses(map[string]bool{"capsule-keep": true}); err != nil {
		t.Fatal(err)
	}
	if got := e.SparedPausedIDs(); len(got) != 1 || got[0] != "capsule-keep" {
		t.Fatalf("spared after the sweep = %v", got)
	}
}

// The digest proves the work tree and changes with any file.
func TestDigestUpperChangesWithContentAndIgnoresMtime(t *testing.T) {
	upper := t.TempDir()
	if err := os.WriteFile(filepath.Join(upper, "f"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	d1, files, bytes, err := digestUpper(context.Background(), upper)
	if err != nil || files != 1 || bytes != 3 {
		t.Fatalf("digest = %q files=%d bytes=%d err=%v", d1, files, bytes, err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(upper, "f"), later, later); err != nil {
		t.Fatal(err)
	}
	if d, _, _, _ := digestUpper(context.Background(), upper); d != d1 {
		t.Fatal("the digest depends on mtime, which a rename preserves but a copy does not")
	}
	if err := os.WriteFile(filepath.Join(upper, "f"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d, _, _, _ := digestUpper(context.Background(), upper); d == d1 {
		t.Fatal("the digest ignores content")
	}
}

// Failure 8: a resume that crashes the computer leaves the record taken
// (pause.consumed.json); the next boot is a crash boot, and even a boot that
// asks to spare finds no pause.json to spare. Failure 14: an older binary has
// no spare check at all, so it sweeps the directory and ignores the record.
func TestBootNeverSparesATakenRecord(t *testing.T) {
	for name, spare := range map[string]func() bool{"spare asked": func() bool { return true }, "older binary (no check)": nil} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "boot-old", "capsule-taken")
			if err := os.MkdirAll(filepath.Join(base, "upper"), 0o755); err != nil {
				t.Fatal(err)
			}
			rec := goodRecord("capsule-taken")
			raw, _ := json.Marshal(rec)
			if err := os.WriteFile(filepath.Join(base, PauseConsumedName), raw, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := bootScopedStateDir(root, "now", spare)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(got, PausedSpareDir)); !os.IsNotExist(err) {
				t.Fatalf("a taken record was spared: %v", err)
			}
		})
	}
}
