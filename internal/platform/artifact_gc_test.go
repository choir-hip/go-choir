package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	embedded "github.com/dolthub/driver/v2"
)

// The artifact GC must only remove entries that are both unreachable and past
// grace. A live ref, an in-grace file, and a dry-run all must leave the file.
func TestRunArtifactGCDryRunAndActive(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)

	write := func(ns, name string) string {
		dir := filepath.Join(artifactsRoot, "sha256", ns)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		return p
	}
	digestOf := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}

	// file-cas-roots: live = manifest_ref basename rows in computer_file_roots.
	liveRoot := digestOf("live-root")
	deadRoot := digestOf("dead-root")
	manifestRef := "sha256/file-cas-roots/computer-a/" + liveRoot + ".json"
	if err := store.RecordFileRoot(ctx, "computer-a", liveRoot, manifestRef, 1); err != nil {
		t.Fatal(err)
	}
	liveRootPath := write("file-cas-roots/computer-a", liveRoot+".json")
	deadRootPath := write("file-cas-roots/computer-a", deadRoot+".json")

	// projection-base: live = computer_replay_watermarks.base_ref (+ sidecar).
	liveBase := digestOf("live-base")
	deadBase := digestOf("dead-base")
	if err := store.RecordReplayWatermark(ctx, "computer-b", 5, liveBase); err != nil {
		t.Fatal(err)
	}
	liveBlobPath := write("projection-base", liveBase)
	liveSidecarPath := write("projection-base", liveBase+".descriptor.json")
	deadBlobPath := write("projection-base", deadBase)
	deadSidecarPath := write("projection-base", deadBase+".descriptor.json")

	// Dry-run: reports unreachable but deletes nothing.
	dry, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "dry-run", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Namespaces["file-cas-roots"].Deleted == 0 || dry.Namespaces["projection-base"].Deleted == 0 {
		t.Fatalf("dry-run reported no candidates: %+v", dry.Namespaces)
	}
	for _, p := range []string{liveRootPath, deadRootPath, liveBlobPath, liveSidecarPath, deadBlobPath, deadSidecarPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("dry-run deleted %s: %v", p, err)
		}
	}

	// Active: unreachable + past grace deleted; live and sidecars kept.
	active, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{liveRootPath, liveBlobPath, liveSidecarPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("live artifact removed %s: %v", p, err)
		}
	}
	for _, p := range []string{deadRootPath, deadBlobPath, deadSidecarPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("dead artifact kept %s: %v", p, err)
		}
	}
	if active.Deleted == 0 {
		t.Fatal("active sweep deleted nothing")
	}
}

