package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

func allowPrivateSourceFetchForTest(t *testing.T) {
	t.Helper()
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() {
		sourcefetch.SetAllowPrivateNetworkForTests(previous)
	})
}

func TestRSSPollerReturnsFetchRecordAndStableItem(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "ChoirTest/1.0" {
			t.Fatalf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Official rate decision</title>
      <link>https://example.test/rates#fragment</link>
      <description>Rates were held steady.</description>
      <pubDate>Thu, 04 Jun 2026 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer server.Close()

	source := Source{
		ID:        "official:test",
		Type:      SourceTypeRSS,
		Name:      "Official Test",
		URL:       server.URL,
		Verticals: []string{"macro_policy"},
		Languages: []string{"en"},
		Regions:   []string{"us"},
	}
	poller := NewRSSPoller("ChoirTest/1.0")
	first, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll first: %v", err)
	}
	second, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll second: %v", err)
	}
	if first.Fetch.Status != "ok" || first.Fetch.ItemCount != 1 {
		t.Fatalf("fetch = %+v, want ok with one item", first.Fetch)
	}
	if len(first.Items) != 1 || len(second.Items) != 1 {
		t.Fatalf("item counts = %d/%d, want 1/1", len(first.Items), len(second.Items))
	}
	if first.Items[0].ID != second.Items[0].ID {
		t.Fatalf("item id changed across polls: %q != %q", first.Items[0].ID, second.Items[0].ID)
	}
	if first.Items[0].CanonicalURL != "https://example.test/rates" {
		t.Fatalf("canonical URL = %q", first.Items[0].CanonicalURL)
	}
	if first.Items[0].SourceID != source.ID || first.Items[0].EvidenceLevel != "source_feed" {
		t.Fatalf("item provenance incomplete: %+v", first.Items[0])
	}
	if first.Items[0].BodyKind != BodyKindFeedSummary || first.Items[0].BodyLength != len("Rates were held steady.") || first.Items[0].ReaderSnapshot {
		t.Fatalf("item body classification incomplete: %+v", first.Items[0])
	}
}

func TestRSSPollerReturnsNotModifiedOnSecondPoll(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("ETag", "etag-conditional")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><title>One</title><link>https://example.test/one</link><description>Body</description></item></channel></rss>`))
			return
		}
		if r.Header.Get("If-None-Match") != "etag-conditional" {
			t.Fatalf("If-None-Match = %q, want etag-conditional", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	source := Source{
		ID:              "rss:conditional",
		Type:            SourceTypeRSS,
		URL:             server.URL,
		ConditionalMode: "etag_last_modified",
	}
	poller := NewRSSPoller("ChoirTest/1.0")
	first, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll first: %v", err)
	}
	if source.LastETag != "etag-conditional" {
		t.Fatalf("LastETag = %q, want etag-conditional", source.LastETag)
	}
	second, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll second: %v", err)
	}
	if first.Fetch.Status != "ok" || second.Fetch.Status != "not_modified" {
		t.Fatalf("fetch statuses = %q/%q, want ok/not_modified", first.Fetch.Status, second.Fetch.Status)
	}
	if len(second.Items) != 0 {
		t.Fatalf("second poll items = %d, want 0", len(second.Items))
	}
}

func TestRSSPollerSkipsConditionalHeadersWhenModeNone(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			t.Fatalf("unexpected If-None-Match when conditional mode is none")
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><title>One</title><link>https://example.test/one</link><description>Body</description></item></channel></rss>`))
	}))
	defer server.Close()

	source := Source{
		ID:              "rss:no-conditional",
		Type:            SourceTypeRSS,
		URL:             server.URL,
		ConditionalMode: "none",
		LastETag:        "stale-etag",
	}
	poller := NewRSSPoller("ChoirTest/1.0")
	if _, err := poller.Poll(context.Background(), &source); err != nil {
		t.Fatalf("poll: %v", err)
	}
}

