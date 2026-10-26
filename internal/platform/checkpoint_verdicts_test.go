package platform

// The host's verdict on a published checkpoint (slice 2 of
// docs/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. a published checkpoint has no pending verdict, so nothing ever asks the
//     host to prove it;
//  2. the verification job targets the platform's head at enqueue time, not
//     the checkpoint's head: the rebuilt witness is then another state's, and
//     every checkpoint on a working computer reads not_derivable;
//  3. the job seeds from a base above the checkpoint's head, which the
//     rebuilder cannot adopt (blocked), or from none when an older one exists;
//  4. a verification job replaces a running job's frozen target;
//  5. a checkpoint too far above every base (beyond the replay bound, no base
//     below it) enqueues a job that can only block the computer's cadence;
//  6. a verdict is overwritten after it was judged (verified becomes
//     pending/unverified on a later pass), or a re-published checkpoint resets
//     it;
//  7. the verdict route answers without authority, or answers for a foreign
//     computer's checkpoint.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const verdictComputer = "computer-owner-recovery"

// pendingVerdict inserts a checkpoint's pending verdict without a publish, at
// the sequence and head the test names.
func pendingVerdict(t *testing.T, s *Store, digest string, sequence uint64, head string) CheckpointVerdictRecord {
	t.Helper()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(t.Context(), `INSERT INTO computer_checkpoint_verdicts (computer_id,checkpoint_digest,accepted_sequence,accepted_head,status,reason,detail,differing_tables,base_ref,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		verdictComputer, digest, sequence, head, selfdevprotocol.CheckpointVerdictPending, "", "", "[]", "", now, now); err != nil {
		t.Fatal(err)
	}
	return CheckpointVerdictRecord{ComputerID: verdictComputer, CheckpointDigest: digest, AcceptedHead: head, Verdict: selfdevprotocol.CheckpointVerdict{Status: selfdevprotocol.CheckpointVerdictPending, AcceptedSequence: sequence}}
}

func setHead(t *testing.T, s *Store, sequence uint64, head string) {
	t.Helper()
	if _, err := s.db.ExecContext(t.Context(), `UPDATE computer_event_heads SET sequence=?, canonical_event_head=? WHERE computer_id=?`, sequence, head, verdictComputer); err != nil {
		t.Fatal(err)
	}
}

func insertBase(t *testing.T, s *Store, ref string, sequence uint64) {
	t.Helper()
	if _, err := s.db.ExecContext(t.Context(), `INSERT INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?)`, verdictComputer, ref, sequence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

// Failure mode 1, and 6 for a replay.
func TestPublishRecordsAPendingVerdictAndAReplayDoesNotResetIt(t *testing.T) {
	authority, request, _, head := ownerRecoveryPlatformFixture(t)
	response, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	store := authority.cas.store
	record, err := store.CheckpointVerdict(t.Context(), request.ComputerID, response.Checkpoint.Digest)
	if err != nil {
		t.Fatalf("no verdict row for a published checkpoint: %v", err)
	}
	if record.Verdict.Status != selfdevprotocol.CheckpointVerdictPending || record.Verdict.AcceptedSequence != 12 || record.AcceptedHead != head {
		t.Fatalf("published checkpoint's verdict = %+v", record)
	}
	resolved, err := store.ResolveCheckpointVerdict(t.Context(), request.ComputerID, response.Checkpoint.Digest, selfdevprotocol.CheckpointVerdict{Status: selfdevprotocol.CheckpointVerdictVerified, BaseRef: strings.Repeat("b", 64)})
	if err != nil || !resolved {
		t.Fatalf("resolve: %v %v", resolved, err)
	}
	if _, err := authority.Publish(t.Context(), request); err != nil {
		t.Fatalf("replay: %v", err)
	}
	record, err = store.CheckpointVerdict(t.Context(), request.ComputerID, response.Checkpoint.Digest)
	if err != nil || record.Verdict.Status != selfdevprotocol.CheckpointVerdictVerified {
		t.Fatalf("a replayed publish reset the verdict: %+v %v", record, err)
	}
}

// Failure mode 6.
func TestResolveCheckpointVerdictJudgesOnlyOnce(t *testing.T) {
	authority, request, _, head := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	digest := strings.Repeat("1", 64)
	pendingVerdict(t, store, digest, 12, head)
	first, err := store.ResolveCheckpointVerdict(t.Context(), request.ComputerID, digest, selfdevprotocol.CheckpointVerdict{
		Status: selfdevprotocol.CheckpointVerdictNotDerivable, Reason: selfdevprotocol.VerdictReasonWitnessMismatch,
		DifferingTables: []string{"og_objects", "runs"}, BaseRef: strings.Repeat("b", 64),
	})
	if err != nil || !first {
		t.Fatalf("first resolve: %v %v", first, err)
	}
	second, err := store.ResolveCheckpointVerdict(t.Context(), request.ComputerID, digest, selfdevprotocol.CheckpointVerdict{Status: selfdevprotocol.CheckpointVerdictVerified})
	if err != nil || second {
		t.Fatalf("a judged verdict was judged again: %v %v", second, err)
	}
	got, err := store.CheckpointVerdict(t.Context(), request.ComputerID, digest)
	if err != nil || got.Verdict.Status != selfdevprotocol.CheckpointVerdictNotDerivable || got.Verdict.Reason != selfdevprotocol.VerdictReasonWitnessMismatch ||
		strings.Join(got.Verdict.DifferingTables, ",") != "og_objects,runs" || got.Verdict.AcceptedSequence != 12 {
		t.Fatalf("stored verdict = %+v %v", got, err)
	}
	pending, err := store.PendingCheckpointVerdicts(t.Context())
	if err != nil || len(pending) != 0 {
		t.Fatalf("a judged verdict is still pending: %+v %v", pending, err)
	}
}

// Failure mode 2: the job names the checkpoint's head even though the
// computer's head moved on while the checkpoint waited.
func TestVerificationJobTargetsTheCheckpointHeadNotTheCurrentHead(t *testing.T) {
	authority, _, _, head := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	record := pendingVerdict(t, store, strings.Repeat("1", 64), 12, head)
	setHead(t, store, 4000, strings.Repeat("f", 64))
	job, err := store.EnqueueCheckpointVerificationJob(t.Context(), record)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" || job.TargetSequence != 12 || job.TargetHead != head {
		t.Fatalf("job = status %s target %d/%s; want queued at the checkpoint's 12/%s", job.Status, job.TargetSequence, job.TargetHead, head)
	}
	if !strings.HasPrefix(job.Reason, "checkpoint-verify") {
		t.Fatalf("job reason %q", job.Reason)
	}
}

// Failure mode 3: seeds. At or above the watermark the job seeds from the
// advertised base; below the watermark, from the newest base at or below the
// checkpoint.
func TestVerificationJobSeedsFromABaseAtOrBelowTheCheckpoint(t *testing.T) {
	authority, request, _, head := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	record := pendingVerdict(t, store, strings.Repeat("1", 64), 12, head)
	older, newer, advertised := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	insertBase(t, store, older, 5)
	insertBase(t, store, newer, 9)
	insertBase(t, store, advertised, 20)
	if err := store.RecordReplayWatermark(t.Context(), request.ComputerID, 20, advertised); err != nil {
		t.Fatal(err)
	}
	setHead(t, store, 30, strings.Repeat("f", 64))
	job, err := store.EnqueueCheckpointVerificationJob(t.Context(), record)
	if err != nil {
		t.Fatal(err)
	}
	if job.SeedBaseRef != newer || job.WatermarkSequence != 9 {
		t.Fatalf("watermark 20 is above the checkpoint at 12: seed %s at %d, want the newest base at or below it (%s at 9)", job.SeedBaseRef, job.WatermarkSequence, newer)
	}

	// A watermark below the checkpoint is the seed.
	store2 := openVerdictStore(t)
	rec2 := pendingVerdict(t, store2, strings.Repeat("2", 64), 25, head)
	insertBase(t, store2, older, 5)
	if err := store2.RecordReplayWatermark(t.Context(), request.ComputerID, 5, older); err != nil {
		t.Fatal(err)
	}
	setHead(t, store2, 30, strings.Repeat("f", 64))
	job2, err := store2.EnqueueCheckpointVerificationJob(t.Context(), rec2)
	if err != nil {
		t.Fatal(err)
	}
	if job2.SeedBaseRef != older || job2.WatermarkSequence != 5 || job2.TargetSequence != 25 {
		t.Fatalf("job = seed %s at %d target %d, want the advertised base %s at 5, target 25", job2.SeedBaseRef, job2.WatermarkSequence, job2.TargetSequence, older)
	}
}

// openVerdictStore is a second fixture store with the same computer head row.
func openVerdictStore(t *testing.T) *Store {
	t.Helper()
	authority, _, _, _ := ownerRecoveryPlatformFixture(t)
	return authority.cas.store
}

// Failure mode 5.
func TestVerificationJobRefusesByNameWhenNoBaseIsInReach(t *testing.T) {
	authority, _, _, _ := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	head := strings.Repeat("e", 64)
	setHead(t, store, 10_500, head)
	record := pendingVerdict(t, store, strings.Repeat("1", 64), 10_050, head)
	if _, err := store.EnqueueCheckpointVerificationJob(t.Context(), record); !errors.Is(err, ErrNoVerificationBase) {
		t.Fatalf("a checkpoint beyond the replay bound with no base below it enqueued: %v", err)
	}
	if _, err := store.ProjectionJob(t.Context(), verdictComputer); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("a refused verification left a job row: %v", err)
	}
}

// Failure mode 4.
func TestVerificationJobDoesNotReplaceARunningJob(t *testing.T) {
	authority, _, _, head := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	cadence, err := store.EnqueueProjectionJob(t.Context(), verdictComputer, "cadence", false)
	if err != nil {
		t.Fatal(err)
	}
	if leased, err := store.ClaimProjectionJob(t.Context()); err != nil || leased == nil {
		t.Fatalf("claim: %v %v", leased, err)
	}
	record := pendingVerdict(t, store, strings.Repeat("1", 64), 12, head)
	got, err := store.EnqueueCheckpointVerificationJob(t.Context(), record)
	if err != nil {
		t.Fatal(err)
	}
	if got.Generation != cadence.Generation || got.Status != "running" {
		t.Fatalf("a running job was replaced: %+v", got)
	}
}

// A job that already succeeded at a higher target does not satisfy a lower
// checkpoint: the verification enqueues again.
func TestVerificationJobIsAdmittedAfterASucceededJobAtAHigherTarget(t *testing.T) {
	authority, _, _, head := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	if _, err := store.EnqueueProjectionJob(t.Context(), verdictComputer, "cadence", false); err != nil {
		t.Fatal(err)
	}
	leased, err := store.ClaimProjectionJob(t.Context())
	if err != nil || leased == nil {
		t.Fatal(err)
	}
	if err := store.FinishProjectionJob(t.Context(), *leased, "succeeded", "", 12, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	setHead(t, store, 20, strings.Repeat("f", 64))
	record := pendingVerdict(t, store, strings.Repeat("1", 64), 12, head)
	job, err := store.EnqueueCheckpointVerificationJob(t.Context(), record)
	if err != nil || job.Status != "queued" || job.Generation != leased.Generation+1 {
		t.Fatalf("verification after a succeeded job: %+v %v", job, err)
	}
}

func TestProjectionBaseAtSequence(t *testing.T) {
	authority, _, _, _ := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	ref := strings.Repeat("a", 64)
	insertBase(t, store, ref, 12)
	got, ok, err := store.ProjectionBaseAtSequence(t.Context(), verdictComputer, 12)
	if err != nil || !ok || got != ref {
		t.Fatalf("base at 12 = %q %v %v", got, ok, err)
	}
	if _, ok, err := store.ProjectionBaseAtSequence(t.Context(), verdictComputer, 13); err != nil || ok {
		t.Fatalf("base at 13 = %v %v", ok, err)
	}
}

func TestStoredCheckpointReturnsThePublishedCheckpoint(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	response, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	got, err := authority.cas.store.StoredCheckpoint(t.Context(), request.ComputerID, response.Checkpoint.Digest)
	if err != nil || got.Digest != response.Checkpoint.Digest || got.Request.VMLocalContentWitness.ContentRoot != request.VMLocalContentWitness.ContentRoot {
		t.Fatalf("stored checkpoint = %+v %v", got.Digest, err)
	}
	if _, err := authority.cas.store.StoredCheckpoint(t.Context(), "computer-other", response.Checkpoint.Digest); err == nil {
		t.Fatal("a checkpoint was read through another computer")
	}
}

// Failure mode 7.
func TestCheckpointVerdictRoute(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	response, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(authority.service)
	get := func(computerID, digest string, internal bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/internal/computers/checkpoints/verdict?computer_id="+computerID+"&checkpoint_digest="+digest, nil)
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		handler.HandleCheckpointVerdict(rec, req)
		return rec
	}
	if got := get(request.ComputerID, response.Checkpoint.Digest, false); got.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated verdict = %d, want 403", got.Code)
	}
	got := get(request.ComputerID, response.Checkpoint.Digest, true)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"status":"pending"`) {
		t.Fatalf("verdict = %d %s", got.Code, got.Body.String())
	}
	if got := get(request.ComputerID, strings.Repeat("0", 64), true); got.Code != http.StatusNotFound {
		t.Fatalf("unknown checkpoint verdict = %d, want 404", got.Code)
	}
	if got := get("computer-other", response.Checkpoint.Digest, true); got.Code != http.StatusNotFound {
		t.Fatalf("a verdict was served through another computer: %d", got.Code)
	}
	post := httptest.NewRequest(http.MethodPost, "/internal/computers/checkpoints/verdict", nil)
	post.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleCheckpointVerdict(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST verdict = %d, want 405", rec.Code)
	}
}

