package platform

// Failure list: the header of account_purge.go (1 to 7). Postgres runs in a
// throwaway cluster, the tape and artifacts live in t.TempDir(); no real
// computer, account or store is touched.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var acctNow = time.Now().UTC()
var acctOld = acctNow.Add(-100 * time.Hour)

// Failure 3: suspend hides takes at once and restore brings them back.
func TestSuspendedTakesAreNotServedAndRestoreServesThem(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	a := mustPublish(t, svc, "user-a", 1)
	b := mustPublish(t, svc, "user-b", 2)
	n, err := svc.SetAccountTakesState(ctx, "user-a", TakeStateLive, TakeStateSuspended)
	if err != nil || n != 1 {
		t.Fatalf("suspend: %d %v", n, err)
	}
	list, _ := svc.ListTakes(ctx, 50, false)
	for _, tk := range list.Takes {
		if tk.TakeID == a.TakeID {
			t.Fatal("a suspended take is listed")
		}
	}
	if audio, _ := svc.TakeAudio(ctx, a.TakeID); audio != nil {
		t.Fatal("a suspended take's audio is served")
	}
	if mine, _ := svc.ListMyTakes(ctx, types.TakePublisherKey("user-a")); len(mine.Takes) != 0 {
		t.Fatal("a suspended take is in the publisher's list")
	}
	if audio, _ := svc.TakeAudio(ctx, b.TakeID); audio == nil {
		t.Fatal("another publisher's take was suspended")
	}
	// Idempotent, then restored.
	if n, _ := svc.SetAccountTakesState(ctx, "user-a", TakeStateLive, TakeStateSuspended); n != 0 {
		t.Fatalf("second suspend moved %d", n)
	}
	if n, err := svc.SetAccountTakesState(ctx, "user-a", TakeStateSuspended, TakeStateLive); err != nil || n != 1 {
		t.Fatalf("restore: %d %v", n, err)
	}
	if audio, _ := svc.TakeAudio(ctx, a.TakeID); audio == nil {
		t.Fatal("a restored take is not served")
	}
}

// Failure 1.
func TestAccountPurgeRefusesWithoutAnOldRecord(t *testing.T) {
	svc := newTakesService(t)
	a := mustPublish(t, svc, "user-a", 1)
	for name, at := range map[string]time.Time{"none": {}, "young": acctNow.Add(-time.Hour), "short": acctNow.Add(-71 * time.Hour), "future": acctNow.Add(time.Hour)} {
		_, err := svc.PurgeAccountData(context.Background(), "user-a", nil, at, acctNow)
		if !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := svc.StartAccountTapePurge("user-a", []string{"dead-1"}, at, acctNow); !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("tape %s: %v", name, err)
		}
	}
	if audio, _ := svc.TakeAudio(context.Background(), a.TakeID); audio == nil {
		t.Fatal("a refused purge removed a take")
	}
}

