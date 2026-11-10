package agentcore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Ways the platform poll can fail (docs/evidence/c2-opeds-failure-list-2026-11-10.md, X5, X6, X19):
//
//  - a replayed page, or a crash between the record and the cursor, mints a
//    second arrival or wedges the poll;
//  - a poll in flight at a crash is resumed, or a poll that crashes the
//    computer loops for ever;
//  - the notice feed being down loses notices or advances the cursor.

type fakeFeed struct {
	mu          sync.Mutex
	opeds       []OpedFeedWire // one page per call, last repeated
	notices     []NoticeFeedWire
	versions    map[string]PublicationVersionWire
	posted      []OpedVerdictWire
	postErr     error
	feedErr     error
	opedCalls   int
	noticeCalls int
	afters      []string
}

func (f *fakeFeed) Opeds(_ context.Context, after string, _ int) (OpedFeedWire, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.afters = append(f.afters, after)
	if f.feedErr != nil {
		return OpedFeedWire{}, f.feedErr
	}
	i := f.opedCalls
	f.opedCalls++
	if len(f.opeds) == 0 {
		return OpedFeedWire{}, nil
	}
	if i >= len(f.opeds) {
		i = len(f.opeds) - 1
	}
	return f.opeds[i], nil
}

func (f *fakeFeed) Notices(_ context.Context, after string, _ int) (NoticeFeedWire, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.feedErr != nil {
		return NoticeFeedWire{}, f.feedErr
	}
	i := f.noticeCalls
	f.noticeCalls++
	if len(f.notices) == 0 {
		return NoticeFeedWire{}, nil
	}
	if i >= len(f.notices) {
		i = len(f.notices) - 1
	}
	return f.notices[i], nil
}

func (f *fakeFeed) ReadVersion(_ context.Context, id string) (PublicationVersionWire, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.versions[id]
	if !ok {
		return PublicationVersionWire{}, errors.New("not found")
	}
	return v, nil
}

func (f *fakeFeed) PostOpedVerdict(_ context.Context, post OpedVerdictWire) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.postErr != nil {
		return f.postErr
	}
	f.posted = append(f.posted, post)
	return nil
}

func opedItem(version string) OpedFeedItemWire {
	return OpedFeedItemWire{
		VersionID: version, RoutePath: "/pub/texture/my-view/v1", AuthorLabel: "writer-aa11bb", ManifestHash: "mh-" + version,
		PublishedAt: "2026-11-10T00:00:00Z", NoticeID: "obj:notice:" + version, StoryDocID: "doc-story", PublisherID: "owner-reader-1",
		RespondsTo: types.OpedRespondsTo{PublicationVersionID: "pub-story-v1", RoutePath: "/pub/texture/story/v1", PublicationID: "pub-story"},
	}
}

func newPollFixture(t *testing.T) (*Runtime, *store.Store, *fakeFeed, string, string) {
	t.Helper()
	rt, s := testRuntime(t)
	feed := &fakeFeed{versions: map[string]PublicationVersionWire{}}
	rt.SetPlatformFeed(feed)
	owner, computer := "owner-paper", rt.cfg.ComputerID
	// The story the fixtures' op-eds answer is held by this computer.
	seedBootTextureWork(t, s, owner, computer, "doc-story", currentTextureAgentID("doc-story"), "texture")
	if err := rt.ensurePlatformPollRows(context.Background(), owner, computer); err != nil {
		t.Fatal(err)
	}
	return rt, s, feed, owner, computer
}

