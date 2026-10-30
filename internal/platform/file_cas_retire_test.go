package platform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

// Failure modes, written before the code
// (docs/problems/platform-artifact-retention-2026-10-22.md section 8, F4):
// a computer's latest root is retired (its Files can no longer hydrate); a root
// inside the 7-day window is retired; a dry-run, or a run that does not list
// file-cas-roots as active, retires rows; a short test grace shortens the
// 7-day window; one computer's rule touches another computer's rows; a
// retired row's manifest is still collected afterwards; the retirement is not
// logged.

type rootsFixture struct {
	t             *testing.T
	store         *Store
	service       *Service
	artifactsRoot string
}

func newRootsFixture(t *testing.T) *rootsFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	return &rootsFixture{t: t, store: store, service: NewService(store, artifactsRoot, ""), artifactsRoot: artifactsRoot}
}

// root records a file root for the computer, backdates its row by age and
// writes its manifest file with an mtime of the same age.
func (f *rootsFixture) root(computer, name string, age time.Duration) string {
	f.t.Helper()
	digest := gcDigest(computer + "/" + name)
	ref := "sha256/file-cas-roots/" + computer + "/" + digest + ".json"
	if err := f.store.RecordFileRoot(context.Background(), computer, digest, ref, 1); err != nil {
		f.t.Fatal(err)
	}
	when := time.Now().UTC().Add(-age)
	if _, err := f.store.db.Exec(`UPDATE computer_file_roots SET created_at=? WHERE computer_id=? AND root=?`, when, computer, digest); err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(f.artifactsRoot, filepath.FromSlash(ref))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(name), 0o640); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chtimes(p, when, when); err != nil {
		f.t.Fatal(err)
	}
	return digest
}

func (f *rootsFixture) rows(computer string) map[string]bool {
	f.t.Helper()
	got := map[string]bool{}
	roots, err := f.store.LatestFileRoots(context.Background(), computer, 1000)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, r := range roots {
		got[r.Root] = true
	}
	return got
}

func (f *rootsFixture) manifestExists(computer, digest string) bool {
	_, err := os.Stat(filepath.Join(f.artifactsRoot, "sha256", "file-cas-roots", computer, digest+".json"))
	return err == nil
}

const day = 24 * time.Hour

