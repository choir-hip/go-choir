package platform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Publication pins both generations before GC can observe the new watermark;
// an explicit restore pin keeps a base independently of rank.
func TestProjectionBasePinsKeepTheLatestTwoAndExplicitPins(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	a, b, c := strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64)
	for i, ref := range []string{a, b, c} {
		if err := s.RecordReplayWatermark(ctx, "computer-jobs", int64(i+1), ref); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := s.projectionBasePins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pins[b]; !ok {
		t.Fatal("previous generation not pinned")
	}
	if _, ok := pins[c]; !ok {
		t.Fatal("current not pinned")
	}
	if _, ok := pins[a]; ok {
		t.Fatal("unreferenced oldest generation retained")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_projection_base_pins (computer_id,base_ref,reference_id,created_at) VALUES (?,?,?,?)`, "computer-jobs", a, "restore:receipt-a", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	pins, err = s.projectionBasePins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pins[a]; !ok {
		t.Fatal("explicit restore pin lost")
	}
}

func insertProjectionTestHead(t *testing.T, s *Store, id string, seq uint64) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `INSERT INTO computer_event_heads (computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,reducer_version,credential_revocation_epoch,created_at,updated_at) VALUES (?,?,?,?,?,?,?,NULL,1,0,?,?)`, id, seq, strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("b", 64), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
}

// Frozen-candidate panel (2026-10-08): two transient failures must not end
// cadence forever. A failed job becomes eligible again after backoff; a
// blocked (deterministic) refusal still waits for changed inputs; the failure
// alert is never masked by a tail alert.
func TestProjectionJobFailedRetriesAfterBackoffBlockedDoesNot(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	insertProjectionTestHead(t, s, "computer-failed", 9000)
	insertProjectionTestHead(t, s, "computer-blocked", 9000)
	for _, id := range []string{"computer-failed", "computer-blocked"} {
		if _, err := s.EnqueueProjectionJob(ctx, id, "cadence", false); err != nil {
			t.Fatal(err)
		}
	}
	setJob := func(id, status string, failures int, age time.Duration) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status=?,failures=?,updated_at=? WHERE computer_id=?`, status, failures, time.Now().UTC().Add(-age), id); err != nil {
			t.Fatal(err)
		}
	}
	setJob("computer-failed", "failed", 2, time.Minute)
	setJob("computer-blocked", "blocked", 2, 48*time.Hour)

	j, err := s.ProjectionJob(ctx, "computer-failed")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j.Alert, "urgent_tail") || !strings.Contains(j.Alert, "repeated_failure") {
		t.Fatalf("alert = %q, want urgent_tail and repeated_failure together", j.Alert)
	}
	again, err := s.EnqueueProjectionJob(ctx, "computer-failed", "repair", false)
	if err != nil || again.Status != "failed" || again.Generation != 1 {
		t.Fatalf("failed job re-admitted inside backoff: %+v %v", again, err)
	}
	setJob("computer-failed", "failed", 2, time.Hour)
	again, err = s.EnqueueProjectionJob(ctx, "computer-failed", "repair", false)
	if err != nil || again.Status != "queued" || again.Generation != 2 || again.Failures != 2 {
		t.Fatalf("failed job not re-admitted after backoff (failures must carry for growing backoff): %+v %v", again, err)
	}
	blocked, err := s.EnqueueProjectionJob(ctx, "computer-blocked", "repair", false)
	if err != nil || blocked.Status != "blocked" || blocked.Generation != 1 {
		t.Fatalf("deterministic refusal re-admitted without changed inputs: %+v %v", blocked, err)
	}
}

