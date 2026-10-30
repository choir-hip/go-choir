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
