package trust

// Failure modes (MA7.10 part 2), written first:
//  J1 the strongest judge fails or is unwired and a weaker one settles in its
//     place: the anchor then claims a strength it does not have. The top
//     configured strength must answer, or nothing settles.
//  J2 a judge's free text counts as a vote: an answer outside the question's
//     closed set is an error, never a vote.
//  J3 a split at the top strength settles by order of arrival: no anchor.
//  J4 the judge prompt tells the judge it is checking a scorer or a sampled
//     record: it must carry no spot-check marker, and the seat calls through
//     the gateway-backed provider only for a catalogued (provider, model).
//  J5 the sampler log carries the seed, or drifts from the sampler on replay:
//     a replay recomputes every draw and names each mismatch.
//  J6 a torn or edited log line is skipped silently: loading refuses it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

type fakeJudge struct {
	name     string
	strength int
	answer   string
	err      error
	asked    *int
}

func (f fakeJudge) Name() string  { return f.name }
func (f fakeJudge) Strength() int { return f.strength }
func (f fakeJudge) Judge(_ context.Context, _ JudgeQuestion) (string, error) {
	if f.asked != nil {
		*f.asked++
	}
	return f.answer, f.err
}

var jq = JudgeQuestion{ItemID: "item-1", Prompt: "Is the claim supported by the cited source?", Answers: []string{"yes", "no"}}

func TestSettleByJudgesStrongestAnswersOrNothingSettles(t *testing.T) {
	ctx := context.Background()
	// the strongest wins over a weaker majority
	a, err := SettleByJudges(ctx, jq, AnchorSpotCheck, []Judge{
		fakeJudge{name: "weak1", strength: 1, answer: "no"},
		fakeJudge{name: "weak2", strength: 1, answer: "no"},
		fakeJudge{name: "strong", strength: 3, answer: "yes"},
	}, t0)
	if err != nil || a.Truth != "yes" || a.Kind != AnchorSpotCheck || a.ItemID != "item-1" {
		t.Fatalf("anchor %+v err %v", a, err)
	}
	// J1: the strongest errors; the weaker must not settle in its place
	_, err = SettleByJudges(ctx, jq, AnchorSpotCheck, []Judge{
		fakeJudge{name: "weak", strength: 1, answer: "yes"},
		fakeJudge{name: "strong", strength: 3, err: errors.New("unavailable")},
	}, t0)
	if !errors.Is(err, ErrUnsettled) {
		t.Fatalf("a weaker judge settled in the strongest's place: %v", err)
	}
	// J2: an answer outside the closed set is an error, not a vote
	_, err = SettleByJudges(ctx, jq, AnchorSpotCheck, []Judge{fakeJudge{name: "strong", strength: 3, answer: "probably"}}, t0)
	if !errors.Is(err, ErrUnsettled) {
		t.Fatalf("free text counted as a vote: %v", err)
	}
	// J3: a split at the top strength settles nothing
	_, err = SettleByJudges(ctx, jq, AnchorSpotCheck, []Judge{
		fakeJudge{name: "a", strength: 3, answer: "yes"},
		fakeJudge{name: "b", strength: 3, answer: "no"},
	}, t0)
	if !errors.Is(err, ErrUnsettled) {
		t.Fatalf("a split panel settled: %v", err)
	}
	// no judges, and an empty answer set, settle nothing
	if _, err = SettleByJudges(ctx, jq, AnchorSpotCheck, nil, t0); !errors.Is(err, ErrUnsettled) {
		t.Fatal(err)
	}
	if _, err = SettleByJudges(ctx, JudgeQuestion{ItemID: "x"}, AnchorSpotCheck, []Judge{fakeJudge{strength: 1, answer: "yes"}}, t0); err == nil {
		t.Fatal("a question with no closed answer set is not askable")
	}
	// a weaker judge is not asked when it cannot change the outcome
	n := 0
	if _, err = SettleByJudges(ctx, jq, AnchorSpotCheck, []Judge{
		fakeJudge{name: "weak", strength: 1, answer: "no", asked: &n},
		fakeJudge{name: "strong", strength: 3, answer: "yes"},
	}, t0); err != nil || n != 0 {
		t.Fatalf("weaker judge asked %d times, err %v", n, err)
	}
}

