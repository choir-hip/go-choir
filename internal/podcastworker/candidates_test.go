package podcastworker

// The operator's hand path (docs/design/podcast-clips-2026-11-15.md 3.5): no
// model, a file of candidates, and the judged ones sent through the operator
// route. Failure list, written first:
//   H1. a candidate is offered for a story with no passage in common, from an
//       episode with no transcript, from one published days before the story,
//       or for an opinion piece;
//   H2. two passages of one episode are offered for one story;
//   H3. a clip is outside 24 to 75 s or off the segment boundaries, or its
//       quote is not words of its passage;
//   H4. put-clips sends a candidate nobody judged, or one with support over 1;
//   H5. one refused clip costs the others their request;
//   H6. the key is not sent, or goes in the URL.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func harborBlocks() []Segment {
	filler := []string{
		"Meanwhile the weather in the valley stays mild with light wind through the evening.",
		"Stock indexes closed a little higher after a quiet day of trading in most sectors.",
		"A listener wrote in to ask about gardening tips for the coming season.",
		"The next segment looks at how schools are planning their calendars.",
	}
	harbor := []string{
		"The harbor reopened at dawn today after a week of storms, port officials said.",
		"Ferries resume on Monday once the inspectors finish their checks of the quay.",
		"Fishing boats were already leaving the harbor this morning after the week of storms.",
		"The harbor master said the repairs after the storms were finished ahead of schedule.",
	}
	var segs []Segment
	for i := 0; i < 40; i++ {
		text := filler[i%len(filler)]
		if i >= 12 && i < 12+len(harbor) {
			text = harbor[i-12]
		}
		segs = append(segs, Segment{Start: float64(i) * 6, End: float64(i)*6 + 5.5, Text: text})
	}
	return segs
}

type fakePlatform struct {
	mu      sync.Mutex
	keys    []string
	queries []string
	puts    []map[string]any
}

