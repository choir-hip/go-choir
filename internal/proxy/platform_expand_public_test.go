package proxy

// Ways the public door of "Research more" can fail (owner 2026-10-20 section
// 32; docs/problems/story-expansion-request-no-reader-door-and-wrong-wake-
// framing-2026-10-20.md, "Public door"), written before the code. Each mode
// has a test below that names it.
//
//  P1  An anonymous request is accepted while PROXY_EXPAND_PUBLIC is off.
//  P2  The owner allowlist path breaks once the flag is on, the owner is
//      charged to the address throttle, or PROXY_EXPAND=off stops winning.
//  P3  One address exhausts the shared daily cap (3 an hour, 10 a UTC day).
//  P4  A spoofed X-Forwarded-For bypasses the per-address limit.
//  P5  A second request on the same published version is sent, or a refused
//      repeat spends the address's allowance.
//  P6  A non-Autopaper or unknown route is asked, answers differently, or
//      spends the address's allowance.
//  P7  The in-flight and daily global caps stop holding for public readers.
//  P8  A restart resets the in-memory counters (accepted residual, pinned).
//  P9  The anonymous requester's identity reaches the guest.

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// addStory publishes story n in the fixture's corpusd and paper.
func (f *expandFixture) addStory(n int) string {
	route := fmt.Sprintf("/pub/texture/story-%d", n)
	doc := fmt.Sprintf("doc-%d", n)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources[route] = platform.ExpansionSource{PublicationID: fmt.Sprintf("pub-%d", n), OwnerID: expandTestPaper, SourceDocID: doc, Route: route, LatestRevision: 1, SourceRevisionID: fmt.Sprintf("rev-%d", n)}
	f.docs[doc] = map[string]string{"doc_id": doc, "owner_id": expandTestPaper, "computer_id": expandTestComputer, "trajectory_id": "traj-" + doc, "current_revision_id": fmt.Sprintf("rev-%d", n)}
	return route
}

// postFrom asks as a reader at a peer address, optionally behind a forwarder
// header; user "" is anonymous.
func (f *expandFixture) postFrom(user, route, remote, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/publications/expand", strings.NewReader(`{"route":"`+route+`"}`))
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	if user != "" {
		req.AddCookie(f.ownerCookie(user))
	}
	w := httptest.NewRecorder()
	f.h.HandleAPI(w, req)
	return w
}

func (f *expandFixture) getFrom(user, route, remote string) (int, map[string]any) {
	req := httptest.NewRequest(http.MethodGet, "https://choir.news/api/platform/publications/expansion?route="+route, nil)
	req.RemoteAddr = remote
	if user != "" {
		req.AddCookie(f.ownerCookie(user))
	}
	w := httptest.NewRecorder()
	f.h.HandleAPI(w, req)
	return w.Code, decodeBody(f.t, w)
}

func (f *expandFixture) setClock(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clock = t
}

const readerAddr = "198.51.100.7:4000"

// P1
func TestExpandPublicOffRefusesAnonymousEvenWithAnAllowlist(t *testing.T) {
	f := newExpandFixture(t, []string{expandTestOwner}, false)
	if w := f.postFrom("", expandTestRoute, readerAddr, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous POST with the flag off = %d, want 401", w.Code)
	}
	if w := f.postFrom("someone-else", expandTestRoute, readerAddr, ""); w.Code != http.StatusForbidden {
		t.Fatalf("ordinary account POST with the flag off = %d, want 403", w.Code)
	}
	if code, body := f.getFrom("", expandTestRoute, readerAddr); code != http.StatusOK || body["allowed"] != false {
		t.Fatalf("anonymous GET with the flag off = %d %v, want allowed:false", code, body)
	}
	f.noUpstreamCalls()
	// The public flag alone turns the feature on, with an empty allowlist.
	g := newExpandPublicFixture(t, nil)
	if w := g.postFrom("", expandTestRoute, readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("anonymous POST with the flag on = %d %s, want 202", w.Code, w.Body.String())
	}
}