// An in-flight write newer than the grace cutoff is never collected even when
// unreachable — pin-before-CAS safety.
func TestRunArtifactGCKeepsInGraceUnreachable(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	ctx := context.Background()
	dir := filepath.Join(artifactsRoot, "sha256", "projection-base")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	inFlight := filepath.Join(dir, "00aa11bb22cc33dd44ee55ff66778899aabbccddeeff00112233445566778899")
	if err := os.WriteFile(inFlight, []byte("in flight"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inFlight); err != nil {
		t.Fatalf("in-flight write collected: %v", err)
	}
	if report.Namespaces["projection-base"].InGrace == 0 {
		t.Fatalf("in-grace not counted: %+v", report.Namespaces)
	}
}

// og body_ref liveness must be computed on Store B (corpus()), not Store A.
// 2026-10-05: the live-set query read s.store.db; in the split topology every
// externalized corpus body resolved unreachable and the active sweep deleted
// ~45.8k live bodies. Regression: a decoy body_ref on Store A plus the live
// ref on Store B — the live file must survive an active sweep.
func TestRunArtifactGCOGLiveSetReadsCorpus(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)

	// Split topology: second embedded dolt as Store B with its own og_objects.
	corpusRoot := filepath.Join(root, "corpus-dolt")
	if err := os.MkdirAll(corpusRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	// Root connector (no database) creates the corpus db, then a second
	// connector binds to it — mirrors openTestPlatformStore.
	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", corpusRoot)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse corpus root dsn: %v", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new corpus root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec(`CREATE DATABASE IF NOT EXISTS corpus`); err != nil {
		t.Fatalf("create corpus db: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	corpusDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=corpus&multistatements=true", corpusRoot)
	corpusCfg, err := embedded.ParseDSN(corpusDSN)
	if err != nil {
		t.Fatalf("parse corpus dsn: %v", err)
	}
	corpusConnector, err := embedded.NewConnector(corpusCfg)
	if err != nil {
		t.Fatalf("new corpus connector: %v", err)
	}
	corpusDB := sql.OpenDB(corpusConnector)
	store.corpusDB = corpusDB
	if _, err := corpusDB.ExecContext(ctx, corpusSchemaDDL); err != nil {
		t.Fatalf("bootstrap corpus schema: %v", err)
	}
	t.Cleanup(func() {
		_ = corpusDB.Close()
		_ = corpusConnector.Close()
	})

	liveDigest := sha256.Sum256([]byte("live og body"))
	deadDigest := sha256.Sum256([]byte("dead og body"))
	liveName := hex.EncodeToString(liveDigest[:]) + ".bin"
	deadName := hex.EncodeToString(deadDigest[:]) + ".bin"

	// Store A's og_objects is a decoy: it references ONLY the dead file. If the
	// live set still read s.store.db, the live file would be collected.
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at)
		 VALUES ('decoy', 'test', 'o', 'c', 'v', 'h', '', ?, 10, '{}', NOW(), NOW())`,
		"sha256/og/"+deadName); err != nil {
		t.Fatalf("decoy store A row: %v", err)
	}
	// Store B carries the real live ref.
	if _, err := corpusDB.ExecContext(ctx,
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at)
		 VALUES ('real', 'test', 'o', 'c', 'v', 'h', '', ?, 10, '{}', NOW(), NOW())`,
		"sha256/og/"+liveName); err != nil {
		t.Fatalf("store B row: %v", err)
	}

	ogDir := filepath.Join(artifactsRoot, "sha256", "og")
	if err := os.MkdirAll(ogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	livePath := filepath.Join(ogDir, liveName)
	deadPath := filepath.Join(ogDir, deadName)
	for _, p := range []string{livePath, deadPath} {
		if err := os.WriteFile(p, []byte("body"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	service := NewService(store, artifactsRoot, "")
	report, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(livePath); err != nil {
		t.Fatalf("live corpus-referenced og body deleted: %v (report %+v)", err, report.Namespaces["og"])
	}
	if _, err := os.Stat(deadPath); !os.IsNotExist(err) {
		t.Fatalf("unreferenced og body kept: %v", err)
	}
}

// A live set that cannot be loaded aborts the sweep rather than deleting with
// an empty set.
func TestRunArtifactGCFailsClosedOnLiveSetError(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)

	// Point corpus() at a second embedded dolt WITHOUT og_objects — the corpus
	// schema was never bootstrapped there, so the live-set query errors.
	corpusRoot := filepath.Join(root, "corpus-dolt")
	if err := os.MkdirAll(corpusRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	corpusDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=corpus&multistatements=true", corpusRoot)
	corpusCfg, err := embedded.ParseDSN(corpusDSN)
	if err != nil {
		t.Fatalf("parse corpus dsn: %v", err)
	}
	corpusConnector, err := embedded.NewConnector(corpusCfg)
	if err != nil {
		t.Fatalf("new corpus connector: %v", err)
	}
	corpusDB := sql.OpenDB(corpusConnector)
	store.corpusDB = corpusDB
	t.Cleanup(func() {
		_ = corpusDB.Close()
		_ = corpusConnector.Close()
	})

	ogDir := filepath.Join(artifactsRoot, "sha256", "og")
	if err := os.MkdirAll(ogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	victimSum := sha256.Sum256([]byte("victim"))
	victim := filepath.Join(ogDir, hex.EncodeToString(victimSum[:])+".bin")
	if err := os.WriteFile(victim, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(victim, old, old); err != nil {
		t.Fatal(err)
	}

	service := NewService(store, artifactsRoot, "")
	report, err := service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err == nil {
		t.Fatalf("expected live-set failure, got report %+v", report.Namespaces)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("sweep proceeded with missing live set and deleted %s: %v", victim, err)
	}
}
