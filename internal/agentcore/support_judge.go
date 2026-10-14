package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// CandidateJudge asks the decision model (Jev) typed questions about one
// state and returns its full answer distribution. Support judgement is its
// one caller since C1e B1 deleted the frozen-candidate judgment (management's
// Verify judges candidates now).
type CandidateJudge interface {
	Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error)
}

// SetCandidateJudge binds the decision model.
func (rt *Runtime) SetCandidateJudge(judge CandidateJudge) {
	rt.candidateJudge = judge
}

// candidateJudgmentModel names the model the gateway pins for judgments.
const candidateJudgmentModel = "typesafe/jev-1.13"

// Support judgement, the host half of management's choir.Judge (Cut 2 2a
// (v)). A Texture commit lists its new citation pairs to management; a
// management cell passes them to choir.Judge, and this resolves each one:
// a judged pair returns its memoized verdict, a pair management judged
// itself keeps its verdict, and the rest go to Jev, the gateway's pinned
// decision model, in one call. Jev reads the committed sentence (from the
// check memo the commit wrote) and the captured block (re-read and checked
// against the Ref's hash), never text the cell supplied. The verdicts return
// to the cell, which stages them; the cell's commit records them. Nothing
// here writes: judging never edits text or mints a version.

// supportJudgeQuestion is the one typed question asked of each pair.
var supportJudgeQuestion = map[string]any{
	"type": "choice",
	"criteria": map[string]string{
		"yes": "The cited passage states, or directly entails, what the sentence relies on it for.",
		"no":  "The cited passage does not say it, says something weaker, narrower or different, or contradicts it.",
	},
	"choices": []string{"yes", "no"},
}

// judgeSupportEgress serves ActionJudge for a management cell.
func (rt *Runtime) judgeSupportEgress(ctx context.Context, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	role := strings.TrimSpace(exec.Role)
	if role == "" && exec.RunRecord != nil {
		role = agentProfileForRun(exec.RunRecord)
	}
	if role != agentprofile.Management {
		return nil, fmt.Errorf("support judgement is management's")
	}
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("judge host unavailable")
	}
	var req yaegikernel.JudgePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode pairs: %w", err)
	}
	if len(req.Pairs) == 0 {
		return nil, fmt.Errorf("no pairs: pass the pairs a support_pairs record lists (r.Support.Pairs)")
	}
	owner, computer := strings.TrimSpace(exec.OwnerID), strings.TrimSpace(exec.ComputerID)
	verdicts := make([]types.SupportVerdict, len(req.Pairs))
	var ask []int
	for i, p := range req.Pairs {
		key := strings.TrimSpace(p.Key)
		if v, found, err := rt.store.SupportVerdict(ctx, owner, computer, key); err != nil {
			return nil, err
		} else if found {
			v.Memo = true
			verdicts[i] = v
			continue
		}
		check, listed, err := rt.store.CitationCheck(ctx, owner, computer, key)
		if err != nil {
			return nil, err
		}
		if !listed {
			return nil, fmt.Errorf("pair %q was not listed by a Texture commit; judge the pairs a support_pairs record lists", key)
		}
		pair := check.Pair
		switch verdict := strings.TrimSpace(p.Verdict); verdict {
		case "":
			verdicts[i] = types.SupportVerdict{Key: key, Pair: &pair}
			ask = append(ask, i)
		case types.SupportSupported, types.SupportUnsupported:
			why := strings.TrimSpace(p.Why)
			if verdict == types.SupportUnsupported && why == "" {
				return nil, fmt.Errorf("pair %s: an unsupported verdict needs Why, which the Texture reads to correct the sentence", key)
			}
			verdicts[i] = types.SupportVerdict{Key: key, Verdict: verdict, Why: why, Judge: agentprofile.Management, Pair: &pair}
		default:
			return nil, fmt.Errorf("pair %s: Verdict is %q; set %q or %q, or leave it empty for Jev", key, verdict, types.SupportSupported, types.SupportUnsupported)
		}
	}
	if len(ask) > 0 {
		if err := rt.askJevForSupport(ctx, owner, verdicts, ask); err != nil {
			return nil, err
		}
	}
	return json.Marshal(verdicts)
}

