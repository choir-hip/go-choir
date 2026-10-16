package outlet

import "testing"

// docs/problems/capture-and-production-are-coupled-2026-10-16.md, F1: a
// story publishes only when its citations span two distinct outlets. Ways
// the outlet key could fail, written before the code:
//   - O1 two pages of one publisher (subdomains, www, feeds hosts, ports,
//     case) read as two outlets, so a one-outlet story publishes;
//   - O2 two publishers on one platform (Telegram channels, X accounts) read
//     as one outlet, so the registry's 73 Telegram sources never corroborate
//     each other;
//   - O3 a public-suffix host (bbc.co.uk) collapses to its suffix (co.uk),
//     making every British outlet one;
//   - O4 an unparseable or hostless URL yields an outlet, so it counts.

func TestOf(t *testing.T) {
	same := [][2]string{ // O1, O3
		{"https://www.example.org/court", "https://news.example.org/harbor"},
		{"https://feeds.bbci.co.uk/news/rss.xml", "https://news.bbci.co.uk/x"},
		{"https://www.bbc.co.uk/news/world-1", "https://BBC.co.uk:443/sport"},
		{"https://www.theguardian.com/a", "https://amp.theguardian.com/b"},
		{"https://t.me/s/rybar/123", "https://t.me/rybar/456"},
		{"https://x.com/AP/status/1", "https://twitter.com/ap/status/2"},
	}
	for _, p := range same {
		a, b := Of(p[0]), Of(p[1])
		if a == "" || a != b {
			t.Errorf("Of(%q)=%q, Of(%q)=%q: want one outlet", p[0], a, p[1], b)
		}
	}
	distinct := [][2]string{ // O2, O3
		{"https://t.me/rybar/1", "https://t.me/meduzalive/1"},
		{"https://x.com/AP/status/1", "https://x.com/Reuters/status/1"},
		{"https://www.bbc.co.uk/a", "https://www.ft.co.uk/a"},
		{"https://www.rfi.fr/a", "https://www.france24.com/a"},
	}
	for _, p := range distinct {
		if a, b := Of(p[0]), Of(p[1]); a == "" || a == b {
			t.Errorf("Of(%q)=%q, Of(%q)=%q: want two outlets", p[0], a, p[1], b)
		}
	}
	for _, bad := range []string{"", "not a url", "cap-court", "https:///nohost", "https://t.me/"} { // O4
		if got := Of(bad); got != "" {
			t.Errorf("Of(%q) = %q, want no outlet", bad, got)
		}
	}
}