// P1, P2
func TestExpandPublicOnLetsAnAnonymousReaderAskAndSeeTheButton(t *testing.T) {
	f := newExpandPublicFixture(t, []string{expandTestOwner})
	if code, body := f.getFrom("", expandTestRoute, readerAddr); code != http.StatusOK || body["allowed"] != true || body["asked"] != false {
		t.Fatalf("anonymous GET = %d %v, want allowed:true", code, body)
	}
	if code, body := f.getFrom("someone-else", expandTestRoute, readerAddr); code != http.StatusOK || body["allowed"] != true {
		t.Fatalf("signed-in ordinary GET = %d %v, want allowed:true", code, body)
	}
	w := f.postFrom("", expandTestRoute, readerAddr, "")
	if w.Code != http.StatusAccepted || decodeBody(t, w)["status"] != "asked" {
		t.Fatalf("anonymous POST = %d %s, want 202 asked", w.Code, w.Body.String())
	}
	if f.reviseCount() != 1 {
		t.Fatalf("revises = %d, want 1", f.reviseCount())
	}
	code, body := f.getFrom("", expandTestRoute, readerAddr)
	if code != http.StatusOK || body["asked"] != true {
		t.Fatalf("GET after asking = %d %v, want asked:true", code, body)
	}
}

// P2
func TestExpandPublicKeepsTheOwnerPathAndTheKillSwitch(t *testing.T) {
	f := newExpandPublicFixture(t, []string{expandTestOwner})
	// The owner is not charged to the address throttle: four stories in under
	// two hours from one address would trip the hourly three if charged.
	for n := 1; n <= 4; n++ {
		route := f.addStory(n)
		if w := f.postFrom(expandTestOwner, route, readerAddr, ""); w.Code != http.StatusAccepted {
			t.Fatalf("owner request %d = %d %s", n, w.Code, w.Body.String())
		}
		f.advance(10 * time.Minute)
		f.setSettled(route)
	}
	// An anonymous reader at the same address still has the whole allowance.
	route := f.addStory(5)
	if w := f.postFrom("", route, readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("anonymous after owner asks = %d %s, want 202", w.Code, w.Body.String())
	}
	// Kill switch wins over the public flag.
	k := newExpandFixtureWith(t, []string{expandTestOwner}, true, true)
	if w := k.postFrom("", expandTestRoute, readerAddr, ""); w.Code != http.StatusNotFound {
		t.Fatalf("kill switch anonymous POST = %d, want 404", w.Code)
	}
	if w := k.postFrom(expandTestOwner, expandTestRoute, readerAddr, ""); w.Code != http.StatusNotFound {
		t.Fatalf("kill switch owner POST = %d, want 404", w.Code)
	}
	if code, body := k.getFrom("", expandTestRoute, readerAddr); code != http.StatusOK || body["allowed"] != false {
		t.Fatalf("kill switch GET = %d %v, want allowed:false", code, body)
	}
	k.noUpstreamCalls()
}

// setSettled frees a story's in-flight slot, as a read that found it revised
// does.
func (f *expandFixture) setSettled(route string) {
	f.mu.Lock()
	src := f.sources[route]
	f.mu.Unlock()
	f.h.expandLedger().settleByKey(src.PublicationID, src.SourceRevisionID)
}

// P3
func TestExpandPublicLimitsOneAddressToThreeAnHourAndTenADay(t *testing.T) {
	f := newExpandPublicFixture(t, nil)
	// Three in an hour, ten minutes apart.
	for n := 1; n <= 3; n++ {
		if w := f.postFrom("", f.addStory(n), readerAddr, ""); w.Code != http.StatusAccepted {
			t.Fatalf("request %d = %d %s", n, w.Code, w.Body.String())
		}
		f.advance(10 * time.Minute)
	}
	route := f.addStory(4)
	w := f.postFrom("", route, readerAddr, "")
	body := decodeBody(t, w)
	if w.Code != http.StatusTooManyRequests || body["status"] != "address_limit" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("fourth in an hour = %d %s retry-after=%q, want 429 address_limit", w.Code, w.Body.String(), w.Header().Get("Retry-After"))
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "too many requests from your address") {
		t.Fatalf("error = %q", msg)
	}
	if f.reviseCount() != 3 {
		t.Fatalf("revises = %d, want 3 (the refused request never reached the paper)", f.reviseCount())
	}
	// Another address is served.
	if w := f.postFrom("", route, "203.0.113.50:1", ""); w.Code != http.StatusAccepted {
		t.Fatalf("other address = %d %s, want 202", w.Code, w.Body.String())
	}
	// The window slides: an hour after the first, the address may ask again.
	f.advance(31 * time.Minute) // 61 minutes after the first
	if w := f.postFrom("", f.addStory(5), readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("after the hour = %d %s, want 202", w.Code, w.Body.String())
	}
	// Ten in the UTC day: this address has asked four (1, 2, 3, 5). 21-minute
	// steps never put three in one hour or three in flight.
	accepted := 4
	for n := 6; n < 40; n++ {
		f.advance(21 * time.Minute)
		w := f.postFrom("", f.addStory(n), readerAddr, "")
		if w.Code == http.StatusAccepted {
			accepted++
			continue
		}
		if decodeBody(t, w)["status"] != "address_limit" {
			t.Fatalf("request %d = %d %s, want address_limit", n, w.Code, w.Body.String())
		}
		break
	}
	if accepted != 10 {
		t.Fatalf("one address was served %d times in a UTC day, want exactly 10", accepted)
	}
	// The next UTC day resets the day count.
	f.setClock(time.Date(2026, 10, 21, 0, 5, 0, 0, time.UTC))
	if w := f.postFrom("", f.addStory(100), readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("next UTC day = %d %s, want 202", w.Code, w.Body.String())
	}
}

