package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The research desk's read ledger, pack check and interim nudge
// (docs/problems/ordinary-prompt-path-2026-11-09.md 1.5 items 2-3, 3.2 item 1;
// definition L5). Ways it can fail, written before the code:
//
//  1. A Read that fails (403, 429, "no readable text") leaves no trace the
//     desk can see: `p, _ := choir.Read(url)` discards the error and the
//     model never learns the page was lost.
//  2. The ledger lists a failed Read as a capture, or drops a successful one,
//     or records the same capture twice for a re-read.
//  3. choir.Captures() includes failed reads, or choir.Reads() omits them.
//  4. A packet that leaves out a captured read passes in silence; or the
//     warning refuses the packet instead of warning; or it names a capture
//     the packet did include (matched by capture_id or by URL).
//  5. The interim nudge fires before any capture, fires twice, fires after
//     the desk has already sent a packet, or fires on a desk that is not
//     the research desk (a research_web child reports once, at its end).
//  6. A search summary lists 40 raw lines (the 8000-rune cut) instead of
//     deduping by host, or loses a host's count.

func readLedgerScope(t *testing.T, role string, fail map[string]string) *ChoirScope {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: t.TempDir(),
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			var in ReadPayload
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, err
			}
			if msg, bad := fail[in.URL]; bad {
				return nil, fmt.Errorf("%s", msg)
			}
			id := strings.TrimPrefix(strings.TrimPrefix(in.URL, "https://"), "http://")
			return json.RawMessage(fmt.Sprintf(`{"capture_id":"cap-%s","url":%q,"title":"T %s","block_count":3}`, id, in.URL, id)), nil
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-ledger", 1, role, "")
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func TestReadLedgerShowsAFailedReadWithItsError(t *testing.T) {
	scope := readLedgerScope(t, "research", map[string]string{"https://openai.com/x": "HTTP 403"})
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	_, _ = scope.Read("https://openai.com/x") // the error discarded, as the desk did
	_, _ = scope.Read("https://a.example/1")
	lines := strings.Join(hooks.SearchCounts(), "\n")
	hooks.End()
	if !strings.Contains(lines, "https://openai.com/x") || !strings.Contains(lines, "403") {
		t.Fatalf("a failed Read is invisible in the cell result:\n%s", lines)
	}
	reads := scope.Reads()
	if len(reads) != 2 || reads[0].OK || reads[0].Error == "" || !reads[1].OK || reads[1].CaptureID != "cap-a.example/1" {
		t.Fatalf("ledger wrong: %+v", reads)
	}
	caps := scope.Captures()
	if len(caps) != 1 || caps[0].URL != "https://a.example/1" {
		t.Fatalf("Captures must hold only successful reads: %+v", caps)
	}
}

func TestReadLedgerRecordsAReReadOnce(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	scope.BindCell().Begin(SessionFrame{})
	_, _ = scope.Read("https://a.example/1")
	_, _ = scope.Read("https://a.example/1")
	if caps := scope.Captures(); len(caps) != 1 {
		t.Fatalf("a re-read must not duplicate its capture: %+v", caps)
	}
}

func TestReportPacketWarnsOfCapturesAbsentFromThePacket(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	for _, u := range []string{"https://a.example/1", "https://b.example/2", "https://c.example/3"} {
		if _, err := scope.Read(u); err != nil {
			t.Fatal(err)
		}
	}
	hooks.SearchCounts() // drain
	packet := `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","source_ids":["s1","s2"]}],
	"sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1","title":"T"},"selectors":[{"kind":"text_quote","quote":"q"}]},
	{"source_id":"s2","kind":"web_source","target":{"uri":"https://elsewhere.example/","capture_id":"cap-b.example/2"},"selectors":[{"kind":"text_quote","quote":"q"}]}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatalf("a warning must not refuse the packet: %v", err)
	}
	lines := strings.Join(hooks.SearchCounts(), "\n")
	if !strings.Contains(lines, "cap-c.example/3") {
		t.Fatalf("the omitted capture is not named:\n%s", lines)
	}
	if strings.Contains(lines, "cap-a.example/1") || strings.Contains(lines, "cap-b.example/2") {
		t.Fatalf("a packed capture was named as absent (matched by URL and by capture_id):\n%s", lines)
	}
	if !strings.Contains(strings.ToLower(lines), "include") {
		t.Fatalf("the warning does not ask for inclusion or a reason:\n%s", lines)
	}
}

func TestReportPacketWithEveryCapturePackedIsSilent(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	_, _ = scope.Read("https://a.example/1")
	hooks.SearchCounts()
	packet := `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1"}}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatal(err)
	}
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "absent") {
			t.Fatalf("warned although every capture is packed: %s", l)
		}
	}
}

