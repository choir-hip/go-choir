package podcastworker

// The automatic matcher in the worker (no model call). Failure list, written
// first:
//   A1. a story older than the story window, an opinion piece or a story with
//       no route is matched (the served window is 72 h);
//   A2. a second run writes the clips again, or matches stories nothing changed
//       for (idempotence);
//   A3. a new transcript or a new story version is not matched, or rewrites the
//       clips already written;
//   A4. --dry-run writes to the platform or saves state;
//   A5. the run's one log line lacks the counts the lead reads (stories,
//       passages scored, accepted, rejected by rule, written);
//   A6. a story that left the window keeps state forever;
//   A7. state does not survive a restart;
//   A8. the loop runs the matcher after every transcript instead of once the
//       queue is drained, or never on the timer;
//   A9. a refusal by the platform is retried every run; a network failure is
//       given up on;
//   A10. the matcher makes a model call (it has no client for one: asserted by
//        the backend interface having only platform reads and the clips write).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastmatch"
)

var matchNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

type fakeMatchBackend struct {
	mu          sync.Mutex
	front       []FrontStory
	eps         []EpisodeInfo
	transcripts map[string][]Segment
	puts        [][]ClipIn
	putErr      func(ClipIn) error
	tcalls      int
	held        []HeldClip
	clipReads   int
}

func (b *fakeMatchBackend) Clips(_ context.Context, routes []string) ([]HeldClip, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clipReads++
	var out []HeldClip
	for _, h := range b.held {
		for _, r := range routes {
			if h.Route == r {
				out = append(out, h)
			}
		}
	}
	return out, nil
}

func (b *fakeMatchBackend) Front(context.Context, int) ([]FrontStory, error) { return b.front, nil }
func (b *fakeMatchBackend) Episodes(context.Context, time.Time, int) ([]EpisodeInfo, error) {
	return b.eps, nil
}
func (b *fakeMatchBackend) Transcript(_ context.Context, _, guid string) ([]Segment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tcalls++
	return b.transcripts[guid], nil
}
func (b *fakeMatchBackend) PutClipsEach(_ context.Context, clips []ClipIn) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.puts = append(b.puts, clips)
	errs := make([]error, len(clips))
	for _, c := range clips {
		if b.putErr != nil && b.putErr(c) != nil {
			continue
		}
		for _, e := range b.eps {
			if e.EpisodeID == c.EpisodeID {
				b.held = append(b.held, HeldClip{Route: c.StoryRoute, FeedURL: e.FeedURL, GUID: e.GUID, Start: c.Start, Quote: c.Quote})
			}
		}
	}
	if b.putErr != nil {
		for i, c := range clips {
			errs[i] = b.putErr(c)
		}
	}
	return errs
}

func frontStory(route, title, dek string, rev int, updated time.Time) FrontStory {
	fs := FrontStory{Title: title, Dek: dek, UpdatedAt: updated, LatestRevision: rev, Kind: "story"}
	fs.Route.Bare = route
	return fs
}

func spoken(sentences ...string) []Segment {
	var segs []Segment
	// Eight filler sentences before and after, so the passage is a clip inside an episode.
	fill := []string{"Good morning and welcome.", "The weather stays mild today.", "A listener asks about gardens.", "Traffic is normal this hour.", "More after the break.", "Sports are next.", "Thanks for listening.", "That is all for now."}
	all := append(append(append([]string{}, fill...), sentences...), fill...)
	for i, t := range all {
		at := float64(i) * 7
		segs = append(segs, Segment{Start: at, End: at + 6.5, Text: t})
	}
	return segs
}

func ferrySpoken() []Segment {
	return spoken("A passenger ferry called the Marlin hit a pier in Tessaly Bay this morning.", "Forty people were hurt, most of them with minor injuries.", "The coast guard says the Harborview vessel lost steering during its crossing.", "Rescue boats reached the ferry within minutes of the collision.", "Investigators will examine the steering gear in the coming days.")
}

