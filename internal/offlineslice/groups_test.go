package offlineslice

import (
	"testing"
	"time"
)

func gi(at string, v ...float32) GItem { return GItem{At: ts(at), Vec: unit(v...)} }

func TestGroupJoinsNearestOpenGroupAboveG(t *testing.T) {
	items := []GItem{
		gi("2026-11-18T00:00:00Z", 1, 0),
		gi("2026-11-18T01:00:00Z", 0, 1),   // far from the first: opens a second group
		gi("2026-11-18T02:00:00Z", 1, 0.1), // near the first
		gi("2026-11-18T03:00:00Z", 0.1, 1), // near the second
		gi("2026-11-18T04:00:00Z", 1, 1),   // between: below G for both (cos 0.707)
	}
	gs := GroupArrivals(items, 0.9, LinkCentroid, 48*time.Hour)
	if len(gs) != 3 {
		t.Fatalf("groups %d", len(gs))
	}
	if len(gs[0].Members) != 2 || gs[0].Members[1] != 2 || len(gs[1].Members) != 2 || gs[1].Members[1] != 3 || len(gs[2].Members) != 1 {
		t.Fatalf("members %+v", gs)
	}
	if !gs[0].Opened.Equal(items[0].At) || !gs[0].Last.Equal(items[2].At) {
		t.Fatalf("times %+v", gs[0])
	}
}

func TestGroupClosesAfter48HoursWithoutAMember(t *testing.T) {
	items := []GItem{
		gi("2026-11-18T00:00:00Z", 1, 0),
		gi("2026-11-19T20:00:00Z", 1, 0), // 44 h later: joins, group's clock restarts
		gi("2026-11-21T10:00:00Z", 1, 0), // 38 h after the last member: joins
		gi("2026-11-24T00:00:01Z", 1, 0), // more than 48 h after the last: a new group
	}
	gs := GroupArrivals(items, 0.9, LinkCentroid, 48*time.Hour)
	if len(gs) != 2 || len(gs[0].Members) != 3 || len(gs[1].Members) != 1 {
		t.Fatalf("%+v", gs)
	}
}

func TestGroupCentroidVersusAnyMember(t *testing.T) {
	// a chain: a-b near, b-c near, a-c far. Single link keeps the chain in one group, the centroid does not.
	a, b, c := unit(1, 0), unit(1, 0.3), unit(1, 0.6)
	items := []GItem{{At: ts("2026-11-18T00:00:00Z"), Vec: a}, {At: ts("2026-11-18T01:00:00Z"), Vec: b}, {At: ts("2026-11-18T02:00:00Z"), Vec: c}}
	sl := GroupArrivals(items, 0.93, LinkAnyMember, 48*time.Hour)
	ce := GroupArrivals(items, 0.93, LinkCentroid, 48*time.Hour)
	if len(sl) != 1 {
		t.Fatalf("single link made %d groups", len(sl))
	}
	if len(ce) != 2 {
		t.Fatalf("centroid made %d groups", len(ce))
	}
}

func TestBestThresholdMaximisesYouden(t *testing.T) {
	pos := []float64{0.9, 0.85, 0.8, 0.7, 0.6}
	neg := []float64{0.75, 0.7, 0.65, 0.6, 0.5, 0.5, 0.4}
	T, tpr, fpr := BestThreshold(pos, neg)
	// at T=0.8: tpr 3/5, fpr 0; at 0.7: tpr 4/5, fpr 2/7; at 0.6: 5/5, 4/7
	if T != 0.8 || tpr != 0.6 || fpr != 0 {
		t.Fatalf("T %v tpr %v fpr %v", T, tpr, fpr)
	}
}

func TestKnee(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4, 5, 6}
	ys := []float64{0, 5, 8, 9, 9.5, 9.8, 10} // concave: knee near x=2
	if i := Knee(xs, ys); i != 2 {
		t.Fatalf("knee %d", i)
	}
}