type fakeCaller struct {
	text string
	err  error
	reqs []provideriface.ToolLoopRequest
}

func (f *fakeCaller) CallWithTools(_ context.Context, r provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	f.reqs = append(f.reqs, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provideriface.ToolLoopResponse{Text: f.text}, nil
}

func TestProviderJudgeCallsTheGatewayWithNoMarker(t *testing.T) {
	ctx := context.Background()
	c := &fakeCaller{text: " Yes. "}
	j := NewProviderJudge("deepseek", "synthetic", "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", 2, c)
	got, err := j.Judge(ctx, jq)
	if err != nil || got != "yes" {
		t.Fatalf("got %q err %v", got, err)
	}
	// J4: the prompt carries the question and the closed set, never a marker
	all := strings.ToLower(c.reqs[0].System + string(c.reqs[0].Messages[0]))
	for _, bad := range []string{"spot", "audit", "anchor", "sampled", "grading", "scorer"} {
		if strings.Contains(all, bad) {
			t.Fatalf("the judge prompt names %q", bad)
		}
	}
	if !strings.Contains(all, "is the claim supported") || c.reqs[0].Provider != "synthetic" {
		t.Fatalf("request %+v", c.reqs[0])
	}
	// a JSON answer is read; prose around a bare word is not a vote
	c.text = `{"answer":"no"}`
	if got, err = j.Judge(ctx, jq); err != nil || got != "no" {
		t.Fatalf("got %q err %v", got, err)
	}
	c.text = "I think the answer is yes because of the source."
	if _, err = j.Judge(ctx, jq); err == nil {
		t.Fatal("prose is not a typed answer")
	}
	// an uncatalogued model and a nil caller never answer
	if _, err = NewProviderJudge("x", "synthetic", "no-such-model", 2, c).Judge(ctx, jq); err == nil {
		t.Fatal("an uncatalogued model answered")
	}
	if _, err = NewProviderJudge("x", "synthetic", "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", 2, nil).Judge(ctx, jq); err == nil {
		t.Fatal("a judge with no provider answered")
	}
	c.err = errors.New("gateway down")
	c.text = "yes"
	if _, err = j.Judge(ctx, jq); err == nil {
		t.Fatal("a provider error must surface")
	}
}

func TestSpotLogReplaysAndRefusesCorruption(t *testing.T) {
	seed := []byte("0123456789abcdef-host-secret")
	s, _ := NewSpotSampler(seed, 0.3)
	path := filepath.Join(t.TempDir(), "spot.jsonl")
	log, err := OpenSpotLog(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if err := log.Append(s.Decide("rec-"+itoa(i), t0)); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), string(seed)) {
		t.Fatal("J5: the log carries the seed")
	}
	entries, err := LoadSpotLog(path)
	if err != nil || len(entries) != 50 {
		t.Fatalf("%d entries, err %v", len(entries), err)
	}
	if bad := VerifySpotLog(s, entries); len(bad) != 0 {
		t.Fatalf("replay mismatches: %v", bad)
	}
	// J5: a log under another seed, or an edited sampled flag, is named
	other, _ := NewSpotSampler([]byte("another-host-secret-xyz"), 0.3)
	if bad := VerifySpotLog(other, entries); len(bad) == 0 {
		t.Fatal("a different seed replayed clean")
	}
	edited := append([]SpotDecision(nil), entries...)
	edited[7].Sampled = !edited[7].Sampled
	if bad := VerifySpotLog(s, edited); len(bad) != 1 || !strings.Contains(bad[0], "rec-7") {
		t.Fatalf("edited entry: %v", bad)
	}
	// J6: a torn final line is a torn tail (R1), a bad middle line refuses
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"record_id":"rec-torn","sam`)
	f.Close()
	if es, err := LoadSpotLog(path); err != nil || len(es) != 50 {
		t.Fatalf("a torn tail: %d %v", len(es), err)
	}
	// a missing log is an empty one, and entries round-trip as JSON
	if es, err := LoadSpotLog(filepath.Join(t.TempDir(), "none")); err != nil || len(es) != 0 {
		t.Fatal(es, err)
	}
	if _, err := json.Marshal(entries[0]); err != nil {
		t.Fatal(err)
	}
}

