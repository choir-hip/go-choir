package platform

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Tests for the real deletion of a destroyed computer: the destroyed sweep
// also removes the computer's repacked segments (tape-seg/<id>/), its file-CAS
// chunks and roots (sha256/file-cas-chunks/<id>/, sha256/file-cas-roots/<id>/),
// its rows in every per-computer table (append receipts, snapshot roots, file
// roots, lifecycle, projection, ...) and its key escrow entry (the crypto-shred).
//
// Every way this can fail, written before the code, and the test that pins it:
//
//	P1  Deleting a live computer's data: a live computer's segments, chunks,
//	    roots, rows and escrow survive a sweep that destroys another computer.
//	    TestPurgeNeverTouchesALiveComputer
//	P2  Deleting a shared digest: a loose payload a live computer names
//	    survives (the SharedPayloadsKept property), and a computer whose
//	    file-CAS paths another computer's root names is not deletable at all.
//	    TestPurgeKeepsSharedDigests
//	P3  The paper or the owner computer destroyed by any request: the hard
//	    refusal list holds against a destroyed proof, no_archive, and a
//	    hand-forged manifest that carries a valid hash.
//	    TestPurgeRefusesProtectedComputers
//	P4  A partially applied apply (a file kept because it was refreshed, a cap,
//	    a context cancel): rows and escrow are NOT removed while any listed file
//	    or directory remains, and a re-apply of the same manifest finishes.
//	    TestPurgePartialApplyKeepsRowsAndEscrow
//	P5  A crash mid-delete leaving rows without files or files without rows:
//	    files go first and all rows (escrow included) go in one transaction
//	    after; a crash between them leaves files gone and rows intact (resumable
//	    by the same manifest), never rows gone with files kept.
//	    TestPurgeCrashBetweenFilesAndRows
//	P6  Escrow shredded but files kept: the escrow row is deleted only in the
//	    rows transaction, after every file is gone; and the archive never
//	    carries the escrow row, so the key is gone from every copy the sweep
//	    controls.
//	    TestPurgePartialApplyKeepsRowsAndEscrow, TestPurgeArchiveCarriesNoKey
//	P7  A manifest/apply mismatch: a row added after the dry-run rolls the whole
//	    rows transaction back (apply removes exactly the manifest's count); a
//	    file added after the dry-run is not in the manifest and survives, which
//	    keeps the directory and the rows; a tampered manifest deletes nothing.
//	    TestPurgeApplyRemovesExactlyTheManifest
//	P8  The manifest leaves something out: every file and directory under the
//	    three per-computer trees and the row count of every table is listed.
//	    TestPurgeManifestListsEverything
//	P9  The sweep reads its own aftermath wrongly: after a purge the receipts
//	    are gone, so a later dry-run must not take the missing chain for a gap
//	    (it would refuse the orphan sweep forever), and a re-apply is idempotent.
//	    TestPurgeAftermathIsReadable
//	P10 Archive and no_archive: archive stays the default and holds segments,
//	    chunks, roots and a rows dump; no_archive writes none; apply must repeat
//	    no_archive exactly as the dry-run had it.
//	    TestPurgeArchiveCarriesNoKey, TestPurgeNoArchive
//	P11 A per-computer table is added and the purge forgets it: every table with
//	    a computer_id column must be classified purged or kept.
//	    TestPurgeCoversEveryComputerTable
//	P12 A symlink or a recent write in a computer's tree: the computer is not
//	    deletable (the sweep never follows or guesses).
//	    TestPurgeRefusesUnexpectedEntries
//	P13 A computer with real tape segments is deletable and its chain still
//	    reads (the segment walk), where it used to be refused outright.
//	    TestPurgeComputerWithSegments
//	P14 Report: bytes and counts per class in the dry-run and in the apply.
//	    TestPurgeReportsClasses

const (
	purgePaperID = "computer-1355b83bced1613eb048ee28d66688a8"
	purgeOwnerID = "computer-03335285269bdba4f94377e56879f9e6"

	purgeEscrowSecret = "WRAPPED-SECRET-KEY-MATERIAL"
)

