package platform

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// docs/problems/paper-never-checkpointed-2026-10-27.md, slice S1: a seed base
// refused deterministically twice is quarantined; job enqueue skips it and
// falls back to the next older base, else to no seed.
func TestRefusedSeedBaseIsQuarantinedAfterTwoRefusalsAndFallsBack(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	const id = "computer-quarantine"
	baseA, baseB := strings.Repeat("a1", 32), strings.Repeat("b2", 32)
	insertProjectionTestHead(t, s, id, 9000)
	for _, q := range []string{
		`INSERT INTO computer_replay_watermarks (computer_id,watermark_sequence,base_ref,updated_at) VALUES ('` + id + `',5000,'` + baseA + `',NOW())`,
		`INSERT INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES ('` + id + `','` + baseB + `',3000,NOW())`,
		`INSERT INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES ('` + id + `','` + baseA + `',5000,NOW())`,
	} {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	refused := "projection base refused: installed base does not open: database not found: texture"
	block := func() {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='blocked',error_text=?,failures=failures+1,updated_at=? WHERE computer_id=?`, refused, time.Now().UTC().Add(-48*time.Hour), id); err != nil {
			t.Fatal(err)
		}
	}
	j, err := s.EnqueueProjectionJob(ctx, id, "cadence", false)
	if err != nil || j.SeedBaseRef != baseA {
		t.Fatalf("first job = %+v %v, want seed A", j, err)
	}
	block()
	if q, err := s.RecordBaseRefusal(ctx, id, baseA, refused); err != nil || q {
		t.Fatalf("one refusal quarantined = %v %v", q, err)
	}
	// The first refusal is confirmed by one more attempt on the same seed.
	j, err = s.EnqueueProjectionJob(ctx, id, "cadence", false)
	if err != nil || j.Status != "queued" || j.SeedBaseRef != baseA || j.Generation != 2 {
		t.Fatalf("second attempt = %+v %v, want queued on seed A", j, err)
	}
	block()
	if q, err := s.RecordBaseRefusal(ctx, id, baseA, refused); err != nil || !q {
		t.Fatalf("two refusals quarantined = %v %v", q, err)
	}
	j, err = s.EnqueueProjectionJob(ctx, id, "cadence", false)
	if err != nil || j.Status != "queued" || j.SeedBaseRef != baseB || j.WatermarkSequence != 3000 || j.Generation != 3 {
		t.Fatalf("after quarantine = %+v %v, want older base B at 3000", j, err)
	}
	// A checkpoint verification job skips the quarantined base too.
	v, err := s.enqueueProjectionJob(ctx, id, "verify", false, &projectionTarget{sequence: 6000, head: strings.Repeat("c", 64)})
	if err != nil || v.SeedBaseRef != baseB {
		t.Fatalf("verification seed = %+v %v, want B", v, err)
	}
	block()
	s.RecordBaseRefusal(ctx, id, baseB, refused)
	s.RecordBaseRefusal(ctx, id, baseB, refused)
	j, err = s.EnqueueProjectionJob(ctx, id, "cadence", false)
	if err != nil || j.Status != "queued" || j.SeedBaseRef != "" || j.WatermarkSequence != 0 {
		t.Fatalf("no base left = %+v %v, want genesis (no seed)", j, err)
	}
}

// A block of another kind has no recorded base refusal and still waits for
// changed inputs.
func TestBlockedRowWithoutRecordedRefusalsIsNotReadmittedByQuarantine(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	insertProjectionTestHead(t, s, "computer-esc", 9000)
	if _, err := s.EnqueueProjectionJob(ctx, "computer-esc", "cadence", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='blocked',error_text='checkpoint escrow changed since job admission',failures=1,updated_at=?`, time.Now().UTC().Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	j, err := s.EnqueueProjectionJob(ctx, "computer-esc", "cadence", false)
	if err != nil || j.Status != "blocked" || j.Generation != 1 {
		t.Fatalf("unrelated block re-admitted: %+v %v", j, err)
	}
}

// A blocked row is re-admitted once when the worker's build identity changes,
// and not again while the build stays the same.
func TestBlockedRowIsReadmittedWhenWorkerBuildChanges(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	insertProjectionTestHead(t, s, "computer-build", 9000)
	if _, err := s.EnqueueProjectionJob(ctx, "computer-build", "cadence", false); err != nil {
		t.Fatal(err)
	}
	j, err := s.ClaimProjectionJob(ctx)
	if err != nil || j == nil {
		t.Fatalf("claim = %+v %v", j, err)
	}
	if err := s.FinishProjectionJob(ctx, *j, "blocked", "checkpoint escrow changed since job admission", 0, ""); err != nil {
		t.Fatal(err)
	}
	got, err := s.enqueueProjectionJobAtBuild(ctx, "computer-build", "cadence", false, nil, buildinfo.Commit)
	if err != nil || got.Status != "blocked" || got.Generation != 1 {
		t.Fatalf("same build re-admitted: %+v %v", got, err)
	}
	got, err = s.enqueueProjectionJobAtBuild(ctx, "computer-build", "cadence", false, nil, "a-newer-build")
	if err != nil || got.Status != "queued" || got.Generation != 2 {
		t.Fatalf("new build not re-admitted: %+v %v", got, err)
	}
}

// Reconcile logs each blocked row at most once an hour.
func TestReconcileLogsBlockedRowsAtMostOncePerHour(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	insertProjectionTestHead(t, s, "computer-loud", 9000)
	if _, err := s.EnqueueProjectionJob(ctx, "computer-loud", "cadence", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='blocked',error_text='boom refusal',failures=3,build_commit=?`, buildinfo.Commit); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	for i := 0; i < 3; i++ {
		if err := s.ReconcileProjectionJobs(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(buf.String(), "computer-loud"); n != 1 || !strings.Contains(buf.String(), "boom refusal") || !strings.Contains(buf.String(), "failures=3") {
		t.Fatalf("log lines = %d:\n%s", n, buf.String())
	}
	s.blockedLogMu.Lock()
	s.blockedLogged["computer-loud"] = time.Now().Add(-61 * time.Minute)
	s.blockedLogMu.Unlock()
	s.ReconcileProjectionJobs(ctx)
	if n := strings.Count(buf.String(), "computer-loud"); n != 2 {
		t.Fatalf("log lines after an hour = %d, want 2", n)
	}
}
