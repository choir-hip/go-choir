package main

// The checkpoint verdict pass, written before the code. Ways it could fail:
//
//  1. a base the host already holds at a checkpoint's head is rebuilt anyway
//     (a 45 to 60 s host job per checkpoint for nothing);
//  2. a mismatch is judged without the checkpoint's deposit mode (residual
//     R1): a retained live store is called not_derivable;
//  3. a judged base is not pinned under the checkpoint, so a restore to the
//     checkpoint has no zero-tail start;
//  4. a pin failure still records the verdict, or a verdict failure for one
//     computer stops the rest;
//  5. a backlog of pending checkpoints for one computer enqueues one job per
//     checkpoint at once (the job row is one per computer: the rest churn),
//     or enqueues the newest first so the older ones never get a base;
//  6. a job the host refused (blocked) or keeps failing leaves the checkpoint
//     pending forever, so a restore and the listing never learn why;
//  7. a base at the right sequence but another head is judged against.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

type fakeVerdictStore struct {
	pending     []platform.CheckpointVerdictRecord
	checkpoints map[string]selfdevprotocol.Checkpoint // by digest
	bases       map[string]string                     // computer/sequence -> ref
	jobs        map[string]platform.ProjectionJob     // by computer
	enqueueErr  map[string]error                      // by computer
	pinErr      error

	enqueued []platform.CheckpointVerdictRecord
	resolved map[string]selfdevprotocol.CheckpointVerdict // by digest
	pinned   []string
}

func newFakeVerdictStore() *fakeVerdictStore {
	return &fakeVerdictStore{
		checkpoints: map[string]selfdevprotocol.Checkpoint{}, bases: map[string]string{}, jobs: map[string]platform.ProjectionJob{},
		enqueueErr: map[string]error{}, resolved: map[string]selfdevprotocol.CheckpointVerdict{},
	}
}