// ownTree writes a computer's file-CAS chunks and roots (and a stray file in
// its tape-seg directory) aged age, returning the slash paths under the
// artifacts root.
func (f *tapeFixture) ownTree(id string, age time.Duration) []string {
	f.t.Helper()
	var paths []string
	write := func(rel string, data []byte) {
		p := f.path(rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o640); err != nil {
			f.t.Fatal(err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(p, when, when); err != nil {
			f.t.Fatal(err)
		}
		paths = append(paths, rel)
	}
	for i := 0; i < 3; i++ {
		body := []byte(fmt.Sprintf("sealed chunk %s %d", id, i))
		sum := sha256.Sum256(body)
		write("sha256/file-cas-chunks/"+id+"/"+hex.EncodeToString(sum[:]), body)
	}
	root := sha256.Sum256([]byte("root " + id))
	write("sha256/file-cas-roots/"+id+"/"+hex.EncodeToString(root[:])+".json", []byte(`{"files":[]}`))
	write("tape-seg/"+id+"/stray.bin", []byte("not a segment "+id))
	return paths
}

// ownRows inserts a computer's per-computer rows beyond its chain: file
// roots, a snapshot root, a replay watermark and a key escrow entry.
func (f *tapeFixture) ownRows(id string) {
	f.t.Helper()
	now := time.Now().UTC()
	if _, err := f.store.db.Exec(`INSERT INTO computer_file_roots (computer_id, root, manifest_ref, head_sequence, created_at) VALUES (?,?,?,?,?)`, id, platformTestDigest('a'), "sha256/file-cas-roots/"+id+"/x.json", 1, now); err != nil {
		f.t.Fatal(err)
	}
	if err := f.store.RecordSnapshotRoot(f.ctx, id, platformTestDigest('b'), 1, []string{platformTestDigest('c')}); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.store.db.Exec(`INSERT INTO computer_replay_watermarks (computer_id, watermark_sequence, base_ref, updated_at) VALUES (?,?,?,?)`, id, 1, "base", now); err != nil {
		f.t.Fatal(err)
	}
	if err := f.store.UpsertKeyEscrow(f.ctx, id, "custodian", []byte(`{"wrap":"`+purgeEscrowSecret+`"}`), platformTestDigest('d')); err != nil {
		f.t.Fatal(err)
	}
}

func (f *tapeFixture) rowCount(table, id string) int64 {
	f.t.Helper()
	var n int64
	if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE computer_id=?`, id).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func purgeTrees(id string) []string {
	return []string{"tape-seg/" + id, "sha256/file-cas-chunks/" + id, "sha256/file-cas-roots/" + id}
}

// treeFiles lists every regular file under the computer's three trees.
func (f *tapeFixture) treeFiles(id string) []string {
	f.t.Helper()
	var out []string
	for _, base := range purgeTrees(id) {
		_ = filepath.WalkDir(f.path(base), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(f.root, p)
				out = append(out, filepath.ToSlash(rel))
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

func (f *tapeFixture) treeDirsExist(id string) bool {
	for _, base := range purgeTrees(id) {
		if _, err := os.Stat(f.path(base)); err == nil {
			return true
		}
	}
	return false
}

// ageTree sets every file under a computer's trees to age ago.
func (f *tapeFixture) ageTree(id string, age time.Duration) {
	for _, rel := range f.treeFiles(id) {
		when := time.Now().Add(-age)
		if err := os.Chtimes(f.path(rel), when, when); err != nil {
			f.t.Fatal(err)
		}
	}
}

// tombstoneStatus returns the computer's tombstone status ("" when none).
func (f *tapeFixture) tombstoneStatus(id string) string {
	tomb, ok, err := f.store.TapeTombstone(f.ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	if !ok {
		return ""
	}
	return tomb.Status
}

// destroyedReq is the standard destroyed-only request.
func destroyedReq(noArchive bool) TapeSweepRequest {
	return TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}, NoArchive: noArchive}
}

// manifestLines reads every manifest line of a report as raw JSON.
func (f *tapeFixture) manifestLines(report TapeSweepReport) [][]byte {
	f.t.Helper()
	file, err := os.Open(report.ManifestPath)
	if err != nil {
		f.t.Fatal(err)
	}
	defer file.Close()
	var lines [][]byte
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		lines = append(lines, append([]byte(nil), sc.Bytes()...))
	}
	return lines
}

func (f *tapeFixture) manifestRows(report TapeSweepReport) map[string]map[string]int64 {
	f.t.Helper()
	out := map[string]map[string]int64{}
	for _, line := range f.manifestLines(report) {
		var r tapeSweepRowsLine
		if json.Unmarshal(line, &r) != nil || r.Kind != "rows" {
			continue
		}
		if out[r.Computer] == nil {
			out[r.Computer] = map[string]int64{}
		}
		out[r.Computer][r.Table] = r.Count
	}
	return out
}

func (f *tapeFixture) manifestDirs(report TapeSweepReport) map[string]bool {
	f.t.Helper()
	out := map[string]bool{}
	for _, line := range f.manifestLines(report) {
		var d tapeSweepDirLine
		if json.Unmarshal(line, &d) == nil && d.Kind == "dir" {
			out[d.Path] = true
		}
	}
	return out
}

// resign rewrites a manifest and its sidecar with a valid hash.
func (f *tapeFixture) resign(report TapeSweepReport, lines [][]byte) string {
	f.t.Helper()
	body := append(bytes.Join(lines, []byte("\n")), '\n')
	if err := os.WriteFile(report.ManifestPath, body, 0o600); err != nil {
		f.t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if err := os.WriteFile(report.ManifestPath+".sha256", []byte(digest+"  "+filepath.Base(report.ManifestPath)+"\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return digest
}

// moveToSegments rewrites a loose chain as real tape segments: the bodies go
// through the production appender and the loose files are removed.
func (f *tapeFixture) moveToSegments(c *tapeChain) {
	f.t.Helper()
	app := f.svc.tapeAppender()
	nsFor := map[string]tapestore.NS{tapeNSEvent: tapestore.NSEvent, tapeNSPayload: tapestore.NSPayload, tapeNSReceipt: tapestore.NSReceipt}
	for ns, digests := range c.files() {
		for _, d := range digests {
			raw, err := os.ReadFile(f.path(tapeSweepRel(ns, d)))
			if err != nil {
				f.t.Fatal(err)
			}
			if err := app.Append(c.id, tapestore.Item{NS: nsFor[ns], Body: raw}); err != nil {
				f.t.Fatal(err)
			}
			if err := os.Remove(f.path(tapeSweepRel(ns, d))); err != nil {
				f.t.Fatal(err)
			}
		}
	}
	if err := app.Seal(c.id); err != nil {
		f.t.Fatal(err)
	}
	f.ageTree(c.id, tapeAged)
}

type purgeSnapshot struct {
	files  []string
	rows   map[string]int64
	escrow int64
}

func (f *tapeFixture) snapshotOf(id string) purgeSnapshot {
	s := purgeSnapshot{files: f.treeFiles(id), rows: map[string]int64{}}
	for _, table := range []string{"computer_event_append_receipts", "computer_file_roots", "computer_snapshot_roots", "computer_replay_watermarks"} {
		s.rows[table] = f.rowCount(table, id)
	}
	s.escrow = f.rowCount("computer_key_escrows", id)
	return s
}

func (a purgeSnapshot) equal(b purgeSnapshot) bool {
	if strings.Join(a.files, "|") != strings.Join(b.files, "|") || a.escrow != b.escrow || len(a.rows) != len(b.rows) {
		return false
	}
	for k, v := range a.rows {
		if b.rows[k] != v {
			return false
		}
	}
	return true
}

// fullComputer makes a computer with everything: chain, own trees, own rows, escrow.
func (f *tapeFixture) fullComputer(id string, n int) *tapeChain {
	c := f.chain(id, n, tapeAged)
	f.ownTree(id, tapeAged)
	f.ownRows(id)
	return c
}

// P1
func TestPurgeNeverTouchesALiveComputer(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	liveChain := f.fullComputer("live-1", 3)
	f.fullComputer("dead-1", 2)
	before := f.snapshotOf("live-1")

	dry := f.dryRun(destroyedReq(false))
	for _, it := range f.manifestItems(dry) {
		if it.Computer == "live-1" {
			t.Fatalf("live computer item listed: %+v", it)
		}
	}
	if _, listed := f.manifestRows(dry)["live-1"]; listed {
		t.Fatal("live computer rows listed")
	}
	if _, err := f.apply(dry, destroyedReq(false)); err != nil {
		t.Fatal(err)
	}
	if after := f.snapshotOf("live-1"); !before.equal(after) {
		t.Fatalf("live computer changed: before %+v after %+v", before, after)
	}
	f.allExist(liveChain, true)
	if got := f.treeFiles("dead-1"); len(got) != 0 {
		t.Fatalf("dead computer files survived: %v", got)
	}
	if f.rowCount("computer_key_escrows", "dead-1") != 0 || f.rowCount("computer_file_roots", "dead-1") != 0 {
		t.Fatal("dead computer rows survived")
	}
}

// P2
func TestPurgeKeepsSharedDigests(t *testing.T) {
	t.Run("a loose payload a live computer names survives", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		live := f.chain("live-1", 1, tapeAged)
		dead := f.chain("dead-1", 1, tapeAged)
		shared := []byte("shared observation")
		f.appendSeq(live, shared, tapeAged)
		deadShared := f.appendSeq(dead, shared, tapeAged)
		f.ownTree("dead-1", tapeAged)
		dry := f.dryRun(destroyedReq(false))
		if dry.DestroyedProofs[0].SharedPayloadsKept != 1 {
			t.Fatalf("proof = %+v", dry.DestroyedProofs[0])
		}
		if _, err := f.apply(dry, destroyedReq(false)); err != nil {
			t.Fatal(err)
		}
		if !f.exists(tapeNSPayload, deadShared.payload) {
			t.Fatal("shared payload deleted")
		}
		f.allExist(live, true)
	})
	t.Run("another computer's root naming this computer's paths blocks the delete", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		f.chain("live-1", 1, tapeAged)
		f.chain("dead-1", 1, tapeAged)
		paths := f.ownTree("dead-1", tapeAged)
		if _, err := f.store.db.Exec(`INSERT INTO computer_file_roots (computer_id, root, manifest_ref, head_sequence, created_at) VALUES (?,?,?,?,?)`, "live-1", platformTestDigest('e'), "sha256/file-cas-chunks/dead-1/"+platformTestDigest('f'), 1, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		dry := f.dryRun(destroyedReq(false))
		if dry.DestroyedListed != 0 {
			t.Fatalf("a referenced computer was listed: %+v", dry.DestroyedProofs)
		}
		if _, err := f.apply(dry, destroyedReq(false)); err != nil {
			t.Fatal(err)
		}
		for _, rel := range paths {
			if _, err := os.Stat(f.path(rel)); err != nil {
				t.Fatalf("%s deleted: %v", rel, err)
			}
		}
	})
}

// P3
func TestPurgeRefusesProtectedComputers(t *testing.T) {
	f := newTapeFixture(t)
	for _, id := range []string{purgePaperID, purgeOwnerID} {
		f.fullComputer(id, 2) // not live: absent from the registry, quiet, aged
	}
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(destroyedReq(true))
	for _, pr := range dry.DestroyedProofs {
		if (pr.ComputerID == purgePaperID || pr.ComputerID == purgeOwnerID) && pr.Deletable {
			t.Fatalf("protected computer deletable: %+v", pr)
		}
	}
	for _, it := range f.manifestItems(dry) {
		if it.Computer == purgePaperID || it.Computer == purgeOwnerID {
			t.Fatalf("protected computer listed: %+v", it)
		}
	}

	// A forged manifest with a valid hash that lists a protected computer.
	lines := f.manifestLines(dry)
	var head tapeSweepManifestHeader
	if err := json.Unmarshal(lines[0], &head); err != nil {
		t.Fatal(err)
	}
	proof := TapeSweepComputerProof{ComputerID: purgePaperID, HeadSequence: head.Heads[purgePaperID].Sequence, CanonicalEventHead: head.Heads[purgePaperID].Canonical, Quiet: true, Destroyed: true, Deletable: true, Reason: "forged"}
	computerLine, _ := json.Marshal(tapeSweepManifestComputer{Kind: "computer", Proof: proof})
	chunk := ""
	for _, rel := range f.treeFiles(purgePaperID) {
		if strings.Contains(rel, "file-cas-chunks") {
			chunk = rel
		}
	}
	itemLine, _ := json.Marshal(tapeSweepItem{Kind: "item", Sweep: TapeSweepDestroyed, Computer: purgePaperID, Namespace: tapeNSFileCASChunks, Digest: filepath.Base(chunk), Path: chunk, Size: 10, Reason: "forged"})
	rowsLine, _ := json.Marshal(tapeSweepRowsLine{Kind: "rows", Computer: purgePaperID, Table: "computer_key_escrows", Count: 1})
	lines = append(lines, computerLine, itemLine, rowsLine)
	sum := f.resign(dry, lines)
	before := f.snapshotOf(purgePaperID)
	req := destroyedReq(true)
	req.ManifestSHA256 = sum
	report, err := f.apply(dry, req)
	if err != nil {
		t.Fatal(err)
	}
	if after := f.snapshotOf(purgePaperID); !before.equal(after) || report.Apply.ComputersSkipped == 0 {
		t.Fatalf("protected computer touched by a forged manifest: apply=%+v", report.Apply)
	}
	if f.rowCount("computer_key_escrows", purgePaperID) != 1 {
		t.Fatal("protected computer escrow shredded")
	}
	// The purge path refuses on its own, apart from the proof.
	if err := tapeSweepProtected(purgeOwnerID); err == nil {
		t.Fatal("owner computer not protected")
	}
}

// P4 + P6
func TestPurgePartialApplyKeepsRowsAndEscrow(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 2)
	dry := f.dryRun(destroyedReq(true))
	var touched string
	f.svc.tapeSweep.beforeDelete = func(it tapeSweepItem) {
		if it.Namespace == tapeNSFileCASChunks && touched == "" {
			touched = it.Path
			now := time.Now()
			_ = os.Chtimes(f.path(it.Path), now, now) // a writer refreshed it: kept_refreshed
		}
	}
	report, err := f.apply(dry, destroyedReq(true))
	if err != nil {
		t.Fatal(err)
	}
	if touched == "" || report.Apply.KeptRefreshed == 0 {
		t.Fatalf("hook did not fire: %+v", report.Apply)
	}
	if f.rowCount("computer_key_escrows", "dead-1") != 1 || f.rowCount("computer_event_append_receipts", "dead-1") != 2 || f.rowCount("computer_file_roots", "dead-1") != 1 {
		t.Fatal("rows or escrow removed while a listed file remains")
	}
	if f.tombstoneStatus("dead-1") == tapeTombstoneDeleted {
		t.Fatal("tombstone marked deleted with a file remaining")
	}
	if _, err := os.Stat(f.path(touched)); err != nil {
		t.Fatal("the refreshed file was deleted")
	}
	// Finish: the file goes quiet again, the same manifest applies again.
	f.svc.tapeSweep.beforeDelete = nil
	old := time.Now().Add(-tapeAged)
	_ = os.Chtimes(f.path(touched), old, old)
	if _, err := f.apply(dry, destroyedReq(true)); err != nil {
		t.Fatal(err)
	}
	if len(f.treeFiles("dead-1")) != 0 || f.treeDirsExist("dead-1") {
		t.Fatalf("files remain: %v", f.treeFiles("dead-1"))
	}
	if f.rowCount("computer_key_escrows", "dead-1") != 0 || f.rowCount("computer_event_append_receipts", "dead-1") != 0 {
		t.Fatal("rows remain after a complete apply")
	}
	if f.tombstoneStatus("dead-1") != tapeTombstoneDeleted {
		t.Fatal("tombstone not deleted after a complete apply")
	}
}

// P5
func TestPurgeCrashBetweenFilesAndRows(t *testing.T) {
	t.Run("a cancelled context mid-delete", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 2)
		dry := f.dryRun(destroyedReq(true))
		ctx, cancel := context.WithCancel(f.ctx)
		defer cancel()
		fired := false
		f.svc.tapeSweep.beforeDelete = func(it tapeSweepItem) {
			if it.Namespace == tapeNSFileCASChunks && !fired {
				fired = true
				cancel()
			}
		}
		req := destroyedReq(true)
		req.Action, req.RunID, req.ManifestSHA256, req.Pause, req.BatchSize = TapeSweepActionApply, dry.RunID, dry.ManifestSHA256, "0s", 1
		if _, err := f.svc.RunTapeSweep(ctx, req); err == nil {
			t.Fatal("cancelled apply returned no error")
		}
		if f.rowCount("computer_key_escrows", "dead-1") != 1 || f.rowCount("computer_event_append_receipts", "dead-1") != 2 {
			t.Fatal("rows removed by an interrupted apply")
		}
		f.svc.tapeSweep.beforeDelete = nil
		if _, err := f.apply(dry, destroyedReq(true)); err != nil {
			t.Fatal(err)
		}
		if len(f.treeFiles("dead-1")) != 0 || f.rowCount("computer_key_escrows", "dead-1") != 0 {
			t.Fatal("resume did not finish")
		}
	})
	t.Run("a failure after the last file", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 2)
		dry := f.dryRun(destroyedReq(true))
		f.svc.tapeSweep.beforeRowsDelete = func(string) error { return errors.New("simulated crash before the rows transaction") }
		report, err := f.apply(dry, destroyedReq(true))
		if err != nil {
			t.Fatal(err)
		}
		if report.Apply.Errors == 0 {
			t.Fatalf("no error counted: %+v", report.Apply)
		}
		if len(f.treeFiles("dead-1")) != 0 {
			t.Fatal("files should be gone before the rows step")
		}
		if f.rowCount("computer_key_escrows", "dead-1") != 1 || f.rowCount("computer_event_append_receipts", "dead-1") != 2 || f.rowCount("computer_snapshot_roots", "dead-1") != 1 {
			t.Fatal("rows must all remain together when the transaction did not run")
		}
		if f.tombstoneStatus("dead-1") == tapeTombstoneDeleted {
			t.Fatal("tombstone deleted before the rows were")
		}
		f.svc.tapeSweep.beforeRowsDelete = nil
		if _, err := f.apply(dry, destroyedReq(true)); err != nil {
			t.Fatal(err)
		}
		if f.rowCount("computer_key_escrows", "dead-1") != 0 || f.rowCount("computer_event_append_receipts", "dead-1") != 0 || f.tombstoneStatus("dead-1") != tapeTombstoneDeleted {
			t.Fatal("resume did not finish the rows")
		}
	})
}

// P7
func TestPurgeApplyRemovesExactlyTheManifest(t *testing.T) {
	t.Run("a row added after the dry-run rolls the rows back whole", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 2)
		dry := f.dryRun(destroyedReq(true))
		if _, err := f.store.db.Exec(`INSERT INTO computer_file_roots (computer_id, root, manifest_ref, head_sequence, created_at) VALUES (?,?,?,?,?)`, "dead-1", platformTestDigest('9'), "late", 2, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		report, err := f.apply(dry, destroyedReq(true))
		if err != nil {
			t.Fatal(err)
		}
		if report.Apply.Errors == 0 {
			t.Fatalf("count mismatch not reported: %+v", report.Apply)
		}
		if f.rowCount("computer_file_roots", "dead-1") != 2 || f.rowCount("computer_key_escrows", "dead-1") != 1 || f.rowCount("computer_event_append_receipts", "dead-1") != 2 {
			t.Fatal("a mismatched rows step removed rows")
		}
		if f.tombstoneStatus("dead-1") == tapeTombstoneDeleted {
			t.Fatal("tombstone deleted on a mismatch")
		}
	})
	t.Run("a file added after the dry-run survives and blocks the rows", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 2)
		dry := f.dryRun(destroyedReq(true))
		late := "sha256/file-cas-chunks/dead-1/" + platformTestDigest('8')
		if err := os.WriteFile(f.path(late), []byte("late"), 0o640); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-tapeAged)
		_ = os.Chtimes(f.path(late), old, old)
		if _, err := f.apply(dry, destroyedReq(true)); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(f.path(late)); err != nil {
			t.Fatal("an unlisted file was deleted")
		}
		if f.rowCount("computer_key_escrows", "dead-1") != 1 {
			t.Fatal("escrow shredded while an unlisted file keeps the tree alive")
		}
	})
	t.Run("a tampered manifest deletes nothing", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 2)
		dry := f.dryRun(destroyedReq(true))
		forged := append(f.manifestLines(dry), []byte(`{"kind":"rows","computer":"dead-1","table":"computer_key_escrows","count":99}`))
		body := append(bytes.Join(forged, []byte("\n")), '\n')
		if err := os.WriteFile(dry.ManifestPath, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.apply(dry, destroyedReq(true)); err == nil {
			t.Fatal("tampered manifest accepted")
		}
		if len(f.treeFiles("dead-1")) == 0 || f.rowCount("computer_key_escrows", "dead-1") != 1 {
			t.Fatal("tampered manifest deleted something")
		}
	})
}

// P8
func TestPurgeManifestListsEverything(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 2)
	want := f.treeFiles("dead-1")
	dry := f.dryRun(destroyedReq(true))
	var got []string
	for _, it := range f.manifestItems(dry) {
		if it.Computer == "dead-1" && (strings.HasPrefix(it.Path, "tape-seg/") || strings.Contains(it.Path, "file-cas-")) {
			got = append(got, it.Path)
		}
	}
	sort.Strings(got)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("manifest paths %v, tree %v", got, want)
	}
	dirs := f.manifestDirs(dry)
	for _, d := range purgeTrees("dead-1") {
		if !dirs[d] {
			t.Fatalf("directory %s not in the manifest: %v", d, dirs)
		}
	}
	rows := f.manifestRows(dry)["dead-1"]
	for table, n := range map[string]int64{"computer_event_append_receipts": 2, "computer_file_roots": 1, "computer_snapshot_roots": 1, "computer_replay_watermarks": 1, "computer_key_escrows": 1} {
		if rows[table] != n {
			t.Fatalf("manifest rows[%s] = %d, want %d (%v)", table, rows[table], n, rows)
		}
	}
	// Apply removes exactly that set.
	report, err := f.apply(dry, destroyedReq(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.treeFiles("dead-1")) != 0 || f.treeDirsExist("dead-1") {
		t.Fatal("tree remains")
	}
	for table, n := range rows {
		if f.rowCount(table, "dead-1") != 0 {
			t.Fatalf("%s rows remain", table)
		}
		if got := report.Apply.Classes["rows:"+table].Rows; got != n {
			t.Fatalf("apply rows[%s] = %d, manifest %d", table, got, n)
		}
	}
}

// P14
func TestPurgeReportsClasses(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 2)
	var chunkBytes int64
	for _, rel := range f.treeFiles("dead-1") {
		if strings.Contains(rel, "file-cas-chunks") {
			info, _ := os.Stat(f.path(rel))
			chunkBytes += info.Size()
		}
	}
	dry := f.dryRun(destroyedReq(true))
	c := dry.DestroyedClasses["file-cas-chunks"]
	if c.Files != 3 || c.Bytes != chunkBytes {
		t.Fatalf("dry-run chunk class = %+v, want 3 files %d bytes", c, chunkBytes)
	}
	if dry.DestroyedClasses["file-cas-roots"].Files != 1 || dry.DestroyedClasses["tape-seg"].Files != 1 || dry.DestroyedClasses[tapeNSEvent].Files != 2 {
		t.Fatalf("classes = %+v", dry.DestroyedClasses)
	}
	if dry.DestroyedClasses["rows:computer_key_escrows"].Rows != 1 {
		t.Fatalf("escrow rows class = %+v", dry.DestroyedClasses)
	}
	report, err := f.apply(dry, destroyedReq(true))
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Apply.Classes["file-cas-chunks"]; got.Files != 3 || got.Bytes != chunkBytes {
		t.Fatalf("apply chunk class = %+v", got)
	}
	if report.Apply.Classes["rows:computer_event_append_receipts"].Rows != 2 || report.Apply.Classes[tapeNSEvent].Files != 2 {
		t.Fatalf("apply classes = %+v", report.Apply.Classes)
	}
}

// P9
func TestPurgeAftermathIsReadable(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.chain("live-1", 2, tapeAged)
	f.fullComputer("dead-1", 2)
	orphan := f.put(tapeNSPayload, []byte("orphan after"), tapeAged)
	req := TapeSweepRequest{AcceptUnrecordedForkLineage: true, NoArchive: true}
	dry := f.dryRun(req)
	if _, err := f.apply(dry, req); err != nil {
		t.Fatal(err)
	}
	if f.exists(tapeNSPayload, orphan) {
		t.Fatal("orphan survived")
	}
	again := f.dryRun(req)
	if again.OrphansRefused != "" || again.DestroyedRefused != "" {
		t.Fatalf("aftermath refused: orphans=%q destroyed=%q warnings=%v", again.OrphansRefused, again.DestroyedRefused, again.Warnings)
	}
	if again.DestroyedListed != 0 || again.DestroyedFiles != 0 {
		t.Fatalf("a purged computer is listed again: %+v", again.DestroyedProofs)
	}
	// Re-applying the first manifest deletes nothing more and breaks nothing.
	report, err := f.apply(dry, req)
	if err != nil {
		t.Fatal(err)
	}
	if report.Apply.Deleted != 0 || report.Apply.Errors != 0 {
		t.Fatalf("re-apply = %+v", report.Apply)
	}
	if f.tombstoneStatus("dead-1") != tapeTombstoneDeleted {
		t.Fatal("tombstone gone")
	}
	var heads int
	if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM computer_event_heads WHERE computer_id='dead-1'`).Scan(&heads); err != nil || heads != 1 {
		t.Fatalf("head row (the reuse guard) = %d err=%v", heads, err)
	}
}

// P10 + P6
func TestPurgeArchiveCarriesNoKey(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 2)
	dry := f.dryRun(destroyedReq(false))
	report, err := f.apply(dry, destroyedReq(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Apply.Archives) != 1 {
		t.Fatalf("archives = %v", report.Apply.Archives)
	}
	archive := report.Apply.Archives[0]
	var sc TapeArchiveSidecar
	raw, err := os.ReadFile(archive + ".json")
	if err != nil || json.Unmarshal(raw, &sc) != nil {
		t.Fatalf("sidecar: %v", err)
	}
	if err := verifyTapeArchive(archive, sc); err != nil {
		t.Fatal(err)
	}
	file, _ := os.Open(archive)
	defer file.Close()
	zr, err := zstd.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	names := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(tr)
		names[hdr.Name] = true
		if bytes.Contains(body, []byte(purgeEscrowSecret)) {
			t.Fatalf("archive entry %s carries the escrowed key", hdr.Name)
		}
		if strings.HasPrefix(hdr.Name, "rows/computer_key") {
			t.Fatalf("archive holds key rows: %s", hdr.Name)
		}
	}
	for _, want := range []string{"tape-seg/dead-1/stray.bin", "rows/computer_event_append_receipts.jsonl", "rows/computer_file_roots.jsonl"} {
		if !names[want] {
			t.Fatalf("archive lacks %s: %v", want, names)
		}
	}
	chunks, roots := 0, 0
	for n := range names {
		if strings.HasPrefix(n, "sha256/file-cas-chunks/dead-1/") {
			chunks++
		}
		if strings.HasPrefix(n, "sha256/file-cas-roots/dead-1/") {
			roots++
		}
	}
	if chunks != 3 || roots != 1 {
		t.Fatalf("archive chunks=%d roots=%d: %v", chunks, roots, names)
	}
	if f.rowCount("computer_key_escrows", "dead-1") != 0 {
		t.Fatal("escrow remains")
	}
}

