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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 1 {
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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 0 || f.emb.Inputs.Load() != before {
		t.Fatalf("second look took %d", n)
	}
}

func TestInboxLeavesPartAndFreshFilesAlone(t *testing.T) {
	f, caps := importFixture(t)
	rows := f.macRows(t, caps...)
	f.inboxPut(t, "copying.jsonl.part", rows, time.Hour)
	fresh := f.inboxPut(t, "fresh.jsonl", rows, time.Second)
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 0 {
		t.Fatalf("took %d", n)
	}
	if !spoolHas(filepath.Join(f.inboxDir("inbox"), "copying.jsonl.part")) || !spoolHas(fresh) || f.count(t, "vec_captures") != 0 {
		t.Fatal("a .part or fresh file was touched")
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(fresh, old, old)
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 1 {
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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 0 || f.count(t, "vec_captures") != 0 {
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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 0 || !spoolHas(filepath.Join(f.inboxDir("inbox"), "w.jsonl")) || f.count(t, "vec_captures") != 0 {
		t.Fatalf("the file ran beside the route's job (took %d)", n)
	}
	f.svc.finishVectorJob(job, VectorImport{}, nil)
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 1 {
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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 3 {
		t.Fatalf("took %d", n)
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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 0 {
		t.Fatalf("a refused file was retried: %d", n)
	}
}

func TestInboxOversizedAndUnreadableFilesAreRefusedNotRetried(t *testing.T) {
	f, caps := importFixture(t)
	f.inboxPut(t, "big.jsonl", f.macRows(t, caps...), time.Hour)
	old := vectorImportMaxBytes
	vectorImportMaxBytes = 100
	t.Cleanup(func() { vectorImportMaxBytes = old })
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 1 {
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
	f.svc.ProcessVectorInbox(context.Background())
	j = readJob(t, filepath.Join(f.inboxDir("refused"), "locked.jsonl.job.json"))
	if j.State == "done" || spoolHas(unreadable) {
		t.Fatalf("unreadable: %+v", j)
	}
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 0 {
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
	if n := f.svc.ProcessVectorInbox(context.Background()); n != 1 {
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
