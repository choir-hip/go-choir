package platform

// The reliance step names what it did in one corpusd log line (staging could
// not otherwise show whether the link was written: the link is data only).
// Ways the line can fail: it is missing when an own-host capture was cited and
// no edge came of it (the case nobody could see); it carries a URL or story
// text; it is unbounded; it logs on every story that cites only other sites.
// And the staging proof's URL shape: a percent-encoded route query resolves.

import (
	"bytes"
	"log"
	"net/url"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old); log.SetFlags(flags) })
	return &buf
}

func relianceLines(buf *bytes.Buffer) []string {
	var out []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "corpusd: reliance:") {
			out = append(out, line)
		}
	}
	return out
}

func TestOwnRouteReadsThePercentEncodedExportURLAndOnlyStoryRoutes(t *testing.T) {
	const host = "choir.news"
	exp := "https://choir.news/api/platform/publications/export?format=txt&raw=1&route=%2Fpub%2Ftexture%2Fqa3-fork-of-bc-pub97db73ba8"
	if got := ownRoute(exp, host); got != "/pub/texture/qa3-fork-of-bc-pub97db73ba8" {
		t.Fatalf("ownRoute(export) = %q", got)
	}
	if got := ownRoute("https://choir.news/pub/texture/qa3/v4", host); got != "/pub/texture/qa3" {
		t.Fatalf("ownRoute(versioned page) = %q", got)
	}
	for _, other := range []string{"https://choir.news/auth/session", "https://choir.news/", "https://choir.news/api/platform/publications/export?route=%2Fauth%2Fx"} {
		if got := ownRoute(other, host); got != "" {
			t.Errorf("ownRoute(%q) = %q, want none: only /pub/texture routes name a story", other, got)
		}
	}
}

func TestRelianceStepLogsWhatItDid(t *testing.T) {
	l := newRelatedLab(t)
	buf := captureLog(t)
	earlier := l.publish("doc-earlier", 1, "Harbor reopens")
	own := l.capture(l.svc.publicationExportReadURL(earlier.RoutePath), "", "Secret story text sentence")
	unknown := l.capture(l.svc.publicationExportReadURL("/pub/texture/no-such-story"), "", "Unknown route text")
	foreign := l.capture("https://wire.example/a", "", "A foreign page")
	l.publish("doc-later", 1, "Ferries resume", own, unknown, foreign)

	lines := relianceLines(buf)
	if len(lines) != 1 {
		t.Fatalf("reliance log lines = %d, want 1: %q", len(lines), lines)
	}
	line := lines[0]
	ident, _ := publicationIdentityFor(labOwner, "doc-later")
	for _, want := range []string{ident.PublicationID, "captures=3", "own_host=2", "edges=1", `route_not_found:"/pub/texture/no-such-story"`, "not_own_host"} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %q: %s", want, line)
		}
	}
	for _, bad := range []string{"Secret story text", "http://", "https://", "wire.example"} {
		if strings.Contains(line, bad) {
			t.Errorf("line carries %q: %s", bad, line)
		}
	}
}

func TestRelianceStepLogReasonCodes(t *testing.T) {
	l := newRelatedLab(t)
	buf := captureLog(t)
	self := l.publish("doc-self", 1, "Self story")
	withdrawn := l.publish("doc-gone", 1, "Withdrawn story")
	other := l.publishAs("other-owner", PublishModeAuto, nil, "doc-other", 1, "Other owner story")
	selfCap := l.capture(l.svc.publicationExportReadURL(self.RoutePath), "", "Self text")
	goneCap := l.capture(l.svc.publicationExportReadURL(withdrawn.RoutePath), "", "Gone text")
	otherCap := l.capture(l.svc.publicationExportReadURL(other.RoutePath), "", "Other text")
	if _, err := l.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, l.pubID("doc-gone")); err != nil {
		t.Fatal(err)
	}
	l.publish("doc-self", 2, "Self story", selfCap, goneCap, otherCap)
	lines := relianceLines(buf)
	if len(lines) != 1 {
		t.Fatalf("reliance log lines = %d: %q", len(lines), lines)
	}
	for _, want := range []string{"edges=0", "self:", "not_published:", "different_owner:"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("line lacks %q: %s", want, lines[0])
		}
	}
}

