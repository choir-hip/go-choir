package platform

// The findings of the two review seats on lane account-delete that live in
// corpusd (seat 1 #5 #6, seat 2 S4). Failure list, written before the fixes:
//
//  Q1 the person's synced Texture documents and revisions (platform_texture_*)
//     survive the purge: nothing a published story reads, and the person's
//     alone;
//  Q2 a take's pre-blank body and its audio stay in the artifact store until the
//     artifact GC's seven day grace, so "takes are removed" is not true for a
//     week;
//  Q3 more takes or publications than one scan reads are silently left behind
//     (deleted takes keep their publisher key and so fill the scan);
//  Q4 suspend takes any user id, so a path-shaped id reaches the take scan;
//  Q5 what remains of the account's rows is not split into what stays by
//     decision (the publication layer) and what nobody decided (named, so an
//     operator sees it).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestAccountPurgeRemovesTheAccountsTextureDocumentsAndNothingElses(t *testing.T) { // Q1
	svc := newTakesService(t)
	ctx := context.Background()
	for _, owner := range []string{"user-a", "user-b"} {
		if err := svc.store.UpsertTextureDocument(ctx, "doc-"+owner, owner, "Notes of "+owner); err != nil {
			t.Fatal(err)
		}
		if err := svc.store.UpsertTextureRevision(ctx, PlatformTextureRevision{RevisionID: "rev-" + owner, DocID: "doc-" + owner, OwnerID: owner, Content: "private words of " + owner}); err != nil {
			t.Fatal(err)
		}
	}
	c, err := svc.PurgeAccountData(ctx, "user-a", nil, acctOld, acctNow)
	if err != nil {
		t.Fatal(err)
	}
	if c.TextureDocumentsDeleted != 1 || c.TextureRevisionsDeleted != 1 {
		t.Fatalf("counts: %+v", c)
	}
	var a, b int64
	_ = svc.store.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_texture_revisions WHERE owner_id = 'user-a'`).Scan(&a)
	_ = svc.store.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_texture_revisions WHERE owner_id = 'user-b'`).Scan(&b)
	if a != 0 || b != 1 {
		t.Fatalf("revisions left: a=%d b=%d", a, b)
	}
	if c.RemainingTextures["platform_texture_documents"] != 0 {
		t.Fatalf("remaining: %+v", c.RemainingTextures)
	}
	// The published piece does not read them: it still answers.
	if _, err := publishOpinion(svc, "user-a", "writer-aa11bb", "doc-pub", 1, "Kept", "Words about the harbor."); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PurgeAccountData(ctx, "user-a", nil, acctOld, acctNow); err != nil {
		t.Fatal(err)
	}
	if res, err := svc.SearchPublishedPieces(ctx, "harbor", 10); err != nil || len(res.Pieces) != 1 {
		t.Fatalf("a published piece lost its text: %v %+v", err, res)
	}
}

func casFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(filepath.Join(root, "sha256", "og"), func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestAccountPurgeUnlinksTheTakeBodiesAtOnce(t *testing.T) { // Q2
	svc := newTakesService(t)
	ctx := context.Background()
	in := takeIn("user-a", takeFixtureAudio(10, 4000), 10)
	in.Transcript = strings.Repeat("the harbor matters ", 100) // over 1 KiB: the row is externalized
	take, err := svc.PublishTake(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := svc.PublishTake(ctx, takeIn("user-b", takeFixtureAudio(10, 5000), 10))
	if err != nil {
		t.Fatal(err)
	}
	before := casFiles(t, svc.artifactsRoot)
	if len(before) < 2 {
		t.Fatalf("setup: the take's bodies were not externalized: %v", before)
	}
	if _, err := svc.PurgeAccountData(ctx, "user-a", nil, acctOld, acctNow); err != nil {
		t.Fatal(err)
	}
	after := casFiles(t, svc.artifactsRoot)
	for _, f := range after {
		raw, _ := os.ReadFile(f)
		if strings.Contains(string(raw), "the harbor matters") {
			t.Fatalf("the transcript of a deleted take is still on disk: %s", f)
		}
	}
	if len(after) >= len(before) {
		t.Fatalf("no body was unlinked: %d -> %d", len(before), len(after))
	}
	// Another publisher's take still plays.
	if audio, _ := svc.TakeAudio(ctx, keep.TakeID); audio == nil {
		t.Fatal("another account's take lost its audio")
	}
	_ = take
}

// Round 2 (seat 2 F6): a crash between a take row's rewrite and its audio's
// removal left the audio for good (the purge skipped a take already deleted).
func TestAPurgeFinishesATakeWhoseRemovalWasInterrupted(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	in := takeIn("user-a", takeFixtureAudio(10, 4000), 10)
	take, err := svc.PublishTake(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// The interrupted removal: the row says deleted and is blanked, the audio stays.
	og := svc.ogStore()
	svc.takeMu.Lock()
	row, err := svc.getTakeRow(ctx, og, take.TakeID)
	svc.takeMu.Unlock()
	if err != nil || row == nil {
		t.Fatal(err)
	}
	row.State, row.Transcript, row.RespondsTo = TakeStateDeleted, "", nil
	if err := svc.putTakeRow(ctx, og, *row, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if audio, _ := svc.TakeAudio(ctx, take.TakeID); audio == nil {
		// TakeAudio serves only live takes; read the object itself.
	}
	canonical, _ := podcastCanonical(ogKindTakeAudio, take.TakeID)
	if _, err := og.GetObject(ctx, canonical); err != nil {
		t.Fatalf("setup: the audio row is not there: %v", err)
	}
	before := len(casFiles(t, svc.artifactsRoot))
	if _, err := svc.PurgeAccountData(ctx, "user-a", nil, acctOld, acctNow); err != nil {
		t.Fatal(err)
	}
	if _, err := og.GetObject(ctx, canonical); err == nil {
		t.Fatal("the audio of an interrupted removal survives the purge")
	}
	if after := len(casFiles(t, svc.artifactsRoot)); after >= before {
		t.Fatalf("the audio's body stays in the artifact store: %d -> %d", before, after)
	}
}

func TestAccountPurgeAndSuspendReachEveryRowNotOnlyTheFirstScan(t *testing.T) { // Q3
	old := accountPurgeScan
	accountPurgeScan = 2
	t.Cleanup(func() { accountPurgeScan = old })
	svc := newTakesService(t)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		mustPublish(t, svc, "user-a", i)
	}
	// Two already deleted: they keep the publisher key and fill a scan.
	list, _ := svc.ListMyTakes(ctx, types.TakePublisherKey("user-a"))
	for _, tk := range list.Takes[:2] {
		if err := svc.DeleteTake(ctx, tk.TakeID, types.TakePublisherKey("user-a")); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := svc.SetAccountTakesState(ctx, "user-a", TakeStateLive, TakeStateSuspended); err != nil || n != 3 {
		t.Fatalf("suspend reached %d of 3 live takes: %v", n, err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := publishOpinion(svc, "user-a", "writer-aa11bb", "doc-"+string(rune('a'+i)), 1, "View "+string(rune('A'+i)), "Words about the harbor "+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	c, err := svc.PurgeAccountData(ctx, "user-a", nil, acctOld, acctNow)
	if err != nil {
		t.Fatal(err)
	}
	if c.TakesDeleted != 3 || c.AuthorsDetached != 5 {
		t.Fatalf("a purge stopped at the first scan: %+v", c)
	}
	for _, r := range opinionRows(t, svc) {
		if r.Author != FormerAuthorLabel {
			t.Fatalf("a byline survives: %q", r.Author)
		}
	}
}

func TestSuspendRefusesAnIdThatIsNotAUserId(t *testing.T) { // Q4
	svc := newTakesService(t)
	for _, bad := range []string{"", "a/b", `a\b`, strings.Repeat("x", 300)} {
		if _, err := svc.SetAccountTakesState(context.Background(), bad, TakeStateLive, TakeStateSuspended); !errors.Is(err, ErrAccountPurgeRefused) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	h := NewHandler(svc)
	for _, user := range []string{"a/b"} {
		req := httptest.NewRequest(http.MethodPost, AccountSuspendPath, strings.NewReader(`{"user_id":"`+user+`","action":"suspend"}`))
		req.Header.Set("X-Internal-Caller", "true")
		req.RemoteAddr = "127.0.0.1:1"
		rec := httptest.NewRecorder()
		h.HandleAccountSuspend(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("the route accepted %q", user)
		}
	}
}

func TestRemainingRowsAreSplitIntoWhatStaysByDecisionAndWhatNobodyDecided(t *testing.T) { // Q5
	svc := newTakesService(t)
	ctx := context.Background()
	if _, err := publishOpinion(svc, "user-a", "writer-aa11bb", "doc-a", 1, "Kept", "Words about the harbor."); err != nil {
		t.Fatal(err)
	}
	// A kind nobody decided about, owned by the account.
	obj := capObject("user-a", "x")
	obj.ObjectKind = "choir.some_future_kind"
	obj.CanonicalID = "choir.some_future_kind:user-a:future1"
	_, _ = svc.store.corpus().ExecContext(ctx, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?, ?, ?, '', '', 'h', '{}', '', 2, '{}', now(), now(), false, '')`,
		obj.CanonicalID, string(obj.ObjectKind), "user-a")
	c, err := svc.PurgeAccountData(ctx, "user-a", nil, acctOld, acctNow)
	if err != nil {
		t.Fatal(err)
	}
	if c.KeptByDecision["choir.publication"] != 1 {
		t.Fatalf("the publication layer is not named as kept by decision: %+v", c)
	}
	if c.Undecided["choir.some_future_kind"] != 1 {
		t.Fatalf("a kind nobody decided about is not named: %+v", c)
	}
	if _, bad := c.Undecided["choir.publication"]; bad {
		t.Fatalf("the publication layer counted as undecided: %+v", c)
	}
}

// Round 2 (blocking 2): the route-slot guard names the computer, not the
// desktop. A slot is computer:<owner>:<desktop>; the computer it realizes is
// made from the pair. The purge deletes the account's own slots, so a slot of a
// computer that no longer exists cannot hold its tape back.
func TestTheRouteSlotGuardNamesTheComputerAndThePurgeClearsTheAccountsSlots(t *testing.T) {
	f := newTapeFixture(t)
	id := routedComputerID()
	if id == "primary" || !strings.HasPrefix(id, "computer-") {
		t.Fatalf("the routed computer id is %q", id)
	}
	f.fullComputer(id, 2)
	f.insertClosure("code-r", platformTestDigest('9'), true)
	job := f.svc.runAccountTapePurge(f.ctx, []string{id})
	if job.State == "done" || !strings.Contains(job.Refusal, "route slot") {
		t.Fatalf("a computer with a route slot was purged or not explained: %+v", job)
	}
	if len(f.treeFiles(id)) == 0 {
		t.Fatal("files of a routed computer were removed")
	}
	if _, err := f.svc.PurgeAccountData(f.ctx, routedOwner, nil, acctOld, acctNow); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = f.store.db.QueryRow(`SELECT COUNT(*) FROM computer_version_route_slots WHERE route_slot_id LIKE 'computer:`+routedOwner+`:%'`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d route slots of the account remain", n)
	}
	if again := f.svc.runAccountTapePurge(f.ctx, []string{id}); again.State != "done" {
		t.Fatalf("after the account's slots went: %+v", again)
	}
}

func TestRouteSlotsOfAnotherAccountSurviveAPurge(t *testing.T) {
	f := newTapeFixture(t)
	f.insertClosure("code-r", platformTestDigest('9'), true)
	if _, err := f.svc.PurgeAccountData(f.ctx, "someone-else", nil, acctOld, acctNow); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = f.store.db.QueryRow(`SELECT COUNT(*) FROM computer_version_route_slots`).Scan(&n)
	if n != 1 {
		t.Fatalf("another account's slot was touched: %d", n)
	}
}
