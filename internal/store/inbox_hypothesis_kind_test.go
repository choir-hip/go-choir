package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// L3-1 starts the clock (docs/definitions/choir-ledger-l3-hypotheses-
// settled-2026-10-13.md): a hypothesis may name its kind, assertion or
// forecast, with a criterion, and a forecast a resolves_by within 7 days of
// its commit. Ways it could fail, written before the code:
//   - K1 a legacy hypothesis (no kind) is refused or gains a kind: it must
//     commit unchanged and stay kind-less (outside the forecast cohort);
//   - K2 a well-formed forecast or assertion is refused, or its kind,
//     criterion and resolves_by are dropped from the head;
//   - K3 a forecast without resolves_by or criterion is admitted;
//   - K4 a resolves_by that is not RFC 3339, not after the commit, or more
//     than 7 days after it is admitted;
//   - K5 an assertion without a criterion, or with a resolves_by, is
//     admitted; an unknown kind is admitted;
//   - K6 a refusal does not name the hypothesis and the field, so
//     management cannot correct and retry;
//   - K7 the fold (replay, VerifyInboxHeads) applies the clock check: a
//     committed forecast whose resolves_by has since passed must fold and
//     verify unchanged, and the fold must project the same fields the
//     admission step does;
//   - K8 a malformed forecast in a story-opening Cast leaves a half-open
//     story: the whole command is refused, and a well-formed open commits;
//   - K9 a revert of L3-1 cannot read heads that carry the new fields: the
//     head decoder must ignore fields its struct does not declare, so the
//     reverted fold and VerifyInboxHeads (which compare decoded,
//     re-marshalled heads) still match.

func forecastHyp(statement string, resolvesBy time.Time) types.CastHypothesis {
	h := hyp(statement, 0.4)
	h.Kind, h.Criterion, h.ResolvesBy = types.HypothesisKindForecast, "the ministry publishes the figure", resolvesBy.UTC().Format(time.RFC3339)
	return h
}

func assertionHyp(statement string) types.CastHypothesis {
	h := hyp(statement, 0.8)
	h.Kind, h.Criterion = types.HypothesisKindAssertion, "an independent outlet reports it"
	return h
}

func TestHypothesisKindsAdmittedAndProjected(t *testing.T) {
	s := openTestStore(t)
	tex, res := "texture:kind", "research:kind"
	conv := types.InboxConversationID(tex, res)
	resolves := time.Now().Add(72 * time.Hour)
	mustCommitInbox(t, s, "cmd-kind-cast-1", nil, inboxCast("kind-cast-1", tex, res, types.CommitmentCast{
		Hypotheses: []types.CastHypothesis{hyp("legacy", 0.6), forecastHyp("rate rises", resolves), assertionHyp("the plant closed")},
	}))
	states, err := s.HypothesisStates(context.Background(), inboxTestOwner, inboxTestComputer, conv)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]types.HypothesisState{}
	for _, st := range states {
		got[st.ID] = st
	}
	// K1: the legacy hypothesis stays kind-less.
	if st := got["H1"]; st.Kind != "" || st.Criterion != "" || st.ResolvesBy != "" {
		t.Fatalf("legacy hypothesis gained fields: %+v", st)
	}
	// K2: kind, criterion and resolves_by reach the head.
	if st := got["H2"]; st.Kind != types.HypothesisKindForecast || st.Criterion == "" || st.ResolvesBy != resolves.UTC().Format(time.RFC3339) {
		t.Fatalf("forecast state = %+v", st)
	}
	if st := got["H3"]; st.Kind != types.HypothesisKindAssertion || st.Criterion == "" || st.ResolvesBy != "" {
		t.Fatalf("assertion state = %+v", st)
	}
	// K7: the fold projects exactly what admission did.
	assertInboxHeadsMatchRebuild(t, s)
}

func TestHypothesisKindRefusals(t *testing.T) {
	s := openTestStore(t)
	tex, res := "texture:kindbad", "research:kindbad"
	now := time.Now()
	refuse := func(cmd string, h types.CastHypothesis, field string) {
		t.Helper()
		_, err := commitInbox(t, s, cmd, nil, inboxCast(cmd, tex, res, types.CommitmentCast{Hypotheses: []types.CastHypothesis{hyp("fine", 0.5), h}}))
		if err == nil || !strings.Contains(err.Error(), "hypothesis 1") || !strings.Contains(err.Error(), field) {
			t.Fatalf("%s: want a refusal naming hypothesis 1 and %q, got %v", cmd, field, err)
		}
	}
	noDate := forecastHyp("x", now.Add(time.Hour))
	noDate.ResolvesBy = ""
	refuse("kind-no-date", noDate, "resolves_by") // K3
	noCriterion := forecastHyp("x", now.Add(time.Hour))
	noCriterion.Criterion = ""
	refuse("kind-no-criterion", noCriterion, "criterion") // K3
	badDate := forecastHyp("x", now.Add(time.Hour))
	badDate.ResolvesBy = "next tuesday"
	refuse("kind-bad-date", badDate, "resolves_by")                                   // K4
	refuse("kind-past-date", forecastHyp("x", now.Add(-time.Hour)), "resolves_by")    // K4
	refuse("kind-far-date", forecastHyp("x", now.Add(8*24*time.Hour)), "resolves_by") // K4
	bareAssertion := assertionHyp("x")
	bareAssertion.Criterion = ""
	refuse("kind-assertion-no-criterion", bareAssertion, "criterion") // K5
	datedAssertion := assertionHyp("x")
	datedAssertion.ResolvesBy = now.Add(time.Hour).UTC().Format(time.RFC3339)
	refuse("kind-assertion-dated", datedAssertion, "resolves_by") // K5
	unknown := assertionHyp("x")
	unknown.Kind = "prediction"
	refuse("kind-unknown", unknown, "kind") // K5
	kindless := hyp("x", 0.5)
	kindless.Criterion = "a filing appears"
	refuse("kind-missing-with-criterion", kindless, "no kind") // K5
	// Nothing was committed by the refusals.
	assertInboxHeadsMatchRebuild(t, s)
}

