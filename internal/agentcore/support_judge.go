package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// CandidateJudge asks the decision model (Jev) typed questions about one
// state and returns its full answer distribution. Support judgement and
// intake labels are its callers since C1e B1 deleted the frozen-candidate
// judgment (management's Verify judges candidates now).
type CandidateJudge interface {
	Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error)
}

// SetCandidateJudge binds the decision model.
func (rt *Runtime) SetCandidateJudge(judge CandidateJudge) {
	rt.candidateJudge = judge
}

// candidateJudgmentModel names the model the gateway pins for judgments.
const candidateJudgmentModel = "~typesafe/jev-latest"

// Support judgement off every desk's turn
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md). A Texture
// revision commit lists its new citation pairs by their check memos
// (store.planSupportPairs) and, once the commit has returned, hands the
// revision's pairs to JudgeRevisionSupport. A background job, bounded by
// one runtime-wide semaphore like intake labels, judges every pair that is
// listed and not yet judged: Jev reads the committed sentence (from the
// check memo) and the captured block (re-read and checked against the
// Ref's hash), never text a desk supplied. The verdicts are recorded as a
// system act (CommitInboxRecords with Judge system:support): a pass writes
// its memo only; a failure is one record per document to texture:<doc>,
// which wakes the Texture to correct it with Write.
//
// Restarts: the job holds no durable obligation. A crash kills it, nothing
// resumes it at boot, and its pairs stay pending (a check memo without a
// verdict). A later live Texture commit that still cites a pending pair
// judges it, as new work; there is no boot sweep.

// Budgets. Vars so a test can shorten them.
var (
	supportJudgeConcurrency = 2
	supportJudgeQueueWait   = 2 * time.Minute
	supportJudgeCallTimeout = 90 * time.Second
)

// supportJudging is the runtime's support-judgement state: the semaphore
// bounding concurrent jobs, the pairs in flight, the jobs running (for
// tests and shutdown), and the legacy support_pairs records already handed
// to Jev.
type supportJudging struct {
	once     sync.Once
	sem      chan struct{}
	inflight sync.Map
	jobs     sync.WaitGroup
	legacy   sync.Map
}

func (s *supportJudging) slots() chan struct{} {
	s.once.Do(func() { s.sem = make(chan struct{}, supportJudgeConcurrency) })
	return s.sem
}

// supportJudgeQuestion is the one typed question asked of each pair.
var supportJudgeQuestion = map[string]any{
	"type": "choice",
	"criteria": map[string]string{
		"yes": "The cited passage states, or directly entails, what the sentence relies on it for.",
		"no":  "The cited passage does not say it, says something weaker, narrower or different, or contradicts it.",
	},
	"choices": []string{"yes", "no"},
}

// JudgeRevisionSupport hands a committed revision's citation pairs to Jev
// and returns at once: the judgement runs off the commit's path and never
// blocks it. runID is the run that committed the revision, which carries
// the Jev call's cost; revisionID names the commit. Pairs that are not
// listed (no check memo), already judged, or in flight are skipped.
func (rt *Runtime) JudgeRevisionSupport(ownerID, computerID, runID, revisionID string, pairs []types.CitationPair) {
	if rt == nil || rt.store == nil || len(pairs) == 0 {
		return
	}
	seen := map[string]bool{}
	var keys []string
	for _, p := range pairs {
		if key := strings.TrimSpace(p.Key); key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	rt.support.jobs.Add(1)
	go func() {
		defer rt.support.jobs.Done()
		rt.judgeSupportJob(ownerID, computerID, strings.TrimSpace(runID), strings.TrimSpace(revisionID), keys)
	}()
}

// WaitSupportJudgements waits for the support judgements started so far.
func (rt *Runtime) WaitSupportJudgements() {
	if rt != nil {
		rt.support.jobs.Wait()
	}
}

func (rt *Runtime) judgeSupportJob(ownerID, computerID, runID, revisionID string, keys []string) {
	slots := rt.support.slots()
	wait := time.NewTimer(supportJudgeQueueWait)
	select {
	case slots <- struct{}{}:
		wait.Stop()
	case <-wait.C:
		log.Printf("support judge: %s: no slot within %s; %d pairs stay pending for the next commit that cites them", revisionID, supportJudgeQueueWait, len(keys))
		return
	}
	defer func() { <-slots }()

	// Claim the pairs no other job is judging.
	var claimed []string
	for _, key := range keys {
		flight := ownerID + "\x00" + computerID + "\x00" + key
		if _, busy := rt.support.inflight.LoadOrStore(flight, struct{}{}); busy {
			continue
		}
		claimed = append(claimed, key)
		defer rt.support.inflight.Delete(flight)
	}
	if len(claimed) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), supportJudgeCallTimeout+30*time.Second)
	defer cancel()
	if err := rt.judgePendingSupport(ctx, ownerID, computerID, runID, revisionID, claimed); err != nil {
		log.Printf("support judge: %s: %v; its pairs stay pending for the next commit that cites them", revisionID, err)
	}
}