// askJevForSupport fills verdicts[ask] from one Jev call.
func (rt *Runtime) askJevForSupport(ctx context.Context, owner string, verdicts []types.SupportVerdict, ask []int) error {
	if rt.candidateJudge == nil {
		return fmt.Errorf("no decision model (Jev) is bound on this computer; judge the pair yourself: set Verdict and Why on it and pass it again")
	}
	captures := rt.TextureCaptures()
	if captures == nil {
		return fmt.Errorf("the capture store is not configured, so Jev cannot read the cited blocks; judge the pair yourself: set Verdict and Why on it and pass it again")
	}
	byDoc := map[string]texturewrite.Capture{}
	pairs := map[string]any{}
	questions := map[string]json.RawMessage{}
	names := make(map[int]string, len(ask))
	for n, i := range ask {
		pair := verdicts[i].Pair
		capture, ok := byDoc[pair.Cited.Doc]
		if !ok {
			var err error
			if capture, err = captures.CaptureBlocks(ctx, owner, pair.Cited.Doc); err != nil {
				return fmt.Errorf("pair %s: read the cited capture %s: %w", pair.Key, pair.Cited.Doc, err)
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
			return fmt.Errorf("pair %s: the cited block %s of %s is gone or changed since it was cited; judge it yourself or let the Texture re-cite it", pair.Key, pair.Cited.CID, pair.Cited.Doc)
		}
		name := fmt.Sprintf("p%d", n+1)
		names[i] = name
		pairs[name] = map[string]string{"sentence": pair.Text, "quote": pair.Cited.Quote, "cited_passage": passage, "source": firstNonEmpty(capture.Title, capture.URL)}
		question := map[string]any{"instructions": fmt.Sprintf("Pair %s: does state.pairs.%s.cited_passage support what state.pairs.%s.sentence says where it cites that source? Judge support, not truth.", name, name, name)}
		for k, v := range supportJudgeQuestion {
			question[k] = v
		}
		raw, err := json.Marshal(question)
		if err != nil {
			return err
		}
		questions[name] = raw
	}
	state, err := json.Marshal(map[string]any{
		"task":  "Each pair is a sentence from a news story and the source passage it cites (quote is the cited words). Judge whether the passage supports the sentence as cited.",
		"pairs": pairs,
	})
	if err != nil {
		return err
	}
	raw, err := rt.candidateJudge.Judge(ctx, state, questions)
	if err != nil {
		return fmt.Errorf("Jev: %w", err)
	}
	var distribution struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(raw, &distribution); err != nil {
		return fmt.Errorf("Jev: decode its answer: %w", err)
	}
	judge := "jev:" + firstNonEmpty(distribution.Model, candidateJudgmentModel)
	var missing []string
	for _, i := range ask {
		answer, ok := distribution.Answers[names[i]]
		if !ok || (answer.Choice != "yes" && answer.Choice != "no") {
			missing = append(missing, verdicts[i].Key)
			continue
		}
		yes, known := answer.Probabilities["yes"]
		if !known {
			yes = 0
			if answer.Choice == "yes" {
				yes = 1
			}
		}
		v := &verdicts[i]
		v.Judge, v.PSupportedBP = judge, int64(math.Round(yes*10000))
		if answer.Choice == "yes" && yes >= 0.5 {
			v.Verdict = types.SupportSupported
		} else {
			v.Verdict = types.SupportUnsupported
			v.Why = fmt.Sprintf("Jev judged the cited passage does not support the sentence (p(supported)=%.2f)", yes)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("Jev gave no answer for %s; call Judge again or judge them yourself", strings.Join(missing, ", "))
	}
	return nil
}
