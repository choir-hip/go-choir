package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Calibration feedback: the in-context digest of the ICRL experiment
// (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md).
//
// The runtime collects each desk's settled self-scores from the tape:
//   - hypotheses a desk opened in its Casts (management, Texture), settled
//     by the counterpart's Report (in protocol) or by an independent
//     scorer's calibration score record;
//   - research's final completed Reports with a p_complete self-score,
//     settled by a scorer's coverage;
//   - Texture's Write pSupported, on each new citation's check memo,
//     settled by Jev's support verdict on the same pair.
//
// While the computer's calibration.feedback setting is on, a run in the
// experiment's digest arm reads its desk's digest as a user turn appended
// after its history (never the system prompt or tools, so the cached
// prefix holds) and in choir.Pack(). Off, nothing is computed.

const (
	// calibrationRefresh is how long a computer's snapshot is reused.
	calibrationRefresh = 30 * time.Minute
	// calibrationListLimit bounds each store listing behind a snapshot.
	calibrationListLimit = 20000
	// calibrationDigestHeader starts the digest's user turn.
	calibrationDigestHeader = "Calibration digest:"
)

// calibrationCache holds one snapshot per computer.
type calibrationCache struct {
	mu         sync.Mutex
	byComputer map[string]*calibrationSnapshot
}

// calibrationSnapshot is a computer's settled self-scores, and the
// self-scores and claims still waiting for an independent settlement.
type calibrationSnapshot struct {
	At           time.Time                 `json:"at"`
	Observations []calibration.Observation `json:"observations"`
	Pending      []CalibrationPending      `json:"pending"`
}

// CalibrationPending is a claim no independent scorer has settled yet:
// what an off-computer scorer settles next (GET /api/calibration).
type CalibrationPending struct {
	Desk           string   `json:"desk"`
	ClaimType      string   `json:"claim_type"`
	Thread         string   `json:"thread"`
	Story          string   `json:"story,omitempty"`
	RecordID       string   `json:"record_id"`
	ConversationID string   `json:"conversation_id,omitempty"`
	HypothesisID   string   `json:"hypothesis_id,omitempty"`
	Statement      string   `json:"statement,omitempty"`
	P              *float64 `json:"p,omitempty"`
	ModelID        string   `json:"model_id,omitempty"`
	CommittedAt    string   `json:"committed_at,omitempty"`
}

// calibrationDeskOfThread maps a desk thread (an agent id or inbox) to the
// desk whose track record it adds to; "" for a desk with no digest.
func calibrationDeskOfThread(thread string) string {
	desk, _, _ := strings.Cut(strings.TrimSpace(thread), ":")
	switch desk {
	case types.ManagementTaskDesk, agentprofile.Management:
		return agentprofile.Management
	case agentprofile.Texture, agentprofile.Research:
		return desk
	}
	return ""
}

// calibrationThreadOfRecord is the desk thread that authored a record: a
// management task's acts go out as management and name the task.
func calibrationThreadOfRecord(rec types.CommitmentRecord) string {
	if task := strings.TrimSpace(rec.Provenance.ActingTask); task != "" {
		return types.ManagementTaskDesk + ":" + task
	}
	if sender := strings.TrimSpace(rec.SenderInbox); sender != "" {
		return sender
	}
	return strings.TrimSpace(rec.Provenance.AgentID)
}

// hypothesisOutcome maps a settlement outcome to y; ok is false for an
// outcome that does not settle (unresolved, unknown).
func hypothesisOutcome(outcome string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case string(types.DiscrepancyConfirmed):
		return 1, true
	case string(types.DiscrepancyQualified):
		return 0.5, true
	case string(types.DiscrepancyContradicted):
		return 0, true
	}
	return 0, false
}

// isCalibrationScoreRecord reports an independent scorer's settlement.
func isCalibrationScoreRecord(rec types.CommitmentRecord) bool {
	return strings.HasPrefix(rec.RecordID, types.CalibrationScoreRecordPrefix) && rec.Resolve != nil && len(rec.RelatedIDs) > 0
}