func TestRunArtifactGCRetiresOldFileRootsKeepingLatestAndWindow(t *testing.T) {
	f := newRootsFixture(t)
	ctx := context.Background()

	// computer-a: two old roots, one near the window edge, one fresh.
	a1 := f.root("computer-a", "a1", 30*day)
	a2 := f.root("computer-a", "a2", 20*day)
	a3 := f.root("computer-a", "a3", 6*day)
	a4 := f.root("computer-a", "a4", time.Minute)
	// computer-b: a single root far outside the window is still its latest.
	b1 := f.root("computer-b", "b1", 40*day)
	// computer-c: the latest root is itself older than the window.
	c1 := f.root("computer-c", "c1", 40*day)
	c2 := f.root("computer-c", "c2", 10*day)

	// Dry-run reports what it would retire and retires nothing.
	dry, err := f.service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "dry-run", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(dry.Namespaces["file-cas-roots"])
	if !strings.Contains(string(raw), `"retired":3`) {
		t.Fatalf("dry-run report = %s, want retired 3", raw)
	}
	if got := len(f.rows("computer-a")) + len(f.rows("computer-b")) + len(f.rows("computer-c")); got != 7 {
		t.Fatalf("dry-run retired rows: %d left of 7", got)
	}

	// A run that does not list file-cas-roots as active retires nothing.
	if _, err := f.service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour, ActiveNamespaces: []string{"projection-base"}}); err != nil {
		t.Fatal(err)
	}
	if got := len(f.rows("computer-a")); got != 4 {
		t.Fatalf("a run without file-cas-roots active retired rows: computer-a has %d", got)
	}

	// Active: a short grace (1h) must not shorten the 7-day window.
	active, err := f.service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(active.Namespaces["file-cas-roots"])
	if !strings.Contains(string(raw), `"retired":3`) {
		t.Fatalf("active report = %s, want retired 3", raw)
	}
	ra := f.rows("computer-a")
	if ra[a1] || ra[a2] || !ra[a3] || !ra[a4] {
		t.Fatalf("computer-a rows = %v (a1 %s, a2 %s retired; a3, a4 kept)", ra, a1, a2)
	}
	if rb := f.rows("computer-b"); !rb[b1] || len(rb) != 1 {
		t.Fatalf("computer-b latest root was retired: %v", rb)
	}
	if rc := f.rows("computer-c"); rc[c1] || !rc[c2] {
		t.Fatalf("computer-c rows = %v (c1 retired, c2 latest kept)", rc)
	}

	// The manifests of retired rows are collected by the same run (past
	// grace); the manifests of kept rows survive.
	for _, gone := range [][2]string{{"computer-a", a1}, {"computer-a", a2}, {"computer-c", c1}} {
		if f.manifestExists(gone[0], gone[1]) {
			t.Fatalf("manifest of retired root %s/%s was kept", gone[0], gone[1])
		}
	}
	for _, kept := range [][2]string{{"computer-a", a3}, {"computer-a", a4}, {"computer-b", b1}, {"computer-c", c2}} {
		if !f.manifestExists(kept[0], kept[1]) {
			t.Fatalf("manifest of kept root %s/%s was removed", kept[0], kept[1])
		}
	}

	// Each retirement is in the day's GC log.
	logData, err := os.ReadFile(filepath.Join(f.artifactsRoot, artifactGCLogDir, time.Now().UTC().Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	retired := 0
	for _, line := range strings.Split(strings.TrimSpace(string(logData)), "\n") {
		var rec artifactGCLogRecord
		if json.Unmarshal([]byte(line), &rec) == nil && rec.Op == "root_retired" {
			retired++
		}
	}
	if retired != 3 {
		t.Fatalf("logged %d root_retired records, want 3", retired)
	}

	// A second active run is a no-op.
	again, err := f.service.RunArtifactGC(ctx, ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(again.Namespaces["file-cas-roots"])
	if strings.Contains(string(raw), `"retired"`) {
		t.Fatalf("second run retired again: %s", raw)
	}
}

func TestListProjectionJobsReturnsEveryRow(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i, status := range []string{"queued", "blocked"} {
		id := []string{"computer-a", "computer-b"}[i]
		if _, err := store.db.Exec(`INSERT INTO computer_projection_jobs (`+projectionJobColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, 3+i, status, "cadence", 10, strings.Repeat("a", 64), 0, "", "", "", "", i == 1, 0, "", now, now); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := store.ListProjectionJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs = %+v", jobs)
	}
	byID := map[string]ProjectionJob{}
	for _, j := range jobs {
		byID[j.ComputerID] = j
	}
	if byID["computer-a"].Generation != 3 || byID["computer-a"].Status != "queued" || byID["computer-a"].GenesisRepair {
		t.Fatalf("computer-a = %+v", byID["computer-a"])
	}
	if byID["computer-b"].Generation != 4 || !byID["computer-b"].GenesisRepair {
		t.Fatalf("computer-b = %+v", byID["computer-b"])
	}
}

// breakCorpusStore points the store's graph database at an empty database
// without og_objects, so the og live-set query fails (the same device as
// TestRunArtifactGCFailsClosedOnLiveSetError).
func breakCorpusStore(t *testing.T, store *Store, root string) {
	t.Helper()
	db, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	store.corpusDB = db
	t.Cleanup(func() { _ = db.Close() })
}

func readGCLogOps(t *testing.T, artifactsRoot string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(artifactsRoot, artifactGCLogDir, time.Now().UTC().Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var ops []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var rec artifactGCLogRecord
		if json.Unmarshal([]byte(line), &rec) == nil {
			ops = append(ops, rec.Op)
		}
	}
	return ops
}

// N2: the live sets are loaded and verified before any row is deleted. A pass
// that cannot load them retires nothing.
func TestRunArtifactGCRetiresNothingWhenTheLiveSetCannotBeLoaded(t *testing.T) {
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	f := &rootsFixture{t: t, store: store, service: NewService(store, artifactsRoot, ""), artifactsRoot: artifactsRoot}
	f.root("computer-a", "a1", 30*day)
	f.root("computer-a", "a2", time.Minute)
	breakCorpusStore(t, store, root)

	if _, err := f.service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err == nil {
		t.Fatal("expected the live-set failure")
	}
	if got := len(f.rows("computer-a")); got != 2 {
		t.Fatalf("rows were retired before the live set was proven: %d left of 2", got)
	}
}

// N5: the intent is durable in the log before the rows are deleted.
func TestRunArtifactGCLogsRetirementIntentFirst(t *testing.T) {
	f := newRootsFixture(t)
	f.root("computer-a", "a1", 30*day)
	f.root("computer-a", "a2", 20*day)
	f.root("computer-a", "a3", time.Minute)
	if _, err := f.service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err != nil {
		t.Fatal(err)
	}
	var intents, retired int
	for _, op := range readGCLogOps(t, f.artifactsRoot) {
		switch op {
		case "root_retire_intent":
			if retired > 0 {
				t.Fatal("an intent record came after an outcome record")
			}
			intents++
		case "root_retired":
			retired++
		}
	}
	if intents != 2 || retired != 2 {
		t.Fatalf("intents=%d retired=%d, want 2 and 2", intents, retired)
	}
}

// N3: the retirement is bounded by MaxDeletes and takes the oldest first.
func TestRunArtifactGCRetirementHonorsTheRowBudget(t *testing.T) {
	f := newRootsFixture(t)
	a1 := f.root("computer-a", "a1", 40*day)
	a2 := f.root("computer-a", "a2", 30*day)
	a3 := f.root("computer-a", "a3", 20*day)
	a4 := f.root("computer-a", "a4", 10*day)
	if _, err := f.service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour, MaxDeletes: 2}); err != nil {
		t.Fatal(err)
	}
	rows := f.rows("computer-a")
	if rows[a1] || rows[a2] || !rows[a3] || !rows[a4] || len(rows) != 2 {
		t.Fatalf("rows after a budget of 2 = %v (oldest two a1, a2 retired)", rows)
	}
}
