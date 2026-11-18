package podcastworker

// The automatic matcher (no model call, so no Jev call: nothing here for
// scripts/check-jev-entry). It does what the operator did by hand with
// `candidates`, a judge and `put-clips`, with the judging replaced by the
// explicit rules of internal/podcastmatch (time, clip shape, sentence edges,
// score floor, entity gate, margin over other stories). The clips go through
// the same operator route and its server-side checks.
//
// Idempotence: a state (kept in a file when StatePath is set) holds, per story
// route, the version and the set of transcribed episodes it was last matched
// against, and the clips already written. A story is matched again only when
// its version or that set changes, a clip written is not written again, and a
// story that has left the window is forgotten.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
	"github.com/yusefmosiah/go-choir/internal/podcastmatch"
)

// StoryWindow is how long a story stays matchable: the platform serves clips
// for stories of the front updated within 72 hours (podcast_read.go).
const StoryWindow = 72 * time.Hour

// MatchBackend is all the matcher can reach: platform reads and the clips write.
type MatchBackend interface {
	Front(ctx context.Context, limit int) ([]FrontStory, error)
	Episodes(ctx context.Context, since time.Time, limit int) ([]EpisodeInfo, error)
	Transcript(ctx context.Context, feedURL, guid string) ([]Segment, error)
	PutClipsEach(ctx context.Context, clips []ClipIn) []error
	Clips(ctx context.Context, routes []string) ([]HeldClip, error)
}

type storyState struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
}

type matchState struct {
	Stories map[string]storyState `json:"stories"`
	Written map[string]time.Time  `json:"written"`
}

// Matcher is one worker's matcher.
type Matcher struct {
	B         MatchBackend
	Params    podcastmatch.Params
	Stories   int // newest front stories considered (default 100)
	DryRun    bool
	StatePath string
	Out       io.Writer // dry-run lines
	Now       func() time.Time
	Logf      func(string, ...any)
	// Tracks, when set, gives each clip its fingerprint from the track kept
	// when the episode was transcribed; a clip of an episode with no track is
	// written without one (the repair pass makes it later).
	Tracks interface {
		Load(episodeID string) (*Track, error)
	}

	state       *matchState
	transcripts map[string][]podcastmatch.Segment
}

// MatchReport is one run's counts.
type MatchReport struct {
	Stories        int // considered (matched this run)
	Unchanged      int // in the window, nothing new since the last match
	Scored         int
	Passages       int
	Accepted       int
	Rejected       map[podcastmatch.Rule]int
	Written        int
	WouldWrite     int
	AlreadyWritten int
	Refused        int
	Failed         int
	Requests       int // calls to the platform this run
}

func (m *Matcher) load() {
	if m.state != nil {
		return
	}
	m.state = &matchState{Stories: map[string]storyState{}, Written: map[string]time.Time{}}
	if m.StatePath == "" {
		return
	}
	if raw, err := os.ReadFile(m.StatePath); err == nil {
		var st matchState
		if json.Unmarshal(raw, &st) == nil {
			if st.Stories != nil {
				m.state.Stories = st.Stories
			}
			if st.Written != nil {
				m.state.Written = st.Written
			}
		}
	}
}

func (m *Matcher) save() {
	if m.StatePath == "" || m.DryRun {
		return
	}
	raw, err := json.MarshalIndent(m.state, "", " ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.StatePath), 0o700); err != nil {
		m.logf("podcast-worker match: state not saved: %v", err)
		return
	}
	tmp := m.StatePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		m.logf("podcast-worker match: state not saved: %v", err)
		return
	}
	if err := os.Rename(tmp, m.StatePath); err != nil {
		os.Remove(tmp)
		m.logf("podcast-worker match: state not saved: %v", err)
	}
}

func (m *Matcher) logf(f string, a ...any) {
	if m.Logf != nil {
		m.Logf(f, a...)
		return
	}
	fmt.Fprintf(os.Stderr, f+"\n", a...)
}

// clipKey is the platform's clip identity: story, episode and the start
// rounded to the second (internal/platform podcastClipID).
func clipKey(route, episodeID string, start float64) string {
	return fmt.Sprintf("%s|%s|%d", route, episodeID, int(math.Round(start)))
}

// DefaultMatchStatePath is where the matcher's state lives when none is named:
// beside the work directory (so it outlives a hand-started process), or in the
// user's cache directory.
func DefaultMatchStatePath(workDir string) string {
	if workDir != "" {
		return filepath.Join(filepath.Dir(filepath.Clean(workDir)), "match-state.json")
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "choir-podcast-worker", "match-state.json")
	}
	return filepath.Join(os.TempDir(), "choir-podcast-match-state.json")
}

func audioKeyURL(raw string) string {
	if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Host != "" {
		return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + u.Path
	}
	return ""
}

// audioKey is the identity of an episode's audio: its enclosure without query
// or fragment, so two feed entries of one file are one; without an enclosure,
// the feed, title and publish time.
func audioKey(e EpisodeInfo) string {
	if k := audioKeyURL(e.EnclosureURL); k != "" {
		return k
	}
	return e.FeedURL + "|" + e.Title + "|" + e.Published.UTC().Format(time.RFC3339)
}