// P4
func TestExpandPublicAddressComesFromTheEdgeNotFromTheClient(t *testing.T) {
	f := newExpandPublicFixture(t, nil)
	// The edge appends the real address as the last entry; whatever the client
	// put before it does not make a new address.
	for n := 1; n <= 3; n++ {
		xff := fmt.Sprintf("10.9.9.%d, 203.0.113.9", n)
		if w := f.postFrom("", f.addStory(n), "127.0.0.1:5000", xff); w.Code != http.StatusAccepted {
			t.Fatalf("request %d = %d %s", n, w.Code, w.Body.String())
		}
		f.advance(time.Minute)
	}
	route := f.addStory(4)
	for _, xff := range []string{"1.2.3.4, 203.0.113.9", "203.0.113.9", "9.9.9.9,   203.0.113.9"} {
		w := f.postFrom("", route, "127.0.0.1:5000", xff)
		if w.Code != http.StatusTooManyRequests || decodeBody(t, w)["status"] != "address_limit" {
			t.Fatalf("spoofed %q = %d %s, want address_limit", xff, w.Code, w.Body.String())
		}
	}
	// A different edge-observed address is a different reader.
	if w := f.postFrom("", route, "127.0.0.1:5000", "203.0.113.10"); w.Code != http.StatusAccepted {
		t.Fatalf("other real address = %d %s, want 202", w.Code, w.Body.String())
	}
	// With no forwarder header the peer address is the address, not the port.
	g := newExpandPublicFixture(t, nil)
	for n := 1; n <= 3; n++ {
		if w := g.postFrom("", g.addStory(n), "192.0.2.77:1111", ""); w.Code != http.StatusAccepted {
			t.Fatalf("peer request %d = %d", n, w.Code)
		}
		g.advance(time.Minute)
	}
	if w := g.postFrom("", g.addStory(4), "192.0.2.77:2222", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("same peer, new port = %d, want 429", w.Code)
	}
}

// P5
func TestExpandPublicSecondRequestOnTheSameVersionIsRefusedAndFree(t *testing.T) {
	f := newExpandPublicFixture(t, []string{expandTestOwner})
	if w := f.postFrom("", expandTestRoute, readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("first = %d %s", w.Code, w.Body.String())
	}
	for _, from := range []string{readerAddr, "203.0.113.20:1"} {
		for i := 0; i < 5; i++ { // more than the hourly allowance
			w := f.postFrom("", expandTestRoute, from, "")
			if w.Code != http.StatusConflict || decodeBody(t, w)["status"] != "already_asked" {
				t.Fatalf("repeat from %s = %d %s, want 409 already_asked", from, w.Code, w.Body.String())
			}
		}
	}
	if w := f.postFrom(expandTestOwner, expandTestRoute, readerAddr, ""); w.Code != http.StatusConflict {
		t.Fatalf("owner repeat = %d, want 409", w.Code)
	}
	if f.reviseCount() != 1 {
		t.Fatalf("revises = %d, want 1", f.reviseCount())
	}
	// Refused repeats spent nothing: two more stories from the same address fit.
	for n := 1; n <= 2; n++ {
		if w := f.postFrom("", f.addStory(n), readerAddr, ""); w.Code != http.StatusAccepted {
			t.Fatalf("story %d after repeats = %d %s, want 202", n, w.Code, w.Body.String())
		}
	}
}