func TestOriginRules(t *testing.T) {
	o := func(c Capture, url string, wire string) string { return OriginOf(c, url, wire) }
	if got := o(Capture{Host: "www.theguardian.com", SourceID: "rss:x"}, "https://www.theguardian.com/world/a", ""); got != "theguardian.com" {
		t.Fatalf("rss %q", got)
	}
	if got := o(Capture{Host: "www.bbc.co.uk", SourceID: "rss:x"}, "https://www.bbc.co.uk/news/1", ""); got != "bbc.com" {
		t.Fatalf("folded %q", got)
	}
	// a wire copy is one origin whatever the outlet
	if got := o(Capture{Host: "a.com", SourceID: "rss:x"}, "https://a.com/1", "wire:reuters"); got != "wire:reuters" {
		t.Fatalf("wire %q", got)
	}
	// a social post that links an article counts as the article's outlet
	soc := Capture{Host: "bsky.app", SourceID: "social:bsky", Lead: "Lammy urges Burnham https://www.theguardian.com/politics/2026/nov/18/x?utm=1"}
	if got := o(soc, "https://bsky.app/profile/gu.bsky.social/post/3abc", ""); got != "theguardian.com" {
		t.Fatalf("linked %q", got)
	}
	// the broken form the extractor leaves
	soc.Lead = "Kerch https:// hermes-vector.buzz/posts/l-i-i nspects"
	if got := o(soc, "https://mastodon.social/@bot/117000000000000001", ""); got != "hermes-vector.buzz" {
		t.Fatalf("broken link %q", got)
	}
	// no link: the account
	soc.Lead = "Neptun struck the bridge."
	if got := o(soc, "https://mastodon.social/@bot/117000000000000001", ""); got != "mastodon.social/@bot" {
		t.Fatalf("mastodon account %q", got)
	}
	if got := o(soc, "https://bsky.app/profile/gu.bsky.social/post/3abc", ""); got != "bsky.app/gu.bsky.social" {
		t.Fatalf("bsky account %q", got)
	}
	// a shortener cannot be resolved from stored data: the account
	soc.Lead = "Look https://dlvr.it/abc"
	if got := o(soc, "https://bsky.app/profile/gu.bsky.social/post/3abc", ""); got != "bsky.app/gu.bsky.social" {
		t.Fatalf("shortener %q", got)
	}
	// a link to the post's own host is not an article
	soc.Lead = "see https://bsky.app/profile/x/post/1"
	if got := o(soc, "https://bsky.app/profile/gu.bsky.social/post/3abc", ""); got != "bsky.app/gu.bsky.social" {
		t.Fatalf("self link %q", got)
	}
}

func TestReachWithin(t *testing.T) {
	g := Group{Opened: ts("2026-11-18T00:00:00Z")}
	times := []string{"2026-11-18T00:00:00Z", "2026-11-18T05:00:00Z", "2026-11-18T05:30:00Z", "2026-11-19T10:00:00Z"}
	origins := []string{"a", "b", "b", "c"}
	var ats []time.Time
	for _, s := range times {
		ats = append(ats, ts(s))
	}
	if d, ok := ReachTime(ats, origins, 2); !ok || d != 5*time.Hour {
		t.Fatalf("2 origins at %v %v", d, ok)
	}
	if d, ok := ReachTime(ats, origins, 3); !ok || d != 34*time.Hour {
		t.Fatalf("3 origins at %v %v", d, ok)
	}
	if _, ok := ReachTime(ats, origins, 4); ok {
		t.Fatal("4 origins")
	}
	_ = g
}

func TestGroupCapsAgeAndSize(t *testing.T) {
	var items []GItem
	for i := 0; i < 8; i++ { // identical vectors, one an hour
		items = append(items, GItem{At: ts("2026-11-18T00:00:00Z").Add(time.Duration(i) * time.Hour), Vec: unit(1, 0)})
	}
	gs := GroupArrivalsCapped(items, 0.9, LinkCentroid, 48*time.Hour, 0, 3)
	if len(gs) != 3 || len(gs[0].Members) != 3 || len(gs[1].Members) != 3 || len(gs[2].Members) != 2 {
		t.Fatalf("size cap: %d groups %+v", len(gs), gs)
	}
	gs = GroupArrivalsCapped(items, 0.9, LinkCentroid, 48*time.Hour, 3*time.Hour, 0)
	// a group takes members until it is 3 h old: arrivals at 0,1,2,3 h; the 4 h arrival opens a new one
	if len(gs) != 2 || len(gs[0].Members) != 4 || len(gs[1].Members) != 4 {
		t.Fatalf("age cap: %d groups", len(gs))
	}
	// no caps equals the uncapped rule
	if got := GroupArrivalsCapped(items, 0.9, LinkCentroid, 48*time.Hour, 0, 0); len(got) != 1 {
		t.Fatal("no caps")
	}
}

func TestOriginCollapsesARelayHashtagToItsOutlet(t *testing.T) {
	labels := map[string]string{"bbc": "bbc.com", "cnn": "cnn.com"}
	soc := Capture{Host: "bsky.app", SourceID: "social:bsky", Lead: "# BBC # News Russell fastest in Las Vegas practice after rain https:// w.st/VnQau"}
	if got := OriginOfRelays(soc, "https://bsky.app/profile/bbc-bot.example/post/3a", "", labels); got != "bbc.com" {
		t.Fatalf("relay %q", got)
	}
	// a hashtag that is not an outlet label leaves the account
	soc.Lead = "# Ukraine # War Neptun struck the bridge"
	if got := OriginOfRelays(soc, "https://bsky.app/profile/bot.example/post/3a", "", labels); got != "bsky.app/bot.example" {
		t.Fatalf("no relay %q", got)
	}
	// an article link still wins over a relay hashtag? no: the hashtag names the outlet the account relays
	soc.Lead = "# CNN # News Susan Smith seeks parole https://www.cnn.com/2026/11/18/us/x"
	if got := OriginOfRelays(soc, "https://bsky.app/profile/bot.example/post/3a", "", labels); got != "cnn.com" {
		t.Fatalf("link %q", got)
	}
	// only a leading hashtag counts, not one in the middle of the text
	soc.Lead = "Talks resume in Geneva # BBC"
	if got := OriginOfRelays(soc, "https://bsky.app/profile/bot.example/post/3a", "", labels); got != "bsky.app/bot.example" {
		t.Fatalf("trailing hashtag %q", got)
	}
}