// reportSelfP is a Report's own p_complete, if it stated one.
func reportSelfP(rec types.CommitmentRecord) *float64 {
	for _, sc := range rec.Scores {
		if p, ok := sc.Probabilities[types.SelfScoreComplete]; sc.Self && ok && p >= 0 && p <= 1 {
			return &p
		}
	}
	return nil
}

type settlementAcc struct {
	ys      []float64
	scorers []string
}

func (a *settlementAcc) add(y float64, scorer string) {
	a.ys = append(a.ys, y)
	a.scorers = append(a.scorers, scorer)
}

func (a *settlementAcc) mean() float64 {
	sum := 0.0
	for _, y := range a.ys {
		sum += y
	}
	return sum / float64(len(a.ys))
}

// collectCalibration derives the settled self-scores and the pending
// claims from the tape. Pure: the snapshot and the tests call it.
// Only settled items become observations (experiment P4); several
// scorers' outcomes on one item are averaged.
func collectCalibration(records []types.CommitmentRecord, checks []store.SupportMemo[types.CitationCheck], verdicts []store.SupportMemo[types.SupportVerdict]) ([]calibration.Observation, []CalibrationPending) {
	hypSettled := map[string]*settlementAcc{}  // record#hyp, and conv|hyp for protocol
	doneSettled := map[string]*settlementAcc{} // report record id
	withdrawn := map[string]bool{}             // conv|hyp
	acc := func(m map[string]*settlementAcc, key string) *settlementAcc {
		if m[key] == nil {
			m[key] = &settlementAcc{}
		}
		return m[key]
	}
	for _, rec := range records {
		if isCalibrationScoreRecord(rec) {
			target := strings.TrimSpace(rec.RelatedIDs[0])
			for _, sc := range rec.Scores {
				if sc.Self {
					continue
				}
				scorer := firstNonEmptyString(sc.ScorerAgentID, rec.Provenance.AgentID)
				hyps := make([]string, 0, len(sc.Answers))
				for id := range sc.Answers {
					hyps = append(hyps, id)
				}
				sort.Strings(hyps)
				for _, id := range hyps {
					if y, ok := hypothesisOutcome(sc.Answers[id]); ok {
						acc(hypSettled, target+"#"+id).add(y, scorer)
					}
				}
				if y, ok := sc.Probabilities[types.CalibrationScoreCoverage]; ok && y >= 0 && y <= 1 {
					acc(doneSettled, target).add(y, scorer)
				}
			}
			continue
		}
		if rec.Report != nil {
			for _, s := range rec.Report.Settled {
				if y, ok := hypothesisOutcome(s.Outcome); ok {
					acc(hypSettled, rec.ConversationID+"|"+s.ID).add(y, "protocol:"+rec.SenderInbox)
				}
			}
		}
		if rec.Cast != nil {
			for _, w := range rec.Cast.Withdrawals {
				withdrawn[rec.ConversationID+"|"+w.ID] = true
			}
		}
	}
	var obs []calibration.Observation
	var pending []CalibrationPending
	for _, rec := range records {
		if isCalibrationScoreRecord(rec) || rec.EmitterKind == types.CommitmentEmitterSystem {
			continue
		}
		thread := calibrationThreadOfRecord(rec)
		desk := calibrationDeskOfThread(thread)
		if desk == "" {
			continue
		}
		model := strings.TrimSpace(rec.Provenance.ModelID)
		committed := strings.TrimSpace(rec.Provenance.CommittedAt)
		if rec.Cast != nil && rec.IsCast() {
			for _, h := range rec.Cast.Hypotheses {
				if strings.TrimSpace(h.ID) == "" || !(h.Probability >= 0 && h.Probability <= 1) {
					continue
				}
				key := rec.RecordID + "#" + h.ID
				var settled settlementAcc
				for _, k := range []string{key, rec.ConversationID + "|" + h.ID} {
					if a := hypSettled[k]; a != nil {
						settled.ys = append(settled.ys, a.ys...)
						settled.scorers = append(settled.scorers, a.scorers...)
					}
				}
				if len(settled.ys) == 0 {
					if !withdrawn[rec.ConversationID+"|"+h.ID] {
						p := h.Probability
						pending = append(pending, CalibrationPending{Desk: desk, ClaimType: types.CalibrationClaimHypothesis, Thread: thread, Story: rec.Inbox,
							RecordID: rec.RecordID, ConversationID: rec.ConversationID, HypothesisID: h.ID, Statement: h.Statement,
							P: &p, ModelID: model, CommittedAt: committed})
					}
					continue
				}
				obs = append(obs, calibration.Observation{Desk: desk, ClaimType: types.CalibrationClaimHypothesis, Thread: thread, Story: rec.Inbox,
					ModelID: model, RecordID: rec.RecordID, Key: key, P: h.Probability, Y: settled.mean(), Scorers: settled.scorers,
					CommittedAt: committed, HeldOut: calibration.HeldOut(key)})
			}
			continue
		}
		if desk == agentprofile.Research && rec.Report != nil && rec.Final && strings.TrimSpace(rec.Report.StopReason) == string(types.WorkItemCompleted) {
			p := reportSelfP(rec)
			settled := doneSettled[rec.RecordID]
			if settled == nil {
				pending = append(pending, CalibrationPending{Desk: desk, ClaimType: types.CalibrationClaimCompleted, Thread: thread, Story: rec.Inbox,
					RecordID: rec.RecordID, ConversationID: rec.ConversationID, P: p, ModelID: model, CommittedAt: committed})
				continue
			}
			if p != nil {
				obs = append(obs, calibration.Observation{Desk: desk, ClaimType: types.CalibrationClaimCompleted, Thread: thread, Story: rec.Inbox,
					ModelID: model, RecordID: rec.RecordID, Key: rec.RecordID, P: *p, Y: settled.mean(), Scorers: settled.scorers,
					CommittedAt: committed, HeldOut: calibration.HeldOut(rec.RecordID)})
			}
		}
	}
	verdictOf := make(map[string]types.SupportVerdict, len(verdicts))
	for _, v := range verdicts {
		verdictOf[v.Value.Key] = v.Value
	}
	for _, c := range checks {
		check := c.Value
		if check.SelfP == nil {
			continue
		}
		v, judged := verdictOf[check.Pair.Key]
		var y float64
		switch v.Verdict {
		case types.SupportSupported:
			y = 1
		case types.SupportUnsupported:
			y = 0
		default:
			judged = false
		}
		if !judged {
			continue // Jev has not judged it: pending in the guest, settled by its own job
		}
		thread := agentprofile.Texture + ":" + check.Pair.DocID
		obs = append(obs, calibration.Observation{Desk: agentprofile.Texture, ClaimType: types.CalibrationClaimCitationSupport, Thread: thread, Story: thread,
			ModelID: check.ModelID, RecordID: check.Pair.RevisionID, Key: check.Pair.Key, P: *check.SelfP, Y: y,
			Scorers: []string{firstNonEmptyString(v.Judge, types.SupportSystemAgentID)}, CommittedAt: c.At.UTC().Format(time.RFC3339Nano),
			HeldOut: calibration.HeldOut(check.Pair.Key)})
	}
	return obs, pending
}

