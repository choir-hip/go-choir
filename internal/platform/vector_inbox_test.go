package platform

// The spool inbox (vector_inbox.go). Failure list, written before the code:
//   1. a file is imported twice, or not moved to done/ with its job record;
//   2. a *.part file or a file younger than the minimum age is read;
//   3. a file left in running/ by a dead process is run again at start;
//   4. a job of the route and a file run together;
//   5. a refused file (wrong model, bad row, failed sample) writes a row, or
//      is not in refused/ with its reason, or is retried by corpusd;
//   6. an oversized or unreadable file is retried for ever;
//   7. a symlink is followed out of the spool;
//   8. the journal line is not the one format, or carries no counts by reason.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func inboxTake(t *testing.T, f *vecFixture) int {
	t.Helper()
	n, err := f.svc.ProcessVectorInbox(context.Background())
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	return n
}

func (f *vecFixture) inboxDir(sub string) string { return f.svc.vectorSpoolDir(sub) }

// inboxPut copies a good file into the inbox as name, old enough to be taken.
func (f *vecFixture) inboxPut(t *testing.T, name string, rows []importRow, age time.Duration) string {
	t.Helper()
	if err := f.svc.ensureVectorInboxDirs(); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	src := writeImportFile(t, rows)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(f.inboxDir("inbox"), name)
	if err := os.WriteFile(dst, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err := os.Chtimes(dst, at, at); err != nil {
		t.Fatal(err)
	}
	return dst
}

func spoolHas(p string) bool { _, err := os.Lstat(p); return err == nil }

func readJob(t *testing.T, p string) VectorImportJob {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var j VectorImportJob
	if err := json.Unmarshal(raw, &j); err != nil {
		t.Fatal(err)
	}
	return j
}

func TestInboxFileIsImportedOnceMovedToDoneWithItsJobRecordAndOneJournalLine(t *testing.T) {
	f, caps := importFixture(t)
	f.inboxPut(t, "pass1.jsonl", f.macRows(t, caps...), time.Hour)
	logs := fullTextLog(t)
	if n := inboxTake(t, f); n != 1 {
		t.Fatalf("took %d files", n)
	}
	if f.count(t, "vec_story_passages") != 8 || f.count(t, "vec_captures") != 2 {
		t.Fatal("rows missing")
	}
	if spoolHas(filepath.Join(f.inboxDir("inbox"), "pass1.jsonl")) || spoolHas(filepath.Join(f.inboxDir("running"), "pass1.jsonl")) || !spoolHas(filepath.Join(f.inboxDir("done"), "pass1.jsonl")) {
		t.Fatal("file not in done/")
	}
	job := readJob(t, filepath.Join(f.inboxDir("done"), "pass1.jsonl.job.json"))
	if job.State != "done" || job.Source != "inbox" || job.File != "pass1.jsonl" || job.Result == nil || job.Result.Written != 10 {
		t.Fatalf("job record %+v", job)
	}
	re := regexp.MustCompile(`corpusd retrieval op=vector_import shape=inbox ms=\d+ rows=10 written=10 refused=0 sample_min_cosine=[0-9.]+ err= reasons= job=vi-\S+ state=done file=pass1.jsonl`)
	if !re.MatchString(logs.String()) {
		t.Fatalf("journal line:\n%s", logs.String())
	}
	// Once and only once: nothing to take, the embedder is not asked again.
	before := f.emb.Inputs.Load()
	if n := inboxTake(t, f); n != 0 || f.emb.Inputs.Load() != before {
		t.Fatalf("second look took %d", n)
	}
}

func TestInboxLeavesPartAndFreshFilesAlone(t *testing.T) {
	f, caps := importFixture(t)
	rows := f.macRows(t, caps...)
	f.inboxPut(t, "copying.jsonl.part", rows, time.Hour)
	fresh := f.inboxPut(t, "fresh.jsonl", rows, time.Second)
	if n := inboxTake(t, f); n != 0 {
		t.Fatalf("took %d", n)
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "copying.jsonl.part")) || !spoolHas(fresh) || f.count(t, "vec_captures") != 0 {
		t.Fatal("a .part or fresh file was touched")
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(fresh, old, old)
	if n := inboxTake(t, f); n != 1 {
		t.Fatalf("an old enough file was not taken: %d", n)
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "copying.jsonl.part")) {
		t.Fatal("the .part file moved")
	}
}

