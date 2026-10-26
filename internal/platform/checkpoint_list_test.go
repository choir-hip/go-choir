package platform

// The read-only checkpoint listing (slice 3 of
// docs/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. a checkpoint published before the host judged them has no sequence or
//     reads as pending/verified: it must read "unjudged" with its sequence
//     placed from the tape's receipts;
//  2. the listing is not newest first, so the cadence reads an old checkpoint
//     as the last and binds every tick;
//  3. a computer lists another computer's checkpoints, or the listing answers
//     without authority;
//  4. an unbounded limit reads the whole table;
//  5. fetching a checkpoint by digest returns a body the restore cannot use
//     (a Checkpoint with its receipt, as published), or another computer's.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func listCheckpoints(t *testing.T, h *Handler, query string, internal bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/computers/checkpoints/list?"+query, nil)
	if internal {
		req.Header.Set("X-Internal-Caller", "true")
	}
	rec := httptest.NewRecorder()
	h.HandleCheckpointList(rec, req)
	return rec
}

func TestCheckpointListShowsVerdictsNewestFirst(t *testing.T) {
	authority, request, _, head := ownerRecoveryPlatformFixture(t)
	first, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	store := authority.cas.store
	// A second checkpoint row, later, judged verified.
	newer := strings.Repeat("2", 64)
	now := time.Now().UTC().Add(time.Minute)
	if _, err := store.db.ExecContext(t.Context(), `INSERT INTO computer_checkpoints (computer_id,idempotency_key,request_commitment,checkpoint_digest,checkpoint_artifact_ref,checkpoint_json,receipt_json,receipt_digest,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		request.ComputerID, "later", strings.Repeat("c", 64), newer, "ref", `{"request":{"computer_id":"`+request.ComputerID+`","accepted_event_head":"`+head+`","release_digest":"`+request.ReleaseDigest+`","owner_recovery":true},"checkpoint_digest":"`+newer+`"}`, `{}`, strings.Repeat("d", 64), now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `INSERT INTO computer_checkpoint_verdicts (`+checkpointVerdictColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		request.ComputerID, newer, 40, head, selfdevprotocol.CheckpointVerdictVerified, "", "", "[]", strings.Repeat("b", 64), now, now); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authority.service)
	rec := listCheckpoints(t, h, "computer_id="+request.ComputerID, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	var list selfdevprotocol.CheckpointList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Checkpoints) != 2 || list.Checkpoints[0].CheckpointDigest != newer || list.Checkpoints[1].CheckpointDigest != first.Checkpoint.Digest {
		t.Fatalf("listing order: %+v", list.Checkpoints)
	}
	if got := list.Checkpoints[0]; got.Verdict.Status != selfdevprotocol.CheckpointVerdictVerified || got.AcceptedSequence != 40 || got.EvidenceClass != selfdevprotocol.CheckpointClassOwnerRecovery || got.AcceptedHead != head {
		t.Fatalf("newest row = %+v", got)
	}
	if got := list.Checkpoints[1]; got.Verdict.Status != selfdevprotocol.CheckpointVerdictPending || got.AcceptedSequence != 12 {
		t.Fatalf("published row = %+v", got)
	}
}

// Failure mode 1.
func TestCheckpointListPlacesALegacyCheckpointFromTheTape(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	published, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `DELETE FROM computer_checkpoint_verdicts WHERE computer_id=?`, request.ComputerID); err != nil {
		t.Fatal(err)
	}
	rec := listCheckpoints(t, NewHandler(authority.service), "computer_id="+request.ComputerID, true)
	var list selfdevprotocol.CheckpointList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Checkpoints) != 1 {
		t.Fatalf("list = %d %s %v", rec.Code, rec.Body.String(), err)
	}
	row := list.Checkpoints[0]
	if row.CheckpointDigest != published.Checkpoint.Digest || row.Verdict.Status != selfdevprotocol.CheckpointVerdictUnjudged || row.AcceptedSequence != 12 {
		t.Fatalf("legacy row = %+v, want unjudged at the sequence the tape places the head", row)
	}
}

// Failures 3 and 4.
func TestCheckpointListIsScopedAuthorizedAndBounded(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	if _, err := authority.Publish(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authority.service)
	if rec := listCheckpoints(t, h, "computer_id="+request.ComputerID, false); rec.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated list = %d", rec.Code)
	}
	rec := listCheckpoints(t, h, "computer_id=computer-other", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"checkpoints":[]`) {
		t.Fatalf("another computer's list = %d %s", rec.Code, rec.Body.String())
	}
	if rec := listCheckpoints(t, h, "computer_id="+request.ComputerID+"&limit=100000", true); rec.Code != http.StatusOK {
		t.Fatalf("a large limit = %d", rec.Code)
	}
	if rec := listCheckpoints(t, h, "computer_id="+request.ComputerID+"&limit=abc", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("a bad limit = %d", rec.Code)
	}
	if got := clampCheckpointListLimit(100000); got != 100 {
		t.Fatalf("clamp(100000) = %d, want 100", got)
	}
	if got := clampCheckpointListLimit(0); got != 20 {
		t.Fatalf("clamp(0) = %d, want the default 20", got)
	}
}

// Failure mode 5.
func TestCheckpointFetchByDigestReturnsThePublishedResponse(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	published, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authority.service)
	rec := listCheckpoints(t, h, "computer_id="+request.ComputerID+"&checkpoint_digest="+published.Checkpoint.Digest, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("fetch = %d %s", rec.Code, rec.Body.String())
	}
	var got selfdevprotocol.CheckpointResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Checkpoint.Digest != published.Checkpoint.Digest || got.Receipt.ArtifactDigest != published.Checkpoint.Digest || got.Receipt.Signature == "" {
		t.Fatalf("fetched %+v", got)
	}
	if rec := listCheckpoints(t, h, "computer_id=computer-other&checkpoint_digest="+published.Checkpoint.Digest, true); rec.Code != http.StatusNotFound {
		t.Fatalf("another computer's fetch = %d", rec.Code)
	}
	if rec := listCheckpoints(t, h, "computer_id="+request.ComputerID+"&checkpoint_digest="+strings.Repeat("0", 64), true); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown digest = %d", rec.Code)
	}
}
