package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

func withPollReaderCtx(ctx context.Context) context.Context {
	return objectgraph.WithCustodianReader(ctx, "choir.platform_poll")
}

// A sibling computer of the same owner does not hold the story an op-ed
// answers. Its intake skips the op-ed by name: no Jev call, no arrival, no wake.
func TestNonLocalOpedIsSkippedByNameAndNotGated(t *testing.T) {
	rt, s := testRuntime(t)
	feed := &fakeFeed{versions: map[string]PublicationVersionWire{}}
	rt.SetPlatformFeed(feed)
	judge := &fakeJudge{answers: map[string]float64{}}
	rt.SetCandidateJudge(judge)
	owner, computer := "owner-sibling", rt.cfg.ComputerID // holds no doc-story
	ctx := context.Background()
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-away-v1")}, NextCursor: "1"}}
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

// No loop starts after Stop.
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