// Failure 2, 4, 7: takes go, the piece stays with the byline detached, only
// the account's rows go, a second run succeeds.
func TestAccountPurgeDeletesTakesDetachesAuthorAndKeepsPieces(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	mine := mustPublish(t, svc, "user-a", 1)
	other := mustPublish(t, svc, "user-b", 2)
	if _, err := publishOpinion(svc, "user-a", "writer-aa11bb", "doc-a", 1, "My view", "Words about the harbor."); err != nil {
		t.Fatal(err)
	}
	if _, err := publishOpinion(svc, "user-b", "writer-bb22cc", "doc-b", 1, "Their view", "Words about the port."); err != nil {
		t.Fatal(err)
	}
	// A capture row of each account.
	og := svc.ogStore()
	for _, owner := range []string{"user-a", "user-b"} {
		if err := og.PutObject(corpusdWriter(ctx), capObject(owner, "cap-"+owner)); err != nil {
			t.Fatal(err)
		}
	}
	c, err := svc.PurgeAccountData(ctx, "user-a", []string{"computer-aaaa"}, acctOld, acctNow)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if c.TakesDeleted != 1 || c.AuthorsDetached != 1 || c.CapturesDeleted != 1 {
		t.Fatalf("counts: %+v", c)
	}
	if audio, _ := svc.TakeAudio(ctx, mine.TakeID); audio != nil {
		t.Fatal("the take is served")
	}
	if audio, _ := svc.TakeAudio(ctx, other.TakeID); audio == nil {
		t.Fatal("another account's take was removed")
	}
	rows := opinionRows(t, svc)
	if len(rows) != 2 {
		t.Fatalf("the pieces must stay: %d", len(rows))
	}
	for _, r := range rows {
		switch r.Title {
		case "My view":
			if r.Author != FormerAuthorLabel {
				t.Fatalf("author not detached: %q", r.Author)
			}
		case "Their view":
			if r.Author != "writer-bb22cc" {
				t.Fatalf("another author changed: %q", r.Author)
			}
		}
	}
	if res, err := svc.SearchPublishedPieces(ctx, "harbor", 10); err != nil || len(res.Pieces) != 1 {
		t.Fatalf("the kept piece's text is gone: %v %+v", err, res)
	}
	var n int64
	_ = svc.store.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE owner_id = 'user-b' AND object_kind = 'choir.web_capture'`).Scan(&n)
	if n != 1 {
		t.Fatalf("another account's capture: %d", n)
	}
	if c.RemainingByKind["choir.publication"] != 1 || c.RemainingByKind["choir.web_capture"] != 0 {
		t.Fatalf("remaining by kind: %+v", c.RemainingByKind)
	}
	// Second run.
	c2, err := svc.PurgeAccountData(ctx, "user-a", []string{"computer-aaaa"}, acctOld, acctNow)
	if err != nil || c2.TakesDeleted != 0 || c2.AuthorsDetached != 0 || c2.CapturesDeleted != 0 {
		t.Fatalf("second run: %+v %v", c2, err)
	}
}

// Failure 5.
func TestAccountPurgeRefusesProtectedComputers(t *testing.T) {
	svc := newTakesService(t)
	a := mustPublish(t, svc, "user-a", 1)
	for _, id := range []string{purgePaperID, purgeOwnerID} {
		if _, err := svc.PurgeAccountData(context.Background(), "user-a", []string{"computer-aaaa", id}, acctOld, acctNow); !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("%s: %v", id, err)
		}
		if _, err := svc.StartAccountTapePurge("user-a", []string{id}, acctOld, acctNow); !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("tape %s: %v", id, err)
		}
		if _, err := svc.RunTapeSweep(context.Background(), TapeSweepRequest{Action: TapeSweepActionDryRun, Sweeps: []string{TapeSweepDestroyed}, Computers: []string{id}}); err == nil {
			t.Fatalf("the sweep accepted a named protected computer %s", id)
		}
	}
	if audio, _ := svc.TakeAudio(context.Background(), a.TakeID); audio == nil {
		t.Fatal("a refused purge removed a take")
	}
	for _, id := range []string{"", "../x", "a/b"} {
		if _, err := svc.PurgeAccountData(context.Background(), "user-a", []string{id}, acctOld, acctNow); !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("id %q: %v", id, err)
		}
	}
	for _, user := range []string{"", "a/b", `a\b`} {
		if _, err := svc.PurgeAccountData(context.Background(), user, nil, acctOld, acctNow); !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("user %q: %v", user, err)
		}
	}
}

// Failure 6: scoped, and no archive.
func TestAccountTapePurgeRemovesOnlyNamedComputersAndWritesNoArchive(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	liveChain := f.fullComputer("live-1", 3)
	f.fullComputer("doomed-1", 2)
	f.fullComputer("other-dead-1", 2) // destroyed too, but not named
	liveBefore := f.snapshotOf("live-1")
	otherBefore := f.snapshotOf("other-dead-1")

	job := f.svc.runAccountTapePurge(f.ctx, []string{"doomed-1"})
	if job.State != "done" || job.Error != "" || job.Refusal != "" {
		t.Fatalf("job: %+v", job)
	}
	if len(f.treeFiles("doomed-1")) != 0 || f.rowCount("computer_key_escrows", "doomed-1") != 0 || f.rowCount("computer_file_roots", "doomed-1") != 0 {
		t.Fatal("the named computer survives")
	}
	if !f.snapshotOf("other-dead-1").equal(otherBefore) {
		t.Fatal("a destroyed computer the request did not name was touched")
	}
	if !f.snapshotOf("live-1").equal(liveBefore) {
		t.Fatal("a live computer was touched")
	}
	f.allExist(liveChain, true)
	if job.Archives != 0 {
		t.Fatalf("archives: %d", job.Archives)
	}
	entries, _ := os.ReadDir(filepath.Join(f.root, tapeSweepArchiveDirName))
	if len(entries) != 0 {
		t.Fatalf("an archive exists: %v", entries)
	}
	// A second run finds nothing to do.
	if again := f.svc.runAccountTapePurge(f.ctx, []string{"doomed-1"}); again.State != "done" {
		t.Fatalf("second run: %+v", again)
	}
}

func TestAccountTapePurgeRefusesALiveOrRoutedComputerAndNamesWhy(t *testing.T) {
	f := newTapeFixture(t)
	f.live("alive-1")
	f.fullComputer("alive-1", 2)
	job := f.svc.runAccountTapePurge(f.ctx, []string{"alive-1"})
	if job.State == "done" || job.Refusal == "" || !strings.Contains(job.Refusal, "registry") {
		t.Fatalf("a computer still in the registry was purged or not explained: %+v", job)
	}
	if len(f.treeFiles("alive-1")) == 0 {
		t.Fatal("files of a registered computer were removed")
	}
}

func TestSweepRequestNamingComputersRunsDestroyedAlone(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	for _, sweeps := range [][]string{nil, {TapeSweepOrphans}, {TapeSweepOrphans, TapeSweepDestroyed}} {
		if _, err := f.svc.RunTapeSweep(f.ctx, TapeSweepRequest{Action: TapeSweepActionDryRun, Sweeps: sweeps, Computers: []string{"dead-1"}}); err == nil {
			t.Fatalf("sweeps %v accepted with named computers", sweeps)
		}
	}
}

func capObject(owner, id string) objectgraph.Object {
	canonical, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, objectgraph.StableSuffixFromKey(id))
	if err != nil {
		panic(err)
	}
	return objectgraph.Object{CanonicalID: canonical, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: "computer-" + owner,
		ContentHash: objectgraph.ContentHash(objectgraph.WebCaptureObjectKind, nil, []byte(id)), Metadata: []byte(`{"url":"https://example.test/` + id + `"}`),
		CreatedAt: acctNow, UpdatedAt: acctNow}
}
