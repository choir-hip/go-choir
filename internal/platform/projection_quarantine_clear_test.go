package platform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Refusals recorded by another worker build do not count: a new build may open
// a base the old one could not.
func TestRefusalsFromAnotherBuildDoNotCount(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	base := strings.Repeat("c3", 32)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_projection_base_refusals (computer_id,base_ref,refusals,last_error,updated_at,build_commit) VALUES ('computer-old',?,2,'x',NOW(),'an-older-build')`, base); err != nil {
		t.Fatal(err)
	}
	if q, err := baseQuarantined(ctx, s.db, "computer-old", base); err != nil || q {
		t.Fatalf("old-build refusals quarantined = %v %v", q, err)
	}
	// A new refusal restarts the count under this build.
	if q, err := s.RecordBaseRefusal(ctx, "computer-old", base, "y"); err != nil || q {
		t.Fatalf("first refusal under a new build quarantined = %v %v", q, err)
	}
	if q, err := s.RecordBaseRefusal(ctx, "computer-old", base, "y"); err != nil || !q {
		t.Fatalf("second refusal under this build quarantined = %v %v", q, err)
	}
}

// A successful publish clears the computer's refusal rows, and a quarantined
// advertised base may be superseded by a result seeded from a fallback.
func TestPublishSupersedesQuarantinedAdvertisedBaseAndClearsRefusals(t *testing.T) {
	s, root := openTestPlatformStore(t)
	svc := NewService(s, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	ctx := context.Background()
	const id = "computer-publish"
	baseA, baseB, baseNew := strings.Repeat("a1", 32), strings.Repeat("b2", 32), strings.Repeat("d4", 32)
	insertProjectionTestHead(t, s, id, 9000)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_replay_watermarks (computer_id,watermark_sequence,base_ref,updated_at) VALUES (?,5000,?,NOW())`, id, baseA); err != nil {
		t.Fatal(err)
	}
	s.RecordBaseRefusal(ctx, id, baseA, "refused")
	s.RecordBaseRefusal(ctx, id, baseA, "refused")
	job := ProjectionJob{ComputerID: id, Generation: 1, LeaseToken: "tok"}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_projection_jobs (`+projectionJobColumns+`) VALUES (?,1,'running','cadence',9000,?,3000,?,'','','',false,0,'tok',NOW(),NOW())`, id, strings.Repeat("a", 64), baseB); err != nil {
		t.Fatal(err)
	}
	if err := svc.PublishCheckpointResult(ctx, job, 9000, baseNew, baseB); err != nil {
		t.Fatalf("publish over a quarantined advertised base: %v", err)
	}
	wm, ref, err := s.ReplayWatermark(ctx, id)
	if err != nil || wm != 9000 || ref != baseNew {
		t.Fatalf("watermark = %d %q %v", wm, ref, err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_projection_base_refusals WHERE computer_id=?`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("refusal rows after publish = %d %v, want 0", n, err)
	}
	// An unquarantined advertised base that differs from the seed still refuses.
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='running' WHERE computer_id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.PublishCheckpointResult(ctx, job, 9500, strings.Repeat("e5", 32), baseB); err == nil {
		t.Fatal("publish with a changed, unquarantined advertisement must refuse")
	}
}