// Review r1 (MA7.10), written first:
//  R1 a torn tail kills sampling: an unterminated final line that does not
//     parse is a torn tail, ignored on read and truncated on open; only a bad
//     line in the middle refuses. An older build's log (unknown fields) loads.
//  R2 a log under another seed or rate is stale, not tampered: it is named
//     stale and rotated aside; a log whose draw does not reproduce is not.
//  R3 a transient judge failure is final: it must be told apart from a split,
//     an answer outside the closed set or prose, which are final.
//  R4 the conversation id names the item.

func TestSpotLogTornTailIsIgnoredOnReadAndTruncatedOnOpen(t *testing.T) {
	seed := []byte("0123456789abcdef-host-secret")
	s, _ := NewSpotSampler(seed, 0.3)
	path := filepath.Join(t.TempDir(), "spot.jsonl")
	log, _ := OpenSpotLog(path)
	for i := 0; i < 5; i++ {
		_ = log.Append(s.Decide("rec-"+itoa(i), t0))
	}
	_ = log.Close()
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"record_id":"rec-torn","sam`)
	f.Close()
	es, err := LoadSpotLog(path)
	if err != nil || len(es) != 5 {
		t.Fatalf("R1: a torn tail refused the read: %d entries, %v", len(es), err)
	}
	log, err = OpenSpotLog(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = log.Append(s.Decide("rec-5", t0))
	_ = log.Close()
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "torn") || !strings.HasSuffix(string(raw), "\n") {
		t.Fatalf("R1: the torn tail survived the open: %q", raw)
	}
	if es, err = LoadSpotLog(path); err != nil || len(es) != 6 {
		t.Fatalf("after the open: %d %v", len(es), err)
	}
	// a valid last line with no newline is kept, and the next append starts a new line
	os.WriteFile(path, []byte(`{"record_id":"a","sampled":false,"draw":0.5,"rate":0.3,"seed_id":"x"}`), 0o600)
	log, _ = OpenSpotLog(path)
	_ = log.Append(s.Decide("b", t0))
	_ = log.Close()
	if es, err = LoadSpotLog(path); err != nil || len(es) != 2 {
		t.Fatalf("unterminated valid tail: %d %v", len(es), err)
	}
	// a bad line in the middle still refuses; an unknown field does not
	os.WriteFile(path, []byte("{\"record_id\":\"a\"}\nnot json\n{\"record_id\":\"b\"}\n"), 0o600)
	if _, err = LoadSpotLog(path); err == nil {
		t.Fatal("R1: a corrupt middle line loaded")
	}
	os.WriteFile(path, []byte("{\"record_id\":\"a\",\"from_a_newer_build\":1}\n"), 0o600)
	if es, err = LoadSpotLog(path); err != nil || len(es) != 1 {
		t.Fatalf("R1: an unknown field refused the log: %v", err)
	}
}