// Staging 2026-10-08 (docs/problems/gc-sweep-holds-service-write-lock):
// an artifact GC sweep held the service-wide writeMu, so credential issuance
// timed out and VM boots failed. A sweep holds only the GC/pin lock; every
// other writer must proceed while it is held.
func TestArtifactGCLockDoesNotBlockCredentialIssuance(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	service.artifactGCMu.Lock() // a sweep in progress
	defer service.artifactGCMu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, _, err := service.MintComputerCredentialEnvelope(context.Background(), "computer-gcwindow", "realization-gc", "issue-gc", time.Now().UTC().Add(4*time.Minute))
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("credential issuance during GC sweep: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("credential issuance blocked behind the artifact GC sweep lock")
	}
}

// docs/problems/projection-job-blocked-by-a-cancel-2026-10-26.md: a run
// cancelled mid read-back was recorded `blocked` and never re-admitted, which
// froze b@c's projection base. Rows already written that way must be treated
// as `failed` (re-admitted after backoff); a deterministic refusal must still
// wait for changed inputs, and head growth alone must not re-admit it.
func TestProjectionJobBlockedByCancelIsReadmittedDeterministicBlockIsNot(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	for _, id := range []string{"computer-cancel", "computer-deadline", "computer-refused"} {
		insertProjectionTestHead(t, s, id, 9000)
		if _, err := s.EnqueueProjectionJob(ctx, id, "cadence", false); err != nil {
			t.Fatal(err)
		}
	}
	set := func(id, status, errText string, failures int, age time.Duration) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status=?,error_text=?,failures=?,updated_at=? WHERE computer_id=?`, status, errText, failures, time.Now().UTC().Add(-age), id); err != nil {
			t.Fatal(err)
		}
	}
	cancelText := "projection base refused: read-back witness: projection base: extract content witness: context canceled"
	deadlineText := "projection base refused: read-back witness: context deadline exceeded"
	refusedText := "projection base refused: read-back witness mismatch: content root differs"
	set("computer-cancel", "blocked", cancelText, 1, time.Minute)
	set("computer-deadline", "blocked", deadlineText, 1, time.Minute)
	set("computer-refused", "blocked", refusedText, 1, 48*time.Hour)

	// Inside the backoff a blocked-by-cancel row is held like a failed one.
	held, err := s.EnqueueProjectionJob(ctx, "computer-cancel", "cadence", false)
	if err != nil || held.Status != "blocked" || held.Generation != 1 {
		t.Fatalf("blocked-by-cancel re-admitted inside backoff: %+v %v", held, err)
	}
	// After the backoff both cancel and deadline rows are re-admitted.
	set("computer-cancel", "blocked", cancelText, 1, time.Hour)
	set("computer-deadline", "blocked", deadlineText, 1, time.Hour)
	for _, id := range []string{"computer-cancel", "computer-deadline"} {
		again, err := s.EnqueueProjectionJob(ctx, id, "cadence", false)
		if err != nil || again.Status != "queued" || again.Generation != 2 || again.Error != "" {
			t.Fatalf("%s: blocked-by-cancel not re-admitted after backoff: %+v %v", id, again, err)
		}
	}
	// A deterministic refusal is held, even old, and even after head growth.
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_event_heads SET sequence=? WHERE computer_id=?`, 20000, "computer-refused"); err != nil {
		t.Fatal(err)
	}
	refused, err := s.EnqueueProjectionJob(ctx, "computer-refused", "cadence", false)
	if err != nil || refused.Status != "blocked" || refused.Generation != 1 || refused.TargetSequence != 9000 {
		t.Fatalf("deterministic block re-admitted by head growth: %+v %v", refused, err)
	}
	// A blocked-by-cancel row with changed inputs is re-admitted by the
	// existing rule regardless of age (seed changed).
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='blocked',error_text=?,updated_at=? WHERE computer_id=?`, refusedText, time.Now().UTC(), "computer-refused"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReplayWatermark(ctx, "computer-refused", 5, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	changed, err := s.EnqueueProjectionJob(ctx, "computer-refused", "cadence", false)
	if err != nil || changed.Status != "queued" || changed.Generation != 2 {
		t.Fatalf("changed seed did not re-admit a blocked job: %+v %v", changed, err)
	}
}