func TestInterimNudgeFiresOnceAfterTheFirstCapture(t *testing.T) {
	scope := readLedgerScope(t, "research", map[string]string{"https://bad.example/": "HTTP 429"})
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	_, _ = scope.Read("https://bad.example/")
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "interim packet") {
			t.Fatalf("nudged before any capture: %s", l)
		}
	}
	_, _ = scope.Read("https://a.example/1")
	_, _ = scope.Read("https://b.example/2")
	count := 0
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "interim packet") {
			count++
			if !strings.Contains(l, "2 captured") {
				t.Errorf("nudge does not count the captures: %s", l)
			}
		}
	}
	if count != 1 {
		t.Fatalf("nudge fired %d times for the first captures, want once", count)
	}
	// Nothing new was read: no second nudge. A new capture is nudged once,
	// naming only itself (B1: each finding is reported as it is read).
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "interim packet") {
			t.Fatalf("nudged again with nothing new read: %s", l)
		}
	}
	_, _ = scope.Read("https://c.example/3")
	later := 0
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "interim packet") {
			later++
			if !strings.Contains(l, "cap-c.example/3") || strings.Contains(l, "cap-a.example/1") {
				t.Errorf("the later nudge must name only the new capture: %s", l)
			}
		}
	}
	if later != 1 {
		t.Fatalf("the third read was nudged %d times, want once", later)
	}
}

func TestNudgeAfterAPacketNamesOnlyCapturesItDoesNotCarry(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	_, _ = scope.Read("https://a.example/1")
	hooks.SearchCounts()
	packet := `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1","capture_id":"cap-a.example/1"}}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatal(err)
	}
	hooks.SearchCounts()
	_, _ = scope.Read("https://b.example/2")
	lines := strings.Join(hooks.SearchCounts(), "\n")
	if !strings.Contains(lines, "cap-b.example/2") || strings.Contains(lines, "cap-a.example/1") || !strings.Contains(lines, "another interim packet") {
		t.Fatalf("a read after a packet must be nudged for itself only:\n%s", lines)
	}
}

func TestInterimNudgeSkippedOnceAPacketWasSent(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	packet := `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1"}}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatal(err)
	}
	_, _ = scope.Read("https://a.example/1")
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "interim packet") {
			t.Fatalf("nudged a desk that already reported: %s", l)
		}
	}
}

func TestInterimNudgeIsTheResearchDeskOnly(t *testing.T) {
	scope := readLedgerScope(t, SessionRoleResearchWeb, nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	_, _ = scope.Read("https://a.example/1")
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "interim packet") {
			t.Fatalf("a research child was nudged to send a packet: %s", l)
		}
	}
	if len(scope.Captures()) != 1 {
		t.Fatal("the child's ledger must still record")
	}
}

func TestSearchSummaryDedupesByHost(t *testing.T) {
	store := newSearchStore()
	store.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "ai", Providers: []string{"keenable"}})
	var urls []string
	for i := 0; i < 30; i++ {
		urls = append(urls, fmt.Sprintf("https://big.example/p%d", i))
	}
	urls = append(urls, "https://one.example/a", "https://two.example/b")
	store.apply(providerFrame("s1", "keenable", true, urls...))
	out, err := store.Summary("s1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "big.example") || !strings.Contains(out, "(30)") || !strings.Contains(out, "one.example") || !strings.Contains(out, "two.example") {
		t.Fatalf("summary lost a host or a count:\n%s", out)
	}
	if strings.Count(out, "big.example/p") > 6 { // two hits, each in title and URL
		t.Fatalf("summary dumps raw lines:\n%s", out)
	}
	if len(out) > 4000 {
		t.Fatalf("summary is %d bytes; it must stay under the output cut", len(out))
	}
}

// Run boundary: the desk's session persists across runs, the ledger does not.
// Failure modes: the nudge fires once ever on a continuing session; Captures
// and Reads carry run 1 into run 2; the pack warning names run 1's captures
// as left out of run 2's packet; a pack warning over many captures lists them
// all.
func TestReadLedgerResetsAtARunBoundary(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{RunID: "run-1"})
	_, _ = scope.Read("https://a.example/1")
	nudges := func() int {
		n := 0
		for _, l := range hooks.SearchCounts() {
			if strings.Contains(l, "interim packet") {
				n++
			}
		}
		return n
	}
	if nudges() != 1 {
		t.Fatal("run 1 must be nudged")
	}
	hooks.Begin(SessionFrame{RunID: "run-1"}) // a second cell of the same run keeps its ledger
	if len(scope.Captures()) != 1 {
		t.Fatal("a second cell of the same run lost the ledger")
	}
	hooks.Begin(SessionFrame{RunID: "run-2"})
	if len(scope.Reads()) != 0 || len(scope.Captures()) != 0 {
		t.Fatalf("run 2 sees run 1's reads: %+v", scope.Reads())
	}
	_, _ = scope.Read("https://b.example/2")
	if nudges() != 1 {
		t.Fatal("run 2 must be nudged once more")
	}
	packet := `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://b.example/2"}}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatal(err)
	}
	for _, l := range hooks.SearchCounts() {
		if strings.Contains(l, "cap-a.example/1") {
			t.Fatalf("run 2's pack warning names run 1's capture: %s", l)
		}
	}
}

func TestPackWarningCapsItsList(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{RunID: "r"})
	for i := 0; i < 25; i++ {
		_, _ = scope.Read(fmt.Sprintf("https://h%d.example/p", i))
	}
	hooks.SearchCounts()
	packet := `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://elsewhere.example/"}}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(hooks.SearchCounts(), "\n")
	if !strings.Contains(lines, "and 15 more") || strings.Contains(lines, "h24.example") {
		t.Fatalf("warning list not capped at 10:\n%s", lines)
	}
}
