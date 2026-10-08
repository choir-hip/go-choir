package platform

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCheckpointDueBoundaries(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		head, watermark uint64
		age             time.Duration
		want            bool
	}{
		{2500, 0, 0, true}, {2499, 0, 0, false}, {6000, 5000, 24 * time.Hour, true},
		{5999, 5000, 24 * time.Hour, false}, {6000, 5000, 23 * time.Hour, false}, {5000, 5000, 48 * time.Hour, false},
	}
	for _, c := range cases {
		if got := checkpointDue(c.head, c.watermark, now.Add(-c.age), now); got != c.want {
			t.Errorf("%+v due=%v", c, got)
		}
	}
}

// A duplicate request cannot replace a running job's frozen head; publication
// pins both generations before GC can observe the new watermark.
func TestProjectionJobCoalescingAndRetention(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	// Empty canonical heads are not a legal job request.
	if _, err := s.EnqueueProjectionJob(ctx, "computer-jobs", "repair", false); err == nil {
		t.Fatal("enqueued nonexistent chain")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_event_heads (computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,reducer_version,credential_revocation_epoch,created_at,updated_at) VALUES (?,?,?,?,?,?,?,NULL,1,0,?,?)`, "computer-jobs", 20, strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("b", 64), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	j, err := s.EnqueueProjectionJob(ctx, "computer-jobs", "repair", false)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := s.ClaimProjectionJob(ctx)
	if err != nil || lease == nil {
		t.Fatalf("claim: %v %v", lease, err)
	}
	again, err := s.EnqueueProjectionJob(ctx, "computer-jobs", "duplicate", false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Generation != j.Generation || again.Status != "running" || again.TargetHead != lease.TargetHead {
		t.Fatalf("running job replaced: %+v / %+v", lease, again)
	}
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
	// A reader/restore pin prevents collection independently of rank.
	if err := s.PinProjectionBase(ctx, "computer-jobs", a, "restore:receipt-a"); err != nil {
		t.Fatal(err)
	}
	pins, err = s.projectionBasePins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pins[a]; !ok {
		t.Fatal("explicit restore pin lost")
	}
	// A stale worker may not finish a newer job.
	stale := *lease
	stale.Generation++
	if err := s.FinishProjectionJob(ctx, stale, "failed", "wrong worker", 0, ""); err == nil {
		t.Fatal("stale job completion admitted")
	}
	if err := s.FinishProjectionJob(ctx, *lease, "failed", "deterministic refusal", 0, ""); err != nil {
		t.Fatal(err)
	}
	reconstructed := NewStore(s.db)
	got, err := reconstructed.ProjectionJob(ctx, "computer-jobs")
	if err != nil || got.Status != "failed" || got.Error != "deterministic refusal" {
		t.Fatalf("durable refusal lost %+v %v", got, err)
	}
}

// Staging 0f7c58ba: reconcile scanned COALESCE(datetime,datetime) into
// time.Time and Dolt returned bytes, so every scheduler pass exited before
// enqueueing. Reconcile must run against the real store and queue a due chain
// whether or not it has a watermark row.
func TestReconcileProjectionJobsEnqueuesDueChains(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	insertHead := func(id string, seq uint64) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_event_heads (computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,reducer_version,credential_revocation_epoch,created_at,updated_at) VALUES (?,?,?,?,?,?,?,NULL,1,0,?,?)`, id, seq, strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("b", 64), time.Now(), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	insertHead("computer-due-nowm", 3000)
	insertHead("computer-due-wm", 9000)
	insertHead("computer-fresh", 100)
	if err := s.RecordReplayWatermark(ctx, "computer-due-wm", 6000, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileProjectionJobs(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	for _, id := range []string{"computer-due-nowm", "computer-due-wm"} {
		j, err := s.ProjectionJob(ctx, id)
		if err != nil || j.Status != "queued" || j.Reason != "cadence" {
			t.Fatalf("%s not queued by cadence: %+v %v", id, j, err)
		}
	}
	if _, err := s.ProjectionJob(ctx, "computer-fresh"); err == nil {
		t.Fatal("chain below cadence threshold was queued")
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

// One computer that cannot be enqueued must not stop cadence for the fleet.
func TestReconcileProjectionJobsContinuesPastPerComputerError(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	insertProjectionTestHead(t, s, "computer-bad/../x", 9000)
	insertProjectionTestHead(t, s, "computer-good", 9000)
	err := s.ReconcileProjectionJobs(ctx)
	if err == nil || !strings.Contains(err.Error(), "computer-bad") {
		t.Fatalf("reconcile error = %v, want the failing computer named", err)
	}
	j, jerr := s.ProjectionJob(ctx, "computer-good")
	if jerr != nil || j.Status != "queued" {
		t.Fatalf("healthy computer not queued after a sibling error: %+v %v", j, jerr)
	}
}
