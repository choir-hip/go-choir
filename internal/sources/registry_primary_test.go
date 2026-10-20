package sources

import (
	"net/url"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// The primary-source registry guard (wave 1,
// docs/problems/primary-source-registry-wave-1-2026-10-20.md). Ways it can
// fail, written before the rows:
//   - a wave 1 row is missing, or its id was reused for another feed;
//   - a row is not marked a claim-maker (official, source_standing), so the
//     outlet and claim-maker statistics cannot be kept apart;
//   - a row's licence is missing or is not one that permits commercial use
//     (Autopaper is commercial), so a non-commercial source slips in;
//   - a UN or WHO host (non-commercial terms) is added as a primary;
//   - the Central Bank row has no raised byte cap and so fails to parse;
//   - a row's URL fails the fetch policy or duplicates another row's;
//   - a row asks for a reader import of pages that block automated access
//     (the Federal Register);
//   - the Kremlin is recorded as a neutral outlet instead of state media.

// commercialLicenses are the licences a primary-source row may carry.
var commercialLicenses = map[string]bool{
	"us-federal-public-domain": true, // 17 USC 105
	"ogl-v3.0":                 true, // UK Open Government Licence
	"ogdl-tw-v1.0":             true, // Taiwan Open Government Data License
	"cc-by-4.0":                true,
}

var primaryStandings = map[string]bool{
	"primary_government": true,
	"state_media":        true,
}

var wave1 = map[string]struct {
	license  string
	standing string
	host     string
}{
	"official:whitehouse:news":                   {"us-federal-public-domain", "primary_government", "www.whitehouse.gov"},
	"official:sec:press":                         {"us-federal-public-domain", "primary_government", "www.sec.gov"},
	"official:doj:news":                          {"us-federal-public-domain", "primary_government", "www.justice.gov"},
	"official:federalregister:significant_rules": {"us-federal-public-domain", "primary_government", "www.federalregister.gov"},
	"official:federalregister:bis":               {"us-federal-public-domain", "primary_government", "www.federalregister.gov"},
	"official:uk_fcdo:news":                      {"ogl-v3.0", "primary_government", "www.gov.uk"},
	"official:uk_dsit:news":                      {"ogl-v3.0", "primary_government", "www.gov.uk"},
	"official:tw_cbc:press":                      {"ogdl-tw-v1.0", "primary_government", "www.cbc.gov.tw"},
	"official:kremlin:en_news":                   {"cc-by-4.0", "state_media", "en.kremlin.ru"},
	"official:cisa:advisories":                   {"us-federal-public-domain", "primary_government", "www.cisa.gov"},
	"official:eia:today_in_energy":               {"us-federal-public-domain", "primary_government", "www.eia.gov"},
}

func TestWave1PrimarySourceRowsAreClaimMakersWithACommercialLicence(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range wave1 {
		s, ok := reg.Lookup(id)
		if !ok {
			t.Errorf("%s: missing from the registry", id)
			continue
		}
		if s.Type != SourceTypeRSS {
			t.Errorf("%s: type = %q, want rss", id, s.Type)
		}
		if !s.Official || !primaryStandings[s.SourceStanding] || s.SourceStanding != want.standing {
			t.Errorf("%s: official=%v source_standing=%q, want official and %q", id, s.Official, s.SourceStanding, want.standing)
		}
		if !commercialLicenses[s.License] || s.License != want.license {
			t.Errorf("%s: license = %q, want %q (a commercial-use licence)", id, s.License, want.license)
		}
		parsed, perr := url.Parse(s.URL)
		if perr != nil || parsed.Hostname() != want.host {
			t.Errorf("%s: url %q does not point at %s", id, s.URL, want.host)
		}
		if verr := sourcefetch.ValidateURL(s.URL); verr != nil {
			t.Errorf("%s: url fails the fetch policy: %v", id, verr)
		}
		if s.PollIntervalSeconds < 1800 {
			t.Errorf("%s: poll_interval_seconds %d is under 30 minutes", id, s.PollIntervalSeconds)
		}
		if s.MaxItemsPerPoll <= 0 || s.MaxItemsPerPoll > 30 {
			t.Errorf("%s: max_items_per_poll %d, want 1..30", id, s.MaxItemsPerPoll)
		}
		if s.Status != "" && s.Status != "active" {
			t.Errorf("%s: status %q", id, s.Status)
		}
		if s.TOSClass == TOSClassTestFixture {
			t.Errorf("%s: a wave 1 row is a test fixture", id)
		}
	}
}

func TestWave1SpecificRowShapes(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	cbc, ok := reg.Lookup("official:tw_cbc:press")
	if !ok {
		t.Fatal("Central Bank row missing")
	}
	// 3.4 MB on 2026-10-20: over the 2 MiB default, so it needs a raised cap,
	// and within the ceiling.
	if cbc.EffectiveMaxBodyBytes() <= DefaultMaxFeedBodyBytes || cbc.MaxBodyBytes > MaxFeedBodyBytesCeiling {
		t.Errorf("Central Bank max_body_bytes = %d, want above %d and within %d", cbc.MaxBodyBytes, DefaultMaxFeedBodyBytes, MaxFeedBodyBytesCeiling)
	}
	if got := firstString(cbc.Languages); got != "zh" {
		t.Errorf("Central Bank language = %q, want zh", got)
	}
	// The Federal Register's HTML pages are CAPTCHA-gated to automated
	// clients; its rows must not ask for a reader import of them.
	for _, id := range []string{"official:federalregister:significant_rules", "official:federalregister:bis"} {
		s, _ := reg.Lookup(id)
		if SourceAllowsReaderImport(s.StoreBodyPolicy) {
			t.Errorf("%s: store_body_policy %q fetches blocked article pages", id, s.StoreBodyPolicy)
		}
	}
	kremlin, _ := reg.Lookup("official:kremlin:en_news")
	if kremlin.SourceStanding != "state_media" {
		t.Errorf("Kremlin source_standing = %q, want state_media", kremlin.SourceStanding)
	}
}

func TestWave1RowsAreUniqueAndNoNonCommercialHostIsAPrimary(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ids, urls := map[string]bool{}, map[string]string{}
	for _, s := range reg.Sources {
		if ids[s.ID] {
			t.Errorf("duplicate source id %s", s.ID)
		}
		ids[s.ID] = true
		if _, isWave1 := wave1[s.ID]; isWave1 {
			if other, dup := urls[NormalizeURL(s.URL)]; dup {
				t.Errorf("%s repeats the url of %s", s.ID, other)
			}
		}
		urls[NormalizeURL(s.URL)] = s.ID
	}
	// UN News and WHO publish for non-commercial use only; they are not
	// wave 1 and no row with a commercial licence may point at them.
	for _, s := range reg.Sources {
		if s.License == "" {
			continue
		}
		host := strings.ToLower(hostOf(s.URL))
		if strings.HasSuffix(host, "un.org") || strings.HasSuffix(host, "who.int") {
			t.Errorf("%s: licence %q on a non-commercial host %s", s.ID, s.License, host)
		}
		if !commercialLicenses[s.License] {
			t.Errorf("%s: licence %q is not on the commercial-use list", s.ID, s.License)
		}
	}
}

func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