func (f *fakePlatform) handler(now time.Time) http.Handler {
	mux := http.NewServeMux()
	note := func(r *http.Request) {
		f.mu.Lock()
		f.keys = append(f.keys, r.Header.Get("Authorization"))
		f.queries = append(f.queries, r.URL.RawQuery)
		f.mu.Unlock()
	}
	story := func(title, dek, route, kind string, ago time.Duration) map[string]any {
		return map[string]any{"title": title, "dek": dek, "updated_at": now.Add(-ago), "latest_revision": 3, "kind": kind, "route": map[string]string{"bare": route}}
	}
	mux.HandleFunc("/api/platform/publications/front", func(w http.ResponseWriter, r *http.Request) {
		note(r)
		_ = json.NewEncoder(w).Encode(map[string]any{"stories": []any{
			story("Harbor reopens after storm", "Ferry service resumes Monday after the harbor reopened at dawn.", "/s/harbor", "story", time.Hour),
			story("Opinion on the harbor", "The harbor reopened at dawn and ferry service resumes Monday.", "/s/op", "opinion", time.Hour),
			story("Volcano erupts near the capital", "Ash covers the city.", "/s/volcano", "story", time.Hour),
		}})
	})
	mux.HandleFunc("/api/platform/publications/export", func(w http.ResponseWriter, r *http.Request) {
		note(r)
		content := "Lava and ash from the volcano covered the capital city overnight."
		if r.URL.Query().Get("route") == "/s/harbor" {
			content = "The harbor reopened at dawn after a week of storms. Ferries resume Monday."
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"content": content})
	})
	mux.HandleFunc("/api/platform/operator/podcasts/episodes", func(w http.ResponseWriter, r *http.Request) {
		note(r)
		ep := func(id, guid string, ago time.Duration, has bool) map[string]any {
			return map[string]any{"episode_id": id, "feed_url": "https://feeds.example.test/news.xml", "guid": guid, "show": "Test Show", "title": "Episode " + guid,
				"published": now.Add(-ago), "duration_seconds": 240, "has_transcript": has}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"episodes": []any{
			ep("ep-fresh", "fresh", 2*time.Hour, true), ep("ep-fresh2", "fresh2", 90*time.Minute, true),
			ep("ep-untranscribed", "untranscribed", time.Hour, false), ep("ep-old", "old", 5*24*time.Hour, true),
		}})
	})
	mux.HandleFunc("/api/platform/podcast-transcript", func(w http.ResponseWriter, r *http.Request) {
		note(r)
		_ = json.NewEncoder(w).Encode(map[string]any{"blocks": harborBlocks()})
	})
	mux.HandleFunc("/api/platform/operator/podcasts/clips", func(w http.ResponseWriter, r *http.Request) {
		note(r)
		var body struct {
			Clips []map[string]any `json:"clips"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		for _, c := range body.Clips {
			f.puts = append(f.puts, c)
		}
		f.mu.Unlock()
		for _, c := range body.Clips {
			if c["quote"] == "words nobody said" {
				http.Error(w, `{"error":"the quote is not in the transcript"}`, http.StatusBadRequest)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"clips": len(body.Clips)})
	})
	return mux
}

func TestH1_H2_H3_CandidatesAreRankedPassagesOnSegmentBoundaries(t *testing.T) {
	now := time.Now().UTC()
	fp := &fakePlatform{}
	srv := httptest.NewServer(fp.handler(now))
	defer srv.Close()
	c, err := NewClient(srv.URL, "secret-key")
	if err != nil {
		t.Fatal(err)
	}
	f, err := BuildCandidates(context.Background(), c, 10, 4, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Candidates) == 0 {
		t.Fatal("H1: no candidates for the harbor story")
	}
	perEpisode := map[string]int{}
	for _, cand := range f.Candidates {
		if cand.StoryRoute != "/s/harbor" {
			t.Fatalf("H1: a candidate for %q: the volcano and the opinion piece share nothing with the transcript", cand.StoryRoute)
		}
		if cand.EpisodeID != "ep-fresh" && cand.EpisodeID != "ep-fresh2" {
			t.Fatalf("H1: a candidate from %s: no transcript, or published days before the story", cand.EpisodeID)
		}
		perEpisode[cand.EpisodeID]++
		if d := cand.End - cand.Start; d < 20 || d > 75 {
			t.Fatalf("H3: a clip of %.1f seconds", d)
		}
		onStart, onEnd := false, false
		for _, s := range harborBlocks() {
			onStart = onStart || s.Start == cand.Start
			onEnd = onEnd || s.End == cand.End
		}
		if !onStart || !onEnd {
			t.Fatalf("H3: [%v, %v] is not on segment boundaries", cand.Start, cand.End)
		}
		if !strings.Contains(cand.Passage, cand.Quote) || cand.Quote == "" || cand.StoryTitle == "" || cand.Show != "Test Show" {
			t.Fatalf("H3: candidate = %+v", cand)
		}
	}
	for ep, n := range perEpisode {
		if n > 1 {
			t.Fatalf("H2: %d passages of %s offered for one story", n, ep)
		}
	}
	if len(perEpisode) != 2 {
		t.Fatalf("H2: candidates from %v, want both fresh episodes", perEpisode)
	}
	for _, k := range fp.keys {
		if k != "Bearer secret-key" {
			t.Fatalf("H6: Authorization = %q", k)
		}
	}
	for _, q := range fp.queries {
		if strings.Contains(q, "secret-key") {
			t.Fatalf("H6: the key is in a URL: %s", q)
		}
	}
}

func TestH4_OnlyJudgedCandidatesAreSent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cand.json")
	f := &CandidatesFile{GeneratedAt: time.Now(), Candidates: []Candidate{
		{StoryRoute: "/s/a", EpisodeID: "ep-1", Start: 10, End: 50, Quote: "q1"},
		{StoryRoute: "/s/a", EpisodeID: "ep-2", Start: 12, End: 52, Quote: "q2", Support: 0.8},
		{StoryRoute: "/s/b", EpisodeID: "ep-3", Start: 1, End: 30, Quote: "q3", Support: 1},
	}}
	if err := WriteCandidates(path, f); err != nil {
		t.Fatal(err)
	}
	got, err := JudgedClips(path)
	if err != nil || len(got) != 2 || got[0].EpisodeID != "ep-2" || got[1].Support != 1 {
		t.Fatalf("H4: %+v %v", got, err)
	}
	f.Candidates[0].Support = 1.5
	_ = WriteCandidates(path, f)
	if _, err := JudgedClips(path); err == nil {
		t.Fatal("H4: a support over 1 was accepted")
	}
	if _, err := JudgedClips(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("H4: a missing file")
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := JudgedClips(path); err == nil {
		t.Fatal("H4: bad JSON")
	}
}

func TestH5_OneRefusedClipCostsOnlyItself(t *testing.T) {
	fp := &fakePlatform{}
	srv := httptest.NewServer(fp.handler(time.Now()))
	defer srv.Close()
	c, _ := NewClient(srv.URL, "k")
	clips := []ClipIn{
		{StoryRoute: "/s/a", EpisodeID: "ep-1", Start: 10, End: 50, Quote: "fine one", Support: 0.9},
		{StoryRoute: "/s/a", EpisodeID: "ep-2", Start: 10, End: 50, Quote: "words nobody said", Support: 0.9},
		{StoryRoute: "/s/b", EpisodeID: "ep-3", Start: 10, End: 50, Quote: "fine two", Support: 0.9},
	}
	stored, refused := c.PutClips(context.Background(), clips)
	if stored != 2 || len(refused) != 1 || !strings.Contains(refused[0], "ep-2") || !strings.Contains(refused[0], "400") {
		t.Fatalf("H5: stored %d refused %v", stored, refused)
	}
	// 450 good clips go in three requests of at most 200.
	fp2 := &fakePlatform{}
	srv2 := httptest.NewServer(fp2.handler(time.Now()))
	defer srv2.Close()
	c2, _ := NewClient(srv2.URL, "k")
	many := make([]ClipIn, 450)
	for i := range many {
		many[i] = ClipIn{StoryRoute: "/s/x", EpisodeID: fmt.Sprint("ep-", i), Start: 10, End: 50, Quote: "ok", Support: 0.7}
	}
	if stored, refused := c2.PutClips(context.Background(), many); stored != 450 || len(refused) != 0 {
		t.Fatalf("H5: stored %d refused %v", stored, refused)
	}
	if len(fp2.puts) != 450 {
		t.Fatalf("H5: the server saw %d clips", len(fp2.puts))
	}
}
