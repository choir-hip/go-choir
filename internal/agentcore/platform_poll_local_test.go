package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Review round 2 on locality. A story is local, not local, or unknown (the
// lookup failed). Only "not local" skips an op-ed; "unknown" must not lose it:
// the cursor stays, the tick stops there and retries, and after
// opedLocalityMaxFailures ticks on the same item it is skipped with the error
// class named. A story published from a plain document is local.

func localityFixture(t *testing.T) (*Runtime, *store.Store, *fakeFeed, string, string) {
	t.Helper()
	rt, s := testRuntime(t)
	feed := &fakeFeed{versions: map[string]PublicationVersionWire{}}
	rt.SetPlatformFeed(feed)
	rt.SetCandidateJudge(&fakeJudge{answers: map[string]float64{}})
	owner, computer := "owner-local", rt.cfg.ComputerID
	if err := rt.ensurePlatformPollRows(context.Background(), owner, computer); err != nil {
		t.Fatal(err)
	}
	return rt, s, feed, owner, computer
}

func TestLocalityLookupErrorDoesNotLoseTheOpedAndIsBounded(t *testing.T) {
	rt, s, feed, owner, computer := localityFixture(t)
	ctx := context.Background()
	rt.polls.docLookup = func(context.Context, string, string, string) error {
		return errors.New("store fault: connection reset")
	}
	it := opedItem("pub-e-v1")
	feed.versions["pub-e-v1"] = PublicationVersionWire{PublicationVersionID: "pub-e-v1", Title: "t", Content: "A view.", ManifestHash: it.ManifestHash}
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{it}, NextCursor: "7"}}
	poll := func() store.PlatformPoll {
		p, err := s.GetPlatformPoll(withPollReaderCtx(ctx), owner, store.PlatformPollOpedIntake)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	for i := 1; i < opedLocalityMaxFailures; i++ {
		if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err == nil {
			t.Fatalf("tick %d: a lookup fault was not reported", i)
		}
		if p := poll(); p.Cursor != "" || len(p.Skipped) != 0 || p.ItemFailures["pub-e-v1"] != i {
			t.Fatalf("tick %d: the item was lost or the cursor moved: cursor %q skipped %+v failures %v", i, p.Cursor, p.Skipped, p.ItemFailures)
		}
	}
	// A recovered lookup (the story is local) adopts it on the next tick.
	rt.polls.docLookup = func(context.Context, string, string, string) error { return nil }
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatal(err)
	}
	if p := poll(); p.Cursor != "7" || len(arrivalRecords(t, s, owner, computer)) != 1 || len(p.ItemFailures) != 0 {
		t.Fatalf("after recovery: cursor %q arrivals %d failures %v", p.Cursor, len(arrivalRecords(t, s, owner, computer)), p.ItemFailures)
	}
}

func TestLocalityLookupPoisonedItemIsSkippedAfterTheBound(t *testing.T) {
	rt, s, feed, owner, _ := localityFixture(t)
	ctx := context.Background()
	rt.polls.docLookup = func(context.Context, string, string, string) error { return errors.New("undecodable row") }
	rt.SetPlatformFeed(&cursorFeed{fakeFeed: feed, item: opedItem("pub-p-v1"), cursor: "9"})
	for i := 0; i < opedLocalityMaxFailures; i++ {
		_ = rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake)
	}
	p, _ := s.GetPlatformPoll(withPollReaderCtx(ctx), owner, store.PlatformPollOpedIntake)
	if p.Cursor != "9" || len(p.Skipped) != 1 || p.Skipped[0].ID != "pub-p-v1" || p.Skipped[0].Reason == "" || len(p.ItemFailures) != 0 {
		t.Fatalf("after the bound: cursor %q skipped %+v failures %v", p.Cursor, p.Skipped, p.ItemFailures)
	}
	if want := "*errors.errorString"; !strings.Contains(p.Skipped[0].Reason, want) {
		t.Fatalf("the skip does not name the error class %s: %q", want, p.Skipped[0].Reason)
	}
}

func TestLocalityNotFoundSkipsByNameAndAdvances(t *testing.T) {
	rt, s, feed, owner, computer := localityFixture(t)
	ctx := context.Background()
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-n-v1")}, NextCursor: "3"}}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatal(err)
	}
	p, _ := s.GetPlatformPoll(withPollReaderCtx(ctx), owner, store.PlatformPollOpedIntake)
	if p.Cursor != "3" || len(p.Skipped) != 1 || len(arrivalRecords(t, s, owner, computer)) != 0 {
		t.Fatalf("not found: cursor %q skipped %+v", p.Cursor, p.Skipped)
	}
}

