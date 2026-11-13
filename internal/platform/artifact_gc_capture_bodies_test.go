package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Capture bodies under the reachability collector (station S7). A body is
// live when any choir.web_capture row in Store B names it, tombstoned or not.
//
// Failure list, each a way this could go wrong:
//  1. a body no row names, past grace, survives an active pass;
//  2. a dry run deletes it, or fails to report it;
//  3. a body named by content_blob_id, extracted_text_blob_id or
//     blocks_blob_id of a row is deleted, a tombstoned row's included;
//  4. a body written inside the grace is deleted;
//  5. a pass is not bounded by max deletes or max bytes;
//  6. with no capture rows at all the pass deletes bodies (the wrong-store
//     signature);
//  7. a row whose metadata does not parse is read as naming nothing;
//  8. a namespace left out of the active list is deleted from;
//  9. a writer that reuses an old body does not refresh it, so a pass that
//     read the live set before the row was written deletes it;
// 10. a file that is not a body (a temp file, a stray name) is deleted;
// 11. the default active list does not carry the namespace.

func captureBodyHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type captureGCFixture struct {
	t     *testing.T
	store *Store
	root  string
	svc   *Service
}

func newCaptureGCFixture(t *testing.T) *captureGCFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	return &captureGCFixture{t: t, store: store, root: artifactsRoot, svc: NewService(store, artifactsRoot, "")}
}

// body writes a body file under capture-bodies/<2 hex>/<hash> with an mtime
// age in the past, and returns its hash and path.
func (f *captureGCFixture) body(content string, age time.Duration) (string, string) {
	f.t.Helper()
	hash := captureBodyHash(content)
	dir := filepath.Join(CaptureBodiesRoot(f.root), hash[:2])
	if err := os.MkdirAll(dir, 0o750); err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(dir, hash)
	if err := os.WriteFile(p, []byte(content), 0o640); err != nil {
		f.t.Fatal(err)
	}
	mt := time.Now().Add(-age)
	if err := os.Chtimes(p, mt, mt); err != nil {
		f.t.Fatal(err)
	}
	return hash, p
}