// calibrationFeedbackOn reports the computer's calibration.feedback setting.
func (rt *Runtime) calibrationFeedbackOn(ctx context.Context, ownerID, computerID string) bool {
	return rt.computerSettingOn(ctx, ownerID, computerID, store.SettingCalibrationFeedback)
}

// calibrationSnapshotFor returns the computer's snapshot, rebuilt when
// older than maxAge. One build at a time per runtime (experiment P7).
func (rt *Runtime) calibrationSnapshotFor(ctx context.Context, ownerID, computerID string, maxAge time.Duration) (*calibrationSnapshot, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("calibration: store unavailable")
	}
	key := strings.TrimSpace(ownerID) + "\x00" + strings.TrimSpace(computerID)
	rt.calibration.mu.Lock()
	defer rt.calibration.mu.Unlock()
	if snap := rt.calibration.byComputer[key]; snap != nil && time.Since(snap.At) < maxAge {
		return snap, nil
	}
	records, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, "", calibrationListLimit)
	if err != nil {
		return nil, fmt.Errorf("calibration: list records: %w", err)
	}
	checks, err := rt.store.ListCitationChecks(ctx, ownerID, computerID, calibrationListLimit)
	if err != nil {
		return nil, fmt.Errorf("calibration: list citation checks: %w", err)
	}
	verdicts, err := rt.store.ListSupportVerdicts(ctx, ownerID, computerID, calibrationListLimit)
	if err != nil {
		return nil, fmt.Errorf("calibration: list support verdicts: %w", err)
	}
	obs, pending := collectCalibration(records, checks, verdicts)
	snap := &calibrationSnapshot{At: time.Now().UTC(), Observations: obs, Pending: pending}
	if rt.calibration.byComputer == nil {
		rt.calibration.byComputer = map[string]*calibrationSnapshot{}
	}
	rt.calibration.byComputer[key] = snap
	return snap, nil
}

