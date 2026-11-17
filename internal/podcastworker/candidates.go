package podcastworker

// The operator's hand path to clips (docs/design/podcast-clips-2026-11-15.md
// 3.5). There is no model call in this code: `candidates` ranks the paper's
// stories against the stored transcripts with internal/podcastmatch (BM25,
// windows, clip boundaries on segment edges) and writes the candidates to a
// file for a human or an agent to judge; `put-clips` sends the ones judged
// "support > 0" through the operator route, whose checks (the story is on the
// front, the feed featured, the quote is in the transcript within the clip's
// times) are the server's and unchanged.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/podcastmatch"
)

const (
	gistRunes            = 700
	episodesBeforeStory  = 48 * time.Hour
	episodesAfterStory   = 6 * time.Hour
	candidateWindowsKept = 12
)

// FrontStory is a story of the paper's front list.
type FrontStory struct {
	Title          string    `json:"title"`
	Dek            string    `json:"dek"`
	UpdatedAt      time.Time `json:"updated_at"`
	LatestRevision int       `json:"latest_revision"`
	Kind           string    `json:"kind"`
	Route          struct {
		Bare string `json:"bare"`
	} `json:"route"`
}

// EpisodeInfo is an episode as the operator listing gives it.
type EpisodeInfo struct {
	EpisodeID       string    `json:"episode_id"`
	FeedURL         string    `json:"feed_url"`
	GUID            string    `json:"guid"`
	Show            string    `json:"show"`
	Title           string    `json:"title"`
	Published       time.Time `json:"published"`
	DurationSeconds float64   `json:"duration_seconds"`
	EnclosureURL    string    `json:"enclosure_url"`
	HasTranscript   bool      `json:"has_transcript"`
}

// Candidate is one (story, passage) pair for someone to judge. The judge sets
// Support (0 to 1) on the pairs that report the same event and leaves the rest.
type Candidate struct {
	StoryRoute   string    `json:"story_route"`
	StoryVersion int       `json:"story_version"`
	StoryTitle   string    `json:"story_title"`
	StoryDek     string    `json:"story_dek"`
	EpisodeID    string    `json:"episode_id"`
	Show         string    `json:"show"`
	EpisodeTitle string    `json:"episode_title"`
	Published    time.Time `json:"published"`
	Start        float64   `json:"start"`
	End          float64   `json:"end"`
	Quote        string    `json:"quote"`
	Passage      string    `json:"passage"`
	Score        float64   `json:"score"`
	// Support is the judge's: 0 or absent means not chosen.
	Support float64 `json:"support,omitempty"`
}

// CandidatesFile is the file `candidates` writes and `put-clips` reads.
type CandidatesFile struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Candidates  []Candidate `json:"candidates"`
}