func newFakeMatch() *fakeMatchBackend {
	b := &fakeMatchBackend{transcripts: map[string][]Segment{}}
	b.front = []FrontStory{
		frontStory("/pub/texture/ferry", "Harborview ferry Marlin strikes pier in Tessaly Bay, 40 hurt", "The coast guard says the vessel lost steering during its morning crossing.", 2, matchNow.Add(-2*time.Hour)),
		frontStory("/pub/texture/other", "Valdoria bans night freight on Merrow Bridge after cracks found", "Inspectors found fatigue cracks in the steel deck of the 1960s crossing.", 1, matchNow.Add(-3*time.Hour)),
	}
	b.eps = []EpisodeInfo{{EpisodeID: "ep-1", FeedURL: "f", GUID: "g1", Show: "Show A", Title: "Morning", Published: matchNow.Add(-4 * time.Hour), HasTranscript: true}}
	b.transcripts["g1"] = ferrySpoken()
	for i := 0; i < 120; i++ { // a corpus of the usual size around it
		id := fmt.Sprintf("pad-%d", i)
		b.eps = append(b.eps, EpisodeInfo{EpisodeID: id, FeedURL: "f", GUID: id, Show: "Show B", Title: id, Published: matchNow.Add(-5 * time.Hour), HasTranscript: true})
		b.transcripts[id] = spoken("Nothing to report from the garden club.", "The fair opens on Saturday.", "Apples are in season.")
	}
	return b
}

func newMatcher(b MatchBackend, logs *[]string) *Matcher {
	return &Matcher{B: b, Params: podcastmatch.DefaultParams, Now: func() time.Time { return matchNow }, Logf: func(f string, a ...any) { *logs = append(*logs, fmt.Sprintf(f, a...)) }}
}

func TestA1_StoriesOutsideTheWindowAreNotMatched(t *testing.T) {
	b := newFakeMatch()
	old := frontStory("/pub/texture/old", "Old story about Tessaly Bay ferry Marlin pier", "Harborview", 1, matchNow.Add(-100*time.Hour))
	op := frontStory("/pub/texture/op", "Opinion on the Marlin ferry in Tessaly Bay", "Harborview", 1, matchNow.Add(-1*time.Hour))
	op.Kind = "opinion"
	b.front = append(b.front, old, op, frontStory("", "No route", "x", 1, matchNow))
	var logs []string
	rep, err := newMatcher(b, &logs).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Stories != 2 {
		t.Fatalf("A1: %d stories considered, want 2", rep.Stories)
	}
	for _, put := range b.puts {
		for _, c := range put {
			if strings.Contains(c.StoryRoute, "old") || strings.Contains(c.StoryRoute, "op") && !strings.Contains(c.StoryRoute, "other") {
				t.Fatalf("A1: clip for %s", c.StoryRoute)
			}
		}
	}
}

func TestA2_A3_SecondRunIsIdempotentAndNewMaterialIsMatched(t *testing.T) {
	b := newFakeMatch()
	var logs []string
	m := newMatcher(b, &logs)
	rep, err := m.Run(context.Background())
	if err != nil || rep.Written != 1 || len(b.puts) != 1 || b.puts[0][0].StoryRoute != "/pub/texture/ferry" {
		t.Fatalf("first run: %+v %v puts %v", rep, err, b.puts)
	}
	c := b.puts[0][0]
	if c.Support <= 0 || c.Support > 1 || c.End-c.Start < 20 || c.Quote == "" || c.EpisodeID != "ep-1" {
		t.Fatalf("clip %+v", c)
	}
	rep2, _ := m.Run(context.Background())
	if len(b.puts) != 1 || rep2.Written != 0 || rep2.Unchanged != 2 || rep2.Scored != 0 {
		t.Fatalf("A2: second run %+v puts %d", rep2, len(b.puts))
	}
	// A new episode of the same event arrives: matched, and the first clip is not written again.
	b.eps = append(b.eps, EpisodeInfo{EpisodeID: "ep-2", FeedURL: "f", GUID: "g2", Show: "Show C", Title: "Noon", Published: matchNow.Add(-1 * time.Hour), HasTranscript: true})
	b.transcripts["g2"] = spoken("Tessaly Bay saw a collision this morning when the Marlin, a Harborview ferry, struck a pier.", "About forty passengers were hurt and the coast guard said the vessel lost steering.", "Crews pulled the ferry clear and divers checked the damaged pier in the bay.", "The Marlin was carrying commuters across Tessaly Bay at the time of the crash.", "Harborview said it would cooperate with the coast guard inquiry.")
	rep3, _ := m.Run(context.Background())
	if rep3.Written != 1 || len(b.puts) != 2 || b.puts[1][0].EpisodeID != "ep-2" {
		t.Fatalf("A3: %+v puts %v", rep3, b.puts)
	}
	// A new story version re-matches, still without rewriting.
	b.front[0].LatestRevision = 3
	rep4, _ := m.Run(context.Background())
	if rep4.Unchanged != 1 || rep4.Written != 0 {
		t.Fatalf("A3: version bump %+v", rep4)
	}
}

