package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Open and OpenTextureWorkspace armed the vocabulary write fence with
// `if rep, err := loadVocabMigrationReport(); err == nil && rep != nil`, so a
// sidecar that exists but does not decode (err != nil) left the fence off and
// every write passed (bunch 157, Devin finding 1). Only a missing sidecar
// means "never cut over". The decision is to open armed and log, never to
// refuse the open: a store that cannot open cannot reach the health listener
// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md,
// section "B. A corrupt vocabulary sidecar must not leave the fence off").
//
// Ways it can fail, written before the code:
//
//  1. A corrupt sidecar opens with the fence disarmed.
//  2. A corrupt sidecar makes Open fail (a stranded boot).
//  3. The armed case is also armed for a missing sidecar (a pre-cutover store
//     refusing its own V1 writes).
//  4. The corrupt-sidecar fence is lost across Reopen.

var unknownRoleCarrier = []objectgraph.Object{{ObjectKind: objectgraph.ObjectKind("choir.agent"), Metadata: []byte(`{"role":"boss"}`), Body: []byte(`{}`)}}

func writeSidecar(t *testing.T, storePath string, raw []byte) {
	t.Helper()
	dir := TextureWorkspacePath(storePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, vocabReportFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenWithACorruptVocabularySidecarArmsTheFenceAndStillOpens(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Control: a store with no sidecar is pre-cutover and its fence is off.
	if first.vocabCutover.Load() {
		t.Fatal("control: a store with no sidecar opened with the fence armed")
	}
	if err := first.ogStore.PreflightWrite(ctx, unknownRoleCarrier, nil); err != nil {
		t.Fatalf("control: a pre-cutover store refused a write: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	writeSidecar(t, path, []byte(`{"counts": {"truncated`))
	s, err := Open(path)
	if err != nil {
		t.Fatalf("a corrupt sidecar must not refuse the open (it would strand the boot): %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if !s.vocabCutover.Load() {
		t.Fatal("a corrupt sidecar left the vocabulary write fence disarmed")
	}
	if err := s.ogStore.PreflightWrite(ctx, unknownRoleCarrier, nil); err == nil {
		t.Fatal("a corrupt sidecar: the write guard accepted an unknown role")
	}

	// The fence holds across a Reopen onto the same corrupt sidecar.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatalf("Reopen with a corrupt sidecar: %v", err)
	}
	if err := s.ogStore.PreflightWrite(ctx, unknownRoleCarrier, nil); err == nil {
		t.Fatal("after Reopen a corrupt sidecar left the fence disarmed")
	}
}

func TestOpenTextureWorkspaceWithACorruptVocabularySidecarArmsTheFence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "texture.db")
	first, err := OpenTextureWorkspace(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.vocabCutover.Load() {
		t.Fatal("control: a workspace with no sidecar opened with the fence armed")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	writeSidecar(t, path, []byte("not json"))
	s, err := OpenTextureWorkspace(path)
	if err != nil {
		t.Fatalf("a corrupt sidecar must not refuse the open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if !s.vocabCutover.Load() {
		t.Fatal("a corrupt sidecar left the texture workspace's fence disarmed")
	}
	if err := s.ogStore.PreflightWrite(ctx, unknownRoleCarrier, nil); err == nil {
		t.Fatal("a corrupt sidecar: the texture workspace's write guard accepted an unknown role")
	}
}
