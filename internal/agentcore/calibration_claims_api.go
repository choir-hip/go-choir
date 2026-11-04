package agentcore

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/calibration/claims[?computer_id=&assignment=|&operation=]: every
// rule-settled precommit on the computer (an engineering round's Cast
// predictions and engineering's own), stated against what happened. The
// owner's read, like /api/calibration: not on management's ProductAPI
// allowlist, so no desk reads its raw outcomes (P11).
//
// Failure modes, written before the code:
//
//	C1 It lists another store (the agent-claim store /api/precommit/), so a
//	   round's predictions read as none (lane C, crouton 05:28Z).
//	   -> it reads the rule precommits themselves (ListRulePrecommitRecords,
//	      or every commitment record for the whole computer).
//	C2 A computer_id naming another computer reads this one's claims as
//	   that one's.
//	   -> a computer_id other than the serving computer is refused.
//	C3 An operation filter misses a claim whose round froze several
//	   revisions.
//	   -> the operation names its assignment, and every claim of the
//	      assignment is listed with the operation its settlement cites.

// calibrationClaimsGovern says which statement of a round is its record
// (live8, luna r3: Texture's Cast packet and management's Cast to
// engineering both stated round numbers, and they differed). Management
// casts the round, so its Cast's predictions are the round's claims; the
// desk field names the stater (management). The calibration digest counts
// them on the round's engineering record, as the 2026-11-03 spec says
// ("these records feed the engineering digest"). Numbers in a Texture
// packet are text Texture sent management: they are not recorded as claims,
// never a second set for the round.
const calibrationClaimsGovern = "management's Cast to engineering states the round's claims (desk management, stated_by cast; the digest counts them on the round's engineering record); engineering's own Precommits are desk engineering (stated_by precommit); numbers in a Texture Cast packet are not claims"

// calibrationClaimsLimit bounds the whole-computer listing.
const calibrationClaimsLimit = 20000

// calibrationClaim is one rule precommit, stated against actual.
type calibrationClaim struct {
	RecordID   string  `json:"id"`
	Desk       string  `json:"desk"`
	StatedBy   string  `json:"stated_by,omitempty"`
	AgentID    string  `json:"agent_id"`
	ModelID    string  `json:"model_id,omitempty"`
	Kind       string  `json:"kind"`
	Question   string  `json:"question"`
	P          float64 `json:"p"`
	Estimate   int     `json:"estimate_minutes,omitempty"`
	Low        int     `json:"low_minutes,omitempty"`
	High       int     `json:"high_minutes,omitempty"`
	Within     int     `json:"within_minutes,omitempty"`
	WithinFrz  int     `json:"within_rounds,omitempty"`
	StatedAt   string  `json:"stated_at"`
	Late       bool    `json:"late"`
	LateReason string  `json:"late_reason,omitempty"`
	Arm        string  `json:"arm,omitempty"`
	Assignment string  `json:"assignment"`
	Operation  string  `json:"operation,omitempty"`
	Cast       string  `json:"cast,omitempty"`
	// The settlement: empty Answer is open.
	Answer        string `json:"answer,omitempty"`
	Excerpt       string `json:"excerpt,omitempty"`
	ActualS       *int64 `json:"actual_s,omitempty"`
	Censored      bool   `json:"censored,omitempty"`
	SettledAt     string `json:"settled_at,omitempty"`
	DecisionActor string `json:"decision_actor,omitempty"`
	Delegated     bool   `json:"delegated,omitempty"`
}

