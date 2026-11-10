package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A computer that published before polls were armed by publishing has no poll
// row. At boot it makes one read of its notice feed (a read, not resumed
// work): if the platform holds anything for its owner, it is a publisher or
// forker and its polls are armed; if not, nothing is armed. Rows that exist
// are restarted as before, and a row-less quiet computer polls nothing.
func TestBootArmsPollsOfAComputerWithNoticesWaiting(t *testing.T) {
	rt2, s2 := testRuntime(t)
	feed2 := &fakeFeed{versions: map[string]PublicationVersionWire{}}
	rt2.SetPlatformFeed(feed2)
	rt2.selfdevRouteOwnerID = "owner-old-publisher"
	ctx := context.Background()
	seedBootTextureWork(t, s2, "owner-old-publisher", rt2.cfg.ComputerID, "doc-story", currentTextureAgentID("doc-story"), "texture")
	list := func() int {
		polls, err := s2.ListPlatformPolls(withPollReaderCtx(ctx), "owner-old-publisher")
		if err != nil {
			t.Fatal(err)
		}
		return len(polls)
	}
	// Nothing waiting: nothing is armed.
	rt2.armPollsIfNoticesWaiting(ctx)
	if list() != 0 {
		t.Fatal("a quiet computer armed polls")
	}
	// Something waiting for the owner: both intakes are armed, once.
	feed2.notices = []NoticeFeedWire{{Items: []NoticeItemWire{{ID: "n1", Notice: types.CommitmentChangelog{
		EventID: "e1", Kind: types.ChangelogParentUsed, Relation: types.ChangelogRelationForkPublished, OccurredAt: "2026-11-10T00:00:00Z",
		Summary: "x", Origin: types.ChangelogOriginCorpusd}}}, NextCursor: "5"}}
	rt2.armPollsIfNoticesWaiting(ctx)
	rt2.armPollsIfNoticesWaiting(ctx)
	polls, _ := s2.ListPlatformPolls(withPollReaderCtx(ctx), "owner-old-publisher")
	if len(polls) != 1 || polls[0].Name != store.PlatformPollNoticeIntake {
		t.Fatalf("polls = %+v, want notice_intake only", polls)
	}
	// An op-ed on a story this computer holds arms the op-ed poll too.
	feed2.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-here-v1")}, NextCursor: "1"}}
	rt2.armPollsIfNoticesWaiting(ctx)
	polls, _ = s2.ListPlatformPolls(withPollReaderCtx(ctx), "owner-old-publisher")
	if len(polls) != 2 {
		t.Fatalf("polls = %+v, want both intakes", polls)
	}
	for _, p := range polls {
		if p.State != store.PlatformPollActive || p.Cursor != "" || !p.PollInFlightSince.IsZero() {
			t.Fatalf("a poll armed at boot carries work: %+v", p)
		}
	}
	rt2.stopPlatformPolls()
}

func withPollReaderCtx(ctx context.Context) context.Context {
	return objectgraph.WithCustodianReader(ctx, "choir.platform_poll")
}

// Review: a sibling computer of the same owner does not hold the story an
// op-ed answers. Neither its boot arm nor its intake gates that op-ed: it is
// skipped by name, with no Jev call, no arrival and no wake.
func TestNonLocalOpedIsSkippedByNameAndNotGated(t *testing.T) {
	rt, s := testRuntime(t)
	feed := &fakeFeed{versions: map[string]PublicationVersionWire{}}
	rt.SetPlatformFeed(feed)
	judge := &fakeJudge{answers: map[string]float64{}}
	rt.SetCandidateJudge(judge)
	owner, computer := "owner-sibling", rt.cfg.ComputerID // holds no doc-story
	rt.selfdevRouteOwnerID = owner
	ctx := context.Background()
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-away-v1")}, NextCursor: "1"}}
	// Boot: a feed full of op-eds on stories it does not hold arms no op-ed poll.
	rt.armPollsIfNoticesWaiting(ctx)
	if polls, _ := s.ListPlatformPolls(withPollReaderCtx(ctx), owner); len(polls) != 0 {
		t.Fatalf("a computer with no local story armed %+v", polls)
	}
	// Intake (armed some other way): the op-ed is a named skip.
	if err := rt.ensurePlatformPollRows(ctx, owner, computer); err != nil {
		t.Fatal(err)
	}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatal(err)
	}
	poll, _ := s.GetPlatformPoll(withPollReaderCtx(ctx), owner, store.PlatformPollOpedIntake)
	if judge.calls != 0 || len(arrivalRecords(t, s, owner, computer)) != 0 || len(poll.GateQueue) != 0 {
		t.Fatalf("a non-local op-ed was adopted: judge calls %d, queue %v", judge.calls, poll.GateQueue)
	}
	if len(poll.Skipped) != 1 || poll.Skipped[0].ID != "pub-away-v1" || poll.Skipped[0].Reason == "" || poll.Cursor != "1" {
		t.Fatalf("skipped = %+v cursor %q", poll.Skipped, poll.Cursor)
	}
	rt.stopPlatformPolls()
}

// Review: no loop starts after Stop, including from the boot arm's goroutine.
func TestNoPollLoopStartsAfterStop(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.SetPlatformFeed(&fakeFeed{versions: map[string]PublicationVersionWire{}})
	rt.stopPlatformPolls()
	rt.EnsurePlatformPolls(context.Background(), "owner-late", rt.cfg.ComputerID)
	rt.polls.mu.Lock()
	n := len(rt.polls.loops)
	rt.polls.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d poll loops started after Stop", n)
	}
}

// Review: a nil runtime or a broken feed never takes the process down from the boot goroutine.
func TestArmPollsIfNoticesWaitingIsSafe(t *testing.T) {
	var nilRt *Runtime
	nilRt.armPollsIfNoticesWaiting(context.Background())
	rt, _ := testRuntime(t)
	rt.selfdevRouteOwnerID = "o"
	rt.SetPlatformFeed(&panicFeed{})
	rt.armPollsIfNoticesWaiting(context.Background()) // must return
}

type panicFeed struct{ fakeFeed }

func (*panicFeed) Notices(context.Context, string, int) (NoticeFeedWire, error) { panic("feed fault") }
