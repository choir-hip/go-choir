package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The per-source feed byte cap (primary-source registry wave 1,
// docs/problems/primary-source-registry-wave-1-2026-10-20.md). Ways it can
// fail, written before the code:
//   - a source with no setting changes its limit (the default must stay
//     2 MiB, so every existing row polls as before);
//   - a feed over the default cap with no setting is read past 2 MiB;
//   - a raised cap does not raise the read, so a 3 MB government feed still
//     loses its tail;
//   - a setting above the ceiling is honoured, so one registry row can make
//     the poller buffer an unbounded body;
//   - a zero or negative setting is read as "no limit";
//   - the registry carries a row whose setting is over the ceiling.

const feedCapTestItemBytes = 1000

// bigFeed returns an RSS feed of n items of about feedCapTestItemBytes
// each, so its size is known.
func bigFeed(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>Big</title>`)
	pad := strings.Repeat("x", feedCapTestItemBytes-200)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<item><title>Item %05d</title><link>https://example.test/i/%05d</link><guid>g-%05d</guid><description>%s</description></item>`, i, i, i, pad)
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

func pollBigFeed(t *testing.T, feed string, source Source) (PollResult, error) {
	t.Helper()
	allowPrivateSourceFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	t.Cleanup(server.Close)
	source.URL = server.URL
	source.Type = SourceTypeRSS
	source.MaxItemsPerPoll = 100000
	return NewRSSPoller("ChoirTest/1.0").Poll(context.Background(), &source)
}

func TestEffectiveMaxBodyBytesDefaultsAndClamps(t *testing.T) {
	cases := []struct {
		name string
		set  int
		want int
	}{
		{"unset keeps the 2 MiB default", 0, 2 << 20},
		{"negative is the default, not unlimited", -1, 2 << 20},
		{"a raised cap is honoured", 4 << 20, 4 << 20},
		{"the ceiling itself is allowed", 8 << 20, 8 << 20},
		{"above the ceiling clamps to 8 MiB", 64 << 20, 8 << 20},
	}
	for _, c := range cases {
		if got := (Source{MaxBodyBytes: c.set}).EffectiveMaxBodyBytes(); got != c.want {
			t.Errorf("%s: EffectiveMaxBodyBytes(%d) = %d, want %d", c.name, c.set, got, c.want)
		}
	}
	if DefaultMaxFeedBodyBytes != 2<<20 || MaxFeedBodyBytesCeiling != 8<<20 {
		t.Fatalf("default/ceiling = %d/%d, want 2 MiB/8 MiB", DefaultMaxFeedBodyBytes, MaxFeedBodyBytesCeiling)
	}
}

func TestFeedOverDefaultCapIsNotReadPastIt(t *testing.T) {
	total := 3500 // about 3.5 MB
	result, err := pollBigFeed(t, bigFeed(total), Source{ID: "rss:big-default"})
	// A body cut at 2 MiB either fails to parse or yields fewer items; it
	// must never yield them all.
	if err == nil && len(result.Items) >= total {
		t.Fatalf("a 3.5 MB feed with no setting yielded %d items; the default cap should have cut it", len(result.Items))
	}
	if err == nil && len(result.Items) > (2<<20)/feedCapTestItemBytes {
		t.Fatalf("yielded %d items, more than 2 MiB holds", len(result.Items))
	}
}

func TestFeedOverDefaultCapParsesWithARaisedCap(t *testing.T) {
	total := 3500
	result, err := pollBigFeed(t, bigFeed(total), Source{ID: "rss:big-raised", MaxBodyBytes: 4 << 20})
	if err != nil {
		t.Fatalf("poll with raised cap: %v", err)
	}
	if len(result.Items) != total {
		t.Fatalf("items = %d, want all %d", len(result.Items), total)
	}
	if result.Fetch.Status != "ok" || result.Fetch.ItemCount != total {
		t.Fatalf("fetch = %+v", result.Fetch)
	}
}

func TestFeedCapAboveTheCeilingIsBoundedAtTheCeiling(t *testing.T) {
	total := 10000 // about 10 MB, over the 8 MiB ceiling
	result, err := pollBigFeed(t, bigFeed(total), Source{ID: "rss:big-ceiling", MaxBodyBytes: 1 << 30})
	if err == nil && len(result.Items) >= total {
		t.Fatalf("a 10 MB feed yielded %d items; the ceiling should have cut it", len(result.Items))
	}
	if err == nil && len(result.Items) > (8<<20)/feedCapTestItemBytes {
		t.Fatalf("yielded %d items, more than 8 MiB holds", len(result.Items))
	}
	if err == nil && len(result.Items) < (6<<20)/feedCapTestItemBytes {
		t.Fatalf("yielded only %d items; the ceiling should still read about 8 MiB", len(result.Items))
	}
}

func TestRegistryFeedCapsStayWithinTheCeiling(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range reg.Sources {
		if s.MaxBodyBytes < 0 || s.MaxBodyBytes > MaxFeedBodyBytesCeiling {
			t.Errorf("%s: max_body_bytes %d is outside 0..%d", s.ID, s.MaxBodyBytes, MaxFeedBodyBytesCeiling)
		}
	}
}