// P6
func TestExpandPublicUnknownAndNonStoryRoutesAreNotAskedAndAreFree(t *testing.T) {
	f := newExpandPublicFixture(t, nil)
	var first string
	for i, route := range []string{"/pub/texture/never-published-zzzzzz", "/somewhere/else", "/pub/texture/story-1/../x"} {
		for k := 0; k < 5; k++ { // more than the hourly allowance
			w := f.postFrom("", route, readerAddr, "")
			if w.Code != http.StatusNotFound {
				t.Fatalf("route %q = %d %s, want 404", route, w.Code, w.Body.String())
			}
			if i == 0 && k == 0 {
				first = w.Body.String()
			} else if w.Body.String() != first {
				t.Fatalf("route %q answers %q, want the same as an unknown route %q", route, w.Body.String(), first)
			}
		}
		if code, body := f.getFrom("", route, readerAddr); code != http.StatusOK || body["allowed"] != false {
			t.Fatalf("GET %q = %d %v, want allowed:false", route, code, body)
		}
	}
	if f.reviseCount() != 0 {
		t.Fatalf("revises = %d, want 0", f.reviseCount())
	}
	f.mu.Lock()
	for _, call := range f.paperCalls {
		t.Errorf("the paper was reached: %s", call)
	}
	f.mu.Unlock()
	// And the failed asks spent no allowance.
	for n := 1; n <= 3; n++ {
		if w := f.postFrom("", f.addStory(n), readerAddr, ""); w.Code != http.StatusAccepted {
			t.Fatalf("story %d = %d %s, want 202", n, w.Code, w.Body.String())
		}
	}
}

// P7
func TestExpandPublicSharesTheGlobalInFlightAndDailyCaps(t *testing.T) {
	f := newExpandPublicFixture(t, nil)
	for n := 1; n <= 3; n++ { // three different addresses, three in flight
		if w := f.postFrom("", f.addStory(n), fmt.Sprintf("203.0.113.%d:1", n), ""); w.Code != http.StatusAccepted {
			t.Fatalf("request %d = %d %s", n, w.Code, w.Body.String())
		}
	}
	w := f.postFrom("", f.addStory(4), "203.0.113.99:1", "")
	if w.Code != http.StatusTooManyRequests || decodeBody(t, w)["status"] != "limit" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("fourth in flight = %d %s, want 429 limit", w.Code, w.Body.String())
	}
	// The daily twelve: one request from each of twelve addresses.
	f.advance(31 * time.Minute)
	for n := 4; n <= 12; n++ {
		if w := f.postFrom("", f.addStory(n), fmt.Sprintf("203.0.113.%d:1", n), ""); w.Code != http.StatusAccepted {
			t.Fatalf("request %d = %d %s", n, w.Code, w.Body.String())
		}
		f.advance(11 * time.Minute)
	}
	f.advance(31 * time.Minute)
	w = f.postFrom("", f.addStory(13), "203.0.113.200:1", "")
	if w.Code != http.StatusTooManyRequests || decodeBody(t, w)["status"] != "limit" {
		t.Fatalf("thirteenth in the day = %d %s, want 429 limit", w.Code, w.Body.String())
	}
}

// P8: pins the accepted residual so nobody reads the counters as durable.
func TestExpandPublicCountersLiveInMemoryAndAProxyRestartResetsThem(t *testing.T) {
	f := newExpandPublicFixture(t, nil)
	for n := 1; n <= 3; n++ {
		f.postFrom("", f.addStory(n), readerAddr, "")
		f.advance(time.Minute)
	}
	route := f.addStory(4)
	if w := f.postFrom("", route, readerAddr, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("before the restart = %d, want 429", w.Code)
	}
	f.h.expandMu.Lock()
	f.h.expand = nil // what a proxy restart does to the memory
	f.h.expandMu.Unlock()
	f.h.expandLedger().now = f.now
	if w := f.postFrom("", route, readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("after the restart = %d %s, want 202 (accepted residual: counters are in memory)", w.Code, w.Body.String())
	}
}

