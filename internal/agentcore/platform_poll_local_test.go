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
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-p-v1")}, NextCursor: "9"}}
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

// The boot arm pages the op-ed feed until it finds an op-ed on a local story
// or the feed ends: 50 foreign op-eds in front do not hide a local one.
func TestBootArmPagesPastForeignOpeds(t *testing.T) {
	rt, s := testRuntime(t)
	feed := &fakeFeed{versions: map[string]PublicationVersionWire{}}
	rt.SetPlatformFeed(feed)
	owner, computer := "owner-paged", rt.cfg.ComputerID
	rt.selfdevRouteOwnerID = owner
	seedBootTextureWork(t, s, owner, computer, "doc-here", currentTextureAgentID("doc-here"), "texture")
	var first []OpedFeedItemWire
	for i := 0; i < bootArmPageLimit; i++ {
		it := opedItem("pub-far-" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
		it.StoryDocID = "doc-elsewhere"
		first = append(first, it)
	}
	here := opedItem("pub-here-v1")
	here.StoryDocID = "doc-here"
	feed.opeds = []OpedFeedWire{{Items: first, NextCursor: "50"}, {Items: []OpedFeedItemWire{here}, NextCursor: "51"}}
	rt.armPollsIfNoticesWaiting(context.Background())
	polls, _ := s.ListPlatformPolls(withPollReaderCtx(context.Background()), owner)
	if len(polls) != 2 {
		t.Fatalf("polls = %+v, want both intakes (the local op-ed is on page two)", polls)
	}
	rt.stopPlatformPolls()
}
