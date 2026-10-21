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
//   - O4 an unparseable or hostless URL yields an outlet, so it counts;
//   - O5 an aggregator, search page or syndicator (Hacker News, Google
//     search, Yahoo News republishing Reuters) counts as an outlet, so an
//     article and its HN thread corroborate each other (offline event
//     study, 2026-10-16: four of 36 "corroborated" events were article+HN);
//   - O6 a reader-proxy URL (r.jina.ai/<url>, web.archive.org/web/<ts>/<url>)
//     names the proxy instead of the publisher it wraps.

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
	for _, none := range []string{ // O5
		"https://news.ycombinator.com/item?id=50083130",
		"https://www.google.com/search?q=memory+prices",
		"https://news.google.com/rss/articles/abc",
		"https://www.reddit.com/r/news/comments/x",
		"https://malaysia.news.yahoo.com/flavio-bolsonaro-slightly-leads-lula-213312820.html",
		"https://aol.co.uk/articles/horror-freight-train.html",
		"https://www.msn.com/en-us/news/world/x",
		"https://lobste.rs/s/abc",
	} {
		if got := Of(none); got != "" {
			t.Errorf("Of(%q) = %q, want no outlet (aggregator, search or syndicator)", none, got)
		}
	}
	for raw, want := range map[string]string{ // O6
		"https://r.jina.ai/https://openai.com/index/introducing-gpt-rosalind/":          "openai.com",
		"https://web.archive.org/web/20261015000000/https://www.bbc.co.uk/news/world-1": "bbc.co.uk",
		"https://r.jina.ai/https://news.ycombinator.com/item?id=1":                      "",
	} {
		if got := Of(raw); got != want {
			t.Errorf("Of(%q) = %q, want %q", raw, got, want)
		}
	}
}

// docs/reports/social-source-seeds-and-pivot-2026-10-20.md: a social post is a
// lead, not an outlet (owner §30). Ways it could fail, written before the
// code:
//   - S1 a Bluesky post (bsky.app/profile/<handle>/post/<id>) names bsky.app
//     as an outlet, so one post plus one real outlet publishes a story;
//   - S2 a Mastodon-style post (https://<any instance>/@user/<digits>, the
//     origin URL a tag feed's items carry) names its instance as an outlet,
//     so a story cites "journa.host" or "c.im" as corroboration;
//   - S3 the post rule over-reaches and drops a real outlet whose path looks
//     like it (a Medium author page, a /@name profile that is not a post).
func TestSocialPostsNameNoOutlet(t *testing.T) {
	for _, post := range []string{ // S1, S2
		"https://bsky.app/profile/apnews.com/post/3mydqg6nu3y2a",
		"https://bsky.app/profile/did:plc:a67zdrt4nl2tv2qojpngogbq/post/3mydqg6nu3y2a",
		"https://c.im/@BBC/117475958177254767",
		"https://journa.host/@someone/116000000000000001",
		"https://mastodon.social/@someone@other.example/115000000000000002",
		"https://backend.newsmast.org/@newsmast/117473481867527856",
		"https://instance.example/users/someone/statuses/115000000000000003",
	} {
		if got := Of(post); got != "" {
			t.Errorf("Of(%q) = %q, want no outlet (a social post is a lead)", post, got)
		}
	}
	for raw, want := range map[string]string{ // S3
		"https://medium.com/@writer/a-long-slug-9f8e7d6c5b4a":      "medium.com",
		"https://medium.com/@writer":                               "medium.com",
		"https://www.theguardian.com/world/2026/oct/20/story":      "theguardian.com",
		"https://example.org/@2026/116000000000000004/extra/parts": "example.org",
	} {
		if got := Of(raw); got != want {
			t.Errorf("Of(%q) = %q, want %q", raw, got, want)
		}
	}
}
