package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Failure modes of the judges' serial form (docs/design/jev-batched-
// questions-2026-11-10.md, section 7). Owner rule, 2026-11-10: never call Jev
// concurrently when one call with more questions would do. Each site gets a
// fake judge that records the most calls in flight and the call count:
//   - the event layer prefetches chunks
//     (TestEventLayerChunksAreAskedOneAtATime);
//   - a later chunk is asked without the events an earlier chunk minted
//     (TestEventLayerLaterChunkIsAskedAfterTheEarlierCommit);
//   - the sub-call, cast and inert judges share a gate of one
//     (TestSubCallGateAdmitsOneJudgeCallAtATime) and over a batch run in
//     order (TestSubCallRoundOverTheBatchRunsOneCallAtATime,
//     TestDeskCastRoundOverTheBatchRunsOneCallAtATime,
//     TestInertTrayOverTheBatchRunsOneCallAtATime);
//   - support revisions are judged together, a queued revision judges a pair
//     an earlier job already judged, or a revision over the byte bound goes
//     out in one call (TestSupportRevisionsAreJudgedOneAtATime,
//     TestSupportRevisionQueuedBehindOneThatJudgedItsPairCallsNobody,
//     TestSupportRevisionOverTheByteBoundSplitsIntoSequentialCalls);
//   - story analysis asks its two chunks together
//     (TestStoryAnalysisChunksAreAskedOneAtATime);
//   - the scope a call names is dropped on the way to the gateway
//     (TestCallJudgeWithinPassesTheScope).

// peakJudge wraps a judge and records the most calls in flight at once.
type peakJudge struct {
	inner     CandidateJudge
	delay     time.Duration
	cur, peak atomic.Int32
	calls     atomic.Int32
}