func TestHypothesisClockIsAdmissionOnly(t *testing.T) {
	commit := time.Date(2026, 10, 15, 22, 0, 0, 0, time.UTC)
	h := forecastHyp("rate rises", commit.Add(48*time.Hour))
	// Admission at its commit time accepts it; the same check run days later
	// would refuse it, which is why the fold must never run it (K7).
	if err := checkHypothesisClock("cast-1", 0, h, commit); err != nil {
		t.Fatalf("at commit: %v", err)
	}
	if err := checkHypothesisClock("cast-1", 0, h, commit.Add(5*24*time.Hour)); err == nil {
		t.Fatal("the clock check is time-relative; it must refuse a resolves_by already past")
	}
	// K4 boundaries: equal to the commit is refused, exactly 7 days is
	// admitted, a second past it is refused.
	for _, c := range []struct {
		at   time.Time
		want bool
	}{{commit, false}, {commit.Add(time.Second), true}, {commit.Add(types.HypothesisForecastHorizon), true}, {commit.Add(types.HypothesisForecastHorizon + time.Second), false}} {
		b := forecastHyp("edge", c.at)
		if err := checkHypothesisClock("cast-edge", 0, b, commit); (err == nil) != c.want {
			t.Fatalf("resolves_by %s at commit %s: err %v, want admitted %v", c.at.Format(time.RFC3339), commit.Format(time.RFC3339), err, c.want)
		}
	}
	h.ID = "H1"
	rec := inboxCast("cast-1", "texture:clk", "research:clk", types.CommitmentCast{Hypotheses: []types.CastHypothesis{h}})
	rec.Round = 1
	head := foldConversation(types.InboxConversationID("texture:clk", "research:clk"), []types.CommitmentRecord{rec})
	if st := head.Hypotheses["H1"]; st.Kind != types.HypothesisKindForecast || st.ResolvesBy != h.ResolvesBy || st.Criterion != h.Criterion {
		t.Fatalf("fold of a past forecast = %+v", st)
	}
}

func TestStoryOpenWithMalformedForecastIsWhollyRefused(t *testing.T) {
	s := openTestStore(t)
	bad := forecastHyp("x", time.Now().Add(time.Hour))
	bad.ResolvesBy = ""
	open := func(cmd string, h types.CastHypothesis) error {
		rec := inboxCast(cmd, "management:"+inboxTestOwner, types.StoryOpenInbox, types.CommitmentCast{
			Hypotheses: []types.CastHypothesis{h}, Captures: []string{"capture-k"}, Publish: types.CastPublishAuto})
		rec.Directive.Objective = "Rates decision " + cmd
		_, err := commitInbox(t, s, cmd, nil, rec)
		return err
	}
	if err := open("open-bad", bad); err == nil || !strings.Contains(err.Error(), "resolves_by") {
		t.Fatalf("story open with a malformed forecast: %v", err)
	}
	// Nothing of the refused command was stored: sending it again is
	// refused again, not replayed.
	if err := open("open-bad", bad); err == nil || !strings.Contains(err.Error(), "resolves_by") {
		t.Fatalf("resent malformed open: %v", err)
	}
	// The clock check sees story-opening Casts after planStoryOpenings
	// rewrites them: a past resolves_by refuses the open too.
	if err := open("open-past", forecastHyp("x", time.Now().Add(-time.Hour))); err == nil || !strings.Contains(err.Error(), "not after the commit") {
		t.Fatalf("story open with a past resolves_by: %v", err)
	}
	if err := open("open-good", forecastHyp("rate rises", time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("well-formed open: %v", err)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestHeadDecodeIgnoresFieldsAFutureOrRevertedStructLacks(t *testing.T) {
	// A head written by L3-1 code, read by code that predates it: stand in
	// for the older struct with a field this build does not declare.
	head := types.ConversationHead{ConversationID: "c-k9", Round: 1, Hypotheses: map[string]types.HypothesisState{
		"H1": {ID: "H1", Statement: "rate rises", Probability: 0.4, Kind: types.HypothesisKindForecast, Status: types.HypothesisOpen},
	}}
	raw, err := json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	generic["hypotheses"].(map[string]any)["H1"].(map[string]any)["field_from_another_build"] = "x"
	body, _ := json.Marshal(generic)
	got, err := decodeLifecycleObject[types.ConversationHead](objectgraph.Object{Body: body})
	if err != nil {
		t.Fatalf("a head with an undeclared field must decode: %v", err)
	}
	again, _ := json.Marshal(got)
	if string(again) != string(raw) {
		t.Fatalf("re-marshalled head = %s, want %s", again, raw)
	}
}
