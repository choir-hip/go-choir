//go:build linux

package capsule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression for docs/problems/capsule-subject-artifact-ephemeral-2026-09-28.md:
// capsule-subject:/receipt refs are committed to the durable store, but their
// bytes lived on tmpfs stateDir — every guest hibernation destroyed them.
// NewExecutorWithArtifacts splits durable artifacts (subjects/, receipts/,
// the .candidate-* staging dir that must share the rename filesystem) onto
// persistent storage while capsule scratch stays ephemeral.
func TestArtifactDirSeparatesDurableFromEphemeralState(t *testing.T) {
	stateDir := t.TempDir()
	artifactDir := t.TempDir()
	e := NewExecutorWithArtifacts(stateDir, artifactDir, t.TempDir(), "", filepath.Join(t.TempDir(), "missing-broker"), 1<<20)

	// Receipts are durable store-adjacent bytes: they must land under
	// artifactDir, never under stateDir.
	ref := "receipt:sha256:" + strings.Repeat("ab", 32)
	if err := e.persistReceiptArtifact("fate", ref, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("persist receipt: %v", err)
	}
	name := receiptArtifactName(ref)
	if _, err := os.Stat(filepath.Join(artifactDir, "receipts", "fate", name)); err != nil {
		t.Fatalf("receipt not on artifact volume: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "receipts", "fate", name)); err == nil {
		t.Fatal("receipt landed on ephemeral state volume")
	}

	// Subject paths resolve under artifactDir (the production env points it
	// at /mnt/persistent).
	root, _, err := e.subjectArtifactPath("capsule-subject:sha256:" + strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(root, artifactDir) {
		t.Fatalf("subject root %q not under artifactDir %q", root, artifactDir)
	}
}

// A subject ref whose bytes are gone (realization lost to tmpfs, or a
// corrupted tree) is permanent — retrying forever wedges the bound operation.
// The sentinel lets reconcile fail the operation instead.
func TestPreflightMissingSubjectCarriesUnavailableSentinel(t *testing.T) {
	e := NewExecutorWithArtifacts(t.TempDir(), t.TempDir(), t.TempDir(), "", filepath.Join(t.TempDir(), "missing-broker"), 1<<20)
	_, err := e.PreflightSourceSnapshot(context.Background(), "capsule-subject:sha256:"+strings.Repeat("ef", 32))
	if !errors.Is(err, ErrSubjectArtifactUnavailable) {
		t.Fatalf("missing subject err = %v, want ErrSubjectArtifactUnavailable", err)
	}
}