func (f *fakeVerdictStore) PendingCheckpointVerdicts(context.Context) ([]platform.CheckpointVerdictRecord, error) {
	var out []platform.CheckpointVerdictRecord
	for _, r := range f.pending {
		if _, done := f.resolved[r.CheckpointDigest]; !done {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeVerdictStore) ResolveCheckpointVerdict(_ context.Context, computerID, digest string, v selfdevprotocol.CheckpointVerdict) (bool, error) {
	if _, done := f.resolved[digest]; done {
		return false, nil
	}
	f.resolved[digest] = v
	return true, nil
}

func (f *fakeVerdictStore) StoredCheckpoint(_ context.Context, computerID, digest string) (selfdevprotocol.Checkpoint, error) {
	c, ok := f.checkpoints[digest]
	if !ok {
		return selfdevprotocol.Checkpoint{}, errors.New("no such checkpoint")
	}
	return c, nil
}

func (f *fakeVerdictStore) ProjectionBaseAtSequence(_ context.Context, computerID string, sequence uint64) (string, bool, error) {
	ref, ok := f.bases[fmt.Sprintf("%s/%d", computerID, sequence)]
	return ref, ok, nil
}

func (f *fakeVerdictStore) EnqueueCheckpointVerificationJob(_ context.Context, r platform.CheckpointVerdictRecord) (platform.ProjectionJob, error) {
	if err := f.enqueueErr[r.ComputerID]; err != nil {
		return platform.ProjectionJob{}, err
	}
	f.enqueued = append(f.enqueued, r)
	return platform.ProjectionJob{ComputerID: r.ComputerID, Status: "queued", TargetSequence: r.Verdict.AcceptedSequence, TargetHead: r.AcceptedHead}, nil
}

func (f *fakeVerdictStore) ProjectionJob(_ context.Context, computerID string) (platform.ProjectionJob, error) {
	j, ok := f.jobs[computerID]
	if !ok {
		return platform.ProjectionJob{}, errNoJob
	}
	return j, nil
}

func (f *fakeVerdictStore) PinProjectionBase(_ context.Context, computerID, baseRef, reference string) error {
	if f.pinErr != nil {
		return f.pinErr
	}
	f.pinned = append(f.pinned, baseRef+" "+reference)
	return nil
}

var errNoJob = sql.ErrNoRows

func hex64(c byte) string { return strings.Repeat(string(c), 64) }

func witnessOf(tables map[string]string) selfdevprotocol.VMLocalContentWitness {
	schema := map[string]string{}
	for table := range tables {
		schema[table] = hex64('5')
	}
	w := selfdevprotocol.VMLocalContentWitness{Database: "texture", ContentRoot: hex64('7'), DoltHead: "h", Schema: schema, Tables: tables}
	sealed, err := selfdevprotocol.SealWitness(w)
	if err != nil {
		panic(err)
	}
	return sealed
}

// scenario adds one pending checkpoint to the fake: its witness, deposit mode
// and the sequence and head it is bound at.
func (f *fakeVerdictStore) addPending(computerID, digest string, sequence uint64, head string, witness selfdevprotocol.VMLocalContentWitness, mode string) platform.CheckpointVerdictRecord {
	rec := platform.CheckpointVerdictRecord{ComputerID: computerID, CheckpointDigest: digest, AcceptedHead: head, Verdict: selfdevprotocol.CheckpointVerdict{Status: selfdevprotocol.CheckpointVerdictPending, AcceptedSequence: sequence}}
	f.pending = append(f.pending, rec)
	f.checkpoints[digest] = selfdevprotocol.Checkpoint{Digest: digest, Request: selfdevprotocol.CheckpointRequest{ComputerID: computerID, AcceptedEventHead: head, VMLocalContentWitness: witness, DepositMode: mode}}
	return rec
}

func descriptorOf(computerID string, sequence uint64, head, blob string, witness selfdevprotocol.VMLocalContentWitness) projectionbase.Descriptor {
	return projectionbase.Descriptor{ComputerID: computerID, Sequence: sequence, CanonicalHead: head, BlobSHA256: blob, VMLocalContentWitness: witness}
}

func loader(ds ...projectionbase.Descriptor) func(string, string) (projectionbase.Descriptor, error) {
	return func(computerID, ref string) (projectionbase.Descriptor, error) {
		for _, d := range ds {
			if d.ComputerID == computerID && d.BlobSHA256 == ref {
				return d, nil
			}
		}
		return projectionbase.Descriptor{}, errors.New("no such base")
	}
}

var tablesA = map[string]string{"agents": hex64('a'), "og_objects": hex64('b')}
var tablesB = map[string]string{"agents": hex64('a'), "og_objects": hex64('c')}

// Failure modes 1 and 3.
func TestReconcileJudgesFromABaseTheHostAlreadyHoldsAndPinsIt(t *testing.T) {
	store := newFakeVerdictStore()
	head, blob := hex64('1'), hex64('9')
	store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.bases["c1/100"] = blob
	d := descriptorOf("c1", 100, head, blob, witnessOf(tablesA))

	if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
		t.Fatal(err)
	}
	if len(store.enqueued) != 0 {
		t.Fatalf("a rebuild was requested for a head the host already holds: %+v", store.enqueued)
	}
	v := store.resolved[hex64('d')]
	if v.Status != selfdevprotocol.CheckpointVerdictVerified || v.BaseRef != blob {
		t.Fatalf("verdict = %+v, want verified against %s", v, blob)
	}
	if want := blob + " checkpoint:" + head; len(store.pinned) != 1 || store.pinned[0] != want {
		t.Fatalf("pins = %v, want %q", store.pinned, want)
	}
}

// Failure mode 2: R1 in the worker.
func TestReconcileNamesTheDepositModeCaseAsUnverifiedNotNotDerivable(t *testing.T) {
	for _, tc := range []struct {
		mode       string
		wantStatus string
		wantReason string
	}{
		{selfdevprotocol.DepositModeUpcast, selfdevprotocol.CheckpointVerdictNotDerivable, selfdevprotocol.VerdictReasonWitnessMismatch},
		{selfdevprotocol.DepositModeRetained, selfdevprotocol.CheckpointVerdictUnverified, selfdevprotocol.VerdictReasonBaseUpcast},
		{"", selfdevprotocol.CheckpointVerdictUnverified, selfdevprotocol.VerdictReasonBaseUpcast},
	} {
		store := newFakeVerdictStore()
		head, blob := hex64('1'), hex64('9')
		store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), tc.mode)
		store.bases["c1/100"] = blob
		d := descriptorOf("c1", 100, head, blob, witnessOf(tablesB))
		if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
			t.Fatal(err)
		}
		v := store.resolved[hex64('d')]
		if v.Status != tc.wantStatus || v.Reason != tc.wantReason || strings.Join(v.DifferingTables, ",") != "og_objects" {
			t.Fatalf("mode %q: verdict = %+v, want %s/%s naming og_objects", tc.mode, v, tc.wantStatus, tc.wantReason)
		}
	}
}

