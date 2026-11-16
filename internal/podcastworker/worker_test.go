package podcastworker

// Ways the worker can fail, written before the code (synthetic feeds and
// words only; no third-party text):
//   1. a feed item with no audio enclosure, or no parsable date, becomes an
//      episode;
//   2. a duration in HH:MM:SS, MM:SS or seconds is read wrong;
//   3. a segment spans a sentence end or a long pause, so a clip would start
//      or end mid-sentence;
//   4. a segment is longer than the hard bound, so a clip could not be cut;
//   5. a word timestamp is lost or moved when words are joined;
//   6. HTML in show notes reaches the platform.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const syntheticFeed = `<?xml version="1.0"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"><channel><title>Synthetic Daily</title>
<item><title>Episode &amp; One</title><guid>g-1</guid><pubDate>Mon, 16 Nov 2026 09:00:00 +0000</pubDate>
<description>&lt;p&gt;Notes &lt;b&gt;here&lt;/b&gt;&lt;/p&gt;</description><itunes:duration>01:02:03</itunes:duration>
<enclosure url="https://cdn.example.test/1.mp3" type="audio/mpeg"/></item>
<item><title>Text only</title><guid>g-2</guid><pubDate>Mon, 16 Nov 2026 08:00:00 +0000</pubDate></item>
<item><title>Bad date</title><guid>g-3</guid><pubDate>soon</pubDate><enclosure url="https://cdn.example.test/3.mp3" type="audio/mpeg"/></item>
<item><title>Video</title><guid>g-4</guid><pubDate>Mon, 16 Nov 2026 07:00:00 +0000</pubDate><enclosure url="https://cdn.example.test/4.mp4" type="video/mp4"/></item>
<item><title>Short</title><pubDate>Sun, 15 Nov 2026 07:00:00 GMT</pubDate><itunes:duration>754</itunes:duration><enclosure url="https://cdn.example.test/5.mp3" type="audio/mpeg"/></item>
</channel></rss>`

func TestParseFeed(t *testing.T) {
	eps, err := ParseFeed("https://f.example.test/x.xml", strings.NewReader(syntheticFeed), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("got %d episodes, want 2 (audio enclosure and date only): %+v", len(eps), eps)
	}
	a := eps[0]
	if a.Show != "Synthetic Daily" || a.Title != "Episode & One" || a.GUID != "g-1" || a.Published != "2026-11-16T09:00:00Z" || a.DurationSeconds != 3723 || a.EnclosureURL != "https://cdn.example.test/1.mp3" {
		t.Fatalf("episode 1 = %+v", a)
	}
	if a.Notes != "Notes here" {
		t.Fatalf("notes = %q", a.Notes)
	}
	b := eps[1]
	if b.GUID != "https://cdn.example.test/5.mp3" || b.DurationSeconds != 754 || b.Published != "2026-11-15T07:00:00Z" {
		t.Fatalf("episode 2 = %+v (guid falls back to the enclosure URL)", b)
	}
	named, _ := ParseFeed("u", strings.NewReader(syntheticFeed), "Chosen Name")
	if named[0].Show != "Chosen Name" {
		t.Fatalf("the feed list's show name wins, got %q", named[0].Show)
	}
}

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]float64{"": 0, "90": 90, "1:30": 90, "1:00:00": 3600, "x": 0, "-5": 0, "1:xx": 0} {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %v, want %v", in, got, want)
		}
	}
}

func w(text string, start, end float64) Word { return Word{W: text, Start: start, End: end} }

func TestSegmentsBreakOnSentencesAndPauses(t *testing.T) {
	words := []Word{
		w("The", 0.1, 0.3), w("market", 0.4, 0.8), w("opened.", 0.9, 1.3), // sentence end
		w("Traders", 1.5, 2.0), w("waited", 2.1, 2.5), // pause of 1.5 s follows
		w("Then", 4.0, 4.2), w("it", 4.3, 4.4), w("rose?", 4.5, 4.9),
		w("Yes!", 5.0, 5.3),
	}
	segs := Segments(words)
	if len(segs) != 4 {
		t.Fatalf("got %d segments, want 4: %+v", len(segs), segs)
	}
	want := []Segment{
		{0.1, 1.3, "The market opened."}, {1.5, 2.5, "Traders waited"}, {4.0, 4.9, "Then it rose?"}, {5.0, 5.3, "Yes!"},
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Errorf("segment %d = %+v, want %+v", i, segs[i], want[i])
		}
	}
}

func TestSegmentsBoundTheLengthOfUnpunctuatedSpeech(t *testing.T) {
	var words []Word
	at := 0.0
	for i := 0; i < 400; i++ {
		gap := 0.05
		if i%25 == 24 {
			gap = 0.3 // a small gap
		}
		words = append(words, w("word", at, at+0.3))
		at += 0.3 + gap
	}
	segs := Segments(words)
	if len(segs) < 3 {
		t.Fatalf("unpunctuated speech of %.0f s made %d segments", at, len(segs))
	}
	for i, s := range segs {
		if s.End-s.Start > hardMax+0.5 {
			t.Fatalf("segment %d is %.1f s, over the hard bound", i, s.End-s.Start)
		}
		if i > 0 && s.Start < segs[i-1].End {
			t.Fatalf("segment %d starts before the last ends", i)
		}
	}
}

func TestParseParakeetOutput(t *testing.T) {
	out, err := ParseParakeet([]byte(`{"text":"Hi there.","frame_sec":0.08,"words":[{"w":"Hi","start":0.08,"end":0.24,"conf":0.99},{"w":"there.","start":0.32,"end":0.56,"conf":0.7}],"tokens":[]}`))
	if err != nil || len(out.Words) != 2 || out.Words[1].End != 0.56 {
		t.Fatalf("parse: %+v %v", out, err)
	}
	if _, err := ParseParakeet([]byte(`{"text":"","words":[]}`)); err == nil {
		t.Fatal("an output with no words is an error")
	}
	if _, err := ParseParakeet([]byte(`not json`)); err == nil {
		t.Fatal("bad JSON is an error")
	}
}

func TestProbeLengthReadsTheTotalAndAnswersZeroWhenUnknown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/partial", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Errorf("range header = %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 0-0/28235701")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0})
	})
	mux.HandleFunc("/whole", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1234")
		_, _ = w.Write(make([]byte, 1234))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/partial", http.StatusFound) })
	mux.HandleFunc("/broken", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := context.Background()
	for path, want := range map[string]int64{"/partial": 28235701, "/whole": 1234, "/redirect": 28235701, "/broken": 0, "/missing": 0} {
		if got := ProbeLength(ctx, srv.Client(), srv.URL+path); got != want {
			t.Errorf("ProbeLength(%s) = %d, want %d", path, got, want)
		}
	}
	if got := ProbeLength(ctx, srv.Client(), "http://127.0.0.1:1/x"); got != 0 {
		t.Errorf("a refused connection = %d", got)
	}
}


// Round 3: one bad item must not cost its request the others, and a failed
// chunk must not stop the chunks after it.
func TestE1_ValidEpisodesDropsWhatTheServerWouldRefuse(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	ok := Episode{FeedURL: "https://f.example.test/x.xml", GUID: "g", Show: "S", Title: "T", Published: "2026-11-16T09:00:00Z", EnclosureURL: "https://cdn.example.test/a.mp3"}
	mut := func(f func(*Episode)) Episode { e := ok; f(&e); return e }
	in := []Episode{
		ok,
		mut(func(e *Episode) { e.Published = "2026-11-18T09:00:00Z" }), // two days ahead
		mut(func(e *Episode) { e.GUID = " " }),
		mut(func(e *Episode) { e.Title = "" }),
		mut(func(e *Episode) { e.Show = "" }),
		mut(func(e *Episode) { e.Published = "yesterday" }),
		mut(func(e *Episode) { e.EnclosureURL = "ftp://cdn.example.test/a.mp3" }),
		mut(func(e *Episode) { e.FeedURL = "javascript:1" }),
		mut(func(e *Episode) { e.Published = "2026-11-17T11:00:00Z" }), // 23 h ahead: allowed
	}
	good, dropped := ValidEpisodes(in, now)
	if len(good) != 2 || dropped != 7 {
		t.Fatalf("E1: %d good and %d dropped, want 2 and 7", len(good), dropped)
	}
}

func TestE2_AFailedChunkDoesNotStopTheNext(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Episodes []Episode `json:"episodes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprintf(w, `{"recorded":%d,"new":%d,"skipped":1}`, len(body.Episodes), len(body.Episodes))
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "key")
	if err != nil {
		t.Fatal(err)
	}
	eps := make([]Episode, 450) // three chunks: 200, 200, 50
	recorded, fresh, skipped, err := c.PostEpisodes(context.Background(), eps)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("E2: the failed chunk is not reported: %v", err)
	}
	if calls != 3 || recorded != 250 || fresh != 250 || skipped != 2 {
		t.Fatalf("E2: calls %d, recorded %d, new %d, skipped %d: the chunks after the failure must still go", calls, recorded, fresh, skipped)
	}
}