func TestInboxFileRunningAtStartIsInterruptedAndNotRun(t *testing.T) {
	f, caps := importFixture(t)
	if err := f.svc.ensureVectorInboxDirs(); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	src := writeImportFile(t, f.macRows(t, caps...))
	raw, _ := os.ReadFile(src)
	_ = os.WriteFile(filepath.Join(f.inboxDir("running"), "half.jsonl"), raw, 0o640)
	logs := fullTextLog(t)
	old := vectorStartDelay
	vectorStartDelay = time.Hour
	t.Cleanup(func() { vectorStartDelay = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.svc.StartVectorWorker(ctx)
	if spoolHas(filepath.Join(f.inboxDir("running"), "half.jsonl")) || !spoolHas(filepath.Join(f.inboxDir("interrupted"), "half.jsonl")) {
		t.Fatal("not moved to interrupted/")
	}
	if !strings.Contains(logs.String(), "moved to interrupted/ and not run again") {
		t.Fatalf("not logged:\n%s", logs.String())
	}
	if n := inboxTake(t, f); n != 0 || f.count(t, "vec_captures") != 0 {
		t.Fatal("an interrupted file was run")
	}
}

func TestInboxFileWaitsWhileTheRoutesJobRuns(t *testing.T) {
	f, caps := importFixture(t)
	f.inboxPut(t, "w.jsonl", f.macRows(t, caps...), time.Hour)
	job, ok := f.svc.beginVectorJob("route", "")
	if !ok {
		t.Fatal("could not begin")
	}
	if n := inboxTake(t, f); n != 0 || !spoolHas(filepath.Join(f.inboxDir("inbox"), "w.jsonl")) || f.count(t, "vec_captures") != 0 {
		t.Fatalf("the file ran beside the route's job (took %d)", n)
	}
	f.svc.finishVectorJob(job, VectorImport{}, nil)
	if n := inboxTake(t, f); n != 1 {
		t.Fatalf("after the job: took %d", n)
	}
}

func TestInboxRefusedFilesLandInRefusedWithTheirReasonAndWriteNothing(t *testing.T) {
	f, caps := importFixture(t)
	good := f.macRows(t, caps...)
	wrongModel := append([]importRow(nil), good...)
	wrongModel[len(wrongModel)-1].Model = strings.Repeat("0", 64)
	badSample := append([]importRow(nil), good...)
	for i := range badSample {
		badSample[i].Vec = embedder.FakeVector(fmt.Sprint("wrong", i))
	}
	f.inboxPut(t, "a-model.jsonl", wrongModel, 3*time.Hour)
	f.inboxPut(t, "b-sample.jsonl", badSample, 2*time.Hour)
	// a malformed row
	_ = f.svc.ensureVectorInboxDirs()
	bad := filepath.Join(f.inboxDir("inbox"), "c-bad.jsonl")
	_ = os.WriteFile(bad, []byte("{not a row}\n"), 0o640)
	at := time.Now().Add(-time.Hour)
	_ = os.Chtimes(bad, at, at)
	for i := 0; i < 3; i++ { // one file a pass
		if n := inboxTake(t, f); n != 1 {
			t.Fatalf("pass %d took %d", i, n)
		}
	}
	if f.count(t, "vec_story_passages")+f.count(t, "vec_captures") != 0 {
		t.Fatal("a refused file wrote rows")
	}
	for name, want := range map[string]string{"a-model.jsonl": "names model", "b-sample.jsonl": "cosine", "c-bad.jsonl": "not a row"} {
		if !spoolHas(filepath.Join(f.inboxDir("refused"), name)) {
			t.Fatalf("%s not in refused/", name)
		}
		j := readJob(t, filepath.Join(f.inboxDir("refused"), name+".job.json"))
		if j.State != "refused" || !strings.Contains(j.Error, want) {
			t.Fatalf("%s: %+v, want a reason containing %q", name, j, want)
		}
	}
	// not retried: the inbox is empty and a second look takes nothing
	if n := inboxTake(t, f); n != 0 {
		t.Fatalf("a refused file was retried: %d", n)
	}
}

func TestInboxOversizedAndUnreadableFilesAreRefusedNotRetried(t *testing.T) {
	f, caps := importFixture(t)
	f.inboxPut(t, "big.jsonl", f.macRows(t, caps...), time.Hour)
	old := vectorImportMaxBytes
	vectorImportMaxBytes = 100
	t.Cleanup(func() { vectorImportMaxBytes = old })
	if n := inboxTake(t, f); n != 1 {
		t.Fatalf("took %d", n)
	}
	j := readJob(t, filepath.Join(f.inboxDir("refused"), "big.jsonl.job.json"))
	if j.State != "refused" || !strings.Contains(j.Error, "over the") {
		t.Fatalf("oversized: %+v", j)
	}
	vectorImportMaxBytes = old
	unreadable := f.inboxPut(t, "locked.jsonl", f.macRows(t, caps...), time.Hour)
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}
	inboxTake(t, f)
	j = readJob(t, filepath.Join(f.inboxDir("refused"), "locked.jsonl.job.json"))
	if j.State == "done" || spoolHas(unreadable) {
		t.Fatalf("unreadable: %+v", j)
	}
	if n := inboxTake(t, f); n != 0 {
		t.Fatalf("retried: %d", n)
	}
}

func TestInboxSymlinkIsRefusedAndNeverFollowed(t *testing.T) {
	f, caps := importFixture(t)
	target := writeImportFile(t, f.macRows(t, caps...))
	if err := f.svc.ensureVectorInboxDirs(); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.inboxDir("inbox"), "link.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	// the link's own modification time, old enough to be taken
	if err := exec.Command("touch", "-h", "-d", "2 hours ago", link).Run(); err != nil {
		t.Skipf("touch -h -d is not available: %v", err)
	}
	if n := inboxTake(t, f); n != 1 {
		t.Fatalf("took %d", n)
	}
	if f.count(t, "vec_captures")+f.count(t, "vec_story_passages") != 0 {
		t.Fatal("the symlink's target was imported")
	}
	if !spoolHas(target) {
		t.Fatal("the target was moved or removed")
	}
	j := readJob(t, filepath.Join(f.inboxDir("refused"), "link.jsonl.job.json"))
	if j.State != "refused" || !strings.Contains(j.Error, "not a regular file") {
		t.Fatalf("symlink: %+v", j)
	}
}

func TestInboxIsTakenByAPassBeforeTheGapWork(t *testing.T) {
	f, caps := importFixture(t)
	f.inboxPut(t, "pass.jsonl", f.macRows(t, caps...), time.Hour)
	res := f.pass(t)
	if res.StoryVersions != 0 || res.Captures != 0 || !spoolHas(filepath.Join(f.inboxDir("done"), "pass.jsonl")) {
		t.Fatalf("the pass embedded what the file held: %+v", res)
	}
	if f.count(t, "vec_story_passages") != 8 {
		t.Fatalf("rows %d", f.count(t, "vec_story_passages"))
	}
}

// ---- round 5: three effects, three callers ----

// effectEmbedder gives the embedder's three outcomes on demand: an input holding
// TOOLONG is refused by name, one holding FLAKY fails in transport, and down
// makes every call a server error that names no input.
type effectEmbedder struct {
	inner    embedder.Embedder
	down     bool
	allFlaky bool
}

func (e *effectEmbedder) Health(ctx context.Context) error { return e.inner.Health(ctx) }
func (e *effectEmbedder) Embed(ctx context.Context, in []string) ([][]float32, error) {
	if e.down {
		return nil, &embedder.StatusError{Code: 500, Body: "internal error"}
	}
	for _, x := range in {
		if strings.Contains(x, "TOOLONG") {
			return nil, &embedder.InputRefusedError{Code: 500, Reason: "input (3000 tokens) is too large to process"}
		}
		if e.allFlaky || strings.Contains(x, "FLAKY") {
			return nil, errors.New("connection reset by peer")
		}
	}
	return e.inner.Embed(ctx, in)
}

// effectFile builds a file of 30 plain captures, 10 TOOLONG ones and 10 FLAKY ones.
func effectFile(t *testing.T, f *vecFixture) []importRow {
	t.Helper()
	base := time.Now().UTC().Add(-100 * time.Hour)
	var ids []string
	for _, tag := range []string{"plain", "TOOLONG", "FLAKY"} {
		n := 30
		if tag != "plain" {
			n = 10
		}
		ids = append(ids, f.captures(t, "o1", "c1", tag, n, base, func(i int) string { return fmt.Sprintf("%s text number%d", tag, i) }, time.Minute)...)
	}
	return f.macRows(t, ids...)
}

func TestEmbedEffectsReachTheWorkerTheImportSampleAndTheInboxTheSameWay(t *testing.T) {
	ctx := context.Background()
	t.Run("refused: only that input is recorded; the sample sets it aside and the file imports; the inbox imports", func(t *testing.T) {
		// worker
		f := newVecFixture(t)
		effectFile(t, f)
		f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
		res := f.pass(t)
		if res.Refused != 10 || res.Captures != 30 || res.Skipped != 10 {
			t.Fatalf("worker: %+v (10 refused by name, 10 failed in transport and skipped, 30 embedded)", res)
		}
		// import sample
		f2 := newVecFixture(t)
		rows := effectFile(t, f2)
		f2.store.SetEmbedder(&effectEmbedder{inner: f2.emb})
		path := writeImportFile(t, rows)
		ir, err := f2.svc.ImportVectorFile(ctx, path)
		if err != nil || ir.Rejected["oversized"] != 10 || ir.Written != 40 {
			t.Fatalf("import: %+v %v", ir, err)
		}
		// inbox
		f3 := newVecFixture(t)
		rows = effectFile(t, f3)
		f3.store.SetEmbedder(&effectEmbedder{inner: f3.emb})
		f3.inboxPut(t, "e.jsonl", rows, time.Hour)
		if n := inboxTake(t, f3); n != 1 || !spoolHas(filepath.Join(f3.inboxDir("done"), "e.jsonl")) {
			t.Fatalf("inbox took %d", n)
		}
		// and the worker then records its own refusal for the rows the import set aside
		if res := f3.pass(t); res.Refused != 10 {
			t.Fatalf("worker after the import: %+v", res)
		}
	})
	t.Run("down: nothing durable anywhere; the same work is found again", func(t *testing.T) {
		f := newVecFixture(t)
		rows := effectFile(t, f)
		eff := &effectEmbedder{inner: f.emb, down: true}
		f.store.SetEmbedder(eff)
		if res, err := f.svc.RunVectorPass(ctx); !errors.Is(err, errEmbedderDown) || res.Refused != 0 || f.count(t, "vec_captures") != 0 {
			t.Fatalf("worker: %+v %v", res, err)
		}
		path := writeImportFile(t, rows)
		if _, err := f.svc.ImportVectorFile(ctx, path); !errors.Is(err, errEmbedderDown) || errors.Is(err, ErrVectorImportRefused) || f.count(t, "vec_captures") != 0 {
			t.Fatalf("import: %v", err)
		}
		f.inboxPut(t, "d.jsonl", rows, time.Hour)
		n, err := f.svc.ProcessVectorInbox(ctx)
		if n != 0 || !errors.Is(err, errEmbedderDown) {
			t.Fatalf("inbox: %d %v (a file put back is not taken)", n, err)
		}
		if !spoolHas(filepath.Join(f.inboxDir("inbox"), "d.jsonl")) || spoolHas(filepath.Join(f.inboxDir("refused"), "d.jsonl")) || spoolHas(filepath.Join(f.inboxDir("done"), "d.jsonl")) || len(globJobs(f)) != 0 {
			t.Fatal("the file did not stay in inbox/ untouched")
		}
		eff.down = false
		if n := inboxTake(t, f); n != 1 || !spoolHas(filepath.Join(f.inboxDir("done"), "d.jsonl")) {
			t.Fatalf("next pass took %d", n)
		}
	})
	t.Run("transport: that input is skipped, nothing written for it; a file that cannot be checked waits", func(t *testing.T) {
		f := newVecFixture(t)
		rows := effectFile(t, f)
		f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
		res, err := f.svc.RunVectorPass(ctx)
		if err != nil || res.Skipped < 10 {
			t.Fatalf("worker: %+v %v", res, err)
		}
		var flaky int
		_ = f.store.corpus().QueryRow(`SELECT COUNT(*) FROM vec_captures v JOIN og_objects o ON o.canonical_id = v.capture_id WHERE o.metadata LIKE '%FLAKY%'`).Scan(&flaky)
		if flaky != 0 {
			t.Fatalf("%d rows written for inputs that failed in transport", flaky)
		}
		// every call failing: too few rows can be checked; the file is not refused, it waits
		f2 := newVecFixture(t)
		rows = effectFile(t, f2)
		eff := &effectEmbedder{inner: f2.emb, allFlaky: true}
		f2.store.SetEmbedder(eff)
		_, err = f2.svc.ImportVectorFile(ctx, writeImportFile(t, rows))
		if !errors.Is(err, errVectorImportCannotCheck) || errors.Is(err, ErrVectorImportRefused) || f2.count(t, "vec_captures") != 0 {
			t.Fatalf("import: %v", err)
		}
		f2.inboxPut(t, "t.jsonl", rows, time.Hour)
		if n, err := f2.svc.ProcessVectorInbox(ctx); n != 0 || err != nil || !spoolHas(filepath.Join(f2.inboxDir("inbox"), "t.jsonl")) || len(globJobs(f2)) != 0 {
			t.Fatalf("inbox: %d %v", n, err)
		}
		eff.allFlaky = false
		if n := inboxTake(t, f2); n != 1 || !spoolHas(filepath.Join(f2.inboxDir("done"), "t.jsonl")) {
			t.Fatalf("later pass took %d", n)
		}
		_ = f
	})
}

func globJobs(f *vecFixture) []string {
	var out []string
	for _, d := range []string{"done", "refused"} {
		m, _ := filepath.Glob(filepath.Join(f.inboxDir(d), "*.job.json"))
		out = append(out, m...)
	}
	return out
}

func TestInboxNameCollisionsNeverOverwriteADirectoryIsRefusedAndOneFileGoesPerPass(t *testing.T) {
	f, caps := importFixture(t)
	rows := f.macRows(t, caps...)
	f.inboxPut(t, "x.jsonl", rows, 2*time.Hour)
	if err := os.WriteFile(filepath.Join(f.inboxDir("done"), "x.jsonl"), []byte("earlier"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.inboxDir("done"), "x.jsonl.job.json"), []byte("earlier record"), 0o640); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.inboxDir("inbox"), "dir.jsonl")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Hour)
	_ = os.Chtimes(dir, at, at)
	f.inboxPut(t, "y.jsonl", rows, time.Hour)
	if n := inboxTake(t, f); n != 1 {
		t.Fatalf("pass 1 took %d (one file a pass)", n)
	}
	if b, _ := os.ReadFile(filepath.Join(f.inboxDir("done"), "x.jsonl")); string(b) != "earlier" {
		t.Fatal("an earlier done/ file was overwritten")
	}
	if b, _ := os.ReadFile(filepath.Join(f.inboxDir("done"), "x.jsonl.job.json")); string(b) != "earlier record" {
		t.Fatal("an earlier job record was overwritten")
	}
	if !spoolHas(filepath.Join(f.inboxDir("done"), "x.jsonl.2")) || !spoolHas(filepath.Join(f.inboxDir("done"), "x.jsonl.2.job.json")) {
		t.Fatal("the new file was not given a free name")
	}
	if spoolHas(filepath.Join(f.inboxDir("inbox"), "dir.jsonl")) == false && spoolHas(filepath.Join(f.inboxDir("inbox"), "y.jsonl")) == false {
		t.Fatal("two files went in one pass")
	}
	inboxTake(t, f)
	inboxTake(t, f)
	if !spoolHas(filepath.Join(f.inboxDir("refused"), "dir.jsonl")) {
		t.Fatal("a directory named *.jsonl was not moved to refused/")
	}
	j := readJob(t, filepath.Join(f.inboxDir("refused"), "dir.jsonl.job.json"))
	if j.State != "refused" || !strings.Contains(j.Error, "not a regular file") {
		t.Fatalf("directory: %+v", j)
	}
}