// Failure mode 7.
func TestReconcileDoesNotJudgeAgainstABaseAtAnotherHead(t *testing.T) {
	store := newFakeVerdictStore()
	store.addPending("c1", hex64('d'), 100, hex64('1'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.bases["c1/100"] = hex64('9')
	d := descriptorOf("c1", 100, hex64('2'), hex64('9'), witnessOf(tablesB))
	if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
		t.Fatal(err)
	}
	v := store.resolved[hex64('d')]
	if v.Status != selfdevprotocol.CheckpointVerdictUnverified || v.Reason != selfdevprotocol.VerdictReasonBaseHeadMismatch {
		t.Fatalf("verdict = %+v, want unverified / base_head_mismatch", v)
	}
}

// Failure mode 5.
func TestReconcileRequestsOneRebuildPerComputerOldestFirst(t *testing.T) {
	store := newFakeVerdictStore()
	store.addPending("c1", hex64('3'), 300, hex64('c'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.addPending("c1", hex64('1'), 100, hex64('a'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.addPending("c1", hex64('2'), 200, hex64('b'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.addPending("c2", hex64('4'), 50, hex64('e'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	if err := reconcileCheckpointVerdicts(t.Context(), store, loader()); err != nil {
		t.Fatal(err)
	}
	got := map[string]uint64{}
	for _, r := range store.enqueued {
		if _, dup := got[r.ComputerID]; dup {
			t.Fatalf("two rebuilds requested for %s in one pass", r.ComputerID)
		}
		got[r.ComputerID] = r.Verdict.AcceptedSequence
	}
	if got["c1"] != 100 || got["c2"] != 50 || len(got) != 2 {
		t.Fatalf("rebuilds requested %v, want c1 at its oldest (100) and c2 at 50", got)
	}
}

// Failure mode 4: one computer's refusal does not stop the others.
func TestReconcileKeepsGoingPastOneComputersError(t *testing.T) {
	store := newFakeVerdictStore()
	store.addPending("bad", hex64('1'), 100, hex64('a'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.addPending("good", hex64('2'), 100, hex64('b'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.enqueueErr["bad"] = errors.New("store unavailable")
	err := reconcileCheckpointVerdicts(t.Context(), store, loader())
	if err == nil {
		t.Fatal("the failure was swallowed")
	}
	if len(store.enqueued) != 1 || store.enqueued[0].ComputerID != "good" {
		t.Fatalf("the healthy computer's rebuild was not requested: %+v", store.enqueued)
	}
	if _, judged := store.resolved[hex64('1')]; judged {
		t.Fatal("a checkpoint whose rebuild could not be requested was judged")
	}
}

// A refusal by name settles the checkpoint instead of leaving it pending.
func TestReconcileSettlesACheckpointWithNoBaseInReach(t *testing.T) {
	store := newFakeVerdictStore()
	store.addPending("c1", hex64('1'), 20_000, hex64('a'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.enqueueErr["c1"] = fmt.Errorf("c1: %w", platform.ErrNoVerificationBase)
	if err := reconcileCheckpointVerdicts(t.Context(), store, loader()); err != nil {
		t.Fatal(err)
	}
	v := store.resolved[hex64('1')]
	if v.Status != selfdevprotocol.CheckpointVerdictUnverified || v.Reason != selfdevprotocol.VerdictReasonNoBase {
		t.Fatalf("verdict = %+v, want unverified / no_base", v)
	}
}

// Failure mode 6.
func TestReconcileSettlesACheckpointWhoseRebuildJobIsRefused(t *testing.T) {
	head := hex64('a')
	for _, tc := range []struct {
		name    string
		job     platform.ProjectionJob
		settles bool
	}{
		{"blocked at the checkpoint's head", platform.ProjectionJob{Status: "blocked", TargetSequence: 100, TargetHead: head, Error: "projection base refused: seed"}, true},
		{"failed three times at the head", platform.ProjectionJob{Status: "failed", Failures: 3, TargetSequence: 100, TargetHead: head, Error: "boom"}, true},
		{"failed once", platform.ProjectionJob{Status: "failed", Failures: 1, TargetSequence: 100, TargetHead: head, Error: "boom"}, false},
		{"blocked at another head", platform.ProjectionJob{Status: "blocked", TargetSequence: 999, TargetHead: hex64('f'), Error: "x"}, false},
		{"running at the head", platform.ProjectionJob{Status: "running", TargetSequence: 100, TargetHead: head}, false},
	} {
		store := newFakeVerdictStore()
		store.addPending("c1", hex64('1'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
		store.jobs["c1"] = tc.job
		if err := reconcileCheckpointVerdicts(t.Context(), store, loader()); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		v, settled := store.resolved[hex64('1')]
		if settled != tc.settles {
			t.Fatalf("%s: settled=%v (%+v), want %v", tc.name, settled, v, tc.settles)
		}
		if settled && (v.Status != selfdevprotocol.CheckpointVerdictUnverified || v.Reason != selfdevprotocol.VerdictReasonJobBlocked || !strings.Contains(v.Detail, tc.job.Error)) {
			t.Fatalf("%s: verdict = %+v, want unverified / job_blocked carrying the job's error", tc.name, v)
		}
	}
}

// Failure mode 3 and 4: after a job publishes a base, the verdicts at exactly
// that head are judged and pinned; others stay pending; a failed pin keeps
// the verdict pending for the next pass.
func TestResolveVerdictsAtBaseJudgesOnlyTheMatchingHead(t *testing.T) {
	store := newFakeVerdictStore()
	head, blob := hex64('1'), hex64('9')
	store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.addPending("c1", hex64('e'), 200, hex64('2'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.addPending("c2", hex64('f'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	d := descriptorOf("c1", 100, head, blob, witnessOf(tablesA))
	if err := resolveVerdictsAtBase(t.Context(), store, d); err != nil {
		t.Fatal(err)
	}
	if len(store.resolved) != 1 || store.resolved[hex64('d')].Status != selfdevprotocol.CheckpointVerdictVerified {
		t.Fatalf("resolved = %+v, want only c1's checkpoint at 100", store.resolved)
	}

	failing := newFakeVerdictStore()
	failing.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	failing.pinErr = errors.New("pin unavailable")
	if err := resolveVerdictsAtBase(t.Context(), failing, d); err == nil {
		t.Fatal("a failed pin was swallowed")
	}
	if len(failing.resolved) != 0 {
		t.Fatalf("a verdict was recorded though its base could not be pinned: %+v", failing.resolved)
	}
}

// Pins are for apply checkpoints. A cadence (owner-recovery) checkpoint comes
// every few minutes and a pin has no release path until the retention slice:
// pinning each would grow the host's disk without bound. Its base is kept by
// the newest-two rule while it is new.
func TestReconcileDoesNotPinTheBaseOfACadenceCheckpoint(t *testing.T) {
	store := newFakeVerdictStore()
	head, blob := hex64('1'), hex64('9')
	store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	cp := store.checkpoints[hex64('d')]
	cp.Request.OwnerRecovery = true
	store.checkpoints[hex64('d')] = cp
	store.bases["c1/100"] = blob
	d := descriptorOf("c1", 100, head, blob, witnessOf(tablesA))
	if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
		t.Fatal(err)
	}
	if v := store.resolved[hex64('d')]; v.Status != selfdevprotocol.CheckpointVerdictVerified {
		t.Fatalf("verdict = %+v", v)
	}
	if len(store.pinned) != 0 {
		t.Fatalf("a cadence checkpoint pinned %v", store.pinned)
	}
}

// Review 329 (DeepSeek 6, Devin 7): a host on a newer reducer than the guest
// can rebuild different bytes from the same tape. A mismatch across reducer
// versions is unverified / version_skew, never not_derivable; equal content is
// verified whatever the versions.
func TestReconcileDoesNotCallAMismatchAcrossReducerVersionsNotDerivable(t *testing.T) {
	store := newFakeVerdictStore()
	head, blob := hex64('1'), hex64('9')
	store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	cp := store.checkpoints[hex64('d')]
	cp.Request.ReducerVersion = 1
	store.checkpoints[hex64('d')] = cp
	store.bases["c1/100"] = blob
	d := descriptorOf("c1", 100, head, blob, witnessOf(tablesB))
	d.ReducerVersion = 2
	if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
		t.Fatal(err)
	}
	v := store.resolved[hex64('d')]
	if v.Status != selfdevprotocol.CheckpointVerdictUnverified || v.Reason != selfdevprotocol.VerdictReasonVersionSkew || strings.Join(v.DifferingTables, ",") != "og_objects" {
		t.Fatalf("verdict = %+v, want unverified / version_skew naming og_objects", v)
	}

	equal := newFakeVerdictStore()
	equal.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	cp = equal.checkpoints[hex64('d')]
	cp.Request.ReducerVersion = 1
	equal.checkpoints[hex64('d')] = cp
	equal.bases["c1/100"] = blob
	d = descriptorOf("c1", 100, head, blob, witnessOf(tablesA))
	d.ReducerVersion = 2
	if err := reconcileCheckpointVerdicts(t.Context(), equal, loader(d)); err != nil {
		t.Fatal(err)
	}
	if v := equal.resolved[hex64('d')]; v.Status != selfdevprotocol.CheckpointVerdictVerified {
		t.Fatalf("equal content across versions = %+v, want verified", v)
	}
}

// Review 329 (Devin 7): a base is not retained for a checkpoint restore will
// refuse.
func TestReconcileDoesNotPinTheBaseOfANotDerivableApplyCheckpoint(t *testing.T) {
	store := newFakeVerdictStore()
	head, blob := hex64('1'), hex64('9')
	store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	store.bases["c1/100"] = blob
	d := descriptorOf("c1", 100, head, blob, witnessOf(tablesB))
	if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
		t.Fatal(err)
	}
	if v := store.resolved[hex64('d')]; v.Status != selfdevprotocol.CheckpointVerdictNotDerivable {
		t.Fatalf("verdict = %+v", v)
	}
	if len(store.pinned) != 0 {
		t.Fatalf("a not_derivable checkpoint's base was pinned: %v", store.pinned)
	}
}

type fakeWorkStore struct {
	*fakeVerdictStore
	calls        *[]string
	reconcileErr error
}

func (f fakeWorkStore) PendingCheckpointVerdicts(ctx context.Context) ([]platform.CheckpointVerdictRecord, error) {
	*f.calls = append(*f.calls, "verdicts")
	return f.fakeVerdictStore.PendingCheckpointVerdicts(ctx)
}

func (f fakeWorkStore) ReconcileProjectionJobs(context.Context) error {
	*f.calls = append(*f.calls, "cadence")
	return f.reconcileErr
}

// Review 329 (Devin 6): on a busy computer the cadence job comes due again as
// soon as the last one finishes. If the cadence is queued first it takes the
// computer's one job row every minute and the checkpoint's rebuild never gets
// it. The verification is queued first; a problem in either is reported and
// stops neither.
func TestReconcileProjectionWorkQueuesVerificationBeforeTheCadenceJob(t *testing.T) {
	var calls []string
	store := fakeWorkStore{fakeVerdictStore: newFakeVerdictStore(), calls: &calls}
	store.addPending("c1", hex64('d'), 100, hex64('1'), witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
	problem := reconcileProjectionWork(t.Context(), store, loader())
	if len(calls) != 2 || calls[0] != "verdicts" || calls[1] != "cadence" {
		t.Fatalf("order %v, want verdicts then cadence", calls)
	}
	if problem != "" {
		t.Fatalf("problem %q", problem)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("enqueued %v", store.enqueued)
	}

	calls = nil
	store.reconcileErr = errors.New("one computer cannot be enqueued")
	store.enqueueErr["c1"] = errors.New("store unavailable")
	problem = reconcileProjectionWork(t.Context(), store, loader())
	if len(calls) != 2 || !strings.Contains(problem, "reconcile: one computer cannot be enqueued") || !strings.Contains(problem, "verdicts:") {
		t.Fatalf("calls %v problem %q: both problems must be reported and neither may stop the other", calls, problem)
	}
}

// A base captured from a running computer's disk (S2,
// docs/problems/paper-never-checkpointed-2026-10-27.md) carries a live
// witness, not a rebuild from the tape: agreeing with it proves the
// checkpoint is that state, not that the tape derives it, and disagreeing
// with it proves nothing about the tape. Neither may read as a tape verdict.
func TestReconcileNeverCallsALiveCapturedBaseATapeVerdict(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tables map[string]string
	}{{"equal", tablesA}, {"different", tablesB}} {
		store := newFakeVerdictStore()
		head, blob := hex64('1'), hex64('9')
		store.addPending("c1", hex64('d'), 100, head, witnessOf(tablesA), selfdevprotocol.DepositModeUpcast)
		store.bases["c1/100"] = blob
		d := descriptorOf("c1", 100, head, blob, witnessOf(tc.tables))
		d.Origin = projectionbase.OriginLiveCaptured
		if err := reconcileCheckpointVerdicts(t.Context(), store, loader(d)); err != nil {
			t.Fatal(err)
		}
		v := store.resolved[hex64('d')]
		if v.Status != selfdevprotocol.CheckpointVerdictUnverified || v.Reason != verdictReasonLiveCaptured {
			t.Fatalf("%s: verdict = %+v, want unverified/%s", tc.name, v, verdictReasonLiveCaptured)
		}
	}
}
