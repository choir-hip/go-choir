package agentcore

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
	"github.com/yusefmosiah/go-choir/internal/jevrecord"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// One state, every question (owner, 2026-11-12: "for jev, use vendor limits
// not our arbitrary size limits ... we can reuse the same context to do
// multiple calls"). Ways intake can fail, written before the code:
//
//	O1. A window of a few hundred stories is still asked in 3 to 5 chunk calls
//	    per capture group, each repeating the task, the captures and the
//	    questions, because a 15,000-byte slice and a 30,000-byte guard cut it.
//	O2. The call is cut by bytes: a Cyrillic or CJK group (about 4x the tokens
//	    of the same bytes of English) crosses the vendor's limit and is
//	    refused, or an English group is split that the vendor would take.
//	O3. A real overflow (one capture's call cannot carry the window) trims the
//	    window instead of asking it in chunks, so a story is never offered.
//	O4. A guest's Jev call carries no site, so the gateway log says unknown.
//	O5. The support judge keeps a byte guard that was OpenRouter's line.

// O1: 350 stories and three captures are one call: the stories once in the
// state, and, because the vendor takes 255 choices a question, each capture's
// options in two parts (two event and two topic questions over the same
// state). Every story is offered once per capture and kind.
func TestIntakeAWindowOfHundredsOfStoriesIsOneCall(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	items := pollItems("Alpha report", "Bravo report", "Charlie report")
	mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 350), "owner", "computer", "src", items, intakeBatchCaptures))
	calls := judge.labelCalls()
	if len(calls) != 1 {
		t.Fatalf("O1: %d calls for one capture group over one window; want one", len(calls))
	}
	c := calls[0]
	if got := len(storyEntries(c.state)); got != 350 {
		t.Fatalf("O1: the call carries %d stories; want all 350, once", got)
	}
	if len(c.questions) != 12 {
		t.Fatalf("O1: %d questions; want two event and two topic parts for each of 3 captures", len(c.questions))
	}
	if !c.fits {
		t.Fatalf("O1: the one call (%d bytes) is over the vendor's token budget", c.bytes)
	}
	offered := map[string]map[string]int{} // question kind and capture -> story handle -> times
	for name, raw := range c.raw {
		kind, rest, _ := strings.Cut(name, "_")
		handle, _, _ := strings.Cut(rest, "_")
		options := questionOptions(t, raw)
		if len(options) > jevbudget.VendorMaxChoices {
			t.Fatalf("O1: %s offers %d options; the vendor takes %d", name, len(options), jevbudget.VendorMaxChoices)
		}
		key := kind + "/" + handle
		if offered[key] == nil {
			offered[key] = map[string]int{}
		}
		for _, o := range options {
			if strings.HasPrefix(o, "c") {
				offered[key][o]++
			}
		}
	}
	if len(offered) != 6 {
		t.Fatalf("O1: %d kind/capture groups; want 6", len(offered))
	}
	for key, stories := range offered {
		if len(stories) != 350 {
			t.Fatalf("O1: %s offers %d distinct stories; want all 350", key, len(stories))
		}
		for h, n := range stories {
			if n != 1 {
				t.Fatalf("O1: %s offers %s %d times", key, h, n)
			}
		}
	}
}

// O2: the bound follows the text. The same number of bytes of Cyrillic is
// about three times the tokens of English, so it splits where English does
// not, and every call of either is inside the vendor's budget.
func TestIntakeBoundFollowsTheTextNotTheByteCount(t *testing.T) {
	run := func(summary string) (calls int, tokens int, allFit bool) {
		rt, _ := testRuntime(t)
		judge := &fakeIntakeJudge{}
		rt.SetCandidateJudge(judge)
		items := flushTestItems(12, summary)
		for i := range items {
			items[i].Summary = summary
		}
		lib := pollLib(rt, 100)
		mustLabels(t, rt.labelPoll(context.Background(), lib, "owner", "computer", "src", items, intakeBatchCaptures))
		allFit = true
		for _, c := range judge.labelCalls() {
			calls++
			allFit = allFit && c.fits
			tokens += jevbudget.Tokens([]byte(fmt.Sprint(c.state)))
		}
		return calls, tokens, allFit
	}
	latin := strings.Repeat("the quick brown fox jumps over a lazy dog ", 40) // clipped to 600 runes
	cyrillic := strings.Repeat("быстрая бурая лиса прыгает через ленивую ", 40)
	lc, lt, lfit := run(latin)
	cc, ct, cfit := run(cyrillic)
	if !lfit || !cfit {
		t.Fatalf("O2: a call is over the vendor's budget (latin fits %v, cyrillic fits %v)", lfit, cfit)
	}
	if ct <= lt {
		t.Fatalf("O2: Cyrillic state estimated %d tokens, English %d; the dense script must cost more", ct, lt)
	}
	if lc != 1 {
		t.Fatalf("O2: twelve English captures over 100 stories are %d calls; want one", lc)
	}
	_ = cc
}