func TestJournalErrorTextHasNoNewlines(t *testing.T) {
	f := newVecFixture(t)
	logs := fullTextLog(t)
	job, _ := f.svc.beginVectorJob("route", "")
	f.svc.finishVectorJob(job, VectorImport{}, fmt.Errorf("%w: line one\nline two\r\nline three", ErrVectorImportRefused))
	var lines int
	for _, ln := range strings.Split(logs.String(), "\n") {
		if strings.Contains(ln, "op=vector_import") {
			lines++
			if !strings.Contains(ln, "line_one_line_two__line_three") {
				t.Fatalf("error text: %q", ln)
			}
		}
	}
	if lines != 1 || strings.Contains(logs.String(), "line two\n") {
		t.Fatalf("%d journal lines:\n%s", lines, logs.String())
	}
}

func TestVectorSearchOfACancelledCallerIsAnErrorOnTheIndexPathToo(t *testing.T) {
	f := newVecFixture(t)
	f.captures(t, "o1", "c1", "p", 10, time.Now().UTC().Add(-50*time.Hour), func(i int) string { return fmt.Sprintf("alpha %d", i) }, time.Minute)
	f.pass(t)
	old, oldExact := vectorSearchStageHook, vectorExactMaxRows
	vectorExactMaxRows = 0 // the index path
	t.Cleanup(func() { vectorSearchStageHook, vectorExactMaxRows = old, oldExact })
	ctx, cancel := context.WithCancel(context.Background())
	vectorSearchStageHook = func(c context.Context, tx *sql.Tx, stage string) error {
		if stage == "index" {
			cancel()
			return c.Err()
		}
		return nil
	}
	if _, err := f.store.SearchVectors(ctx, VectorSearchRequest{Query: "alpha", Owner: "o1", Computer: "c1"}); err == nil {
		t.Fatal("a cancelled caller got a result on the index path")
	}
}