func TestA4_DryRunWritesNothingAndSavesNothing(t *testing.T) {
	b := newFakeMatch()
	var logs []string
	var out strings.Builder
	dir := t.TempDir()
	m := newMatcher(b, &logs)
	m.DryRun, m.Out, m.StatePath = true, &out, filepath.Join(dir, "state.json")
	rep, err := m.Run(context.Background())
	if err != nil || len(b.puts) != 0 || rep.Written != 0 || rep.WouldWrite != 1 {
		t.Fatalf("A4: %+v %v puts %d", rep, err, len(b.puts))
	}
	if _, err := os.Stat(m.StatePath); err == nil {
		t.Fatal("A4: dry run saved state")
	}
	if !strings.Contains(out.String(), "would write") || !strings.Contains(out.String(), "Show A") {
		t.Fatalf("A4: output %q", out.String())
	}
	rep2, _ := m.Run(context.Background())
	if rep2.WouldWrite != 1 {
		t.Fatal("A4: a dry run must show the same clips again")
	}
}

func TestA5_OneLogLinePerRunWithTheCounts(t *testing.T) {
	b := newFakeMatch()
	var logs []string
	if _, err := newMatcher(b, &logs).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("A5: %d log lines: %v", len(logs), logs)
	}
	for _, want := range []string{"stories considered 2", "passages scored", "accepted 1", "rejected time=", "entity=", "margin=", "score=", "clip=", "boundary=", "cap=", "written 1"} {
		if !strings.Contains(logs[0], want) {
			t.Errorf("A5: %q missing from %q", want, logs[0])
		}
	}
}

func TestA6_A7_StateSurvivesARestartAndForgetsStoriesThatLeft(t *testing.T) {
	b := newFakeMatch()
	var logs []string
	path := filepath.Join(t.TempDir(), "state.json")
	m := newMatcher(b, &logs)
	m.StatePath = path
	if _, err := m.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	m2 := newMatcher(b, &logs)
	m2.StatePath = path
	rep, _ := m2.Run(context.Background())
	if rep.Unchanged != 2 || len(b.puts) != 1 {
		t.Fatalf("A7: after a restart %+v puts %d", rep, len(b.puts))
	}
	b.front = b.front[1:] // the ferry story left the front
	m2.Run(context.Background())
	if _, ok := m2.state.Stories["/pub/texture/ferry"]; ok {
		t.Fatal("A6: a story that left the window is still in the state")
	}
	for k := range m2.state.Written {
		if strings.HasPrefix(k, "/pub/texture/ferry|") {
			t.Fatal("A6: its written clips are still in the state")
		}
	}
}

func TestA8_TheLoopMatchesWhenTheQueueIsDrainedAndOnTheTimer(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("a"), ep("b"), ep("c")}}
	e := &fakeEngine{}
	calls := 0
	storedWhenCalled := []int{}
	cfg := ServeConfig{Match: func(context.Context) error {
		calls++
		storedWhenCalled = append(storedWhenCalled, len(b.put))
		return nil
	}}
	runServe(t, b, e, 3, cfg)
	// Once at the start (timer never run), once after the queue drained: never between episodes.
	for _, n := range storedWhenCalled {
		if n != 0 && n != 3 {
			t.Fatalf("A8: matched after %d of 3 transcripts: %v", n, storedWhenCalled)
		}
	}
	if calls < 2 || storedWhenCalled[len(storedWhenCalled)-1] != 3 {
		t.Fatalf("A8: calls %v", storedWhenCalled)
	}
}

func TestA9_APlatformRefusalIsFinalANetworkFailureIsRetried(t *testing.T) {
	b := newFakeMatch()
	b.putErr = func(ClipIn) error { return &HTTPError{Method: "PUT", Path: "/clips", Status: 400, Body: "refused"} }
	var logs []string
	m := newMatcher(b, &logs)
	rep, _ := m.Run(context.Background())
	if rep.Refused != 1 {
		t.Fatalf("A9: %+v", rep)
	}
	rep, _ = m.Run(context.Background())
	if len(b.puts) != 1 {
		t.Fatalf("A9: a refused clip was sent again (%d puts)", len(b.puts))
	}
	b2 := newFakeMatch()
	b2.putErr = func(ClipIn) error { return fmt.Errorf("network down") }
	m2 := newMatcher(b2, &logs)
	m2.Run(context.Background())
	m2.Run(context.Background())
	if len(b2.puts) != 2 {
		t.Fatalf("A9: a transient failure was not retried (%d puts)", len(b2.puts))
	}
}