func (f *captureGCFixture) row(id, metadata string, tombstone bool) {
	f.t.Helper()
	if _, err := f.store.corpus().ExecContext(context.Background(),
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone)
		 VALUES (?, 'choir.web_capture', 'o', 'c', 'v', 'h', '', '', 0, ?, NOW(), NOW(), ?)`, id, metadata, tombstone); err != nil {
		f.t.Fatal(err)
	}
}

func captureMeta(content, text, blocks string) string {
	return `{"schema_version":"choir.web_capture.v1","content_blob_id":"sha256:` + content + `","extracted_text_blob_id":"sha256:` + text + `","blocks_blob_id":"sha256:` + blocks + `"}`
}


func (f *captureGCFixture) run(cfg ArtifactGCConfig) ArtifactGCReport {
	f.t.Helper()
	if cfg.Grace == 0 {
		cfg.Grace = time.Hour
	}
	rep, err := f.svc.RunArtifactGC(context.Background(), cfg)
	if err != nil {
		f.t.Fatal(err)
	}
	return rep
}

func TestCaptureBodiesGCReferencedSurviveUnreferencedGoes(t *testing.T) {
	f := newCaptureGCFixture(t)
	old := 48 * time.Hour
	rawH, rawP := f.body("raw", old)
	txtH, txtP := f.body("text", old)
	blkH, blkP := f.body("blocks", old)
	f.row("cap-live", captureMeta(rawH, txtH, blkH), false)
	tH, tP := f.body("tomb raw", old)
	f.row("cap-tomb", captureMeta(tH, tH, ""), true)
	_, deadP := f.body("nobody names me", old)
	_, freshP := f.body("written during the pass", 0)

	dry := f.run(ArtifactGCConfig{Mode: "dry-run"})
	rep := dry.Namespaces["capture-bodies"]
	if rep.Scanned != 6 || rep.Live != 4 || rep.Eligible != 1 || rep.InGrace != 1 || rep.Deleted != 1 {
		t.Fatalf("dry-run accounting: %+v", rep)
	}
	if !exists(deadP) {
		t.Fatal("dry run deleted a body")
	}

	act := f.run(ArtifactGCConfig{Mode: "active"})
	if got := act.Namespaces["capture-bodies"]; got.Deleted != 1 || got.Mode != ArtifactGCModeActive {
		t.Fatalf("active: %+v", got)
	}
	if exists(deadP) {
		t.Fatal("unreferenced old body kept")
	}
	for name, p := range map[string]string{"raw": rawP, "text": txtP, "blocks": blkP, "tombstoned": tP, "in grace": freshP} {
		if !exists(p) {
			t.Fatalf("%s body deleted", name)
		}
	}
}

func TestCaptureBodiesGCPassIsBounded(t *testing.T) {
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	var dead []string
	for _, c := range []string{"d1", "d2", "d3", "d4", "d5"} {
		_, p := f.body(c, 48*time.Hour)
		dead = append(dead, p)
	}
	rep := f.run(ArtifactGCConfig{Mode: "active", MaxDeletes: 2})
	ns := rep.Namespaces["capture-bodies"]
	if ns.Deleted != 2 || ns.DeferredByCap != 3 || ns.Eligible != 5 {
		t.Fatalf("max deletes: %+v", ns)
	}
	left := 0
	for _, p := range dead {
		if exists(p) {
			left++
		}
	}
	if left != 3 {
		t.Fatalf("%d bodies left, want 3", left)
	}
	rep = f.run(ArtifactGCConfig{Mode: "active", MaxBytes: 1})
	if ns := rep.Namespaces["capture-bodies"]; ns.Deleted != 0 || ns.DeferredByCap != 3 {
		t.Fatalf("max bytes: %+v", ns)
	}
}

func TestCaptureBodiesGCNoRowsDeletesNothing(t *testing.T) {
	f := newCaptureGCFixture(t)
	_, p := f.body("orphan", 48*time.Hour)
	rep := f.run(ArtifactGCConfig{Mode: "active"})
	if !exists(p) {
		t.Fatal("deleted with an empty live set")
	}
	if rep.Namespaces["capture-bodies"].Deleted != 0 || len(rep.Warnings) == 0 {
		t.Fatalf("report: %+v", rep)
	}
}

func TestCaptureBodiesGCUnparseableRowAbortsPass(t *testing.T) {
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	f.row("cap-bad", `{not json`, false)
	_, p := f.body("dead", 48*time.Hour)
	if _, err := f.svc.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err == nil {
		t.Fatal("a row that does not parse must fail the pass")
	}
	if !exists(p) {
		t.Fatal("deleted on an unprovable live set")
	}
}

func TestCaptureBodiesGCOnlyActiveNamespacesDelete(t *testing.T) {
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	_, p := f.body("dead", 48*time.Hour)
	rep := f.run(ArtifactGCConfig{Mode: "active", ActiveNamespaces: []string{"og"}})
	if !exists(p) || rep.Namespaces["capture-bodies"].Eligible != 1 || rep.Namespaces["capture-bodies"].Mode != ArtifactGCModeDryRun {
		t.Fatalf("report-only namespace: %+v", rep.Namespaces["capture-bodies"])
	}
}

func TestCaptureBodiesGCLeavesNonBodies(t *testing.T) {
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	dir := filepath.Join(CaptureBodiesRoot(f.root), h[:2])
	old := time.Now().Add(-48 * time.Hour)
	var keep []string
	for _, n := range []string{".body-123", "notes.txt", strings.Repeat("A", 64)} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("x"), 0o640); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, old, old)
		keep = append(keep, p)
	}
	f.run(ArtifactGCConfig{Mode: "active"})
	for _, p := range keep {
		if !exists(p) {
			t.Fatalf("%s deleted", p)
		}
	}
}

// A body a new capture reuses must look new: the pass reads the live set
// before the writer's row lands, and only the body's mtime can tell it.
func TestCaptureBodyPutRefreshesExistingBody(t *testing.T) {
	f := newCaptureGCFixture(t)
	_, p := f.body("reused", 48*time.Hour)
	store := &CaptureBodyStore{root: CaptureBodiesRoot(f.root)}
	if _, err := store.PutBody([]byte("reused")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("reused body keeps mtime %v", info.ModTime())
	}
}

func TestDefaultArtifactGCActiveNamespacesCarriesCaptureBodies(t *testing.T) {
	for _, ns := range DefaultArtifactGCActiveNamespaces {
		if ns == "capture-bodies" {
			return
		}
	}
	t.Fatal("capture-bodies is not in the default active namespaces")
}
