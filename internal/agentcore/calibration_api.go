package agentcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner's calibration routes (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md):
//
//   - GET /api/calibration: each desk's digest as the guest computes it;
//     with detail=1, the settled observations (arm and held-out marked) and
//     the claims still pending an independent settlement. Read-only.
//   - POST /api/calibration/settlements: an independent scorer's
//     settlements, one calibration score record per (scorer, target): a
//     resolve with the scorer's scores and model, discrepancy unresolved
//     and no observation, so no acting pack reads it as a verdict (P10).
//
// Both are owner acts. Neither is on management's ProductAPI allowlist, so
// no desk can read another's raw outcomes or settle its own claims (P11).

const maxCalibrationSettlements = 500

var calibrationScorerName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type calibrationSettlement struct {
	TargetRecordID string            `json:"target_record_id"`
	ClaimType      string            `json:"claim_type"`
	Answers        map[string]string `json:"answers,omitempty"`
	Coverage       *float64          `json:"coverage,omitempty"`
	EvidenceRefs   []string          `json:"evidence_refs,omitempty"`
}

type calibrationSettlementsRequest struct {
	Scorer      string                  `json:"scorer"`
	ModelID     string                  `json:"model_id"`
	Settlements []calibrationSettlement `json:"settlements"`
}

type calibrationRefusal struct {
	TargetRecordID string `json:"target_record_id"`
	Reason         string `json:"reason"`
}

// HandleCalibration serves GET /api/calibration.
func (h *APIHandler) HandleCalibration(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	rt := h.rt
	if rt == nil || rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "store unavailable"})
		return
	}
	ctx := r.Context()
	computerID := rt.TextureComputerID()
	snap, err := rt.calibrationSnapshotFor(ctx, ownerID, computerID, time.Minute)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	digests := map[string]types.CalibrationDigest{}
	for _, desk := range []string{agentprofile.Management, agentprofile.Texture, agentprofile.Research} {
		d := calibration.Compute(desk, snap.Observations, calibration.DefaultOptions)
		d.ComputedAt = snap.At.Format(time.RFC3339)
		digests[desk] = d
	}
	resp := map[string]any{
		"setting":      map[string]any{"key": store.SettingCalibrationFeedback, "value": rt.calibrationFeedbackOn(ctx, ownerID, computerID)},
		"computed_at":  snap.At.Format(time.RFC3339),
		"digests":      digests,
		"observations": len(snap.Observations),
		"pending":      len(snap.Pending),
		"arm_rule":     `sha256("choir-icrl-v1:" + thread), first byte even = digest`,
	}
	if r.URL.Query().Get("detail") == "1" {
		type armed struct {
			calibration.Observation
			Arm string `json:"arm"`
		}
		detail := make([]armed, 0, len(snap.Observations))
		for _, o := range snap.Observations {
			detail = append(detail, armed{Observation: o, Arm: calibration.Arm(o.Thread)})
		}
		resp["observations"] = detail
		resp["pending"] = snap.Pending
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleCalibrationSettlements serves POST /api/calibration/settlements.
func (h *APIHandler) HandleCalibrationSettlements(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	rt := h.rt
	if rt == nil || rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "store unavailable"})
		return
	}
	var req calibrationSettlementsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON body"})
		return
	}
	req.Scorer, req.ModelID = strings.TrimSpace(req.Scorer), strings.TrimSpace(req.ModelID)
	switch {
	case !calibrationScorerName.MatchString(req.Scorer):
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "scorer must be 1-64 of a-z, 0-9, '.', '_', '-'"})
		return
	case req.ModelID == "":
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model_id is required: every score names the scorer's model"})
		return
	case len(req.Settlements) == 0 || len(req.Settlements) > maxCalibrationSettlements:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("settlements must hold 1 to %d items", maxCalibrationSettlements)})
		return
	}
	ctx := r.Context()
	computerID := rt.TextureComputerID()
	scorerAgent := "scorer:" + req.Scorer
	written, existing := 0, 0
	var refused []calibrationRefusal
	for _, s := range req.Settlements {
		target := strings.TrimSpace(s.TargetRecordID)
		refuse := func(reason string) {
			refused = append(refused, calibrationRefusal{TargetRecordID: target, Reason: reason})
		}
		if target == "" || strings.HasPrefix(target, types.CalibrationScoreRecordPrefix) {
			refuse("target_record_id must name a desk record")
			continue
		}
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, target)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		if rec == nil {
			refuse("no such record on this computer")
			continue
		}
		score := types.CommitmentScore{ScorerModelID: req.ModelID, ScorerAgentID: scorerAgent}
		switch s.ClaimType {
		case types.CalibrationClaimHypothesis:
			if reason := validHypothesisSettlement(*rec, s.Answers); reason != "" {
				refuse(reason)
				continue
			}
			score.Answers = s.Answers
		case types.CalibrationClaimCompleted:
			if rec.Report == nil || !rec.Final || strings.TrimSpace(rec.Report.StopReason) != string(types.WorkItemCompleted) ||
				calibrationDeskOfThread(calibrationThreadOfRecord(*rec)) != agentprofile.Research {
				refuse("a completed settlement targets a final completed research Report")
				continue
			}
			if s.Coverage == nil || !(*s.Coverage >= 0 && *s.Coverage <= 1) {
				refuse("coverage must be a share from 0 to 1")
				continue
			}
			score.Probabilities = map[string]float64{types.CalibrationScoreCoverage: *s.Coverage}
		default:
			refuse(fmt.Sprintf("claim_type must be %q or %q (citations are settled in the guest by the support judge)", types.CalibrationClaimHypothesis, types.CalibrationClaimCompleted))
			continue
		}
		recordID := types.CalibrationScoreRecordPrefix + req.Scorer + ":" + target
		if exists, err := rt.store.CommitmentRecordExists(ctx, ownerID, computerID, recordID); err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		} else if exists {
			existing++
			continue
		}
		stamp := time.Now().UTC().Format(time.RFC3339Nano)
		score.ScoredAt = stamp
		scoreRec := types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID,
			Kind: types.CommitmentKindResolve, Discrepancy: types.DiscrepancyUnresolved,
			Resolve:     &types.CommitmentResolve{Verdict: "calibration_score", EvidenceRefs: s.EvidenceRefs},
			Provenance:  types.CommitmentProvenance{AgentID: scorerAgent, ModelID: req.ModelID, CommittedAt: stamp},
			EmitterKind: types.CommitmentEmitterOwner, ComputerID: computerID,
			RelatedIDs: []string{target}, Scores: []types.CommitmentScore{score},
		}
		if _, err := rt.store.AppendCommitmentRecord(ctx, ownerID, computerID, scoreRec); err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		written++
	}
	if written > 0 {
		rt.dropCalibrationSnapshot(ownerID, computerID)
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"written": written, "existing": existing, "refused": refused})
}