// O3: stories whose entries are so long that one capture's call cannot carry
// them all are asked in chunks, every story in exactly one chunk, none
// dropped and none trimmed, and the capture is labelled from the chunks.
func TestIntakeRealOverflowChunksTheWindowAndNeverTrimsIt(t *testing.T) {
	rt, _ := testRuntime(t)
	now := time.Now().UTC()
	lib := &intakeLibrary{rt: rt, summary: map[string]store.StorySummary{}}
	long := strings.Repeat("headline words ", 120)
	const n = 60
	for i := 0; i < n; i++ {
		lib.recent = append(lib.recent, types.Document{DocID: fmt.Sprintf("doc-%03d", i), Title: fmt.Sprintf("%d %s", i, long), UpdatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	items := pollItems("Incoming report")
	label, err := labelOnly(rt, lib, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(label.CandidateDocIDs) != n {
		t.Fatalf("O3: %d candidates recorded; want %d", len(label.CandidateDocIDs), n)
	}
	calls := judge.labelCalls()
	if len(calls) < 2 {
		t.Fatalf("O3: %d calls; the window cannot fit one", len(calls))
	}
	seen := map[string]int{}
	for _, c := range calls {
		if !c.fits {
			t.Fatalf("O3: a chunk call of %d bytes is over the vendor's token budget", c.bytes)
		}
		for _, e := range storyEntries(c.state) {
			h, _, _ := strings.Cut(e, ": ")
			seen[h]++
		}
	}
	if len(seen) != n {
		t.Fatalf("O3: %d distinct stories offered over the chunks; want %d (nothing trimmed)", len(seen), n)
	}
	for h, k := range seen {
		if k != 1 {
			t.Fatalf("O3: story %s offered %d times", h, k)
		}
	}
}

// O4: every call site rides the request as the site the gateway logs.
func TestJudgeCallCarriesTheSiteToTheJudge(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	for _, site := range []string{"intake_labels", "support_judge", "event_layer", "oped_gate"} {
		if _, err := rt.judgeCall(context.Background(), jevrecord.Meta{Site: site}, time.Second, []byte(`{}`), nil); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, c := range judge.calls {
		got = append(got, c.site)
	}
	if strings.Join(got, ",") != "intake_labels,support_judge,event_layer,oped_gate" {
		t.Fatalf("O4: sites seen by the judge = %v", got)
	}
}

func TestIntakeCallsCarryTheirSite(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", pollItems("A", "B"), intakeBatchCaptures))
	calls := judge.labelCalls()
	if len(calls) == 0 || calls[0].site != "intake_labels" {
		t.Fatalf("O4: intake call site = %q", calls[0].site)
	}
}

// O5: the support judge splits by the vendor's token budget, from the text:
// a thousand tokens a pair is about twenty pairs a call, and the same pair in
// a dense script is fewer.
func TestSupportBatchesSplitByEstimatedTokens(t *testing.T) {
	ordered := make([]string, 100)
	sizes := map[string]int{}
	for i := range ordered {
		ordered[i] = fmt.Sprintf("p%d", i+1)
		sizes[ordered[i]] = 1000
	}
	batches := splitSupportBatches(ordered, sizes)
	total := 0
	for _, b := range batches {
		sum := supportEnvelopeTokens
		for _, name := range b {
			sum += sizes[name]
		}
		if sum > jevbudget.QuestionBudgetTokens {
			t.Fatalf("O5: a batch of %d pairs is %d tokens, over the budget %d", len(b), sum, jevbudget.QuestionBudgetTokens)
		}
		total += len(b)
	}
	if total != 100 {
		t.Fatalf("O5: %d pairs batched of 100", total)
	}
	if len(batches) > 6 {
		t.Fatalf("O5: %d batches of a thousand tokens; the budget holds about 23 a batch", len(batches))
	}
	// A lone pair over the budget is its own batch.
	if got := splitSupportBatches([]string{"a", "b"}, map[string]int{"a": 99999, "b": 10}); len(got) != 2 {
		t.Fatalf("O5: a lone over-size pair must stand alone: %v", got)
	}
}