func TestVectorEnsureMigratesWhenOnlyOneTableExists(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`DROP TABLE vec_story_passages, vec_captures`,
		`CREATE TABLE vec_captures (capture_id TEXT NOT NULL PRIMARY KEY, owner_id TEXT NOT NULL, computer_id TEXT NOT NULL, text_sha TEXT NOT NULL, day DATE NOT NULL, model TEXT NOT NULL, embedding halfvec(256) NOT NULL, embedded_on DATE NOT NULL)`,
	} {
		if _, err := f.store.corpus().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatalf("ensure with one old table: %v", err)
	}
	if !f.store.VectorsReady() {
		t.Fatal("not ready after the first ensure")
	}
}

func TestAFileWhoseRowsCannotBeReadNowWaitsAndIsNotRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}
	f := newVecFixture(t)
	ids := f.captures(t, "o1", "c1", "u", 40, time.Now().UTC().Add(-50*time.Hour), func(i int) string { return fmt.Sprintf("unreadable text number%d", i) }, time.Minute)
	rows := f.macRows(t, ids...)
	// the blobs become unreadable (a disk fault, a permission): the host's state, not the file's
	objs, _ := f.store.captureObjectsByIDAny(context.Background(), ids)
	for _, o := range objs {
		meta, _ := objectgraph.WebCaptureMetadataFromObject(o)
		p := f.svc.captureBodies.path(strings.TrimPrefix(meta.ExtractedTextBlobID, "sha256:"))
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
	}
	_, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if !errors.Is(err, errVectorImportCannotCheck) || errors.Is(err, ErrVectorImportRefused) {
		t.Fatalf("a file whose rows the host cannot read now: %v", err)
	}
}

