package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hnrss.org feed descriptions carry the item's live points and comment
// count. Kept in the body, every poll that saw them move minted a new
// capture and woke the story about it
// (docs/problems/hn-point-counts-mint-a-capture-every-poll-2026-10-17.md).
//
// Ways this could fail, written first:
//  1. a counter change still yields a different body or content hash;
//  2. the counters are stripped from a feed that is not HN, or from text
//     that merely contains "Points:";
//  3. the article and comments URLs, the item's only content, are lost;
//  4. an Ask HN self-post loses its text.
func TestRSSPollerDropsHackerNewsCounters(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	points, comments := 50, 21
	selfText := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0"><channel><title>Hacker News: Front Page</title>
<item>
  <title>Show HN: Llmspeed</title>
  <link>https://llmspeed.dev/</link>
  <description><![CDATA[%s
<p>Article URL: <a href="https://llmspeed.dev/">https://llmspeed.dev/</a></p>
<p>Comments URL: <a href="https://news.ycombinator.com/item?id=50127979">https://news.ycombinator.com/item?id=50127979</a></p>
<p>Points: %d</p>
<p># Comments: %d</p>
]]></description>
  <guid isPermaLink="false">https://news.ycombinator.com/item?id=50127979</guid>
</item>
<item>
  <title>League table</title>
  <link>https://example.test/league</link>
  <description><![CDATA[<p>Points: %d</p><p># Comments: %d</p>]]></description>
</item>
</channel></rss>`, selfText, points, comments, points, comments)
	}))
	defer server.Close()

	source := Source{ID: "rss:hn_frontpage", Type: SourceTypeRSS, Name: "Hacker News Front Page", URL: server.URL}
	poller := NewRSSPoller("ChoirTest/1.0")
	poll := func() []Item {
		t.Helper()
		res, err := poller.Poll(context.Background(), &source)
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		if len(res.Items) != 2 {
			t.Fatalf("items = %d, want 2", len(res.Items))
		}
		return res.Items
	}

	first := poll()
	hn, other := first[0], first[1]
	for _, want := range []string{"Article URL: https://llmspeed.dev/", "Comments URL: https://news.ycombinator.com/item?id=50127979"} {
		if !strings.Contains(hn.Body, want) {
			t.Fatalf("HN body lost %q: %q", want, hn.Body)
		}
	}
	if strings.Contains(hn.Body, "Points") || strings.Contains(hn.Body, "Comments: 21") {
		t.Fatalf("HN body kept its live counters: %q", hn.Body)
	}
	if other.Body != "Points: 50 # Comments: 21" {
		t.Fatalf("a feed item without an HN comments link was changed: %q", other.Body)
	}

	points, comments = 52, 34
	second := poll()
	if second[0].Body != hn.Body || second[0].ContentHash != hn.ContentHash || second[0].ID != hn.ID {
		t.Fatalf("moving counters changed the HN item:\n%q %s\n%q %s", hn.Body, hn.ContentHash, second[0].Body, second[0].ContentHash)
	}
	if second[1].Body == other.Body {
		t.Fatalf("the non-HN item should still reflect its own text: %q", second[1].Body)
	}

	selfText = "<p>I built a tool that shows token speed.</p><hr>"
	asked := poll()
	if !strings.Contains(asked[0].Body, "I built a tool that shows token speed.") || strings.Contains(asked[0].Body, "Points") {
		t.Fatalf("self-post text lost or counters kept: %q", asked[0].Body)
	}
}
