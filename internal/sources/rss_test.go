package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
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
}