var _ = context.Background

// Review 329 (DeepSeek 4): a deterministic refusal or a failure backoff of the
// computer's cadence job at another target must not starve a checkpoint's
// verification, which asks for a different head and would not hit the same
// refusal. The refusal still stands for a request at its own target.
func TestVerificationJobIsAdmittedPastARefusalAtAnotherTarget(t *testing.T) {
	for _, status := range []string{"blocked", "failed"} {
		authority, _, _, head := ownerRecoveryPlatformFixture(t)
		store := authority.cas.store
		setHead(t, store, 3000, strings.Repeat("f", 64))
		if _, err := store.EnqueueProjectionJob(t.Context(), verdictComputer, "cadence", false); err != nil {
			t.Fatal(err)
		}
		leased, err := store.ClaimProjectionJob(t.Context())
		if err != nil || leased == nil {
			t.Fatal(err)
		}
		if err := store.FinishProjectionJob(t.Context(), *leased, status, "refused at 3000", 0, ""); err != nil {
			t.Fatal(err)
		}
		record := pendingVerdict(t, store, strings.Repeat("1", 64), 12, head)
		job, err := store.EnqueueCheckpointVerificationJob(t.Context(), record)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status != "queued" || job.TargetSequence != 12 || job.TargetHead != head {
			t.Fatalf("%s at another target starved the verification: %+v", status, job)
		}
	}
}

func TestVerificationJobStaysRefusedAtItsOwnTarget(t *testing.T) {
	authority, _, _, head := ownerRecoveryPlatformFixture(t)
	store := authority.cas.store
	record := pendingVerdict(t, store, strings.Repeat("1", 64), 12, head)
	if _, err := store.EnqueueCheckpointVerificationJob(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	leased, err := store.ClaimProjectionJob(t.Context())
	if err != nil || leased == nil {
		t.Fatal(err)
	}
	if err := store.FinishProjectionJob(t.Context(), *leased, "blocked", "projection base refused", 0, ""); err != nil {
		t.Fatal(err)
	}
	again, err := store.EnqueueCheckpointVerificationJob(t.Context(), record)
	if err != nil || again.Status != "blocked" || again.Generation != leased.Generation {
		t.Fatalf("a refusal at the checkpoint's own target was re-admitted: %+v %v", again, err)
	}
}