// P10
func TestPurgeNoArchive(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 2)
	dry := f.dryRun(destroyedReq(true))
	// Apply must repeat no_archive exactly as the dry-run had it.
	if _, err := f.apply(dry, destroyedReq(false)); err == nil {
		t.Fatal("apply without no_archive accepted a no_archive manifest")
	}
	if len(f.treeFiles("dead-1")) == 0 {
		t.Fatal("a refused apply deleted files")
	}
	report, err := f.apply(dry, destroyedReq(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Apply.Archives) != 0 {
		t.Fatalf("no_archive wrote %v", report.Apply.Archives)
	}
	entries, _ := os.ReadDir(filepath.Join(f.root, tapeSweepArchiveDirName))
	if len(entries) != 0 {
		t.Fatalf("archive dir not empty: %v", entries)
	}
	if len(f.treeFiles("dead-1")) != 0 || f.rowCount("computer_key_escrows", "dead-1") != 0 {
		t.Fatal("no_archive purge incomplete")
	}
	// The default stays archive: an archive manifest cannot be applied as no_archive.
	f2 := newTapeFixture(t)
	f2.fullComputer("dead-2", 1)
	dry2 := f2.dryRun(destroyedReq(false))
	if _, err := f2.apply(dry2, destroyedReq(true)); err == nil {
		t.Fatal("an archive manifest was applied as no_archive")
	}
}