// Front lists the paper's front stories (a public route).
func (c *Client) Front(ctx context.Context, limit int) ([]FrontStory, error) {
	var ans struct {
		Stories []FrontStory `json:"stories"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/platform/publications/front?limit=%d", limit), nil, &ans); err != nil {
		return nil, err
	}
	return ans.Stories, nil
}

// StoryText reads a story's text (a public route).
func (c *Client) StoryText(ctx context.Context, route string) (string, error) {
	var ans struct {
		Content string `json:"content"`
	}
	q := url.Values{"route": {route}, "format": {"txt"}}
	if err := c.do(ctx, http.MethodGet, "/api/platform/publications/export?"+q.Encode(), nil, &ans); err != nil {
		return "", err
	}
	return ans.Content, nil
}

// Episodes lists the operator's episodes, newest first (all of them, not only
// those that need a transcript).
func (c *Client) Episodes(ctx context.Context, since time.Time, limit int) ([]EpisodeInfo, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	if !since.IsZero() {
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	var ans struct {
		Episodes []EpisodeInfo `json:"episodes"`
	}
	if err := c.do(ctx, http.MethodGet, opBase+"episodes?"+q.Encode(), nil, &ans); err != nil {
		return nil, err
	}
	return ans.Episodes, nil
}

// Transcript reads an episode's transcript blocks (the signed-in route).
func (c *Client) Transcript(ctx context.Context, feedURL, guid string) ([]Segment, error) {
	var ans struct {
		Blocks []Segment `json:"blocks"`
	}
	q := url.Values{"feed": {feedURL}, "guid": {guid}}
	if err := c.do(ctx, http.MethodGet, "/api/platform/podcast-transcript?"+q.Encode(), nil, &ans); err != nil {
		return nil, err
	}
	return ans.Blocks, nil
}

// ClipIn is what the operator's clips route takes.
type ClipIn struct {
	StoryRoute string  `json:"story_route"`
	EpisodeID  string  `json:"episode_id"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Quote      string  `json:"quote"`
	Support    float64 `json:"support"`
}

// PutClips sends clips in requests of at most 200. A request the server refuses
// is retried clip by clip, so one bad clip costs only itself; the refusals are
// returned with the count stored.
func (c *Client) PutClips(ctx context.Context, clips []ClipIn) (stored int, refused []string) {
	put := func(batch []ClipIn) (int, error) {
		var ans struct {
			Clips int `json:"clips"`
		}
		err := c.do(ctx, http.MethodPut, opBase+"clips", map[string]any{"clips": batch}, &ans)
		return ans.Clips, err
	}
	for start := 0; start < len(clips); start += 200 {
		batch := clips[start:min(start+200, len(clips))]
		n, err := put(batch)
		if err == nil {
			stored += n
			continue
		}
		for _, one := range batch {
			if n, err := put([]ClipIn{one}); err == nil {
				stored += n
			} else {
				refused = append(refused, fmt.Sprintf("%s episode %s %.0f-%.0f: %v", one.StoryRoute, one.EpisodeID, one.Start, one.End, err))
			}
		}
	}
	return stored, refused
}

func gistOf(title, dek, body string) string {
	g := strings.TrimSpace(title + ". " + dek)
	body = strings.Join(strings.Fields(body), " ")
	if dek != "" {
		if i := strings.Index(body, dek); i >= 0 {
			body = body[i+len(dek):]
		}
	}
	g += " " + body
	if utf8.RuneCountInString(g) > gistRunes {
		g = string([]rune(g)[:gistRunes])
	}
	return g
}

// BuildCandidates ranks the paper's recent stories against the stored
// transcripts and returns, for each story, up to perStory passages from
// episodes published within two days before it and six hours after its last
// update, on segment boundaries, best first, at most one per episode.
func BuildCandidates(ctx context.Context, c *Client, stories, perStory int, logf func(string, ...any)) (*CandidatesFile, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	front, err := c.Front(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("front list: %w", err)
	}
	var picked []FrontStory
	for _, st := range front {
		if st.Kind == "opinion" || st.Route.Bare == "" {
			continue
		}
		picked = append(picked, st)
		if len(picked) == stories {
			break
		}
	}
	if len(picked) == 0 {
		return &CandidatesFile{GeneratedAt: time.Now().UTC()}, nil
	}
	earliest := picked[len(picked)-1].UpdatedAt.Add(-episodesBeforeStory)
	for _, st := range picked {
		if st.UpdatedAt.Before(earliest.Add(episodesBeforeStory)) {
			earliest = st.UpdatedAt.Add(-episodesBeforeStory)
		}
	}
	eps, err := c.Episodes(ctx, earliest, 500)
	if err != nil {
		return nil, fmt.Errorf("episodes: %w", err)
	}
	type loaded struct {
		info EpisodeInfo
		segs []podcastmatch.Segment
	}
	byID := map[string]*loaded{}
	var wins []podcastmatch.Window
	for _, e := range eps {
		if !e.HasTranscript {
			continue
		}
		blocks, err := c.Transcript(ctx, e.FeedURL, e.GUID)
		if err != nil {
			logf("episode %s: transcript unavailable: %v", e.EpisodeID, err)
			continue
		}
		segs := make([]podcastmatch.Segment, len(blocks))
		for i, b := range blocks {
			segs[i] = podcastmatch.Segment{Start: b.Start, End: b.End, Text: b.Text}
		}
		byID[e.EpisodeID] = &loaded{info: e, segs: segs}
		wins = append(wins, podcastmatch.Windows(e.EpisodeID, segs)...)
	}
	out := &CandidatesFile{GeneratedAt: time.Now().UTC()}
	if len(wins) == 0 {
		return out, nil
	}
	gists := make([]string, len(picked))
	var queries []podcastmatch.Query
	for i, st := range picked {
		body, err := c.StoryText(ctx, st.Route.Bare)
		if err != nil {
			logf("story %s: text unavailable, using its title and dek: %v", st.Route.Bare, err)
		}
		gists[i] = gistOf(st.Title, st.Dek, body)
		queries = append(queries, podcastmatch.Query{ID: fmt.Sprint(i), Text: gists[i]})
	}
	ranked, err := podcastmatch.Rank(ctx, queries, wins, candidateWindowsKept)
	if err != nil {
		return nil, err
	}
	for i, st := range picked {
		seen := map[string]bool{}
		for _, h := range ranked[fmt.Sprint(i)] {
			l := byID[h.Window.Key]
			if l == nil || seen[l.info.EpisodeID] {
				continue
			}
			if l.info.Published.Before(st.UpdatedAt.Add(-episodesBeforeStory)) || l.info.Published.After(st.UpdatedAt.Add(episodesAfterStory)) {
				continue
			}
			clip, ok := podcastmatch.ChooseClip(l.segs, *h.Window, gists[i])
			if !ok {
				continue
			}
			if l.info.DurationSeconds > 0 && clip.End > l.info.DurationSeconds+5 {
				continue
			}
			seen[l.info.EpisodeID] = true
			out.Candidates = append(out.Candidates, Candidate{
				StoryRoute: st.Route.Bare, StoryVersion: st.LatestRevision, StoryTitle: st.Title, StoryDek: st.Dek,
				EpisodeID: l.info.EpisodeID, Show: l.info.Show, EpisodeTitle: l.info.Title, Published: l.info.Published,
				Start: clip.Start, End: clip.End, Quote: clip.Quote, Passage: clip.Text, Score: h.Score,
			})
			if len(seen) == perStory {
				break
			}
		}
	}
	return out, nil
}

// WriteCandidates writes the file.
func WriteCandidates(path string, f *CandidatesFile) error {
	raw, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// JudgedClips reads a candidates file and returns the pairs the judge chose
// (Support above 0 and at most 1), best supported first per story, in file
// order otherwise.
func JudgedClips(path string) ([]ClipIn, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f CandidatesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var out []ClipIn
	for _, c := range f.Candidates {
		if c.Support <= 0 {
			continue
		}
		if c.Support > 1 {
			return nil, fmt.Errorf("%s: support %v of story %s is outside (0, 1]", path, c.Support, c.StoryRoute)
		}
		out = append(out, ClipIn{StoryRoute: c.StoryRoute, EpisodeID: c.EpisodeID, Start: c.Start, End: c.End, Quote: c.Quote, Support: c.Support})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StoryRoute < out[j].StoryRoute })
	return out, nil
}
