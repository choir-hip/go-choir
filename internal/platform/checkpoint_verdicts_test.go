package platform

// The host's verdict on a published checkpoint (slice 2 of
// docs/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. a published checkpoint has no pending verdict, so nothing ever asks the
//     host to prove it;
//  2-5. (the host-side verification job: its target, seed, coalescing and
//     refusal; deleted with the host rebuild at the data system cutover);
//  6. a verdict is overwritten after it was judged (verified becomes
//     pending/unverified on a later pass), or a re-published checkpoint resets
//     it;
//  7. the verdict route answers without authority, or answers for a foreign
//     computer's checkpoint.

import (
	"context"
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