func arrivalRecords(t *testing.T, s *store.Store, owner, computer string) []types.CommitmentRecord {
	t.Helper()
	recs, err := s.ListInboxRecords(context.Background(), owner, computer, deskmanifest.AdmitterInbox(owner), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, r := range recs {
		if r.Oped != nil && r.Oped.Kind == types.OpedRecordArrival {
			out = append(out, r)
		}
	}
	return out
}

func TestOpedIntakeReplayMintsOneArrival(t *testing.T) {
	rt, s, feed, owner, computer := newPollFixture(t)
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{opedItem("pub-a-v1")}, NextCursor: "2026-11-10T00:00:00Z"}}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}
	arrivals := arrivalRecords(t, s, owner, computer)
	if len(arrivals) != 1 {
		t.Fatalf("a replayed page minted %d arrivals, want 1", len(arrivals))
	}
	a := arrivals[0]
	if a.Oped.Arrival.VersionID != "pub-a-v1" || a.Oped.Arrival.TextHash != "mh-pub-a-v1" || a.SubjectRefs["story_doc_id"] != "doc-story" {
		t.Fatalf("arrival = %+v refs %v", a.Oped.Arrival, a.SubjectRefs)
	}
	if store.RecordWakesReader(a) {
		t.Fatal("an arrival wakes the desk")
	}
	poll, err := s.GetPlatformPoll(ctx, owner, store.PlatformPollOpedIntake)
	if err != nil || poll.Cursor != "2026-11-10T00:00:00Z" || !poll.PollInFlightSince.IsZero() {
		t.Fatalf("poll = %+v %v", poll, err)
	}
	// The second and third polls asked from the saved cursor.
	if len(feed.afters) < 3 || feed.afters[0] != "" || feed.afters[1] != "2026-11-10T00:00:00Z" {
		t.Fatalf("afters = %v", feed.afters)
	}
}

// X5: a crash between the arrival record and the cursor must not wedge the poll.
func TestOpedIntakeCrashBetweenRecordAndCursor(t *testing.T) {
	rt, s, feed, owner, computer := newPollFixture(t)
	ctx := context.Background()
	// The record committed; the cursor did not move.
	item := opedItem("pub-b-v1")
	if err := rt.commitOpedArrival(ctx, owner, computer, item); err != nil {
		t.Fatal(err)
	}
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{item}, NextCursor: "2026-11-10T01:00:00Z"}}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatalf("poll after the crash: %v", err)
	}
	if n := len(arrivalRecords(t, s, owner, computer)); n != 1 {
		t.Fatalf("arrivals = %d, want 1", n)
	}
	poll, _ := s.GetPlatformPoll(ctx, owner, store.PlatformPollOpedIntake)
	if poll.Cursor != "2026-11-10T01:00:00Z" {
		t.Fatalf("the cursor did not advance: %+v", poll)
	}
}

// X6: a crash closes the poll in flight and resumes nothing; two crashes in a
// row with the poll in flight close the poll with that cause.
func TestPlatformPollCrashTwiceCloses(t *testing.T) {
	rt, s, feed, owner, _ := newPollFixture(t)
	ctx := context.Background()
	inFlight := func() {
		poll, err := s.GetPlatformPoll(ctx, owner, store.PlatformPollOpedIntake)
		if err != nil {
			t.Fatal(err)
		}
		poll.PollInFlightSince = time.Now().UTC()
		if err := rt.putPlatformPoll(ctx, poll); err != nil {
			t.Fatal(err)
		}
	}
	inFlight()
	rt.recoverPlatformPollsAtBoot(ctx, false)
	poll, _ := s.GetPlatformPoll(ctx, owner, store.PlatformPollOpedIntake)
	if poll.State != store.PlatformPollActive || poll.CrashesWithPollInFlight != 1 || !poll.PollInFlightSince.IsZero() {
		t.Fatalf("after one crash: %+v", poll)
	}
	if feed.opedCalls != 0 {
		t.Fatalf("boot polled: %d calls", feed.opedCalls)
	}
	inFlight()
	rt.recoverPlatformPollsAtBoot(ctx, false)
	poll, _ = s.GetPlatformPoll(ctx, owner, store.PlatformPollOpedIntake)
	if poll.State != store.PlatformPollClosed || poll.StateReason == "" {
		t.Fatalf("after two crashes: %+v", poll)
	}
	// A closed poll does not run.
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil || feed.opedCalls != 0 {
		t.Fatalf("a closed poll ran: %v calls=%d", err, feed.opedCalls)
	}
	// A finished poll resets the crash count of the other.
	other, _ := s.GetPlatformPoll(ctx, owner, store.PlatformPollNoticeIntake)
	if other.CrashesWithPollInFlight != 0 || other.State != store.PlatformPollActive {
		t.Fatalf("notice poll = %+v", other)
	}
	rt.stopPlatformPolls()
}