func (j *peakJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	n := j.cur.Add(1)
	defer j.cur.Add(-1)
	for p := j.peak.Load(); n > p && !j.peak.CompareAndSwap(p, n); p = j.peak.Load() {
	}
	j.calls.Add(1)
	if j.delay > 0 {
		select {
		case <-time.After(j.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if j.inner == nil {
		return json.RawMessage(`{"model":"m","answers":{}}`), nil
	}
	return j.inner.Judge(ctx, state, questions)
}

func (j *peakJudge) assertSerial(t *testing.T, wantCalls int) {
	t.Helper()
	if got := int(j.calls.Load()); got != wantCalls {
		t.Fatalf("judge calls = %d, want %d", got, wantCalls)
	}
	if peak := j.peak.Load(); peak != 1 {
		t.Fatalf("most calls in flight = %d, want 1", peak)
	}
}

// heldJudge holds every call until the test releases it, so a test can
// assert that a second call never starts while the first is held. It records
// the most calls in flight.
type heldJudge struct {
	inner     CandidateJudge
	started   chan struct{}
	release   chan struct{}
	cur, peak atomic.Int32
	calls     atomic.Int32
}

func newHeldJudge(inner CandidateJudge) *heldJudge {
	return &heldJudge{inner: inner, started: make(chan struct{}, 256), release: make(chan struct{}, 256)}
}

func (j *heldJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	n := j.cur.Add(1)
	defer j.cur.Add(-1)
	for p := j.peak.Load(); n > p && !j.peak.CompareAndSwap(p, n); p = j.peak.Load() {
	}
	j.calls.Add(1)
	j.started <- struct{}{}
	select {
	case <-j.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if j.inner == nil {
		return json.RawMessage(`{"model":"m","answers":{}}`), nil
	}
	return j.inner.Judge(ctx, state, questions)
}

// waitStarted waits (bounded) for the next call to start.
func (j *heldJudge) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-j.started:
	case <-time.After(20 * time.Second):
		t.Fatal("the judge was never called")
	}
}

// assertNoSecondCall checks that no further call starts while one is held:
// the others have had time to try and are waiting.
func (j *heldJudge) assertNoSecondCall(t *testing.T) {
	t.Helper()
	select {
	case <-j.started:
		t.Fatal("a second call started while the first was held")
	case <-time.After(150 * time.Millisecond):
	}
	if cur := j.cur.Load(); cur != 1 {
		t.Fatalf("calls in flight = %d, want 1", cur)
	}
}

// drain releases and observes n held calls one at a time.
func (j *heldJudge) drain(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if i > 0 {
			j.waitStarted(t)
		}
		j.assertNoSecondCall(t)
		j.release <- struct{}{}
	}
	if peak := j.peak.Load(); peak != 1 {
		t.Fatalf("most calls in flight = %d, want 1", peak)
	}
	if got := int(j.calls.Load()); got != n {
		t.Fatalf("judge calls = %d, want %d", got, n)
	}
}

func TestSubCallRoundOfThirtyTwoCastsIsOneCall(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings for gamma"})
	// The judge is down while two fan-outs of 16 are cast, so their Casts
	// stay unsettled; then one round settles all 32 in one call (owner,
	// 2026-11-15: as many questions in the same call as possible).
	f.judge.err = errors.New("judge down")
	var ids []string
	for r := 0; r < 2; r++ {
		var children []yaegikernel.ResearchChild
		for i := 0; i < 16; i++ {
			children = append(children, yaegikernel.ResearchChild{Objective: fmt.Sprintf("gamma %d %d", r, i), Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)})
		}
		for _, res := range f.research(t, children) {
			ids = append(ids, res.CastID)
		}
	}
	f.rt.waitSubCallJudge()
	f.judge.mu.Lock()
	f.judge.err = nil
	f.judge.mu.Unlock()
	peak := &peakJudge{inner: f.judge, delay: 10 * time.Millisecond}
	f.rt.SetCandidateJudge(peak)
	if err := f.rt.settleSubCalls(context.Background(), f.owner, f.computer, "fan-serial", 0, ids); err != nil {
		t.Fatal(err)
	}
	if got := int(peak.calls.Load()); got != 1 {
		t.Fatalf("judge calls = %d for 32 Casts that fit the vendor's limits; want exactly 1", got)
	}
	for _, id := range ids {
		if !f.exists(types.SubCallSettlementRecordID(id, types.SubCallJudgeScorerID)) {
			t.Fatalf("cast %s was not settled", id)
		}
	}
}

func TestDeskCastPassOfTwentyFiveCastsIsOneCall(t *testing.T) {
	f := deskJudgeFixture(t)
	peak := &peakJudge{inner: f.judge, delay: 10 * time.Millisecond}
	f.rt.SetCandidateJudge(peak)
	var casts []types.CommitmentRecord
	for i := 0; i < 25; i++ {
		casts = append(casts, deskCast(fmt.Sprintf("dj-serial-%d", i), deskTex, deskRes, fmt.Sprintf("Question %d.", i), 0.6))
	}
	f.commitInbox(t, "dj-serial", nil, casts...)
	f.commitInbox(t, "dj-serial-final", f.readAll(t, deskRes), deskReport("dj-serial-final", deskRes, deskTex, "Answers to every question.", true))
	f.rt.waitSubCallJudge()
	if got := int(peak.calls.Load()); got != 1 {
		t.Fatalf("judge calls = %d for 25 Casts that fit the vendor's limits; want exactly 1", got)
	}
	for _, c := range casts {
		if !f.exists(judgeSettlementID(c.RecordID)) {
			t.Fatalf("cast %s was not settled", c.RecordID)
		}
	}
}

