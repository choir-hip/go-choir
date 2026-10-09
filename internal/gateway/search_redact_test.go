package gateway

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// docs/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md.
// Failure modes: Brave asked for more than its 20-result limit (422, a
// week-long cooldown); a summary stored before redaction (with a key in its
// URL) returned to a computer.

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

func TestOutageResponseRedactsLegacyStoredSummary(t *testing.T) {
	const secret = "sk-secret-0123456789"
	legacy := `timeout: http request: Get "https://serpapi.com/search.json?api_key=` + secret + `&q=private": context canceled`
	resp, ok := searchOutageResponse(&searchplane.OutageError{
		Query:    "q",
		Health:   map[string]searchplane.ProviderHealth{"serpapi": {Provider: "serpapi", LastErrorSummary: legacy}},
		Attempts: []searchplane.Attempt{{Provider: "serpapi", Error: legacy}},
	})
	if !ok {
		t.Fatal("not an outage response")
	}
	if s := resp.ProviderHealth["serpapi"].LastErrorSummary; strings.Contains(s, secret) || strings.Contains(s, "private") {
		t.Fatalf("health summary returned to the computer holds the key: %q", s)
	}
	if s := resp.Attempts[0].Error; strings.Contains(s, secret) {
		t.Fatalf("attempt error returned to the computer holds the key: %q", s)
	}
}

func TestPlaneResultRedactsLegacyStoredSummary(t *testing.T) {
	const secret = "sk-secret-0123456789"
	legacy := `Get "https://serpapi.com/search.json?api_key=` + secret + `": context canceled`
	resp := convertPlaneResult("q", &searchplane.SearchResult{
		ProviderHealth: map[string]searchplane.ProviderHealth{"serpapi": {Provider: "serpapi", LastErrorSummary: legacy}},
		Attempts:       []searchplane.Attempt{{Provider: "serpapi", Error: legacy}},
	})
	if strings.Contains(resp.ProviderHealth["serpapi"].LastErrorSummary, secret) || strings.Contains(resp.Attempts[0].Error, secret) {
		t.Fatalf("success response holds the key: %+v %+v", resp.ProviderHealth, resp.Attempts)
	}
}