// validHypothesisSettlement checks a hypothesis settlement against its
// target: a Cast naming every answered hypothesis id, and outcomes from the
// closed set. Returns the refusal reason, or "".
func validHypothesisSettlement(rec types.CommitmentRecord, answers map[string]string) string {
	if !rec.IsCast() || rec.Cast == nil || len(rec.Cast.Hypotheses) == 0 {
		return "a hypothesis settlement targets a Cast that opened hypotheses"
	}
	if len(answers) == 0 {
		return "answers must map hypothesis ids to outcomes"
	}
	ids := map[string]bool{}
	for _, h := range rec.Cast.Hypotheses {
		ids[h.ID] = true
	}
	for id, outcome := range answers {
		if !ids[id] {
			return fmt.Sprintf("the Cast opened no hypothesis %q", id)
		}
		if _, ok := hypothesisOutcome(outcome); !ok && strings.TrimSpace(outcome) != string(types.DiscrepancyUnresolved) {
			return fmt.Sprintf("outcome %q must be confirmed, qualified, contradicted or unresolved", outcome)
		}
	}
	return ""
}

// dropCalibrationSnapshot forgets a computer's snapshot so the next read
// rebuilds it.
func (rt *Runtime) dropCalibrationSnapshot(ownerID, computerID string) {
	rt.calibration.mu.Lock()
	defer rt.calibration.mu.Unlock()
	delete(rt.calibration.byComputer, strings.TrimSpace(ownerID)+"\x00"+strings.TrimSpace(computerID))
}