func TestInertTrayOfFortyThreeRecordsIsOneCall(t *testing.T) {
	rt, _ := testRuntime(t)
	peak := &peakJudge{delay: 10 * time.Millisecond}
	rt.SetCandidateJudge(peak)
	var recs []types.CommitmentRecord
	for i := 0; i < 43; i++ {
		recs = append(recs, types.CommitmentRecord{RecordID: fmt.Sprintf("inert-serial-%d", i), Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast}, EmitterKind: types.CommitmentEmitterAgent,
			Prediction: types.CommitmentPrediction{Hypothesis: "a prediction"}, PSuccess: fp(0.6), Weight: fp(0.5),
			Provenance: types.CommitmentProvenance{Desk: "management:o", Origin: types.OriginDeskRun}})
	}
	if err := rt.judgeInertness(context.Background(), "owner-in", "computer-in", recs); err != nil {
		t.Fatal(err)
	}
	if got := int(peak.calls.Load()); got != 1 {
		t.Fatalf("judge calls = %d for 43 records that fit the vendor's limits; want exactly 1", got)
	}
}

func TestEventLayerFlushOfThirtySixCapturesIsOneCall(t *testing.T) {
	// Every answer is omitted: no capture mints an event, and the 36 captures
	// cost exactly one call.
	judge := &eventJudge{answers: map[string]eventAnswer{}, omit: map[string]bool{}, delay: 20 * time.Millisecond}
	rt, owner, computer := eventRuntime(t, judge)
	var inputs []eventInput
	for i := 0; i < 36; i++ {
		title := fmt.Sprintf("Story number %d about topic", i)
		judge.omit[title] = true
		inputs = append(inputs, capItem(fmt.Sprintf("p%d", i), title, fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	judge.mu.Lock()
	peak := judge.maxIn
	judge.mu.Unlock()
	if judge.callCount() != 1 || r.Calls != 1 || peak != 1 {
		t.Fatalf("judge calls = %d, report calls = %d, most in flight = %d; want 1, 1 and 1", judge.callCount(), r.Calls, peak)
	}
}

// A flush of many chunks has no shared deadline: every chunk gets its own, so
// the last chunks are resolved, not dropped as unresolved.
func TestEventLayerLongFlushResolvesEveryChunk(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}, omit: map[string]bool{}}
	rt, owner, computer := eventRuntime(t, judge)
	var inputs []eventInput
	for i := 0; i < 450; i++ {
		title := fmt.Sprintf("Story number %d about topic", i)
		inputs = append(inputs, capItem(fmt.Sprintf("long%d", i), title, fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	// 450 captures are three calls under the 255-choice limit (199 each); the
	// 64k request limit forces about seven more (logged, named); none is left.
	if r.Calls < 3 || r.Calls > 20 || r.Unresolved != 0 {
		t.Fatalf("report = calls %d, unresolved %d; want 3 to 20 calls, 0", r.Calls, r.Unresolved)
	}
}

func TestEventLayerSiblingJoinNeedsNoLaterChunk(t *testing.T) {
	// The first chunk mints events, so the second chunk's question offers
	// them and the cross-chunk join happens: two calls, one at a time.
	judge := &eventJudge{answers: map[string]eventAnswer{}, delay: 10 * time.Millisecond}
	rt, owner, computer := eventRuntime(t, judge)
	var inputs []eventInput
	for i := 0; i < 15; i++ {
		inputs = append(inputs, capItem(fmt.Sprintf("y%d", i), fmt.Sprintf("Story number %d about topic", i), fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	judge.answers[inputs[len(inputs)-1].Item.Title] = eventAnswer{target: inputs[0].Item.Title}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if r.Calls != 1 || len(r.Corroborated) != 1 {
		t.Fatalf("report = %+v; want 1 call and the sibling join", r)
	}
	judge.mu.Lock()
	peak := judge.maxIn
	judge.mu.Unlock()
	if judge.callCount() != 1 || peak != 1 {
		t.Fatalf("judge calls = %d, most in flight = %d; want 1 and 1", judge.callCount(), peak)
	}
}

func TestStoryAnalysisChunksAreAskedOneAtATime(t *testing.T) {
	f := newAnalysisFixture(t)
	f.turnOn(t)
	peak := &peakJudge{inner: f.judge, delay: 10 * time.Millisecond}
	f.rt.SetCandidateJudge(peak)
	var cands []types.EntityCandidate
	for i := 0; i < types.EntityCandidatesMax; i++ {
		name := fmt.Sprintf("Entity %d", i)
		cands = append(cands, types.EntityCandidate{Source: types.EntityCandidateExisting, EntityID: fmt.Sprintf("ent:%024x", i+1), Name: name, Type: types.EntityTypePerson, Phrase: name, Count: 2, Sentences: 2})
	}
	f.cpub.resp = types.EntityCandidatesResponse{Candidates: cands}
	a := f.analyse(t, agentprofile.Research, "run-serial", "doc-story")
	if a == nil || a.Error != "" {
		t.Fatalf("analysis = %+v", a)
	}
	// The System 1 cap is 12 questions a call, so 13 to 24 questions are two
	// calls, never at once.
	peak.assertSerial(t, 2)
}

// Revisions that wait while a call is in flight are one call, not one each
// (owner, 2026-11-15: as many questions in the same call as possible, neither
// concurrently nor in sequence): the first revision is asked at once, and the
// two that arrive during its call go out together in the next.
func TestSupportRevisionsThatWaitAreOneCall(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	hold := newHeldJudge(jev)
	rt, s := supportRuntime(t, hold)
	owner, computer := "owner-support-group", rt.cfg.ComputerID
	one := seedListedSupportPairs(t, s, owner, computer, "doc-group-1", "cid-1")
	two := seedListedSupportPairs(t, s, owner, computer, "doc-group-2", "cid-2")
	three := seedListedSupportPairs(t, s, owner, computer, "doc-group-3", "cid-3")
	rt.JudgeRevisionSupport(owner, computer, one.RunID, one.RevisionID, one.Pairs)
	hold.waitStarted(t)
	rt.JudgeRevisionSupport(owner, computer, two.RunID, two.RevisionID, two.Pairs)
	rt.JudgeRevisionSupport(owner, computer, three.RunID, three.RevisionID, three.Pairs)
	hold.assertNoSecondCall(t)
	hold.release <- struct{}{}
	hold.waitStarted(t)
	hold.release <- struct{}{}
	rt.WaitSupportJudgements()
	if got := hold.calls.Load(); got != 2 {
		t.Fatalf("judge calls = %d for three revisions; want 2 (the first, then the two that waited together)", got)
	}
	if peak := hold.peak.Load(); peak != 1 {
		t.Fatalf("most calls in flight = %d, want 1", peak)
	}
}

// A revision queued behind a slow one waits as long as it takes; the queue has
// no deadline that would drop its pairs.
func TestSupportRevisionQueuedForALongTimeIsStillJudged(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	hold := newHeldJudge(jev)
	rt, s := supportRuntime(t, hold)
	owner, computer := "owner-support-long", rt.cfg.ComputerID
	one := seedListedSupportPairs(t, s, owner, computer, "doc-long-1", "cid-1")
	two := seedListedSupportPairs(t, s, owner, computer, "doc-long-2", "cid-2")
	rt.JudgeRevisionSupport(owner, computer, one.RunID, one.RevisionID, one.Pairs)
	hold.waitStarted(t)
	rt.JudgeRevisionSupport(owner, computer, two.RunID, two.RevisionID, two.Pairs)
	time.Sleep(300 * time.Millisecond)
	hold.release <- struct{}{}
	hold.waitStarted(t)
	hold.release <- struct{}{}
	rt.WaitSupportJudgements()
	if jev.callCount() != 2 {
		t.Fatalf("judge calls = %d; the queued revision was dropped", jev.callCount())
	}
}

func TestSupportRevisionQueuedBehindOneThatJudgedItsPairCallsNobody(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	hold := newHeldJudge(jev)
	rt, s := supportRuntime(t, hold)
	owner, computer := "owner-support-queued", rt.cfg.ComputerID
	one := seedListedSupportPairs(t, s, owner, computer, "doc-queued-1", "cid-1")
	two := seedListedSupportPairs(t, s, owner, computer, "doc-queued-2", "cid-1")
	rt.JudgeRevisionSupport(owner, computer, one.RunID, one.RevisionID, one.Pairs)
	hold.waitStarted(t)
	rt.JudgeRevisionSupport(owner, computer, two.RunID, two.RevisionID, two.Pairs)
	hold.assertNoSecondCall(t)
	hold.release <- struct{}{}
	rt.WaitSupportJudgements()
	// Both revisions cite the same passage for the same sentence, so one
	// pair key: the queued job finds it judged and asks nobody.
	if hold.calls.Load() != 1 {
		t.Fatalf("judge calls = %d, want 1", hold.calls.Load())
	}
}

func TestSupportRevisionOverTheVendorBoundSplitsIntoTheFewestCalls(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	peak := &peakJudge{inner: jev, delay: 10 * time.Millisecond}
	rt, _ := testRuntime(t)
	rt.SetCandidateJudge(peak)
	var blocks []quotematch.Block
	var pending []types.SupportVerdict
	for i := 0; i < 3; i++ {
		cid := fmt.Sprintf("big-%d", i)
		text := strings.Repeat(fmt.Sprintf("passage %d words. ", i), 3000)[:40000]
		blocks = append(blocks, quotematch.Block{ID: cid, Text: text})
		pair := types.CitationPair{Key: "k-" + cid, Text: fmt.Sprintf("Sentence %d.", i), Cited: types.Ref{Doc: "cap-big", CID: cid, Hash: texturewrite.TextHash(text), Quote: "passage"}}
		pending = append(pending, types.SupportVerdict{Key: pair.Key, Pair: &pair})
	}
	rt.SetTextureCaptures(supportCaptures{"cap-big": {Doc: "cap-big", Title: "Big", Blocks: blocks}})
	judged, err := rt.askJevForSupport(context.Background(), "owner-big", "run-big", pending)
	if err != nil {
		t.Fatal(err)
	}
	if len(judged) != 3 {
		t.Fatalf("judged %d of 3 pairs", len(judged))
	}
	// Three passages of 40,000 bytes (about 14,000 estimated tokens each) are
	// over the vendor's 32k per-question limit together; two fit, so two calls
	// is the fewest (state_plus_question_tokens_32k forces the split).
	if got := int(peak.calls.Load()); got != 2 {
		t.Fatalf("judge calls = %d, want 2 (two passages fit the 32k limit together)", got)
	}
	if got := peak.peak.Load(); got != 1 {
		t.Fatalf("most calls in flight = %d; a split's calls are not a reason to run them at once", got)
	}
}

type scopeJudge struct {
	mu     sync.Mutex
	scopes []string
}

func (j *scopeJudge) Judge(ctx context.Context, s json.RawMessage, q map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", s, q)
}

func (j *scopeJudge) JudgeScoped(_ context.Context, scope string, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	j.mu.Lock()
	j.scopes = append(j.scopes, scope)
	j.mu.Unlock()
	return json.RawMessage(`{"answers":{}}`), nil
}

func TestCallJudgeWithinPassesTheScope(t *testing.T) {
	judge := &scopeJudge{}
	for _, scope := range []string{judgmentScopeInteractive, "", judgmentScopeIntakeLabels} {
		if _, err := callJudgeWithin(context.Background(), judge, scope, time.Second, json.RawMessage(`{}`), nil); err != nil {
			t.Fatal(err)
		}
	}
	// An unscoped call goes through Judge, which asks with the empty scope.
	want := []string{judgmentScopeInteractive, "", judgmentScopeIntakeLabels}
	for i, got := range judge.scopes {
		if got != want[i] {
			t.Fatalf("scopes = %v, want %v", judge.scopes, want)
		}
	}
}