// judgePendingSupport judges the listed, unjudged pairs among keys and
// records their verdicts.
func (rt *Runtime) judgePendingSupport(ctx context.Context, ownerID, computerID, runID, revisionID string, keys []string) error {
	var pending []types.SupportVerdict
	for _, key := range keys {
		if _, judged, err := rt.store.SupportVerdict(ctx, ownerID, computerID, key); err != nil {
			return err
		} else if judged {
			continue
		}
		check, listed, err := rt.store.CitationCheck(ctx, ownerID, computerID, key)
		if err != nil {
			return err
		}
		if !listed {
			continue
		}
		pair := check.Pair
		pending = append(pending, types.SupportVerdict{Key: key, Pair: &pair})
	}
	if len(pending) == 0 {
		return nil
	}
	if rt.candidateJudge == nil {
		return fmt.Errorf("no decision model (Jev) is bound on this computer")
	}
	judged, err := rt.askJevForSupport(ctx, ownerID, runID, pending)
	if err != nil {
		return err
	}
	if len(judged) == 0 {
		return nil
	}
	judgedKeys := make([]string, 0, len(judged))
	for _, v := range judged {
		judgedKeys = append(judgedKeys, v.Key)
	}
	sort.Strings(judgedKeys)
	sum := sha256.Sum256([]byte(strings.Join(judgedKeys, "\n")))
	req := types.CommitInboxRecordsRequest{
		OwnerID: ownerID, ComputerID: computerID,
		CommandID: "support-judge:" + revisionID + ":" + hex.EncodeToString(sum[:8]),
		Judge:     types.SupportSystemAgentID, Verdicts: judged,
	}
	if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
		return err
	}
	committed, err := rt.store.CommitInboxRecords(ctx, req)
	if err != nil {
		return fmt.Errorf("record verdicts: %w", err)
	}
	// Verdicts reach the published overlay of the stories they judge, off
	// this path (Cut 2 2b).
	if len(committed.Verdicts) > 0 {
		rt.publishStoryVerdicts(ownerID, computerID, committed.Verdicts)
	}
	return nil
}

// askJevForSupport asks Jev about the pending pairs in one call and returns
// those it judged. A pair whose cited block cannot be read, or no longer
// hashes to its Ref, is not asked about; a pair Jev gives no answer for is
// not returned. Both stay pending.
func (rt *Runtime) askJevForSupport(ctx context.Context, owner, runID string, pending []types.SupportVerdict) ([]types.SupportVerdict, error) {
	captures := rt.TextureCaptures()
	if captures == nil {
		return nil, fmt.Errorf("the capture store is not configured, so Jev cannot read the cited blocks")
	}
	byDoc := map[string]texturewrite.Capture{}
	unreadable := map[string]error{}
	pairs := map[string]any{}
	questions := map[string]json.RawMessage{}
	names := map[string]int{}
	for i := range pending {
		pair := pending[i].Pair
		if err, bad := unreadable[pair.Cited.Doc]; bad {
			log.Printf("support judge: pair %s stays pending: read the cited capture %s: %v", pair.Key, pair.Cited.Doc, err)
			continue
		}
		capture, ok := byDoc[pair.Cited.Doc]
		if !ok {
			var err error
			if capture, err = captures.CaptureBlocks(ctx, owner, pair.Cited.Doc); err != nil {
				unreadable[pair.Cited.Doc] = err
				log.Printf("support judge: pair %s stays pending: read the cited capture %s: %v", pair.Key, pair.Cited.Doc, err)
				continue
			}
			byDoc[pair.Cited.Doc] = capture
		}
		passage := ""
		for _, b := range capture.Blocks {
			if b.ID == pair.Cited.CID {
				passage = b.Text
				break
			}
		}
		if passage == "" || texturewrite.TextHash(passage) != pair.Cited.Hash {
			log.Printf("support judge: pair %s stays pending: the cited block %s of %s is gone or changed since it was cited", pair.Key, pair.Cited.CID, pair.Cited.Doc)
			continue
		}
		name := fmt.Sprintf("p%d", len(names)+1)
		names[name] = i
		entry := map[string]string{"sentence": pair.Text, "quote": pair.Cited.Quote, "cited_passage": passage, "source": firstNonEmpty(capture.Title, capture.URL)}
		if pair.Context != "" {
			entry["paragraph"] = pair.Context
		}
		pairs[name] = entry
		question := map[string]any{"instructions": fmt.Sprintf("Pair %s: does state.pairs.%s.cited_passage support what state.pairs.%s.sentence says where it cites that source? Judge support, not truth.", name, name, name)}
		for k, v := range supportJudgeQuestion {
			question[k] = v
		}
		raw, err := json.Marshal(question)
		if err != nil {
			return nil, err
		}
		questions[name] = raw
	}
	if len(questions) == 0 {
		return nil, nil
	}
	state, err := json.Marshal(map[string]any{
		"task":  "Each pair is a sentence from a news story and the source passage it cites (quote is the cited words). Judge whether the passage supports the sentence as cited. A pair's paragraph, when given, is the sentence's surroundings, there only to resolve names and references: judge only the sentence, not the rest of the paragraph, which cites other sources.",
		"pairs": pairs,
	})
	if err != nil {
		return nil, err
	}
	raw, err := callJudgeWithin(ctx, rt.candidateJudge, "", supportJudgeCallTimeout, state, questions)
	if err != nil {
		return nil, fmt.Errorf("Jev: %w", err)
	}
	var distribution jevDistribution
	if err := json.Unmarshal(raw, &distribution); err != nil {
		return nil, fmt.Errorf("Jev: decode its answer: %w", err)
	}
	model := firstNonEmpty(distribution.Model, candidateJudgmentModel)
	rt.recordJevCost(ctx, owner, runID, model, distribution.Usage, len(questions))
	judge := "jev:" + model
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Slice(ordered, func(a, b int) bool { return names[ordered[a]] < names[ordered[b]] })
	var judged []types.SupportVerdict
	for _, name := range ordered {
		v := pending[names[name]]
		answer, ok := distribution.Answers[name]
		if !ok || (answer.Choice != "yes" && answer.Choice != "no") {
			log.Printf("support judge: pair %s stays pending: Jev gave no answer for it", v.Key)
			continue
		}
		yes, known := answer.Probabilities["yes"]
		if !known {
			yes = 0
			if answer.Choice == "yes" {
				yes = 1
			}
		}
		v.Judge, v.PSupportedBP = judge, int64(math.Round(yes*10000))
		if answer.Choice == "yes" && yes >= 0.5 {
			v.Verdict = types.SupportSupported
		} else {
			v.Verdict = types.SupportUnsupported
			v.Why = fmt.Sprintf("Jev judged the cited passage does not support the sentence (p(supported)=%.2f)", yes)
		}
		judged = append(judged, v)
	}
	return judged, nil
}

