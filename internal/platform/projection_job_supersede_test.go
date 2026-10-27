package platform

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A base advertised past a job's frozen target supersedes the job (S2,
// docs/problems/paper-never-checkpointed-2026-10-27.md: the paper's captured
// base lands while its cadence job replays from 34660). Ways it could fail:
//  1. the advertisement is refused or waits while the computer's job runs;
//  2. the stale job keeps its lease (renewal succeeds) and replays for
//     another deadline, or is reclaimed as `retry`, on the old seed;
//  3. the next reconcile does not re-admit the row on the new base;
//  4. a job the new base does not cover (target past it), a checkpoint
//     verification job, or a genesis repair is superseded.
func TestAdvertisedBaseSupersedesAStaleRunningJob(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	id := "computer-supersede"
	insertTestHead(t, s, id, 5000)
	if err := s.RecordReplayWatermark(ctx, id, 100, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnqueueProjectionJob(ctx, id, "cadence", false); err != nil {
		t.Fatal(err)
	}
	lease, err := s.ClaimProjectionJob(ctx)
	if err != nil || lease == nil || lease.TargetSequence != 5000 {
		t.Fatalf("claim: %+v %v", lease, err)
	}
	captured := strings.Repeat("d", 64)
	if err := s.RecordReplayWatermark(ctx, id, 6000, captured); err != nil {
		t.Fatalf("advertise over a running job: %v", err)
	}
	if wm, ref, err := s.ReplayWatermark(ctx, id); err != nil || wm != 6000 || ref != captured {
		t.Fatalf("watermark = %d %s %v", wm, ref, err)
	}
	if err := s.RenewProjectionJob(ctx, *lease); err == nil {
		t.Fatal("the superseded job's lease still renews")
	}
	if got, _ := s.ProjectionJob(ctx, id); got.Status != "superseded" {
		t.Fatalf("job status = %q, want superseded", got.Status)
	}
	if again, err := s.ClaimProjectionJob(ctx); err != nil || again != nil {
		t.Fatalf("superseded job reclaimed: %+v %v", again, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_event_heads SET sequence=? WHERE computer_id=?`, 9000, id); err != nil {
		t.Fatal(err)
	}
	next, err := s.EnqueueProjectionJob(ctx, id, "cadence", false)
	if err != nil || next.Status != "queued" || next.SeedBaseRef != captured || next.WatermarkSequence != 6000 || next.Generation != lease.Generation+1 {
		t.Fatalf("re-admitted job = %+v %v, want queued on the advertised base", next, err)
	}
}

func TestAdvertisedBaseLeavesJobsItDoesNotCover(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		target  uint64
		reason  string
		genesis bool
	}{
		{"target past the base", 7000, "cadence", false},
		{"checkpoint verification", 5000, checkpointVerifyReason + strings.Repeat("e", 64), false},
		{"genesis repair", 5000, "repair", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := openTestPlatformStore(t)
			id := "computer-keep"
			insertTestHead(t, s, id, tc.target)
			if _, err := s.EnqueueProjectionJob(ctx, id, "cadence", tc.genesis); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET reason=? WHERE computer_id=?`, tc.reason, id); err != nil {
				t.Fatal(err)
			}
			lease, err := s.ClaimProjectionJob(ctx)
			if err != nil || lease == nil {
				t.Fatalf("claim: %v", err)
			}
			if err := s.RecordReplayWatermark(ctx, id, 6000, strings.Repeat("d", 64)); err != nil {
				t.Fatal(err)
			}
			if err := s.RenewProjectionJob(ctx, *lease); err != nil {
				t.Fatalf("job superseded: %v", err)
			}
		})
	}
}

func insertTestHead(t *testing.T, s *Store, id string, seq uint64) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `INSERT INTO computer_event_heads (computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,reducer_version,credential_revocation_epoch,created_at,updated_at) VALUES (?,?,?,?,?,?,?,NULL,1,0,?,?)`, id, seq, strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("b", 64), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
}
