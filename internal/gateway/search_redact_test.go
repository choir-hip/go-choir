package gateway

import (
	"net/url"
	"strconv"
	"testing"
)

// docs/archive/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md.
// Failure modes: Brave asked for more than its 20-result limit (422, a
// week-long cooldown). Redaction of the provider errors a search stream
// returns is covered in search_stream_test.go.

func TestBraveCountNeverExceedsLimit(t *testing.T) {
	for _, n := range []int{0, 5, 20, 40, 100} {
		u, err := url.Parse(braveSearchURL("q", n))
		if err != nil {
			t.Fatal(err)
		}
		if c, _ := strconv.Atoi(u.Query().Get("count")); c < 1 || c > 20 {
			t.Fatalf("maxResults %d -> count %d, want 1..20", n, c)
		}
	}
}