// jevDistribution is the judge's answer as the transport returns it: the
// model that served it, each question's choice with the full probability
// distribution, and the usage. The support judge and choir.System1 read it.
type jevDistribution struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Answers map[string]struct {
		Choice        string             `json:"choice"`
		Probabilities map[string]float64 `json:"probabilities"`
	} `json:"answers"`
	Usage jevUsage `json:"usage"`
}

// jevUsage is the token usage a Jev answer reports.
type jevUsage struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

// jevJudgmentPhase is the run-progress phase of a Jev call's cost event;
// llmcost reads it beside tool_loop.
const jevJudgmentPhase = "jev_judgment"

// recordJevCost appends a Jev call's token usage as a run-progress event on
// the run that caused it, where /api/costs reads it. Best effort: a call
// with no usage or no run records nothing.
func (rt *Runtime) recordJevCost(ctx context.Context, ownerID, runID, model string, usage jevUsage, questions int) {
	rt.recordJevCostFor(ctx, ownerID, runID, types.SupportSystemAgentID, "support_judgement", model, usage, questions)
}

// recordJevCostFor is recordJevCost for a caller that is not the support
// judge: it names the agent the cost is booked under and the purpose.
func (rt *Runtime) recordJevCostFor(ctx context.Context, ownerID, runID, agentID, purpose, model string, usage jevUsage, questions int) {
	input, output := usage.InputTokens, usage.OutputTokens
	if input == 0 && output == 0 {
		input, output = usage.PromptTokens, usage.CompletionTokens
	}
	if rt == nil || rt.store == nil || runID == "" || (input == 0 && output == 0) {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"phase": jevJudgmentPhase, "llm_model": model, "llm_provider": "openrouter",
		"input_tokens": input, "output_tokens": output, "cost_usd": usage.Cost,
		"purpose": purpose, "questions": questions,
	})
	if err != nil {
		return
	}
	ev := types.EventRecord{EventID: uuid.New().String(), Timestamp: time.Now().UTC(), RunID: runID, OwnerID: ownerID,
		AgentID: agentID, Kind: types.EventRunProgress, Phase: jevJudgmentPhase, Payload: payload}
	if err := rt.store.AppendEvent(ctx, &ev); err != nil {
		log.Printf("support judge: record Jev cost on run %s: %v", runID, err)
	}
}

// judgeLegacySupportPairs hands the pairs of a support_pairs record minted
// before support judgement left management to Jev, once per record per
// process. The record is shown to management closed
// (legacySupportPairsBody).
func (rt *Runtime) judgeLegacySupportPairs(ownerID, computerID, runID string, rec types.CommitmentRecord) {
	if rec.Support == nil || len(rec.Support.Pairs) == 0 {
		return
	}
	if _, handed := rt.support.legacy.LoadOrStore(ownerID+"\x00"+computerID+"\x00"+rec.RecordID, struct{}{}); handed {
		return
	}
	revisionID := strings.TrimSpace(rec.SubjectRefs["revision_id"])
	if revisionID == "" {
		revisionID = rec.RecordID
	}
	rt.JudgeRevisionSupport(ownerID, computerID, runID, revisionID, rec.Support.Pairs)
}

// legacySupportPairsBody is what management reads of a legacy support_pairs
// record: its visible fate.
const legacySupportPairsBody = "Closed: Jev judges citation support off your turn now; this record needs no answer."
