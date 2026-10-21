package sources

import (
	"net/url"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/outlet"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// The social-lead registry guard (docs/reports/social-source-seeds-and-pivot-2026-10-20.md;
// owner 2026-10-20, notes sections 30 and 32: social posts are leads, not
// outlets; a story still needs two distinct outlets). Ways it can fail,
// written before the rows:
//   - a seed row is missing, or its id was reused for another feed;
//   - a row is not marked a social lead, so the claim-maker and outlet
//     statistics mix with it;
//   - a row is marked official, or carries a licence, so a post reads as a
//     primary document or as content Autopaper may republish (a post is its
//     author's, not licensed to us);
//   - a post from a row's platform names an outlet in the publish gate, so
//     one post plus one real outlet publishes a story;
//   - a row asks for a reader import of the pages its posts link to, which
//     would capture a third party's page under a social row's name;
//   - a row polls faster than a polite 30 minutes, or asks for a large batch;
//   - a row's URL fails the fetch policy, duplicates another row's, or does
//     not match its id (a copy-paste handle).

// socialSeeds maps each seed id to the feed it must poll. Bluesky rows poll
// the account's DID feed (a handle can change; the DID cannot), and the id
// keeps the handle the owner reads.
var socialSeeds = map[string]string{
	"social:bsky:apnews.com":                           "https://bsky.app/profile/did:plc:a67zdrt4nl2tv2qojpngogbq/rss",
	"social:bsky:reuters.com":                          "https://bsky.app/profile/did:plc:jbvnehrrdqoulco4rf5gxg5r/rss",
	"social:bsky:afp.com":                              "https://bsky.app/profile/did:plc:ffccycfh6c6pmxwkhvq5clkv/rss",
	"social:bsky:npr.org":                              "https://bsky.app/profile/did:plc:ln72v57ivz2g46uqf4xxqiuh/rss",
	"social:bsky:aljazeera.com":                        "https://bsky.app/profile/did:plc:2lofqead276vtc5647ye7sl2/rss",
	"social:bsky:theguardian.com":                      "https://bsky.app/profile/did:plc:vovinwhtulbsx4mwfw26r5ni/rss",
	"social:bsky:france24.com":                         "https://bsky.app/profile/did:plc:wkreszch2k4mumsrlkeaflgn/rss",
	"social:bsky:spiegel.de":                           "https://bsky.app/profile/did:plc:6xofcnvvojjnmggqx43zghwh/rss",
	"social:bsky:lemonde.fr":                           "https://bsky.app/profile/did:plc:qqxqxgdu5z3he2piqfbfaku4/rss",
	"social:bsky:elpais.com":                           "https://bsky.app/profile/did:plc:u6mkbcgviwlbhuwqirmhcgu3/rss",
	"social:bsky:kyivindependent.com":                  "https://bsky.app/profile/did:plc:yazbevg3wkzp5llnzb44tqgh/rss",
	"social:bsky:meduza.io":                            "https://bsky.app/profile/did:plc:kpkqu7odzhelr2idxytethe7/rss",
	"social:bsky:timesofisrael.com":                    "https://bsky.app/profile/did:plc:pcqzb7h2f5f3hyxvjuphjvdm/rss",
	"social:bsky:thehindu.com":                         "https://bsky.app/profile/did:plc:ppvk7zgqniviafvy46nl6op3/rss",
	"social:bsky:japantimes.co.jp":                     "https://bsky.app/profile/did:plc:rddeinqb45gzem3n3zyma7im/rss",
	"social:bsky:bellingcat.com":                       "https://bsky.app/profile/did:plc:sb54dpdfefflykmf5bcfvr7t/rss",
	"social:bsky:icij.org":                             "https://bsky.app/profile/did:plc:qnhy4hutfhm2lyx4geipgoer/rss",
	"social:bsky:occrp.org":                            "https://bsky.app/profile/did:plc:axqfxntrrgfanh4pguuhsknp/rss",
	"social:bsky:globalvoices.org":                     "https://bsky.app/profile/did:plc:5x3a6jke5p3ni7mnpd57ijhw/rss",
	"social:bsky:restofworld.org":                      "https://bsky.app/profile/did:plc:hbdc3q6k5lforao5vyuarvsp/rss",
	"social:mastodon:mastodon.social:tag:news":         "https://mastodon.social/tags/news.rss",
	"social:mastodon:mastodon.social:tag:breaking":     "https://mastodon.social/tags/breaking.rss",
	"social:mastodon:mastodon.social:tag:breakingnews": "https://mastodon.social/tags/breakingnews.rss",
	"social:mastodon:mastodon.social:tag:osint":        "https://mastodon.social/tags/osint.rss",
	"social:mastodon:journa.host:tag:news":             "https://journa.host/tags/news.rss",
	"social:mastodon:journa.host:tag:breaking":         "https://journa.host/tags/breaking.rss",
	"social:mastodon:newsie.social:tag:news":           "https://newsie.social/tags/news.rss",
}

func TestSocialSeedRowsAreLeadsNotOutletsOrPrimaries(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	seeds := 0
	for _, s := range reg.Sources {
		if strings.HasPrefix(s.ID, "social:") {
			seeds++
			if _, ok := socialSeeds[s.ID]; !ok {
				t.Errorf("%s: a social row not listed in socialSeeds (add it to this guard)", s.ID)
			}
		}
	}
	if seeds != len(socialSeeds) {
		t.Errorf("registry has %d social rows, guard lists %d", seeds, len(socialSeeds))
	}
	urls := map[string]string{}
	for id, wantURL := range socialSeeds {
		s, ok := reg.Lookup(id)
		if !ok {
			t.Errorf("%s: missing from the registry", id)
			continue
		}
		if s.Type != SourceTypeRSS {
			t.Errorf("%s: type = %q, want rss", id, s.Type)
		}
		if s.SourceStanding != SourceStandingSocialLead {
			t.Errorf("%s: source_standing = %q, want %q", id, s.SourceStanding, SourceStandingSocialLead)
		}
		if s.Official {
			t.Errorf("%s: a social row must not be official", id)
		}
		if s.License != "" {
			t.Errorf("%s: licence %q; a post is its author's, we hold no licence to republish", id, s.License)
		}
		if SourceAllowsReaderImport(s.StoreBodyPolicy) || s.StoreBodyPolicy != "excerpt_only" {
			t.Errorf("%s: store_body_policy %q, want excerpt_only (no reader import of linked pages)", id, s.StoreBodyPolicy)
		}
		if wantURL != "" && s.URL != wantURL {
			t.Errorf("%s: url = %q, want %q", id, s.URL, wantURL)
		}
		if verr := sourcefetch.ValidateURL(s.URL); verr != nil {
			t.Errorf("%s: url fails the fetch policy: %v", id, verr)
		}
		if other, dup := urls[NormalizeURL(s.URL)]; dup {
			t.Errorf("%s repeats the url of %s", id, other)
		}
		urls[NormalizeURL(s.URL)] = id
		if s.PollIntervalSeconds < 1800 {
			t.Errorf("%s: poll_interval_seconds %d is under 30 minutes", id, s.PollIntervalSeconds)
		}
		if s.MaxItemsPerPoll <= 0 || s.MaxItemsPerPoll > 20 {
			t.Errorf("%s: max_items_per_poll %d, want 1..20", id, s.MaxItemsPerPoll)
		}
		if s.Status != "" && s.Status != "active" {
			t.Errorf("%s: status %q", id, s.Status)
		}
		if s.TOSClass != "public_feed" || s.AuthPolicy != "none" {
			t.Errorf("%s: tos_class %q auth_policy %q, want public_feed and none", id, s.TOSClass, s.AuthPolicy)
		}
		parsed, perr := url.Parse(s.URL)
		if perr != nil {
			t.Errorf("%s: %v", id, perr)
			continue
		}
		// The id names the feed: a Bluesky id carries the handle and the url
		// is bsky.app; a Mastodon id carries instance and tag, the url the
		// same instance and tag.
		parts := strings.Split(id, ":")
		switch parts[1] {
		case "bsky":
			if parsed.Hostname() != "bsky.app" || !strings.HasSuffix(parsed.Path, "/rss") {
				t.Errorf("%s: url %q is not a bsky.app profile rss feed", id, s.URL)
			}
		case "mastodon":
			if len(parts) != 5 || parts[3] != "tag" || parsed.Hostname() != parts[2] || parsed.Path != "/tags/"+parts[4]+".rss" {
				t.Errorf("%s: url %q does not match the instance and tag in the id", id, s.URL)
			}
		default:
			t.Errorf("%s: unknown platform %q", id, parts[1])
		}
		// No post from this row's platform names an outlet, so none of its
		// captures counts toward the two-outlet publish rule.
		for _, post := range samplePostURLs(id, parsed) {
			if got := outlet.Of(post); got != "" {
				t.Errorf("%s: outlet.Of(%q) = %q, want no outlet", id, post, got)
			}
		}
	}
}

// samplePostURLs are post URLs a row's feed can carry: a Bluesky item links
// bsky.app/profile/<handle>/post/<id>; a Mastodon tag item links the origin
// instance's /@user/<id>, which is any instance.
func samplePostURLs(id string, feed *url.URL) []string {
	if strings.HasPrefix(id, "social:bsky:") {
		handle := strings.TrimPrefix(id, "social:bsky:")
		return []string{
			"https://bsky.app/profile/" + handle + "/post/3mydqg6nu3y2a",
			"https://bsky.app/profile/did:plc:abcdefghijklmnopqrstuvwx/post/3mydqg6nu3y2a",
		}
	}
	return []string{
		"https://" + feed.Hostname() + "/@someone/116000000000000001",
		"https://c.im/@someone/116000000000000002",
		"https://other.example/@someone@" + feed.Hostname() + "/116000000000000003",
	}
}