func TestSpotLogStaleIsSeedOrRateAndRotatesAside(t *testing.T) {
	seed := []byte("0123456789abcdef-host-secret")
	s, _ := NewSpotSampler(seed, 0.3)
	otherSeed, _ := NewSpotSampler([]byte("another-host-secret-xyz"), 0.3)
	otherRate, _ := NewSpotSampler(seed, 0.2)
	es := []SpotDecision{s.Decide("a", t0), s.Decide("b", t0)}
	if SpotLogStale(s, es) || !SpotLogStale(otherSeed, es) || !SpotLogStale(otherRate, es) {
		t.Fatal("R2: stale means another seed or another rate")
	}
	if bad := VerifySpotLog(otherRate, es); len(bad) == 0 {
		t.Fatal("R2: a log at another rate replayed clean")
	}
	tampered := append([]SpotDecision(nil), es...)
	tampered[0].Sampled = !tampered[0].Sampled
	if SpotLogStale(s, tampered) {
		t.Fatal("R2: a tampered flag is not staleness")
	}
	path := filepath.Join(t.TempDir(), "spot.jsonl")
	l, _ := OpenSpotLog(path)
	_ = l.Append(es[0])
	_ = l.Close()
	aside, err := RotateSpotLog(path, t0)
	if err != nil || !strings.Contains(aside, ".stale-") {
		t.Fatalf("%q %v", aside, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the stale log is still in place")
	}
	if got, err := LoadSpotLog(aside); err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestJudgeFailuresAreToldApartAndTheConversationIdIsOpaque(t *testing.T) {
	ctx := context.Background()
	_, err := SettleByJudges(ctx, jq, AnchorSpotCheck, []Judge{fakeJudge{name: "s", strength: 3, err: errors.New("down")}}, t0)
	if !errors.Is(err, ErrUnsettled) || !errors.Is(err, ErrJudgeUnavailable) {
		t.Fatalf("R3: %v", err)
	}
	for name, js := range map[string][]Judge{
		"outside the set": {fakeJudge{name: "s", strength: 3, answer: "maybe"}},
		"split":           {fakeJudge{name: "a", strength: 3, answer: "yes"}, fakeJudge{name: "b", strength: 3, answer: "no"}},
		"prose":           {fakeJudge{name: "s", strength: 3, err: fmt.Errorf("%w: prose", ErrBadAnswer)}},
	} {
		_, err := SettleByJudges(ctx, jq, AnchorSpotCheck, js, t0)
		if !errors.Is(err, ErrUnsettled) || errors.Is(err, ErrJudgeUnavailable) {
			t.Fatalf("R3: %s is final, not transient: %v", name, err)
		}
	}
	c := &fakeCaller{text: "prose around yes"}
	j := NewProviderJudge("deepseek", "synthetic", "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", 2, c)
	if _, err := j.Judge(ctx, jq); !errors.Is(err, ErrBadAnswer) {
		t.Fatalf("R3: %v", err)
	}
	if strings.Contains(c.reqs[0].ConversationID, "item-1") {
		t.Fatalf("R4: the conversation id names the item: %s", c.reqs[0].ConversationID)
	}
	// a cancelled context is transient
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = SettleByJudges(cctx, jq, AnchorSpotCheck, []Judge{fakeJudge{name: "s", strength: 3, answer: "yes"}}, t0)
	if !errors.Is(err, ErrJudgeUnavailable) {
		t.Fatalf("R3: a cancelled context settled or was final: %v", err)
	}
}

// Review r2 (MA7.10), written first:
//  Q1 a duplicated decision for one record id is flagged by the replay;
//     deferral records (an item the judge could not answer yet) may repeat.
//  Q2 an unreadable log and a corrupt one are told apart.
//  Q3 two rotations in the same second do not collide.

func TestSpotLogFlagsADuplicateDecisionButNotDeferrals(t *testing.T) {
	s, _ := NewSpotSampler([]byte("0123456789abcdef-host-secret"), 0.3)
	d := s.Decide("a", t0)
	if bad := VerifySpotLog(s, []SpotDecision{d, d}); len(bad) != 1 || !strings.Contains(bad[0], "twice") {
		t.Fatalf("Q1: %v", bad)
	}
	def1, def2 := d, d
	def1.Deferral, def2.Deferral = 1, 2
	if bad := VerifySpotLog(s, []SpotDecision{def1, def2, d}); len(bad) != 0 {
		t.Fatalf("Q1: deferrals are not duplicates: %v", bad)
	}
}

func TestSpotLogCorruptionIsATypedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spot.jsonl")
	os.WriteFile(path, []byte("{\"record_id\":\"a\"}\nnot json\n{\"record_id\":\"b\"}\n"), 0o600)
	if _, err := LoadSpotLog(path); !errors.Is(err, ErrSpotLogCorrupt) {
		t.Fatalf("Q2: %v", err)
	}
	if _, err := LoadSpotLog(t.TempDir()); err == nil || errors.Is(err, ErrSpotLogCorrupt) {
		t.Fatalf("Q2: a directory is an IO error, not corruption: %v", err)
	}
}

func TestRotationsInTheSameSecondDoNotCollide(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spot.jsonl")
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		os.WriteFile(path, []byte("{\"record_id\":\"a\"}\n"), 0o600)
		aside, err := RotateSpotLog(path, t0)
		if err != nil || seen[aside] {
			t.Fatalf("Q3: %q %v", aside, err)
		}
		seen[aside] = true
	}
}
