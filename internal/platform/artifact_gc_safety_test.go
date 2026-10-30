package platform

// Failure modes the active artifact GC must hold (written before the code;
// docs/problems/artifact-store-grows-12gb-a-day-2026-10-14.md):
//  1. deleting a referenced object (a root's chunks; a route-receipt
//     payload; a platform-update payload; projection bases are no longer
//     referenced by anything and are collected);
//  2. a live set that fails or loads partially treated as empty (route
//     tables, an unreadable manifest, an empty namespace live set);
//  3. a race with a writer that reuses an old object;
//  4. a crash mid-GC (unlogged unlink, resumability);
//  5. a crash mid-write leaving a torn object under its final name;
//  6. runaway deletion (caps, the on-demand endpoint bypassing policy);
//  7. a dry-run report that hides the eligible set.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

const gcTestWindow = 7 * 24 * time.Hour

type gcFixture struct {
	t       *testing.T
	store   *Store
	service *Service
	root    string
	ctx     context.Context
}

func newGCFixture(t *testing.T) *gcFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	return &gcFixture{t: t, store: store, service: NewService(store, artifactsRoot, ""), root: artifactsRoot, ctx: context.Background()}
}

// ensureGCRouteTables creates the route-ledger and code-closure tables the
// platform-update live set reads (vmctl owns them in production).
func ensureGCRouteTables(t *testing.T, store *Store) {
	t.Helper()
	if err := computerversion.NewSQLInputCatalog(store.db).EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := routeledger.NewSQLLedger(store.db).EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func gcDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// writeAged writes rel under the artifacts root with an mtime age ago.
func (f *gcFixture) writeAged(rel string, data []byte, age time.Duration) string {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
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
	return p
}

func (f *gcFixture) run(cfg ArtifactGCConfig) ArtifactGCReport {
	f.t.Helper()
	report, err := f.service.RunArtifactGC(f.ctx, cfg)
	if err != nil {
		f.t.Fatalf("gc: %v (report %+v)", err, report)
	}
	return report
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func activeWindow() ArtifactGCConfig {
	return ArtifactGCConfig{Mode: ArtifactGCModeActive, Grace: gcTestWindow}
}

// Projection bases are no longer in the GC's live set (the host keeps none;
// the guest recovers from its own sealed snapshots). Ways this could fail,
// written before the code:
//  1. a base still named by a row the GC used to honour (an explicit pin, a
//     computer's latest two, an active job's seed, an advertised watermark)
//     is kept, so the 60 GB is never released;
//  2. a base is collected inside the grace window, or its descriptor sidecar
//     outlives its blob;
//  3. the empty-live-set guard trips on the now-empty set and the namespace is
//     reported but never swept;
//  4. removing the live-set source makes the loader error, so the whole sweep
//     aborts and no namespace is collected;
//  5. another namespace loses its pins or keeps what it should drop: the
//     file-cas-roots live set and the platform-update live set must be exactly
//     what they were.
func TestArtifactGCCollectsProjectionBasesNoLiveSetNamesAny(t *testing.T) {
	f := newGCFixture(t)
	old := 9 * 24 * time.Hour
	base := func(name string, age time.Duration) (string, string) {
		d := gcDigest(name)
		return f.writeAged("sha256/projection-base/"+d, []byte(name), age),
			f.writeAged("sha256/projection-base/"+d+".descriptor.json", []byte("{}"), age)
	}
	now := time.Now().UTC()
	for i, name := range []string{"b1", "b2", "b3", "b4"} {
		if _, err := f.store.db.ExecContext(f.ctx, `INSERT INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?)`, "computer-p", gcDigest(name), i+1, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.db.ExecContext(f.ctx, `INSERT INTO computer_projection_base_pins (computer_id,base_ref,reference_id,created_at) VALUES (?,?,?,?)`, "computer-p", gcDigest("b1"), "checkpoint:head-1", now); err != nil {
		t.Fatal(err)
	}
	seedReplayWatermark(t, f.store, "computer-w", 5, gcDigest("wm"))
	if _, err := f.store.db.ExecContext(f.ctx, `INSERT INTO computer_projection_jobs (`+projectionJobColumns+`) VALUES (?,1,'queued','cadence',10,'h',5,?,'k','','',false,0,'',?,?)`, "computer-j", gcDigest("seed"), now, now); err != nil {
		t.Fatal(err)
	}
	paths := map[string][2]string{}
	for _, name := range []string{"b1", "b2", "b3", "b4", "wm", "seed"} {
		blob, sidecar := base(name, old)
		paths[name] = [2]string{blob, sidecar}
	}
	young := [2]string{}
	young[0], young[1] = base("young", time.Hour)

	// Another namespace: a live file root and a dead one, to show its live set
	// is unchanged by the bases leaving theirs.
	liveRoot, deadRoot := gcDigest("live-root"), gcDigest("dead-root")
	if err := f.store.RecordFileRoot(f.ctx, "computer-r", liveRoot, "sha256/file-cas-roots/computer-r/"+liveRoot+".json", 1); err != nil {
		t.Fatal(err)
	}
	liveRootPath := f.writeAged("sha256/file-cas-roots/computer-r/"+liveRoot+".json", []byte("live"), old)
	deadRootPath := f.writeAged("sha256/file-cas-roots/computer-r/"+deadRoot+".json", []byte("dead"), old)

	report := f.run(activeWindow())
	if len(report.Warnings) != 0 {
		t.Fatalf("sweep warned (a loader error or a guard tripped): %v", report.Warnings)
	}
	for name, ps := range paths {
		for _, p := range ps {
			if exists(p) {
				t.Fatalf("base %s kept though no live set names it: %s (report %+v)", name, p, report.Namespaces["projection-base"])
			}
		}
	}
	for _, p := range young {
		if !exists(p) {
			t.Fatalf("a base inside the grace window was collected: %s", p)
		}
	}
	if !exists(liveRootPath) || exists(deadRootPath) {
		t.Fatalf("file-cas-roots live set changed: live kept=%v dead kept=%v", exists(liveRootPath), exists(deadRootPath))
	}
	if ns := report.Namespaces["projection-base"]; ns.Deleted != 12 || ns.InGrace != 2 || ns.Live != 0 {
		t.Fatalf("projection-base report = %+v, want 12 deleted, 2 in grace, 0 live", ns)
	}
}

// Failure mode 1: chunks of the latest root and of every root inside the
// window are live; a chunk named only by an old root goes. Failure mode 2: an
// unreadable manifest, or a computer with chunks but no recorded root, keeps
// all of that computer's chunks.
func TestArtifactGCFileChunksKeepLatestAndWindowRoots(t *testing.T) {
	f := newGCFixture(t)
	old := 9 * 24 * time.Hour
	chunk := func(computer, name string) (string, string) {
		d := gcDigest(computer + "/" + name)
		return d, f.writeAged("sha256/file-cas-chunks/"+computer+"/"+d, []byte(name), old)
	}
	root := func(computer string, created time.Time, chunks ...string) string {
		manifest, err := filecas.BuildManifest(computer, []filecas.FileEntry{{Path: "f", Mode: 0o600, Size: 1, Chunks: chunks}}, created)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(manifest)
		ref, err := f.service.pinFileManifest(computer, manifest.Root, data)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.store.RecordFileRoot(f.ctx, computer, manifest.Root, ref, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.db.ExecContext(f.ctx, `UPDATE computer_file_roots SET created_at=? WHERE computer_id=? AND root=?`, created.UTC(), computer, manifest.Root); err != nil {
			t.Fatal(err)
		}
		p, _ := f.service.artifactPath(ref)
		return p
	}
	a, aPath := chunk("computer-c", "a")
	b, bPath := chunk("computer-c", "b")
	c, cPath := chunk("computer-c", "c")
	d, dPath := chunk("computer-c", "d")
	root("computer-c", time.Now().Add(-10*24*time.Hour), a, b)
	root("computer-c", time.Now().Add(-2*24*time.Hour), b, c)
	root("computer-c", time.Now().Add(-time.Minute), d)

	x, xPath := chunk("computer-bad", "x")
	badManifest := root("computer-bad", time.Now().Add(-time.Minute), gcDigest("other"))
	_ = x
	if err := os.WriteFile(badManifest, []byte("{torn"), 0o640); err != nil {
		t.Fatal(err)
	}
	_, orphanPath := chunk("computer-noroot", "o")

	report := f.run(activeWindow())
	if exists(aPath) {
		t.Fatalf("chunk named only by an old root kept (report %+v)", report.Namespaces["file-cas-chunks"])
	}
	for name, p := range map[string]string{"b": bPath, "c": cPath, "d": dPath, "unreadable-manifest": xPath, "no-root": orphanPath} {
		if !exists(p) {
			t.Fatalf("live or unprovable chunk %s removed", name)
		}
	}
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "computer-bad") {
		t.Fatalf("unreadable manifest not reported: %v", report.Warnings)
	}
}

// Failure mode 2: a platform-update live set that cannot be loaded aborts the
// sweep instead of collecting rollback payloads as unreachable.
func TestArtifactGCFailsClosedOnRouteLedgerError(t *testing.T) {
	f := newGCFixture(t)
	payload := f.writeAged("sha256/platform-update/"+gcDigest("payload"), []byte("payload"), 30*24*time.Hour)
	if _, err := f.store.db.ExecContext(f.ctx, `DROP TABLE computer_version_route_transition_receipts`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunArtifactGC(f.ctx, activeWindow()); err == nil {
		t.Fatal("sweep ran with a route ledger it could not read")
	}
	if !exists(payload) {
		t.Fatal("rollback payload deleted on a partial live set")
	}
}

// Failure mode 2: a namespace whose live set is empty while old files exist
// is the wrong-store signature (2026-10-05); it is reported, not swept.
//
// projection-base is exempt: its live set is empty by design (the host keeps
// no bases), so the guard is exercised on platform-update, whose live set is
// read from the route ledger.
func TestArtifactGCRefusesNamespaceWithEmptyLiveSet(t *testing.T) {
	f := newGCFixture(t)
	orphan := f.writeAged("sha256/platform-update/"+gcDigest("orphan"), []byte("x"), 30*24*time.Hour)
	report := f.run(activeWindow())
	if !exists(orphan) {
		t.Fatal("namespace with an empty live set was swept")
	}
	ns := report.Namespaces["platform-update"]
	if ns.Eligible != 1 || ns.Deleted != 0 || !strings.Contains(strings.Join(report.Warnings, "\n"), "platform-update") {
		t.Fatalf("empty live set not reported: %+v %v", ns, report.Warnings)
	}
}

// Failure mode 3: a writer reusing an old chunk refreshes it, so the sweep's
// re-check under the reuse lock keeps it even after it was classified dead.
func TestArtifactGCReuseRaceKeepsRefreshedChunk(t *testing.T) {
	f := newGCFixture(t)
	computer := "computer-r"
	live := []byte("live sealed")
	reused := []byte("reused sealed")
	liveDigest, reusedDigest := gcDigest(string(live)), gcDigest(string(reused))
	if err := f.service.PinFileChunk(f.ctx, computer, liveDigest, live); err != nil {
		t.Fatal(err)
	}
	manifest, _ := filecas.BuildManifest(computer, []filecas.FileEntry{{Path: "f", Mode: 0o600, Size: 1, Chunks: []string{liveDigest}}}, time.Now())
	data, _ := json.Marshal(manifest)
	ref, err := f.service.pinFileManifest(computer, manifest.Root, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordFileRoot(f.ctx, computer, manifest.Root, ref, 1); err != nil {
		t.Fatal(err)
	}
	reusedPath := f.writeAged("sha256/file-cas-chunks/"+computer+"/"+reusedDigest, reused, 9*24*time.Hour)

	f.service.artifactGCBeforeUnlink = func(rel string) {
		if strings.HasSuffix(rel, reusedDigest) {
			if err := f.service.PinFileChunk(f.ctx, computer, reusedDigest, reused); err != nil {
				t.Errorf("reuse: %v", err)
			}
		}
	}
	report := f.run(activeWindow())
	if !exists(reusedPath) {
		t.Fatal("chunk reused during the sweep was deleted")
	}
	if report.Namespaces["file-cas-chunks"].KeptOnRecheck != 1 {
		t.Fatalf("recheck not reported: %+v", report.Namespaces["file-cas-chunks"])
	}
}

// Failure mode 3: a dedup hit (chunk or og body) refreshes the object's
// mtime, so the grace window protects it until its reference commits.
func TestCASReuseRefreshesExistingObjects(t *testing.T) {
	f := newGCFixture(t)
	data := []byte("sealed chunk")
	digest := gcDigest(string(data))
	chunkPath := f.writeAged("sha256/file-cas-chunks/computer-x/"+digest, data, 9*24*time.Hour)
	if err := f.service.PinFileChunk(f.ctx, "computer-x", digest, data); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(chunkPath); time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("chunk reuse did not refresh mtime: %v", info.ModTime())
	}
	body := bytes.Repeat([]byte("b"), ogBodyExternalizeMinBytes+1)
	bodyPath := f.writeAged("sha256/og/"+gcDigest(string(body))+".bin", body, 9*24*time.Hour)
	if _, err := NewObjectGraphStore(f.store).externalizeBody(body); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(bodyPath); time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("og body reuse did not refresh mtime: %v", info.ModTime())
	}
}

func readGCLog(t *testing.T, root string) []map[string]any {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(root, artifactGCLogDir))
	var out []map[string]any
	for _, e := range entries {
		file, err := os.Open(filepath.Join(root, artifactGCLogDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var rec map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				t.Fatalf("log line %q: %v", scanner.Text(), err)
			}
			out = append(out, rec)
		}
		file.Close()
	}
	return out
}

// Failure mode 4: every unlink is preceded by a durable intent record; a run
// that dies mid-batch leaves the rest for the next run, which completes.
func TestArtifactGCLogsIntentBeforeUnlinkAndResumesAfterCrash(t *testing.T) {
	f := newGCFixture(t)
	var dead []string
	for _, name := range []string{"d1", "d2", "d3"} {
		dead = append(dead, f.writeAged("sha256/projection-base/"+gcDigest(name), []byte(name), 9*24*time.Hour))
	}
	ctx, cancel := context.WithCancel(f.ctx)
	calls := 0
	f.service.artifactGCBeforeUnlink = func(string) {
		calls++
		if calls == 2 {
			cancel() // the process dies before the second unlink
		}
	}
	if _, err := f.service.RunArtifactGC(ctx, activeWindow()); err == nil {
		t.Fatal("interrupted run reported success")
	}
	remaining := 0
	for _, p := range dead {
		if exists(p) {
			remaining++
		}
	}
	if remaining != 2 {
		t.Fatalf("interrupted run removed %d of 3", 3-remaining)
	}
	intents := map[string]bool{}
	for _, rec := range readGCLog(t, f.root) {
		if rec["op"] == "intent" {
			intents[rec["path"].(string)] = true
		}
	}
	if len(intents) != 3 {
		t.Fatalf("intent records = %v, want all three before any unlink", intents)
	}
	f.service.artifactGCBeforeUnlink = nil
	f.run(activeWindow())
	for _, p := range dead {
		if exists(p) {
			t.Fatalf("resumed run left %s", p)
		}
		rel, _ := filepath.Rel(f.root, p)
		if !intents[filepath.ToSlash(rel)] {
			t.Fatalf("deleted %s without an intent record", rel)
		}
	}
}

// Failure mode 4: no log, no deletion.
func TestArtifactGCRefusesToDeleteWithoutLog(t *testing.T) {
	f := newGCFixture(t)
	dead := f.writeAged("sha256/projection-base/"+gcDigest("dead"), []byte("dead"), 9*24*time.Hour)
	if err := os.WriteFile(filepath.Join(f.root, artifactGCLogDir), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunArtifactGC(f.ctx, activeWindow()); err == nil {
		t.Fatal("active run without a deletion log reported success")
	}
	if !exists(dead) {
		t.Fatal("deleted without a log")
	}
}

// Failure mode 5: a torn chunk under its final name is replaced by the next
// PUT of the same digest instead of failing it forever.
func TestPinFileChunkReplacesTornChunk(t *testing.T) {
	f := newGCFixture(t)
	data := []byte("complete sealed chunk")
	digest := gcDigest(string(data))
	torn := f.writeAged("sha256/file-cas-chunks/computer-t/"+digest, data[:5], time.Hour)
	if err := f.service.PinFileChunk(f.ctx, "computer-t", digest, data); err != nil {
		t.Fatalf("PUT over torn chunk: %v", err)
	}
	got, err := os.ReadFile(torn)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("chunk = %q, %v", got, err)
	}
	if err := f.service.PinFileChunk(f.ctx, "computer-t", digest, data); err != nil {
		t.Fatalf("idempotent PUT: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(torn), ".tmp-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temp files left: %v", leftovers)
	}
}

// Failure modes 6 and 7: the byte cap is never exceeded (an object that would
// cross it is deferred), the count cap holds, and dry-run reports the whole
// eligible set while deleting nothing.
func TestArtifactGCCapsAndUncappedDryRunReport(t *testing.T) {
	f := newGCFixture(t)
	small1 := f.writeAged("sha256/projection-base/"+gcDigest("s1"), bytes.Repeat([]byte("a"), 10), 9*24*time.Hour)
	small2 := f.writeAged("sha256/projection-base/"+gcDigest("s2"), bytes.Repeat([]byte("b"), 10), 9*24*time.Hour)
	large := f.writeAged("sha256/projection-base/"+gcDigest("l"), bytes.Repeat([]byte("c"), 100), 9*24*time.Hour)
	chunk := f.writeAged("sha256/file-cas-chunks/computer-z/"+gcDigest("z"), []byte("z"), 9*24*time.Hour)
	manifest, _ := filecas.BuildManifest("computer-z", nil, time.Now())
	data, _ := json.Marshal(manifest)
	ref, _ := f.service.pinFileManifest("computer-z", manifest.Root, data)
	if err := f.store.RecordFileRoot(f.ctx, "computer-z", manifest.Root, ref, 1); err != nil {
		t.Fatal(err)
	}

	capped := ArtifactGCConfig{Mode: ArtifactGCModeDryRun, Grace: gcTestWindow, MaxDeletes: 1, MaxBytes: 25}
	dry := f.run(capped)
	pb := dry.Namespaces["projection-base"]
	if pb.Eligible != 3 || pb.EligibleBytes != 120 || dry.Namespaces["file-cas-chunks"].Eligible != 1 {
		t.Fatalf("dry-run hid the eligible set: %+v", dry.Namespaces)
	}
	for _, p := range []string{small1, small2, large, chunk} {
		if !exists(p) {
			t.Fatalf("dry-run deleted %s", p)
		}
	}

	capped.Mode = ArtifactGCModeActive
	capped.MaxDeletes = 100
	active := f.run(capped)
	if active.BytesDeleted > 25 || exists(small1) || exists(small2) || !exists(large) {
		t.Fatalf("byte cap: deleted %d bytes, report %+v", active.BytesDeleted, active.Namespaces["projection-base"])
	}
	if active.Namespaces["projection-base"].DeferredByCap != 1 {
		t.Fatalf("deferral not reported: %+v", active.Namespaces["projection-base"])
	}
	countCapped := f.run(ArtifactGCConfig{Mode: ArtifactGCModeActive, Grace: gcTestWindow, MaxDeletes: 1, MaxBytes: 1 << 30})
	if countCapped.Deleted != 1 {
		t.Fatalf("count cap: deleted %d", countCapped.Deleted)
	}
}

// Failure mode 6: namespaces outside the active list are report-only.
func TestArtifactGCActiveNamespacesLimitDeletion(t *testing.T) {
	f := newGCFixture(t)
	dead := f.writeAged("sha256/projection-base/"+gcDigest("dead"), []byte("dead"), 9*24*time.Hour)
	cfg := activeWindow()
	cfg.ActiveNamespaces = []string{"file-cas-chunks"}
	report := f.run(cfg)
	if !exists(dead) {
		t.Fatal("namespace outside the active list was swept")
	}
	if ns := report.Namespaces["projection-base"]; ns.Mode != ArtifactGCModeDryRun || ns.Eligible != 1 {
		t.Fatalf("report-only namespace = %+v", ns)
	}
}

// Failure mode 6: the on-demand endpoint can never exceed the configured
// policy: no active run while the service is configured dry-run, and no
// grace below the configured window.
func TestHandleArtifactGCClampsToConfiguredPolicy(t *testing.T) {
	f := newGCFixture(t)
	twoHours := f.writeAged("sha256/projection-base/"+gcDigest("recent"), []byte("recent"), 2*time.Hour)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/artifact-gc", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:5555"
		req.Header.Set("X-Internal-Caller", "true")
		rec := httptest.NewRecorder()
		NewHandler(f.service).HandleArtifactGC(rec, req)
		return rec
	}
	f.service.SetArtifactGCPolicy(ArtifactGCConfig{Mode: ArtifactGCModeDryRun, Grace: gcTestWindow})
	if rec := post(`{"mode":"active","grace":3600000000000}`); rec.Code != http.StatusConflict {
		t.Fatalf("active on a dry-run policy = %d %s", rec.Code, rec.Body.String())
	}
	f.service.SetArtifactGCPolicy(ArtifactGCConfig{Mode: ArtifactGCModeActive, Grace: gcTestWindow})
	if rec := post(`{"mode":"active","grace":3600000000000}`); rec.Code != http.StatusOK {
		t.Fatalf("active on an active policy = %d %s", rec.Code, rec.Body.String())
	}
	if !exists(twoHours) {
		t.Fatal("endpoint grace undercut the configured window")
	}
}

// A chunk no live root names waits for the upload grace (a day), not the
// run's week: a discarded snapshot attempt of the paper is tens of GB. Ways
// it could fail, written before the code:
//
//  1. an unnamed chunk younger than the upload grace is deleted (a snapshot
//     or file save still uploading loses a chunk its root will name);
//  2. a chunk named only by a file root inside the week is deleted (the
//     owner's file history shrinks to a day);
//  3. a chunk of a live snapshot root, or of a computer with no root at all,
//     is deleted;
//  4. an unnamed chunk older than the upload grace is still kept a week.
func TestArtifactGCFileChunksWaitTheUploadGraceNotTheWeek(t *testing.T) {
	f := newGCFixture(t)
	chunk := func(computer, name string, age time.Duration) (string, string) {
		d := gcDigest(computer + "/" + name)
		return d, f.writeAged("sha256/file-cas-chunks/"+computer+"/"+d, []byte(name), age)
	}
	fileRoot := func(computer string, created time.Time, chunks ...string) {
		manifest, err := filecas.BuildManifest(computer, []filecas.FileEntry{{Path: "f", Mode: 0o600, Size: 1, Chunks: chunks}}, created)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(manifest)
		ref, err := f.service.pinFileManifest(computer, manifest.Root, data)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.store.RecordFileRoot(f.ctx, computer, manifest.Root, ref, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.db.ExecContext(f.ctx, `UPDATE computer_file_roots SET created_at=? WHERE computer_id=? AND root=?`, created.UTC(), computer, manifest.Root); err != nil {
			t.Fatal(err)
		}
	}
	history, historyPath := chunk("computer-g", "file history", 30*time.Hour)
	latest, latestPath := chunk("computer-g", "latest file", 30*time.Hour)
	fileRoot("computer-g", time.Now().Add(-2*24*time.Hour), history)
	fileRoot("computer-g", time.Now().Add(-time.Minute), latest)
	snap, snapPath := chunk("computer-g", "live snapshot", 30*time.Hour)
	if err := f.store.RecordSnapshotRoot(f.ctx, "computer-g", gcDigest("snapshot root"), 7, []string{snap}); err != nil {
		t.Fatal(err)
	}
	_, discardedPath := chunk("computer-g", "discarded attempt", 30*time.Hour)
	_, uploadingPath := chunk("computer-g", "still uploading", 20*time.Hour)
	_, rootlessPath := chunk("computer-rootless", "no root yet", 30*time.Hour)

	report := f.run(activeWindow())
	if exists(discardedPath) {
		t.Fatalf("an unnamed chunk past the upload grace was kept (report %+v)", report.Namespaces["file-cas-chunks"])
	}
	for name, p := range map[string]string{
		"file history in the window": historyPath, "latest file root": latestPath, "live snapshot root": snapPath,
		"inside the upload grace": uploadingPath, "computer with no root": rootlessPath,
	} {
		if !exists(p) {
			t.Fatalf("chunk %q removed (report %+v)", name, report.Namespaces["file-cas-chunks"])
		}
	}
}

const projectionJobColumns = `computer_id,generation,status,reason,target_sequence,target_head,watermark_sequence,seed_base_ref,key_digest,base_ref,error_text,genesis_repair,failures,lease_token,lease_until,updated_at`