func TestRelianceStepLogIsBoundedAndQuietWithoutOwnHostCaptures(t *testing.T) {
	l := newRelatedLab(t)
	buf := captureLog(t)
	foreign := l.capture("https://wire.example/a", "", "A foreign page")
	l.publish("doc-quiet", 1, "Quiet story", foreign)
	if lines := relianceLines(buf); len(lines) != 0 {
		t.Fatalf("a story citing only other sites logged: %q", lines)
	}
	var caps []string
	for i := 0; i < 9; i++ {
		caps = append(caps, l.capture(l.svc.publicationExportReadURL("/pub/texture/missing-"+string(rune('a'+i))), "", "Missing text "+string(rune('a'+i))))
	}
	l.publish("doc-many", 1, "Many story", caps...)
	lines := relianceLines(buf)
	if len(lines) != 1 || strings.Count(lines[0], "route_not_found:") != 5 || !strings.Contains(lines[0], "own_host=9") {
		t.Fatalf("bounded line = %q", lines)
	}
}

// B1: the route in a skip entry comes from a cited capture's URL, which a page
// controls. A newline in it must not forge a log line, and a huge slug must not
// make the line unbounded.
func TestRelianceStepLogCannotBeForgedOrInflatedByACitedURL(t *testing.T) {
	l := newRelatedLab(t)
	buf := captureLog(t)
	base := l.svc.publicBase() + "/api/platform/publications/export?format=txt&raw=1&route="
	forged := l.capture(base+url.QueryEscape("/pub/texture/x\ncorpusd: reliance: publication=forged captures=9 edges=99\r\x1b[2J"), "", "Forged route text")
	huge := l.capture(base+url.QueryEscape("/pub/texture/"+strings.Repeat("a", 5000)), "", "Huge slug text")
	pagePath := l.capture(l.svc.publicBase()+"/pub/texture/y%0Acorpusd:%20reliance:%20publication=forged2", "", "Escaped page path text")
	l.publish("doc-forge", 1, "Forge story", forged, huge, pagePath)

	var ours []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "forged") && !strings.Contains(line, "corpusd: reliance: publication=pub-") {
			t.Errorf("a forged line reached the log: %q", line)
		}
		if strings.Contains(line, "corpusd: reliance: publication=pub-") {
			ours = append(ours, line)
		}
	}
	if len(ours) != 1 {
		t.Fatalf("reliance lines = %d, want exactly 1: %q", len(ours), ours)
	}
	line := ours[0]
	for _, r := range line {
		if r < 0x20 || r == 0x7f {
			t.Fatalf("control character %q in the line: %q", r, line)
		}
	}
	if len(line) > 1500 {
		t.Fatalf("line is %d bytes for 3 candidates, each entry must be byte-capped: %q", len(line), line[:200])
	}
	if strings.Contains(line, strings.Repeat("a", 200)) {
		t.Fatalf("a 5000-byte slug was not cut")
	}
}

// N1, N2, N4: entries that carry a route are shown first, an own-host URL that
// is not a story route has its own code, and the line keeps a count.
func TestRelianceStepLogPrefersRoutesNamesNotStoryRouteAndCounts(t *testing.T) {
	l := newRelatedLab(t)
	buf := captureLog(t)
	var caps []string
	for i := 0; i < 6; i++ {
		caps = append(caps, l.capture("https://wire.example/"+string(rune('a'+i)), "", "Foreign "+string(rune('a'+i))))
	}
	caps = append(caps, l.capture(l.svc.publicBase()+"/auth/session", "", "Not a story route"))
	caps = append(caps, l.capture(l.svc.publicationExportReadURL("/pub/texture/missing-one"), "", "Missing one"))
	l.publish("doc-order", 1, "Order story", caps...)
	lines := relianceLines(buf)
	if len(lines) != 1 {
		t.Fatalf("lines = %q", lines)
	}
	line := lines[0]
	for _, want := range []string{"captures=8", "own_host=2", "skipped_total=8", `route_not_found:"/pub/texture/missing-one"`, "not_story_route", "more=3"} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %q: %s", want, line)
		}
	}
	if strings.Count(line, "not_own_host") > 3 {
		t.Errorf("foreign entries crowded out the rest: %s", line)
	}
}
