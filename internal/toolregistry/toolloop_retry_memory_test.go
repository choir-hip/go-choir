package toolregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The computer remembers which providers are down and does not keep asking
// (owner, 2026-11-09: "always be intelligent about not retrying"). Ways it
// could fail, written down before the change:
//  1. a restart forgets every hold and strike, so the first runs after it
//     re-learn each dead member by failing;
//  2. a restart resets the strikes, so the backoff starts over at one minute;
//  3. a corrupt or oversized state file blocks boot or grows without bound;
//  4. the every-model incident forgets a member whose reset is in the future
//     and asks the gateway again each round;
//  5. a bad key or a model that is gone is retried on the transient schedule
//     (15 min cap) instead of a long one;
//  6. a hard hold never grows, or never ends on a success;
//  7. the hard-failure marker matches an ordinary 5xx.

func TestLedgerRestoresHoldsAndStrikesAfterARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.provider-availability.json")
	now := time.Now()
	a := &QuotaLedger{}
	if err := a.Persist(path, now); err != nil {
		t.Fatal(err)
	}
	a.Mark(lunaMember, now.Add(time.Hour))
	a.Demote(synthMember, now)
	a.Demote(synthMember, now) // second strike: 2 min
	a.DemoteHard(routerSel, now)

	b := &QuotaLedger{} // the restarted process
	if err := b.Persist(path, now.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	got := map[string]QuotaHold{}
	for _, h := range b.Holds(now.Add(10 * time.Second)) {
		got[h.Member] = h
	}
	if len(got) != 3 {
		t.Fatalf("restored holds = %+v, want luna, synth and the router", got)
	}
	if h := got[servingSelectionID(lunaMember)]; h.Source != QuotaHoldProviderReset || !h.Until.Equal(now.Add(time.Hour)) {
		t.Fatalf("luna hold = %+v", h)
	}
	if h := got[servingSelectionID(routerSel)]; h.Source != QuotaHoldHardFailure {
		t.Fatalf("router hold = %+v, want a hard failure", h)
	}
	// The strikes survive: the next failure is the third, 4 min, not 1 min.
	at := now.Add(5 * time.Minute)
	if until := b.Demote(synthMember, at); !until.Equal(at.Add(4 * time.Minute)) {
		t.Fatalf("third strike holds until %s, want %s (strikes restored)", until, at.Add(4*time.Minute))
	}
	// A success is remembered too.
	b.Succeed(synthMember)
	c := &QuotaLedger{}
	if err := c.Persist(path, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, h := range c.Holds(now.Add(time.Minute)) {
		if h.Member == servingSelectionID(synthMember) {
			t.Fatalf("synth still held after its success: %+v", h)
		}
	}
}

func TestLedgerDropsExpiredHoldsAndToleratesACorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	now := time.Now()
	a := &QuotaLedger{}
	_ = a.Persist(path, now)
	a.Mark(lunaMember, now.Add(time.Minute))
	b := &QuotaLedger{}
	if err := b.Persist(path, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if h := b.Holds(now.Add(time.Hour)); len(h) != 0 {
		t.Fatalf("expired hold restored: %+v", h)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &QuotaLedger{}
	if err := c.Persist(path, now); err != nil {
		t.Fatalf("a corrupt file must not block boot: %v", err)
	}
	c.Mark(lunaMember, now.Add(time.Hour)) // and the ledger still works and rewrites it
	d := &QuotaLedger{}
	_ = d.Persist(path, now)
	if len(d.Holds(now)) != 1 {
		t.Fatalf("holds after a corrupt file = %+v", d.Holds(now))
	}
}

func TestLedgerStateFileIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	now := time.Now()
	a := &QuotaLedger{}
	_ = a.Persist(path, now)
	for i := 0; i < maxPersistedHolds*2; i++ {
		a.Mark(provideriface.LLMSelection{Provider: "p", Model: fmt.Sprintf("m%d", i)}, now.Add(time.Duration(i+1)*time.Minute))
	}
	b := &QuotaLedger{}
	_ = b.Persist(path, now)
	if n := len(b.Holds(now)); n != maxPersistedHolds {
		t.Fatalf("restored %d holds, want the bound %d", n, maxPersistedHolds)
	}
}

func TestHardFailureHoldIsLongDoublesAndEndsOnSuccess(t *testing.T) {
	l := NewQuotaLedger()
	now := time.Now()
	want := []time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour, 6 * time.Hour, 6 * time.Hour}
	for i, w := range want {
		if got := l.DemoteHard(synthMember, now).Sub(now); got != w {
			t.Fatalf("hard failure %d holds %s, want %s", i+1, got, w)
		}
	}
	if h := l.Holds(now); len(h) != 1 || h[0].Source != QuotaHoldHardFailure {
		t.Fatalf("holds = %+v", h)
	}
	// A transient failure never shortens a hard hold.
	if until := l.Demote(synthMember, now); until.Sub(now) != 6*time.Hour {
		t.Fatalf("transient demotion shortened a hard hold to %s", until.Sub(now))
	}
	l.Succeed(synthMember)
	if len(l.Holds(now)) != 0 {
		t.Fatal("a success must end a hard hold")
	}
	if got := l.DemoteHard(synthMember, now).Sub(now); got != 15*time.Minute {
		t.Fatalf("hard strikes survived a success: %s", got)
	}
}