// ---- round 6: a file that waits is not work ----

func TestADeferredFileIsNotTakenSoThePassWaitsAndTheIdleMarkStays(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "A", storyText("aaa"), time.Time{})
	f.pass(t)
	if !f.svc.vecIdle.valid {
		t.Fatal("no idle mark after a complete walk")
	}
	ids := f.captures(t, "o1", "c1", "plain", 40, time.Now().UTC().Add(-100*time.Hour), func(i int) string { return fmt.Sprintf("plain text number%d", i) }, time.Minute)
	rows := f.macRows(t, ids...)
	eff := &effectEmbedder{inner: f.emb, allFlaky: true}
	f.store.SetEmbedder(eff)
	f.inboxPut(t, "w.jsonl", rows, time.Hour)
	for i := 0; i < 3; i++ {
		res, err := f.svc.RunVectorPass(context.Background())
		if err != nil || res.Worked() || res.Imports != 0 {
			t.Fatalf("pass %d with a waiting file counted as work: %+v %v (a worked pass runs again at once)", i, res, err)
		}
		if !f.svc.vecIdle.valid {
			t.Fatalf("pass %d cleared the idle mark", i)
		}
		if !spoolHas(filepath.Join(f.inboxDir("inbox"), "w.jsonl")) {
			t.Fatal("the file left the inbox")
		}
	}
	// a file that waits leaves no job entry behind; finished jobs are bounded
	f.store.vec.importMu.Lock()
	n := len(f.store.vec.jobs)
	f.store.vec.importMu.Unlock()
	if n != 0 {
		t.Fatalf("%d job entries for a file that waits", n)
	}
	for i := 0; i < vectorJobsKept+10; i++ {
		j, _ := f.svc.beginVectorJob("route", "")
		f.svc.finishVectorJob(j, VectorImport{}, nil)
	}
	f.store.vec.importMu.Lock()
	n = len(f.store.vec.jobs)
	f.store.vec.importMu.Unlock()
	if n > vectorJobsKept {
		t.Fatalf("%d job entries kept, want at most %d", n, vectorJobsKept)
	}
}

func TestAWaitingFileDoesNotStarveANewerOne(t *testing.T) {
	f, caps := importFixture(t)
	// the older file holds only FLAKY rows: it cannot be checked while those fail
	base := time.Now().UTC().Add(-100 * time.Hour)
	flaky := f.captures(t, "o1", "c1", "FLAKY", 40, base, func(i int) string { return fmt.Sprintf("FLAKY text number%d", i) }, time.Minute)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	f.inboxPut(t, "a-old.jsonl", f.macRows(t, flaky...), 5*time.Hour)
	f.inboxPut(t, "b-new.jsonl", f.macRows(t, caps...), time.Hour)
	if n := inboxTake(t, f); n != 0 {
		t.Fatalf("the old file should have waited, took %d", n)
	}
	if n := inboxTake(t, f); n != 1 || !spoolHas(filepath.Join(f.inboxDir("done"), "b-new.jsonl")) {
		t.Fatalf("the newer file was starved by the waiting one (took %d)", n)
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "a-old.jsonl")) {
		t.Fatal("the waiting file was lost")
	}
}