func TestNoticeIntakeReplayAndPlatformDown(t *testing.T) {
	rt, s, feed, owner, computer := newPollFixture(t)
	ctx := context.Background()
	notice := types.CommitmentChangelog{
		EventID: types.ChangelogEventID(types.ChangelogParentUsed, owner, "pub-child-v1", "pub-parent-v1"), Kind: types.ChangelogParentUsed,
		Relation: types.ChangelogRelationForkPublished, OccurredAt: "2026-11-10T00:00:00Z", ActorLabel: "writer-cc22dd",
		Subject: types.ChangelogSubject{PublicationVersionID: "pub-child-v1", ParentVersionID: "pub-parent-v1"},
		Summary: "writer-cc22dd published a fork of your op-ed.", Origin: types.ChangelogOriginCorpusd,
	}
	feed.notices = []NoticeFeedWire{{Items: []NoticeItemWire{{ID: "obj:notice:1", CreatedAt: "2026-11-10T00:00:00Z", Notice: notice}}, NextCursor: "2026-11-10T00:00:00Z"}}
	feed.feedErr = errors.New("platform down")
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollNoticeIntake); err == nil {
		t.Fatal("a down platform was not an error")
	}
	poll, _ := s.GetPlatformPoll(ctx, owner, store.PlatformPollNoticeIntake)
	if poll.Cursor != "" || poll.ConsecutiveFailures != 1 || poll.LastError == "" || poll.State != store.PlatformPollActive {
		t.Fatalf("after the platform was down: %+v", poll)
	}
	feed.feedErr = nil
	for i := 0; i < 2; i++ {
		if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollNoticeIntake); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, types.ChangelogInbox(owner), 0, 0)
	if err != nil || len(recs) != 1 {
		t.Fatalf("changelog records = %d %v, want 1", len(recs), err)
	}
	got := recs[0].Changelog
	if got.Kind != types.ChangelogParentUsed || got.Level != types.ChangelogMajor || got.RemoteRef != "obj:notice:1" || got.Relation != types.ChangelogRelationForkPublished {
		t.Fatalf("changelog = %+v", got)
	}
	if store.RecordWakesReader(recs[0]) {
		t.Fatal("a notice wakes a desk")
	}
	poll, _ = s.GetPlatformPoll(ctx, owner, store.PlatformPollNoticeIntake)
	if poll.Cursor != "2026-11-10T00:00:00Z" || poll.ConsecutiveFailures != 0 || poll.LastError != "" {
		t.Fatalf("after recovery: %+v", poll)
	}
}

// Review N5: one malformed feed item is recorded as a named skip and the
// cursor moves past it; the good items on the page are still adopted.
func TestMalformedFeedItemIsSkippedNotStalled(t *testing.T) {
	rt, s, feed, owner, computer := newPollFixture(t)
	ctx := context.Background()
	bad := opedItem("pub-bad-v1")
	bad.ManifestHash = ""
	noStory := opedItem("pub-nostory-v1")
	noStory.StoryDocID = ""
	good := opedItem("pub-good-v1")
	feed.opeds = []OpedFeedWire{{Items: []OpedFeedItemWire{bad, noStory, good}, NextCursor: "5"}}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollOpedIntake); err != nil {
		t.Fatalf("a malformed item failed the poll: %v", err)
	}
	if n := len(arrivalRecords(t, s, owner, computer)); n != 1 {
		t.Fatalf("arrivals = %d, want only the good one", n)
	}
	poll, _ := s.GetPlatformPoll(ctx, owner, store.PlatformPollOpedIntake)
	if poll.Cursor != "5" || len(poll.Skipped) != 2 || poll.Skipped[0].ID != "pub-bad-v1" || poll.Skipped[0].Reason == "" || poll.Skipped[1].ID != "pub-nostory-v1" {
		t.Fatalf("poll = cursor %q skipped %+v", poll.Cursor, poll.Skipped)
	}
	// Notices: an unknown kind is skipped too.
	feed.notices = []NoticeFeedWire{{Items: []NoticeItemWire{{ID: "obj:n:bad", Notice: types.CommitmentChangelog{EventID: "e", Kind: "update_exploded", Summary: "x", Origin: types.ChangelogOriginCorpusd}}}, NextCursor: "9"}}
	if err := rt.runPlatformPollOnce(ctx, owner, store.PlatformPollNoticeIntake); err != nil {
		t.Fatalf("a malformed notice failed the poll: %v", err)
	}
	np, _ := s.GetPlatformPoll(ctx, owner, store.PlatformPollNoticeIntake)
	if np.Cursor != "9" || len(np.Skipped) != 1 || np.Skipped[0].ID != "obj:n:bad" {
		t.Fatalf("notice poll = %+v", np)
	}
}