func TestIsHardProviderFailureNamesBadKeysAndGoneModelsOnly(t *testing.T) {
	for _, hard := range []string{
		"gateway client: provider zai model glm: status 401 Unauthorized",
		"gateway client: model_not_found: the model gpt-x does not exist",
	} {
		if !IsHardProviderFailure(fmt.Errorf("%s", hard)) {
			t.Errorf("%q should be a hard failure", hard)
		}
	}
	for _, soft := range []string{"gateway client: status 403 Forbidden (sanitized)", "gateway client: status 404 Not Found (sanitized)", "gateway client: status 503 Service Unavailable", "circuit open (model unhealthy)", "status 429 Too Many Requests", "deadline exceeded", "status 500 after 4040 ms"} {
		if IsHardProviderFailure(fmt.Errorf("%s", soft)) {
			t.Errorf("%q must not be a hard failure", soft)
		}
	}
}

func TestGoneModelIsHeldLongAndNotAskedByTheNextRun(t *testing.T) {
	p := &poolProvider{errs: map[string][]error{"synthetic": {&gatewayLikeError{"gateway client: model_not_found: the model does not exist", true}}}}
	ledger := NewQuotaLedger()
	for run := 1; run <= 3; run++ {
		if text, err := runWithLedger(p, ledger, synthMember, lunaMember); err != nil || text != "done by chatgpt" {
			t.Fatalf("run %d = %q, %v", run, text, err)
		}
	}
	if p.count("synthetic") != 1 {
		t.Fatalf("synthetic asked %d times over three runs, want once", p.count("synthetic"))
	}
	h := ledger.Holds(time.Now())
	if len(h) != 1 || h[0].Source != QuotaHoldHardFailure || time.Until(h[0].Until) < 14*time.Minute {
		t.Fatalf("holds = %+v, want one long hard-failure hold", h)
	}
}

func TestIncidentDoesNotReAskAMemberWhoseResetIsInTheFuture(t *testing.T) {
	clock := useFakeQuotaClock(t, 0)
	// Both members are out of quota: synthetic back in a minute, luna in an
	// hour. The incident waits for the minute, then asks only synthetic.
	p := &poolProvider{errs: map[string][]error{
		"synthetic": {quotaErrFor("synthetic", clock.Now().Add(time.Minute)), nil},
		"chatgpt":   {quotaErrFor("chatgpt", clock.Now().Add(time.Hour))},
	}}
	text, err := runStamped(p)
	if err != nil || text != "done by synthetic" {
		t.Fatalf("run = %q, %v", text, err)
	}
	if p.count("chatgpt") != 1 {
		t.Fatalf("chatgpt asked %d times, want once: its reset was an hour away", p.count("chatgpt"))
	}
}