func TestA10_TheBackendHasNoModelCall(t *testing.T) {
	// MatchBackend is the whole of what the matcher can reach.
	var _ MatchBackend = (*Client)(nil)
	var _ MatchBackend = (*fakeMatchBackend)(nil)
}

// Review of 5cc739079, written first:
//   B1. a restart with no state file writes again what the server holds, or a
//       fourth clip, or a second clip of an episode the story has;
//   B2. one audio under two feed entries gives two clips;
//   B3. a 401 or 403 marks clips written;
//   B4. the report does not count the requests of the run;
//   B5. the state file defaults to "only for this run".

func TestB1_ARestartWithNoStateDoesNotWriteWhatTheServerHolds(t *testing.T) {
	b := newFakeMatch()
	var logs []string
	if rep, _ := newMatcher(b, &logs).Run(context.Background()); rep.Written != 1 {
		t.Fatalf("first: %+v", rep)
	}
	// A new process, no state, the story's version bumped: the server's clip is the memory.
	b.front[0].LatestRevision = 9
	rep, _ := newMatcher(b, &logs).Run(context.Background())
	if rep.Written != 0 || len(b.puts) != 1 {
		t.Fatalf("B1: wrote again: %+v puts %d", rep, len(b.puts))
	}
}

func TestB1_TheServersThreeClipsFillTheStory(t *testing.T) {
	b := newFakeMatch()
	for i := 0; i < 3; i++ {
		b.held = append(b.held, HeldClip{Route: "/pub/texture/ferry", FeedURL: "f", GUID: fmt.Sprintf("other-%d", i), Quote: fmt.Sprintf("something else entirely %d", i)})
	}
	var logs []string
	rep, _ := newMatcher(b, &logs).Run(context.Background())
	if rep.Written != 0 || len(b.puts) != 0 {
		t.Fatalf("B1: a fourth clip: %+v", rep)
	}
}

func TestB1_TheWrittenKeyIsTheServersClipIdentity(t *testing.T) {
	if clipKey("/r", "e", 32.6) != clipKey("/r", "e", 32.7) || clipKey("/r", "e", 32.4) == clipKey("/r", "e", 33.6) {
		t.Fatal("B1: the key is not int(round(start))")
	}
}

func TestB2_OneAudioUnderTwoFeedEntriesGivesOneClip(t *testing.T) {
	b := newFakeMatch()
	b.eps[0].EnclosureURL = "https://cdn.example.test/a.mp3?session=1"
	dup := b.eps[0]
	dup.EpisodeID, dup.GUID, dup.EnclosureURL = "ep-dup", "gdup", "https://CDN.example.test/a.mp3?session=2"
	b.eps = append(b.eps, dup)
	b.transcripts["gdup"] = ferrySpoken()
	var logs []string
	rep, _ := newMatcher(b, &logs).Run(context.Background())
	if rep.Written != 1 {
		t.Fatalf("B2: %+v", rep)
	}
}

func TestB3_AnAuthFailureIsNotAContentRefusal(t *testing.T) {
	b := newFakeMatch()
	b.putErr = func(ClipIn) error { return &HTTPError{Method: "PUT", Path: "/clips", Status: 401, Body: "no"} }
	var logs []string
	m := newMatcher(b, &logs)
	rep, _ := m.Run(context.Background())
	if rep.Refused != 0 || rep.Failed != 1 {
		t.Fatalf("B3: %+v", rep)
	}
	m.Run(context.Background())
	if len(b.puts) != 2 {
		t.Fatalf("B3: not retried after an auth failure (%d puts)", len(b.puts))
	}
}

func TestB4_TheReportCountsRequests(t *testing.T) {
	b := newFakeMatch()
	var logs []string
	rep, _ := newMatcher(b, &logs).Run(context.Background())
	// front, episodes, clips read, 31 transcripts (1 + 120 pads: all uncached), clip write
	want := 1 + 1 + 1 + len(b.eps) + 1
	if rep.Requests != want || !strings.Contains(logs[0], fmt.Sprintf("requests %d", want)) {
		t.Fatalf("B4: requests %d want %d; %s", rep.Requests, want, logs[0])
	}
}

func TestB5_TheStateFileHasAStableDefaultBesideTheWorkDir(t *testing.T) {
	if got := DefaultMatchStatePath("/data/work"); got != "/data/match-state.json" {
		t.Fatalf("B5: %q", got)
	}
	if got := DefaultMatchStatePath(""); !strings.HasSuffix(got, "match-state.json") || got == "match-state.json" {
		t.Fatalf("B5: %q", got)
	}
}