// calibrationDeskOfRun is the desk whose digest a run reads; "" when none.
func calibrationDeskOfRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	return calibrationDeskOfThread(agentIDForRun(rec))
}

// calibrationArmOfRun is the run's experiment arm, by its desk thread.
func calibrationArmOfRun(rec *types.RunRecord) string {
	return calibration.Arm(agentIDForRun(rec))
}

// calibrationDigestForRun returns the digest a run reads, or nil: the
// setting is off, the run is in the control arm, its desk has no digest,
// or the record is too small to show (experiment P3, P5).
func (rt *Runtime) calibrationDigestForRun(ctx context.Context, rec *types.RunRecord) *types.CalibrationDigest {
	desk := calibrationDeskOfRun(rec)
	if desk == "" || calibrationArmOfRun(rec) != "digest" || !rt.calibrationFeedbackOn(ctx, rec.OwnerID, rec.ComputerID) {
		return nil
	}
	snap, err := rt.calibrationSnapshotFor(ctx, rec.OwnerID, rec.ComputerID, calibrationRefresh)
	if err != nil {
		log.Printf("calibration: %s: %v", rec.RunID, err)
		return nil
	}
	opts := calibration.DefaultOptions
	opts.ModelID = recordModelID(ctx, rec)
	digest := calibration.Compute(desk, snap.Observations, opts)
	if digest.Text == "" {
		return nil
	}
	digest.ComputedAt = snap.At.Format(time.RFC3339)
	return &digest
}

// calibrationDigestMessage is the digest's user turn.
func calibrationDigestMessage(text string) json.RawMessage {
	msg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []any{map[string]string{"type": "text", "text": calibrationDigestHeader + "\n" + text}},
	})
	return msg
}

// lastCalibrationDigestText finds the newest digest turn in a run's
// replayed history, so a resumed activation does not repeat it.
func lastCalibrationDigestText(history []json.RawMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		var msg struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(history[i], &msg) != nil || msg.Role != "user" || len(msg.Content) != 1 {
			continue
		}
		if text, ok := strings.CutPrefix(msg.Content[0].Text, calibrationDigestHeader+"\n"); ok {
			return text
		}
	}
	return ""
}

// calibrationDigestInjector appends the digest turn when the run should
// read it and it differs from the last one this run read (experiment P1,
// P2): at the activation's start, and after a tool batch when a refresh
// changed it. Nil for runs that never read a digest.
func (rt *Runtime) calibrationDigestInjector(rec *types.RunRecord, history []json.RawMessage) toolregistry.InjectUserTurnsFunc {
	if rt == nil || rt.store == nil || calibrationDeskOfRun(rec) == "" || calibrationArmOfRun(rec) != "digest" {
		return nil
	}
	last := lastCalibrationDigestText(history)
	return func(bool) ([]json.RawMessage, error) {
		digest := rt.calibrationDigestForRun(context.Background(), rec)
		if digest == nil || digest.Text == last {
			return nil, nil
		}
		last = digest.Text
		return []json.RawMessage{calibrationDigestMessage(digest.Text)}, nil
	}
}
