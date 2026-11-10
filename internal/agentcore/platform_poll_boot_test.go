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
