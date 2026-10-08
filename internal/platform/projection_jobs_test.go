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