func TestACutMismatchRefusesTheFileEvenWhenSomeCandidatesWereSetAside(t *testing.T) {
	f := newVecFixture(t)
	rows := effectFile(t, f)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	// every plain row's text is cut differently by the Mac; the TOOLONG and FLAKY ones are as before
	plain := 0
	for i := range rows {
		meta, _ := f.store.captureObjectsByIDAny(context.Background(), []string{rows[i].ID})
		o := meta[rows[i].ID]
		if strings.Contains(string(o.Metadata), "T plain-") {
			rows[i].TextSHA = embedder.TextSHA("another cut")
			plain++
		}
	}
	if plain != 30 {
		t.Fatalf("plain rows %d", plain)
	}
	_, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if !errors.Is(err, ErrVectorImportRefused) || !strings.Contains(err.Error(), "the text or its cut differs") {
		t.Fatalf("a file this host cuts differently must be refused, not wait: %v", err)
	}
}

// ---- round 7: which waiter is retried, and when a short sample waits ----
// Failure list, written before the code:
//   a. TestOnlyTheOldestWaitingFileIsRetriedSoANewerWaiterIsNeverTried: two
//      deferred files, the older never importable; the newer is never retried;
//   b. TestAFileWhoseCheckableRowsAreAllSetAsideIsRefusedNotLeftWaiting: rows set
//      aside for the file's own reason (too large) can never be checked, so the
//      file waits for ever;
//   c. TestAFileWhoseShortfallIsOnlyTransientSkipsStaysInTheInbox: the guard that
//      repair (b) does not refuse a file the server merely could not answer for;
//   d. the existing waiting-file tests stay as they are.

// stuckEmbedder fails in transport on an input holding STUCK always, and on one
// holding LATER until open is set.
type stuckEmbedder struct {
	inner embedder.Embedder
	open  atomic.Bool
}

func (e *stuckEmbedder) Health(ctx context.Context) error { return e.inner.Health(ctx) }
func (e *stuckEmbedder) Embed(ctx context.Context, in []string) ([][]float32, error) {
	for _, x := range in {
		if strings.Contains(x, "STUCK") || (strings.Contains(x, "LATER") && !e.open.Load()) {
			return nil, errors.New("connection reset by peer")
		}
	}
	return e.inner.Embed(ctx, in)
}

func TestOnlyTheOldestWaitingFileIsRetriedSoANewerWaiterIsNeverTried(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	stuck := f.captures(t, "o1", "c1", "STUCK", 40, base, func(i int) string { return fmt.Sprintf("STUCK text number%d", i) }, time.Minute)
	later := f.captures(t, "o1", "c1", "LATER", 40, base, func(i int) string { return fmt.Sprintf("LATER text number%d", i) }, time.Minute)
	emb := &stuckEmbedder{inner: f.emb}
	f.store.SetEmbedder(emb)
	f.inboxPut(t, "a-old.jsonl", f.macRows(t, stuck...), 5*time.Hour)
	f.inboxPut(t, "b-new.jsonl", f.macRows(t, later...), time.Hour)
	for i := 0; i < 2; i++ { // both are tried once and both wait
		if n := inboxTake(t, f); n != 0 {
			t.Fatalf("pass %d took %d", i, n)
		}
	}
	emb.open.Store(true) // the newer file can be imported now; the older never can
	for i := 0; i < 4; i++ {
		inboxTake(t, f)
	}
	if !spoolHas(filepath.Join(f.inboxDir("done"), "b-new.jsonl")) {
		t.Fatal("the newer waiting file was never tried again: the oldest waiter starves it")
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "a-old.jsonl")) {
		t.Fatal("the stuck file left the inbox")
	}
}

func TestAFileWhoseCheckableRowsAreAllSetAsideIsRefusedNotLeftWaiting(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	ids := f.captures(t, "o1", "c1", "TOOLONG", 40, base, func(i int) string { return fmt.Sprintf("TOOLONG text number%d", i) }, time.Minute)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	_, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, f.macRows(t, ids...)))
	if !errors.Is(err, ErrVectorImportRefused) || errors.Is(err, errVectorImportCannotCheck) {
		t.Fatalf("a file whose rows are all too large for this host must be refused, not wait: %v", err)
	}
}

func TestAFileWhoseShortfallIsOnlyTransientSkipsStaysInTheInbox(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	ids := f.captures(t, "o1", "c1", "FLAKY", 40, base, func(i int) string { return fmt.Sprintf("FLAKY text number%d", i) }, time.Minute)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	f.inboxPut(t, "t.jsonl", f.macRows(t, ids...), time.Hour)
	if n := inboxTake(t, f); n != 0 {
		t.Fatalf("took %d", n)
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "t.jsonl")) || spoolHas(filepath.Join(f.inboxDir("refused"), "t.jsonl")) {
		t.Fatal("a file the server could not answer for was not left in the inbox")
	}
}

// ---- round 7, review: a short draw of a big file is not proof ----
// Failure list, written before the code:
//   e. TestAShortDrawOfAFileOverTheCandidateCapWaitsAtFirst: a file of more rows
//      than the candidate cap, mostly set aside, must not be refused on one random
//      draw (a guard: the old code waited for ever on it, it does not fail there);
//   f. TestAFileOverTheCandidateCapThatStaysShortIsRefusedAfterSeveralDraws: it
//      must not wait for ever either;
//   g. TestTheRefusalOfAShortSampleNamesTheCountsOfEveryKind: the record the owner
//      reads says checked, floor, and each kind of row not checked.

// bigMostlyTooLong builds a file of vectorImportCandidates+100 captures of which
// only 20 can be checked by this host (the floor is 32, so it is short whatever
// the draw).
func bigMostlyTooLong(t *testing.T, f *vecFixture) []importRow {
	t.Helper()
	base := time.Now().UTC().Add(-100 * time.Hour)
	n := vectorImportCandidates + 100
	plain := f.captures(t, "o1", "c1", "plain", 20, base, func(i int) string { return fmt.Sprintf("plain text number%d", i) }, time.Minute)
	long := f.captures(t, "o1", "c1", "TOOLONG", n-20, base, func(i int) string { return fmt.Sprintf("TOOLONG text number%d", i) }, time.Minute)
	return f.macRows(t, append(plain, long...)...)
}

