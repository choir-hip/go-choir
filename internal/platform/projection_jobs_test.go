package platform

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

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