// Run matches once.
func (m *Matcher) Run(ctx context.Context) (MatchReport, error) {
	m.load()
	var before int
	counter, counts := m.B.(interface{ Requests() int })
	if counts {
		before = counter.Requests()
	}
	var rep MatchReport
	done := func() {
		if counts {
			rep.Requests = counter.Requests() - before
		}
		m.report(rep)
	}
	now := time.Now()
	if m.Now != nil {
		now = m.Now()
	}
	rep.Rejected = map[podcastmatch.Rule]int{}
	limit := m.Stories
	if limit <= 0 {
		limit = 100
	}
	front, err := m.B.Front(ctx, 100)
	if err != nil {
		return rep, fmt.Errorf("front list: %w", err)
	}
	var stories []FrontStory
	for _, st := range front {
		if st.Kind == "opinion" || st.Route.Bare == "" || now.Sub(st.UpdatedAt) > StoryWindow {
			continue
		}
		stories = append(stories, st)
		if len(stories) == limit {
			break
		}
	}
	// Forget the stories that left the window, with their written clips.
	live := map[string]bool{}
	for _, st := range stories {
		live[st.Route.Bare] = true
	}
	for route := range m.state.Stories {
		if !live[route] {
			delete(m.state.Stories, route)
		}
	}
	for k := range m.state.Written {
		if !live[strings.SplitN(k, "|", 2)[0]] {
			delete(m.state.Written, k)
		}
	}
	if len(stories) == 0 {
		m.save()
		done()
		return rep, nil
	}
	earliest := now
	for _, st := range stories {
		if t := st.UpdatedAt.Add(-m.Params.Before); t.Before(earliest) {
			earliest = t
		}
	}
	eps, err := m.B.Episodes(ctx, earliest, 500)
	if err != nil {
		return rep, fmt.Errorf("episodes: %w", err)
	}
	// One audio under two feed entries is one episode: the first by id stays.
	sort.SliceStable(eps, func(i, j int) bool { return eps[i].EpisodeID < eps[j].EpisodeID })
	var have []EpisodeInfo
	audioOf := map[string]string{} // "feed\x00guid" -> audio key
	seenAudio := map[string]bool{}
	for _, e := range eps {
		audioOf[e.FeedURL+"\x00"+e.GUID] = audioKey(e)
		if !e.HasTranscript || seenAudio[audioKey(e)] {
			continue
		}
		seenAudio[audioKey(e)] = true
		have = append(have, e)
	}
	// Which stories have something new to match against.
	digests := map[string]string{}
	subjects := map[string]bool{}
	for _, st := range stories {
		var ids []string
		for _, e := range have {
			if !e.Published.Before(st.UpdatedAt.Add(-m.Params.Before)) && !e.Published.After(st.UpdatedAt.Add(m.Params.After)) {
				ids = append(ids, e.EpisodeID)
			}
		}
		sort.Strings(ids)
		sum := sha256.Sum256([]byte(strings.Join(ids, ",")))
		digests[st.Route.Bare] = hex.EncodeToString(sum[:8])
		if old, ok := m.state.Stories[st.Route.Bare]; ok && old.Version == st.LatestRevision && old.Digest == digests[st.Route.Bare] {
			rep.Unchanged++
			continue
		}
		subjects[st.Route.Bare] = true
	}
	if len(subjects) == 0 {
		m.save()
		done()
		return rep, nil
	}
	if m.transcripts == nil {
		m.transcripts = map[string][]podcastmatch.Segment{}
	}
	var pm []podcastmatch.Episode
	info := map[string]EpisodeInfo{}
	seen := map[string]bool{}
	for _, e := range have {
		seen[e.EpisodeID] = true
		segs, cached := m.transcripts[e.EpisodeID]
		if !cached {
			blocks, err := m.B.Transcript(ctx, e.FeedURL, e.GUID)
			if err != nil {
				m.logf("podcast-worker match: episode %s: transcript unavailable: %v", e.EpisodeID, err)
				continue
			}
			segs = make([]podcastmatch.Segment, len(blocks))
			for i, b := range blocks {
				segs[i] = podcastmatch.Segment{Start: b.Start, End: b.End, Text: b.Text}
			}
			m.transcripts[e.EpisodeID] = segs
		}
		info[e.EpisodeID] = e
		pm = append(pm, podcastmatch.Episode{Key: e.EpisodeID, AudioKey: audioKey(e), Published: e.Published, DurationSeconds: e.DurationSeconds, Segs: segs})
	}
	for id := range m.transcripts {
		if !seen[id] {
			delete(m.transcripts, id)
		}
	}
	// What the platform already holds for these stories: it counts toward the
	// cap, and its audio and words are not offered again. The clips written in
	// an earlier run, or by hand, are the memory; the state file only saves the
	// writes.
	var routes []string
	for _, st := range stories {
		routes = append(routes, st.Route.Bare)
	}
	heldRows, err := m.B.Clips(ctx, routes)
	if err != nil {
		return rep, fmt.Errorf("read the clips the platform holds: %w", err)
	}
	held := map[string][]podcastmatch.Held{}
	for _, h := range heldRows {
		ak := audioKeyURL(h.AudioURL) // the platform's own enclosure, as for the episodes
		if ak == "" {
			var ok bool
			if ak, ok = audioOf[h.FeedURL+"\x00"+h.GUID]; !ok {
				ak = h.FeedURL + "\x00" + h.GUID
			}
		}
		held[h.Route] = append(held[h.Route], podcastmatch.Held{AudioKey: ak, Text: h.Quote})
	}
	var ms []podcastmatch.Story
	byRoute := map[string]FrontStory{}
	for _, st := range stories {
		ms = append(ms, podcastmatch.Story{ID: st.Route.Bare, Title: st.Title, Dek: st.Dek, Updated: st.UpdatedAt})
		byRoute[st.Route.Bare] = st
	}
	res, err := podcastmatch.MatchSubjects(ctx, ms, subjects, held, pm, m.Params)
	if err != nil {
		return rep, err
	}
	rep.Stories, rep.Scored, rep.Passages, rep.Accepted, rep.Rejected = res.Stories, res.Scored, res.Passages, len(res.Accepted), res.Rejected
	var fresh []ClipIn
	var freshKeys []string
	for _, a := range res.Accepted {
		key := clipKey(a.StoryID, a.EpisodeKey, a.Start)
		if _, done := m.state.Written[key]; done {
			rep.AlreadyWritten++
			continue
		}
		in := ClipIn{StoryRoute: a.StoryID, EpisodeID: a.EpisodeKey, Start: a.Start, End: a.End, Quote: a.Quote, Support: a.Support}
		in.Fingerprint = m.fingerprint(a.EpisodeKey, a.Start, a.End)
		fresh = append(fresh, in)
		freshKeys = append(freshKeys, key)
		if m.DryRun && m.Out != nil {
			e := info[a.EpisodeKey]
			fmt.Fprintf(m.Out, "would write: story %q | %s | %q | %.1f-%.1f s | support %.2f (score %.2f, share %.2f, margin %.1f)\n  quote: %s\n",
				byRoute[a.StoryID].Title, e.Show, e.Title, a.Start, a.End, a.Support, a.Score, a.EntityShare, a.Margin, a.Quote)
		}
	}
	failed := map[string]bool{}
	if m.DryRun {
		rep.WouldWrite = len(fresh)
	} else if len(fresh) > 0 {
		errs := m.B.PutClipsEach(ctx, fresh)
		for i, err := range errs {
			switch {
			case err == nil:
				rep.Written++
				m.state.Written[freshKeys[i]] = now
			case IsRefusal(err):
				// The platform's checks refuse this clip every time.
				rep.Refused++
				m.state.Written[freshKeys[i]] = now
				m.logf("podcast-worker match: clip refused: story %s episode %s %.0f-%.0f: %v", fresh[i].StoryRoute, fresh[i].EpisodeID, fresh[i].Start, fresh[i].End, err)
			default:
				rep.Failed++
				failed[fresh[i].StoryRoute] = true
				m.logf("podcast-worker match: clip not written, will retry: story %s: %v", fresh[i].StoryRoute, err)
			}
		}
	}
	if !m.DryRun {
		for route := range subjects {
			if failed[route] {
				continue // matched again next run
			}
			m.state.Stories[route] = storyState{Version: byRoute[route].LatestRevision, Digest: digests[route]}
		}
	}
	m.save()
	done()
	return rep, nil
}