func TestAShortDrawOfAFileOverTheCandidateCapWaitsAtFirst(t *testing.T) {
	f := newVecFixture(t)
	rows := bigMostlyTooLong(t, f)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	_, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, rows))
	if !errors.Is(err, errVectorImportCannotCheck) || errors.Is(err, ErrVectorImportRefused) {
		t.Fatalf("one draw of %d candidates from %d rows proves nothing about the rest: %v", vectorImportCandidates, len(rows), err)
	}
}

func TestAFileOverTheCandidateCapThatStaysShortIsRefusedAfterSeveralDraws(t *testing.T) {
	f := newVecFixture(t)
	rows := bigMostlyTooLong(t, f)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	f.inboxPut(t, "big.jsonl", rows, time.Hour)
	for i := 0; i < vectorInboxShortDraws-1; i++ {
		if n := inboxTake(t, f); n != 0 || !spoolHas(filepath.Join(f.inboxDir("inbox"), "big.jsonl")) {
			t.Fatalf("draw %d: the file should still wait (took %d)", i+1, n)
		}
	}
	if n := inboxTake(t, f); n != 1 || !spoolHas(filepath.Join(f.inboxDir("refused"), "big.jsonl")) {
		t.Fatalf("after %d short draws the file must be refused (took %d)", vectorInboxShortDraws, n)
	}
}

func TestTheRefusalOfAShortSampleNamesTheCountsOfEveryKind(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	ids := f.captures(t, "o1", "c1", "TOOLONG", 40, base, func(i int) string { return fmt.Sprintf("TOOLONG text number%d", i) }, time.Minute)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb})
	_, err := f.svc.ImportVectorFile(context.Background(), writeImportFile(t, f.macRows(t, ids...)))
	if !errors.Is(err, ErrVectorImportRefused) {
		t.Fatal(err)
	}
	for _, want := range []string{"only 0 rows", "need 20", "40 are too large", "0 failed this call"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal lacks %q: %v", want, err)
		}
	}
}

// ---- round 8 ----
// Failure list, written before the code:
//   j. TestABigFileWhoseRowsAreAllSkippedNowIsNeverRefused: a file over the
//      candidate cap whose every row fails in transport is the host's state, not
//      the file's; it counted as a short draw and was refused after three passes;
//   k. TestAPassYieldsToAFilePlacedMidPass: a pass embedding the gap does not
//      look at the inbox until it ends (days on the paper's first pass);
//   l. TestTwoFilesPlacedMidPassAreBothImportedInOrderBeforeTheGapIsDone;
//   m. TestAPermanentlyWaitingFileDoesNotMakePassesSpin;
//   n. TestAWaiterTriedLongAgoIsYieldedToAgain: the bound on (m) is the idle
//      interval, not for ever.

func TestABigFileWhoseRowsAreAllSkippedNowIsNeverRefused(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	ids := f.captures(t, "o1", "c1", "plain", vectorImportCandidates+100, base, func(i int) string { return fmt.Sprintf("plain text number%d", i) }, time.Minute)
	f.store.SetEmbedder(&effectEmbedder{inner: f.emb, allFlaky: true})
	f.inboxPut(t, "skip.jsonl", f.macRows(t, ids...), time.Hour)
	for i := 0; i < vectorInboxShortDraws+1; i++ {
		inboxTake(t, f)
		if !spoolHas(filepath.Join(f.inboxDir("inbox"), "skip.jsonl")) {
			t.Fatalf("pass %d: a file short only because the server could not answer was moved out of the inbox", i+1)
		}
	}
}

// placeOnFirstEmbed puts the given files in the inbox when the embedder is first
// asked for a batch, that is in the middle of a pass, in the pass's own goroutine.
type placeOnFirstEmbed struct {
	inner embedder.Embedder
	once  sync.Once
	place func()
}

func (e *placeOnFirstEmbed) Health(ctx context.Context) error { return e.inner.Health(ctx) }
func (e *placeOnFirstEmbed) Embed(ctx context.Context, in []string) ([][]float32, error) {
	e.once.Do(e.place)
	return e.inner.Embed(ctx, in)
}

func withCheckEvery(d time.Duration) func() {
	old := vectorInboxCheckEvery
	vectorInboxCheckEvery = d
	return func() { vectorInboxCheckEvery = old }
}

func TestAPassYieldsToAFilePlacedMidPass(t *testing.T) {
	defer withCheckEvery(0)()
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	gapIDs := f.captures(t, "o1", "c1", "gap", 20, base, func(i int) string { return fmt.Sprintf("gap text number%d", i) }, time.Minute)
	fileIDs := f.captures(t, "o1", "c1", "file", 10, base, func(i int) string { return fmt.Sprintf("file text number%d", i) }, time.Minute)
	rows := f.macRows(t, fileIDs...)
	total := len(gapIDs) + len(fileIDs)
	f.store.SetEmbedder(&placeOnFirstEmbed{inner: f.emb, place: func() { f.inboxPut(t, "m.jsonl", rows, time.Hour) }})
	res := f.pass(t)
	if !res.Yielded {
		t.Fatalf("the pass did not yield to the file: %+v", res)
	}
	if got := f.count(t, "vec_captures"); got >= total {
		t.Fatalf("the gap was done (%d of %d) before the pass yielded", got, total)
	}
	res = f.pass(t)
	if res.Imports != 1 || res.Yielded || !spoolHas(filepath.Join(f.inboxDir("done"), "m.jsonl")) {
		t.Fatalf("the next pass should take the file and finish the gap: %+v", res)
	}
	if got := f.count(t, "vec_captures"); got != total {
		t.Fatalf("%d captures with vectors, want %d", got, total)
	}
}