func TestRSSPollerImportsReaderSnapshotWhenPolicyAllows(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	articleBody := strings.Repeat("This is a long article paragraph about policy and markets. ", 12)
	articleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Article</title></head><body><article><p>` + articleBody + `</p></article></body></html>`))
	}))
	defer articleServer.Close()

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><title>Story</title><link>` + articleServer.URL + `</link><description>Short feed excerpt.</description></item></channel></rss>`))
	}))
	defer feedServer.Close()

	source := Source{
		ID:              "rss:reader-import",
		Type:            SourceTypeRSS,
		URL:             feedServer.URL,
		StoreBodyPolicy: "bounded_text",
	}
	poller := NewRSSPoller("ChoirTest/1.0")
	result, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if !item.ReaderSnapshot || item.BodyKind != BodyKindReaderSnapshot {
		t.Fatalf("item = %+v, want reader_snapshot classification", item)
	}
	if item.BodyLength < minReaderSnapshotRunes {
		t.Fatalf("body length = %d, want at least %d", item.BodyLength, minReaderSnapshotRunes)
	}
}

func TestRSSPollerSkipsReaderImportForExcerptOnlyPolicy(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	articleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("article fetch should not run for excerpt_only policy")
	}))
	defer articleServer.Close()

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><title>Story</title><link>` + articleServer.URL + `</link><description>Short feed excerpt.</description></item></channel></rss>`))
	}))
	defer feedServer.Close()

	source := Source{
		ID:              "rss:excerpt-only",
		Type:            SourceTypeRSS,
		URL:             feedServer.URL,
		StoreBodyPolicy: "excerpt_only",
	}
	poller := NewRSSPoller("ChoirTest/1.0")
	result, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ReaderSnapshot {
		t.Fatalf("item = %+v, want feed summary without reader snapshot", result.Items)
	}
}

func TestRSSPollerCleansHTMLDescriptionsForSourceBody(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>HTML body item</title>
      <link>https://example.test/html-body</link>
      <description><![CDATA[<div><p>Markets &amp; policy <strong>shifted</strong>.</p><p><a href="https://example.test">Read more</a></p></div>]]></description>
      <pubDate>Thu, 04 Jun 2026 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer server.Close()

	source := Source{
		ID:        "rss:html-body",
		Type:      SourceTypeRSS,
		Name:      "HTML Body Test",
		URL:       server.URL,
		Languages: []string{"en"},
	}
	poller := NewRSSPoller("ChoirTest/1.0")
	result, err := poller.Poll(context.Background(), &source)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(result.Items))
	}
	if got := result.Items[0].Body; got != "Markets & policy shifted. Read more" {
		t.Fatalf("cleaned body = %q", got)
	}
	if strings.Contains(result.Items[0].Body, "<") || strings.Contains(result.Items[0].Body, "&amp;") {
		t.Fatalf("body still contains markup/entities: %q", result.Items[0].Body)
	}
}

func TestParseRSSLikeFeedHandlesDeclaredLatin1Charset(t *testing.T) {
	feed, err := parseRSSLikeFeed([]byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n" +
		"<rss version=\"2.0\"><channel><item><title>Golem Pr\xfcfung</title>" +
		"<link>https://example.test/latin1</link><description>ISO feed</description>" +
		"</item></channel></rss>"))
	if err != nil {
		t.Fatalf("parse latin1 feed: %v", err)
	}
	if len(feed.Items) != 1 || feed.Items[0].Title != "Golem Prüfung" {
		t.Fatalf("items = %+v, want decoded latin1 title", feed.Items)
	}
}

func TestParseRSSLikeFeedToleratesMalformedEntity(t *testing.T) {
	feed, err := parseRSSLikeFeed([]byte(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <item>
      <title>Markets &3 policy update</title>
      <link>https://example.test/malformed-entity</link>
      <description>Entity-like text from a feed</description>
    </item>
  </channel>
</rss>`))
	if err != nil {
		t.Fatalf("parse malformed entity feed: %v", err)
	}
	if len(feed.Items) != 1 || feed.Items[0].Title != "Markets &3 policy update" {
		t.Fatalf("items = %+v, want malformed entity preserved", feed.Items)
	}
}

func TestParseRSSLikeFeedStripsInvalidXMLControlBytes(t *testing.T) {
	feed, err := parseRSSLikeFeed([]byte("<?xml version=\"1.0\"?>\n" +
		"<rss version=\"2.0\"><channel><item><title>Euronews\x1b France</title>" +
		"<link>https://example.test/control-byte</link><description>Control byte feed</description>" +
		"</item></channel></rss>"))
	if err != nil {
		t.Fatalf("parse control-byte feed: %v", err)
	}
	if len(feed.Items) != 1 || feed.Items[0].Title != "Euronews France" {
		t.Fatalf("items = %+v, want invalid XML control byte stripped", feed.Items)
	}
}