// P11
func TestPurgeCoversEveryComputerTable(t *testing.T) {
	f := newTapeFixture(t)
	known := map[string]bool{}
	for _, tbl := range tapePurgeTables {
		known[tbl.name] = true
	}
	for _, kept := range tapeKeptComputerTables {
		known[kept] = true
	}
	rows, err := f.store.db.Query(`SELECT DISTINCT table_name FROM information_schema.columns WHERE table_schema = current_schema() AND column_name = 'computer_id'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	rows.Close()
	for _, name := range names {
		if !known[name] {
			t.Errorf("table %s has a computer_id column and is neither purged nor kept by the tape sweep", name)
		}
	}
	// Every listed table exists and its predicate runs (a typo would fail at delete time).
	for _, tbl := range tapePurgeTables {
		if _, err := f.store.db.Exec(`SELECT 1 FROM `+tbl.name+` WHERE `+tbl.where+` LIMIT 1`, "none"); err != nil {
			t.Errorf("purge table %s: %v", tbl.name, err)
		}
	}
}

// P12
func TestPurgeRefusesUnexpectedEntries(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 1)
		target := filepath.Join(t.TempDir(), "elsewhere")
		_ = os.WriteFile(target, []byte("precious"), 0o600)
		if err := os.Symlink(target, f.path("sha256/file-cas-chunks/dead-1/"+platformTestDigest('7'))); err != nil {
			t.Fatal(err)
		}
		dry := f.dryRun(destroyedReq(true))
		if dry.DestroyedListed != 0 {
			t.Fatalf("a computer with a symlink was listed: %+v", dry.DestroyedProofs)
		}
		if _, err := f.apply(dry, destroyedReq(true)); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatal("symlink target deleted")
		}
		if f.rowCount("computer_key_escrows", "dead-1") != 1 {
			t.Fatal("escrow shredded for a computer the sweep refused")
		}
	})
	t.Run("recent write", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 1)
		now := time.Now()
		_ = os.Chtimes(f.path("tape-seg/dead-1/stray.bin"), now, now)
		dry := f.dryRun(destroyedReq(true))
		if dry.DestroyedListed != 0 {
			t.Fatalf("a computer with a fresh file was listed: %+v", dry.DestroyedProofs)
		}
	})
}

// P13
func TestPurgeComputerWithSegments(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.chain("live-1", 1, tapeAged)
	dead := f.chain("dead-1", 3, tapeAged)
	f.moveToSegments(dead)
	f.ownRows("dead-1")
	segs := f.treeFiles("dead-1")
	if len(segs) < 2 {
		t.Fatalf("segment files = %v", segs)
	}
	dry := f.dryRun(destroyedReq(false))
	if dry.DestroyedListed != 1 {
		t.Fatalf("segment computer not listed: %+v", dry.DestroyedProofs)
	}
	if dry.DestroyedClasses["tape-seg"].Files != len(segs) {
		t.Fatalf("tape-seg class = %+v, files %v", dry.DestroyedClasses["tape-seg"], segs)
	}
	if _, err := f.apply(dry, destroyedReq(false)); err != nil {
		t.Fatal(err)
	}
	if len(f.treeFiles("dead-1")) != 0 || f.treeDirsExist("dead-1") {
		t.Fatal("segments remain")
	}
	if f.rowCount("computer_key_escrows", "dead-1") != 0 {
		t.Fatal("escrow remains")
	}
	if has, err := f.svc.tapeReader().HasSegments("dead-1"); err != nil || has {
		t.Fatalf("HasSegments = %v, %v", has, err)
	}
	// And the next dry-run reads the world without the chain.
	again := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true})
	if again.OrphansRefused != "" {
		t.Fatalf("orphans refused after the purge: %s", again.OrphansRefused)
	}
}