func TestTwoFilesPlacedMidPassAreBothImportedInOrderBeforeTheGapIsDone(t *testing.T) {
	defer withCheckEvery(0)()
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	gapIDs := f.captures(t, "o1", "c1", "gap", 40, base, func(i int) string { return fmt.Sprintf("gap text number%d", i) }, time.Minute)
	ids1 := f.captures(t, "o1", "c1", "one", 10, base, func(i int) string { return fmt.Sprintf("one text number%d", i) }, time.Minute)
	ids2 := f.captures(t, "o1", "c1", "two", 10, base, func(i int) string { return fmt.Sprintf("two text number%d", i) }, time.Minute)
	rows1, rows2 := f.macRows(t, ids1...), f.macRows(t, ids2...)
	total := len(gapIDs) + len(ids1) + len(ids2)
	f.store.SetEmbedder(&placeOnFirstEmbed{inner: f.emb, place: func() {
		f.inboxPut(t, "a.jsonl", rows1, 2*time.Hour)
		f.inboxPut(t, "b.jsonl", rows2, time.Hour)
	}})
	if res := f.pass(t); !res.Yielded {
		t.Fatalf("pass 1 did not yield: %+v", res)
	}
	res := f.pass(t)
	if res.Imports != 1 || !res.Yielded || !spoolHas(filepath.Join(f.inboxDir("done"), "a.jsonl")) || spoolHas(filepath.Join(f.inboxDir("done"), "b.jsonl")) {
		t.Fatalf("pass 2 should take a.jsonl only and yield for b.jsonl: %+v", res)
	}
	if got := f.count(t, "vec_captures"); got >= total {
		t.Fatalf("the gap was done (%d of %d) before the second file was taken", got, total)
	}
	res = f.pass(t)
	if res.Imports != 1 || res.Yielded || !spoolHas(filepath.Join(f.inboxDir("done"), "b.jsonl")) {
		t.Fatalf("pass 3 should take b.jsonl and finish: %+v", res)
	}
	if got := f.count(t, "vec_captures"); got != total {
		t.Fatalf("%d captures with vectors, want %d", got, total)
	}
}

func TestAPermanentlyWaitingFileDoesNotMakePassesSpin(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	f.captures(t, "o1", "c1", "gap", 40, base, func(i int) string { return fmt.Sprintf("gap text number%d", i) }, time.Minute)
	stuck := f.captures(t, "o1", "c1", "STUCK", 40, base, func(i int) string { return fmt.Sprintf("STUCK text number%d", i) }, time.Minute)
	logs := fullTextLog(t)
	f.store.SetEmbedder(&stuckEmbedder{inner: f.emb})
	f.inboxPut(t, "stuck.jsonl", f.macRows(t, stuck...), time.Hour)
	// The timings are read once by StartVectorWorker, so they are put back at
	// once, before the worker has run a pass.
	oStart, oIdle, oCheck := vectorStartDelay, vectorIdleInterval, vectorInboxCheckEvery
	vectorStartDelay, vectorIdleInterval, vectorInboxCheckEvery = 0, time.Hour, 0
	ctx, cancel := context.WithCancel(context.Background())
	f.svc.StartVectorWorker(ctx)
	vectorStartDelay, vectorIdleInterval, vectorInboxCheckEvery = oStart, oIdle, oCheck
	defer cancel()
	deadline := time.Now().Add(20 * time.Second)
	for f.count(t, "vec_captures") < 40 {
		if time.Now().After(deadline) {
			t.Fatalf("the gap was not embedded (%d)", f.count(t, "vec_captures"))
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	// every pass that tries the waiter says so once
	if n := strings.Count(logs.String(), "stuck.jsonl was not imported now"); n > 4 {
		t.Fatalf("the waiting file was tried %d times in the window", n)
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "stuck.jsonl")) {
		t.Fatal("the waiting file left the inbox")
	}
}

func TestAWaiterTriedLongAgoIsYieldedToAgain(t *testing.T) {
	defer withCheckEvery(0)()
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-100 * time.Hour)
	f.captures(t, "o1", "c1", "gap", 20, base, func(i int) string { return fmt.Sprintf("gap text number%d", i) }, time.Minute)
	stuck := f.captures(t, "o1", "c1", "STUCK", 40, base, func(i int) string { return fmt.Sprintf("STUCK text number%d", i) }, time.Minute)
	f.store.SetEmbedder(&stuckEmbedder{inner: f.emb})
	f.inboxPut(t, "stuck.jsonl", f.macRows(t, stuck...), time.Hour)
	oIdle := vectorIdleInterval
	defer func() { vectorIdleInterval = oIdle }()
	vectorIdleInterval = time.Hour
	if res := f.pass(t); res.Yielded {
		t.Fatalf("a waiter tried at the start of this pass must not make it yield: %+v", res)
	}
	// more gap work, and the interval has run out for the waiter
	f.captures(t, "o1", "c1", "more", 20, base, func(i int) string { return fmt.Sprintf("more text number%d", i) }, time.Minute)
	vectorIdleInterval = 0
	if res := f.pass(t); !res.Yielded {
		t.Fatalf("a waiter last tried more than the idle interval ago should be yielded to: %+v", res)
	}
}
