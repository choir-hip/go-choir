package platform

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A base advertised past a job's frozen target supersedes the job (S2,
// docs/problems/paper-never-checkpointed-2026-10-27.md: the paper's captured
// base lands while its cadence job replays from 34660). Supersession is by
// seed and watermark, not by target (review 364). Ways it could fail:
//  1. the advertisement is refused or waits while the computer's job stands;
//  2. the stale job stays on the old seed;
//  3. a job the new base does not cover, a checkpoint verification job, or a
//     genesis repair is superseded.
func TestAdvertisedBaseSupersedesAStaleJob(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	id := "computer-supersede"
	insertTestHead(t, s, id, 5000)
	if err := s.RecordReplayWatermark(ctx, id, 100, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	insertTestJob(t, s, id, "cadence", false, 5000, 100, strings.Repeat("c", 64))
	captured := strings.Repeat("d", 64)
	if err := s.RecordReplayWatermark(ctx, id, 6000, captured); err != nil {
		t.Fatalf("advertise over a standing job: %v", err)
	}
	if wm, ref, err := s.ReplayWatermark(ctx, id); err != nil || wm != 6000 || ref != captured {
		t.Fatalf("watermark = %d %s %v", wm, ref, err)
	}
	if got := testJobStatus(t, s, id); got != "superseded" {
		t.Fatalf("job status = %q, want superseded", got)
	}
}

// Review 364 (DeepSeek, MEDIUM): a job on an older seed is superseded even
// when its target is past the new base (the next one starts from the new base
// and replays less).
func TestAdvertisedBaseSupersedesAnOlderSeedPastItsTarget(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	id := "computer-supersede-past"
	insertTestHead(t, s, id, 7000)
	if err := s.RecordReplayWatermark(ctx, id, 100, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	insertTestJob(t, s, id, "cadence", false, 7000, 100, strings.Repeat("c", 64))
	if err := s.RecordReplayWatermark(ctx, id, 6000, strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if got := testJobStatus(t, s, id); got != "superseded" {
		t.Fatalf("job status = %q, want superseded", got)
	}
}

func TestAdvertisedBaseLeavesJobsItDoesNotCover(t *testing.T) {
	ctx := context.Background()
	advertised := strings.Repeat("d", 64)
	for _, tc := range []struct {
		name    string
		target  uint64
		reason  string
		genesis bool
		seed    string
	}{
		{"already on the advertised seed", 7000, "cadence", false, advertised},
		{"checkpoint verification", 5000, checkpointVerifyReason + strings.Repeat("e", 64), false, ""},
		{"genesis repair", 5000, "repair", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := openTestPlatformStore(t)
			id := "computer-keep"
			insertTestHead(t, s, id, tc.target)
			insertTestJob(t, s, id, tc.reason, tc.genesis, tc.target, 100, tc.seed)
			if err := s.RecordReplayWatermark(ctx, id, 6000, strings.Repeat("d", 64)); err != nil {
				t.Fatal(err)
			}
			if got := testJobStatus(t, s, id); got != "queued" {
				t.Fatalf("job superseded: status = %q", got)
			}
		})
	}
}

// insertTestJob writes a legacy queued job row directly: nothing enqueues
// jobs any more, but rows an earlier build wrote still pin bases until the
// advertisement supersedes them.
func insertTestJob(t *testing.T, s *Store, id, reason string, genesis bool, target, watermark uint64, seed string) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(context.Background(), `INSERT INTO computer_projection_jobs (`+projectionJobColumns+`) VALUES (?,1,'queued',?,?,'h',?,?,'k','','',?,0,'',?,?)`, id, reason, target, watermark, seed, genesis, now, now); err != nil {
		t.Fatal(err)
	}
}

func testJobStatus(t *testing.T, s *Store, id string) string {
	t.Helper()
	var status string
	if err := s.db.QueryRowContext(context.Background(), `SELECT status FROM computer_projection_jobs WHERE computer_id=?`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func insertTestHead(t *testing.T, s *Store, id string, seq uint64) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `INSERT INTO computer_event_heads (computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,reducer_version,credential_revocation_epoch,created_at,updated_at) VALUES (?,?,?,?,?,?,?,NULL,1,0,?,?)`, id, seq, strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("b", 64), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
}