// P9
func TestExpandPublicRequesterIdentityNeverReachesThePaper(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(io.Discard)

	const addr = "198.51.100.231"
	f := newExpandPublicFixture(t, []string{expandTestOwner})
	if w := f.postFrom("", expandTestRoute, "127.0.0.1:9", "172.16.5.5, "+addr); w.Code != http.StatusAccepted {
		t.Fatalf("anonymous POST = %d %s", w.Code, w.Body.String())
	}
	g := newExpandPublicFixture(t, []string{expandTestOwner})
	if w := g.postFrom("signed-in-stranger", expandTestRoute, "127.0.0.1:9", addr); w.Code != http.StatusAccepted {
		t.Fatalf("signed-in stranger POST = %d %s", w.Code, w.Body.String())
	}
	for name, fx := range map[string]*expandFixture{"anonymous": f, "signed-in stranger": g} {
		fx.mu.Lock()
		if len(fx.revises) != 1 {
			t.Fatalf("%s: revises = %d", name, len(fx.revises))
		}
		call := fx.revises[0]
		if call.User != expandTestPaper {
			t.Errorf("%s: revise acted as %q, want the story owner %q", name, call.User, expandTestPaper)
		}
		if !strings.Contains(call.Body.Prompt, "provenance: public reader") {
			t.Errorf("%s: request text lacks the public reader provenance: %q", name, call.Body.Prompt)
		}
		for _, secret := range []string{addr, "172.16.5.5", "signed-in-stranger", "choir_access"} {
			if strings.Contains(call.Body.Prompt, secret) || strings.Contains(call.User, secret) {
				t.Errorf("%s: the paper request carries %q", name, secret)
			}
		}
		for _, c := range fx.paperCalls {
			if strings.Contains(c, addr) || strings.Contains(c, "signed-in-stranger") {
				t.Errorf("%s: paper path carries identity: %s", name, c)
			}
		}
		fx.mu.Unlock()
	}
	line := logs.String()
	for _, secret := range []string{addr, "172.16.5.5", "signed-in-stranger"} {
		if strings.Contains(line, secret) {
			t.Fatalf("log line carries %q: %q", secret, line)
		}
	}
	// The owner's own request keeps the owner provenance.
	h := newExpandPublicFixture(t, []string{expandTestOwner})
	if w := h.postFrom(expandTestOwner, expandTestRoute, readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("owner POST = %d", w.Code)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if p := h.revises[0].Body.Prompt; strings.Contains(p, "public reader") || !strings.Contains(p, "provenance: owner") {
		t.Fatalf("owner request text = %q", p)
	}
}

// A public reader's read follows only the newest request, so an anonymous
// poll cannot fan out into many paper reads.
func TestExpansionReadForAPublicReaderReturnsOnlyTheNewestRequest(t *testing.T) {
	f := newExpandPublicFixture(t, []string{expandTestOwner})
	if w := f.postFrom("", expandTestRoute, readerAddr, ""); w.Code != http.StatusAccepted {
		t.Fatalf("POST = %d", w.Code)
	}
	// A second, newer request on the same story (a newer published version).
	f.mu.Lock()
	src := f.sources[expandTestRoute]
	src.SourceRevisionID = "rev-head-4"
	f.sources[expandTestRoute] = src
	f.docs[expandTestDocID]["current_revision_id"] = "rev-head-4"
	f.mu.Unlock()
	f.advance(time.Minute)
	if w := f.postFrom("", expandTestRoute, "203.0.113.5:1", ""); w.Code != http.StatusAccepted {
		t.Fatalf("second POST = %d %s", w.Code, w.Body.String())
	}
	code, body := f.getFrom("", expandTestRoute, "203.0.113.6:1")
	reqs, _ := body["requests"].([]any)
	if code != http.StatusOK || len(reqs) != 1 {
		t.Fatalf("public GET = %d %v, want one request", code, body)
	}
	_, ownerBody := f.getFrom(expandTestOwner, expandTestRoute, readerAddr)
	if all, _ := ownerBody["requests"].([]any); len(all) != 2 {
		t.Fatalf("owner GET = %v, want both requests", ownerBody)
	}
}

// E11 for the deeper request: web search, primary documents, distinct outlets,
// citations; still an expansion and not a rewrite.
func TestExpansionRequestTextAsksResearchToDigDeeper(t *testing.T) {
	text := expansionRequestText("rx-abc", expandProvenancePublic)
	for _, must := range []string{
		"[expansion rx-abc]", "provenance: public reader", "search the web", "primary documents",
		"additional distinct outlets", "new reporting", "citations",
		"Keep the story's existing text, structure and citations",
		"change nothing and say why in a note to management",
	} {
		if !strings.Contains(text, must) {
			t.Errorf("request text lacks %q: %q", must, text)
		}
	}
}