// A story published from a plain document (CreateDocument, no lifecycle) is local.
func TestOpedOnAPlainDocumentStoryIsGatedNotSkipped(t *testing.T) {
	rt, s, feed, owner, computer := localityFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{DocID: "doc-story", OwnerID: owner, ComputerID: computer, Title: "Plain story", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	item := addOped(feed, "pub-plain-v1", "A view.")
	feed.versions["pub-story-v1"] = PublicationVersionWire{PublicationVersionID: "pub-story-v1", Title: "Plain story", Content: "Story.", ManifestHash: "mh-story"}
	_ = item
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatal(err)
	}
	p, _ := s.GetPlatformPoll(withPollReaderCtx(ctx), owner, store.PlatformPollOpedIntake)
	if len(p.Skipped) != 0 || len(arrivalRecords(t, s, owner, computer)) != 1 {
		t.Fatalf("an op-ed on a plain document's story was skipped: %+v", p.Skipped)
	}
	if v := tapeByKind(t, s, owner, computer, "doc-story", types.OpedRecordVerdict); len(v) != 1 {
		t.Fatalf("it was not gated: %d verdicts", len(v))
	}
}

// A plain document scoped to another computer of the same owner is not local;
// one with no computer scope (a legacy row in this computer's own store) is.
func TestPlainDocumentIsLocalOnlyForThisComputer(t *testing.T) {
	rt, s, _, owner, computer := localityFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for doc, scope := range map[string]string{"doc-other-computer": "some-other-computer", "doc-legacy": "", "doc-this": computer} {
		if err := s.CreateDocument(ctx, types.Document{DocID: doc, OwnerID: owner, ComputerID: scope, Title: doc, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	for doc, want := range map[string]bool{"doc-other-computer": false, "doc-legacy": true, "doc-this": true, "doc-nowhere": false} {
		got, err := rt.storyDocLocality(ctx, owner, computer, doc)
		if err != nil || got != want {
			t.Fatalf("%s: local=%v err=%v, want %v", doc, got, err, want)
		}
	}
}

// A cancelled or timed-out lookup is the tick's fault, not the item's: it does not count toward the bound.
func TestCancelledLookupDoesNotCountAgainstTheItem(t *testing.T) {
	rt, s, feed, owner, _ := localityFixture(t)
	ctx := context.Background()
	rt.polls.docLookup = func(context.Context, string, string, string) error { return context.DeadlineExceeded }
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-c-v1")}, NextCursor: "4"}}
	for i := 0; i < opedLocalityMaxFailures+2; i++ {
		_ = rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake)
	}
	p, _ := s.GetPlatformPoll(withPollReaderCtx(ctx), owner, store.PlatformPollOpedIntake)
	if len(p.ItemFailures) != 0 || len(p.Skipped) != 0 || p.Cursor != "" {
		t.Fatalf("deadline errors counted: failures %v skipped %+v cursor %q", p.ItemFailures, p.Skipped, p.Cursor)
	}
}

// The server bounds a page by notice rows and filters items after, so a page
// may hold fewer items than the limit and still not be the last: a tick drains
// while the cursor advances.
func TestIntakeDrainsMixedPages(t *testing.T) {
	rt, s, feed, owner, computer := newPollFixture(t)
	ctx := context.Background()
	feed.opeds = []OpedFeedWire{
		{Items: []OpedFeedItemWire{opedItem("pub-m1-v1")}, NextCursor: "10"},
		{Items: []OpedFeedItemWire{opedItem("pub-m2-v1")}, NextCursor: "20"},
	}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatal(err)
	}
	if n := len(arrivalRecords(t, s, owner, computer)); n != 2 {
		t.Fatalf("one tick adopted %d of 2 op-eds across a mixed page", n)
	}
	n1 := types.CommitmentChangelog{EventID: "e1", Kind: types.ChangelogParentUsed, Relation: types.ChangelogRelationTakenInto, OccurredAt: "2026-11-10T00:00:00Z", Summary: "x", Origin: types.ChangelogOriginCorpusd}
	n2 := n1
	n2.EventID = "e2"
	feed.notices = []NoticeFeedWire{
		{Items: []NoticeItemWire{{ID: "n1", Notice: n1}}, NextCursor: "5"},
		{Items: []NoticeItemWire{{ID: "n2", Notice: n2}}, NextCursor: "9"},
	}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollNoticeIntake); err != nil {
		t.Fatal(err)
	}
	recs, _ := s.ListInboxRecords(ctx, owner, computer, types.ChangelogInbox(owner), 0, 0)
	if len(recs) != 2 {
		t.Fatalf("one tick adopted %d of 2 notices across a mixed page", len(recs))
	}
}

// cursorFeed serves one op-ed from the start of the feed and nothing after its cursor, as a server does.
type cursorFeed struct {
	*fakeFeed
	item   OpedFeedItemWire
	cursor string
}

func (f *cursorFeed) Opeds(_ context.Context, after string, _ int) (OpedFeedWire, error) {
	if after == f.cursor {
		return OpedFeedWire{NextCursor: f.cursor}, nil
	}
	return OpedFeedWire{Items: []OpedFeedItemWire{f.item}, NextCursor: f.cursor}, nil
}