// fingerprint cuts a clip's fingerprint from the episode's kept track, or nil.
func (m *Matcher) fingerprint(episodeID string, start, end float64) *podcastfp.ClipRecord {
	if m.Tracks == nil {
		return nil
	}
	tr, err := m.Tracks.Load(episodeID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			m.logf("podcast-worker match: episode %s: track unreadable, clip written without a fingerprint: %v", episodeID, err)
		}
		return nil
	}
	rec, err := tr.FingerprintClip(start, end)
	if err != nil {
		m.logf("podcast-worker match: episode %s: no fingerprint for %.0f-%.0f: %v", episodeID, start, end, err)
		return nil
	}
	return rec
}

func (m *Matcher) report(r MatchReport) {
	var rej []string
	for _, rule := range podcastmatch.Rules {
		rej = append(rej, fmt.Sprintf("%s=%d", rule, r.Rejected[rule]))
	}
	verb := fmt.Sprintf("written %d (already written %d, refused %d, failed %d)", r.Written, r.AlreadyWritten, r.Refused, r.Failed)
	if m.DryRun {
		verb = fmt.Sprintf("dry run: would write %d (already written %d)", r.WouldWrite, r.AlreadyWritten)
	}
	m.logf("podcast-worker match: stories considered %d (unchanged %d), passages scored %d, passages judged %d, accepted %d, rejected %s, %s, requests %d",
		r.Stories, r.Unchanged, r.Scored, r.Passages, r.Accepted, strings.Join(rej, " "), verb, r.Requests)
}