// calibrationClaimOf joins a rule precommit with its rule settlement.
func calibrationClaimOf(rec types.CommitmentRecord, resolve *types.CommitmentRecord, operationOf func(assignment string) string) calibrationClaim {
	rule := rec.Precommit.Rule
	refs := rec.SubjectRefs
	desk, _, _ := strings.Cut(strings.TrimSpace(rec.Provenance.AgentID), ":")
	c := calibrationClaim{
		RecordID: rec.RecordID, Desk: desk, StatedBy: refs[types.RuleRefStatedBy], AgentID: rec.Provenance.AgentID, ModelID: rec.Provenance.ModelID,
		Kind: string(rule.Kind), Question: rec.Precommit.Question, P: rec.Precommit.YesProbability(),
		Estimate: rule.EstimateMinutes, Low: rule.LowMinutes, High: rule.HighMinutes, Within: rule.WithinMinutes, WithinFrz: rule.WithinRounds,
		StatedAt: firstNonEmpty(refs[types.RuleRefStatedAt], rec.Provenance.CommittedAt),
		Late:     types.LateRulePrecommit(rec), LateReason: refs[types.RuleRefLateReason], Arm: refs[calibrationArmRef],
		Assignment: rule.AssignmentID, Cast: refs["cast"],
	}
	if resolve != nil && resolve.Resolve != nil {
		c.Answer, c.Excerpt = resolve.Resolve.Verdict, resolve.Observation.Excerpt
		c.SettledAt = firstNonEmpty(resolve.Provenance.ResolvedAt, resolve.Observation.ObservedAt)
		rr := resolve.SubjectRefs
		if n, err := strconv.ParseInt(rr[types.RuleRefActualS], 10, 64); err == nil {
			c.ActualS = &n
		}
		c.Censored = rr[types.RuleRefCensored] == "true"
		c.DecisionActor, c.Delegated = rr[types.RuleRefDecisionActor], rr[types.RuleRefDelegated] == "true"
		for _, ref := range resolve.EvidenceRefs {
			if op, ok := strings.CutPrefix(ref, "selfdev-operation:"); ok && c.Operation == "" {
				c.Operation = op
			}
		}
	}
	if c.Operation == "" && operationOf != nil {
		c.Operation = operationOf(c.Assignment)
	}
	return c
}

// HandleCalibrationClaims serves GET /api/calibration/claims.
func (h *APIHandler) HandleCalibrationClaims(w http.ResponseWriter, r *http.Request) {
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
	q := r.URL.Query()
	if want := strings.TrimSpace(q.Get("computer_id")); want != "" && want != computerID {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "computer_id " + want + " is not this computer (" + computerID + "); target it with X-Choir-Computer"})
		return
	}
	assignment, operation := strings.TrimSpace(q.Get("assignment")), strings.TrimSpace(q.Get("operation"))
	if assignment != "" && operation != "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "filter by assignment or by operation, not both"})
		return
	}
	if operation != "" {
		if rt.selfdevOperations == nil {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development operations unavailable"})
			return
		}
		op, err := rt.selfdevOperations.Get(ctx, computerID, operation)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "operation " + operation + ": " + err.Error()})
			return
		}
		if assignment = ownerActAssignmentID(op); assignment == "" {
			writeAPIJSON(w, http.StatusOK, map[string]any{"computer_id": computerID, "claims": []calibrationClaim{}})
			return
		}
	}
	claims, err := rt.calibrationClaims(ctx, ownerID, computerID, assignment)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"computer_id": computerID, "assignment": assignment, "governs": calibrationClaimsGovern, "claims": claims})
}

// calibrationClaims lists the rule precommits of one assignment, or of the
// whole computer, oldest first, each with its settlement.
func (rt *Runtime) calibrationClaims(ctx context.Context, ownerID, computerID, assignment string) ([]calibrationClaim, error) {
	var precommits []types.CommitmentRecord
	resolves := map[string]*types.CommitmentRecord{}
	if assignment != "" {
		listed, err := rt.store.ListRulePrecommitRecords(ctx, ownerID, computerID, assignment)
		if err != nil {
			return nil, err
		}
		precommits = listed
		for _, rec := range listed {
			if resolve, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.RuleResolveRecordID(rec.RecordID)); err == nil && resolve != nil {
				resolves[rec.RecordID] = resolve
			}
		}
	} else {
		all, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, "", calibrationClaimsLimit)
		if err != nil {
			return nil, err
		}
		for i, rec := range all {
			switch {
			case rec.Resolve != nil && rec.Provenance.AgentID == types.RuleScorerAgentID && rec.ParentID != "":
				if _, seen := resolves[rec.ParentID]; !seen {
					resolves[rec.ParentID] = &all[i]
				}
			case rec.Precommit != nil && rec.Precommit.Rule != nil && rec.Resolve == nil:
				precommits = append(precommits, rec)
			}
		}
	}
	operations := map[string]string{}
	operationOf := func(assignment string) string {
		if assignment == "" || rt.selfdevOperations == nil {
			return ""
		}
		if op, seen := operations[assignment]; seen {
			return op
		}
		op, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey(assignment))
		id := ""
		if err == nil && found {
			id = op.OperationID
		}
		operations[assignment] = id
		return id
	}
	out := make([]calibrationClaim, 0, len(precommits))
	for _, rec := range precommits {
		out = append(out, calibrationClaimOf(rec, resolves[rec.RecordID], operationOf))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ti, tj := parseRoundTime(out[i].StatedAt), parseRoundTime(out[j].StatedAt); !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return out[i].RecordID < out[j].RecordID
	})
	return out, nil
}
