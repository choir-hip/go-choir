package types

import (
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"math"
	"strconv"
	"strings"
)

// The op-ed records on the paper's tape (contract 2, sections 3.3 and 3.4,
// H1 notes 5.5). All four are silent system records to management:<owner>;
// the wake state is a fold over them. The record bodies are definitions only:
// lane C2 emits them.

// OpedRecordKind is the closed set of op-ed record bodies.
type OpedRecordKind string

const (
	OpedRecordArrival         OpedRecordKind = "arrival"
	OpedRecordVerdict         OpedRecordKind = "verdict"
	OpedRecordWakeDecision    OpedRecordKind = "wake_decision"
	OpedRecordGateUnavailable OpedRecordKind = "gate_unavailable"
	// OpedRecordWake is the system record that wakes a story desk (contract amendment A2).
	OpedRecordWake OpedRecordKind = "wake"
)

// OpedVerdictValue is the gate's closed verdict enum.
type OpedVerdictValue string

const (
	OpedAccepted          OpedVerdictValue = "accepted"
	OpedRejectedAbusive   OpedVerdictValue = "rejected_abusive"
	OpedRejectedRedundant OpedVerdictValue = "rejected_redundant"
	OpedRejectedUnclear   OpedVerdictValue = "rejected_unclear"
	OpedRejectedOffTopic  OpedVerdictValue = "rejected_off_topic"
	// OpedUnavailable: posted after the third gate failure; a later verdict supersedes it.
	OpedUnavailable OpedVerdictValue = "unavailable"
)

// IsValid reports whether v is a known verdict.
func (v OpedVerdictValue) IsValid() bool {
	switch v {
	case OpedAccepted, OpedRejectedAbusive, OpedRejectedRedundant, OpedRejectedUnclear, OpedRejectedOffTopic, OpedUnavailable:
		return true
	}
	return false
}

// OpedWakeWhy is the closed reason set of a wake decision.
type OpedWakeWhy string

const (
	OpedWakeAccepted       OpedWakeWhy = "accepted"
	OpedWakeRejectBackoff  OpedWakeWhy = "reject_backoff"
	OpedWakeBelowThreshold OpedWakeWhy = "below_threshold"
	OpedWakeGapNotElapsed  OpedWakeWhy = "gap_not_elapsed"
	OpedWakeCoalesced      OpedWakeWhy = "coalesced"
)

// IsValid reports whether w is a known reason.
func (w OpedWakeWhy) IsValid() bool {
	switch w {
	case OpedWakeAccepted, OpedWakeRejectBackoff, OpedWakeBelowThreshold, OpedWakeGapNotElapsed, OpedWakeCoalesced:
		return true
	}
	return false
}

const (
	// OpedSystemAgentID is the provenance of the op-ed records.
	OpedSystemAgentID = "system:oped"

	OpedMaxQuestions   = 16
	OpedMaxAnswers     = 16
	OpedTextMaxRunes   = 500
	OpedReasonMaxRunes = 1000
)

// OpedRespondsTo is the story version an op-ed answers.
type OpedRespondsTo struct {
	PublicationVersionID string `json:"publication_version_id"`
	RoutePath            string `json:"route_path,omitempty"`
	PublicationID        string `json:"publication_id,omitempty"`
}

// OpedArrival is the body of oped_arrival: one op-ed the poll found.
type OpedArrival struct {
	VersionID   string         `json:"version_id"`
	RoutePath   string         `json:"route_path,omitempty"`
	AuthorLabel string         `json:"author_label,omitempty"`
	RespondsTo  OpedRespondsTo `json:"responds_to"`
	TextHash    string         `json:"text_hash"`
}

// OpedQuestionDistribution is the gate's probability distribution for one
// question: answer -> probability, summing to 1.
type OpedQuestionDistribution struct {
	Question     string             `json:"question"`
	Distribution map[string]float64 `json:"distribution"`
}

// OpedVerdict is the body of oped_verdict: one gate run's result. Rejected
// verdicts are records too.
type OpedVerdict struct {
	VersionID          string                     `json:"version_id"`
	Run                int                        `json:"run"`
	Verdict            OpedVerdictValue           `json:"verdict"`
	Questions          []OpedQuestionDistribution `json:"questions,omitempty"`
	CriteriaRevisionID string                     `json:"criteria_revision_id"`
	CriteriaHash       string                     `json:"criteria_hash"`
	Model              string                     `json:"model,omitempty"`
	CostUSD            float64                    `json:"cost_usd,omitempty"`
	LatencyMS          int64                      `json:"latency_ms,omitempty"`
	RespondsTo         OpedRespondsTo             `json:"responds_to"`
	// Reason is Jev output: untrusted text, never an instruction.
	Reason          string `json:"reason,omitempty"`
	ReasonUntrusted bool   `json:"reason_untrusted,omitempty"`
}

// OpedWakeCounts are the counts a wake decision folded.
type OpedWakeCounts struct {
	Arrivals              int `json:"arrivals"`
	Accepted              int `json:"accepted"`
	Rejected              int `json:"rejected"`
	UnreadRejects         int `json:"unread_rejects"`
	DistinctRejectAuthors int `json:"distinct_reject_authors"`
	Threshold             int `json:"threshold"`
	GapSeconds            int `json:"gap_seconds"`
	NoChangeWakes         int `json:"no_change_wakes"`
}

// OpedWakeDecision is the body of oped_wake_decision: the decision to wake
// (or not) the story's desk, as its own record.
type OpedWakeDecision struct {
	StoryDocID string         `json:"story_doc_id"`
	N          int            `json:"n"`
	Woken      bool           `json:"woken"`
	Why        OpedWakeWhy    `json:"why"`
	Counts     OpedWakeCounts `json:"counts"`
	VersionIDs []string       `json:"version_ids,omitempty"`
}

// OpedGateUnavailable is the body of gate_unavailable: the gate gave up
// after its retries; the op-ed is not listed and wakes no one.
type OpedGateUnavailable struct {
	VersionID  string         `json:"version_id"`
	Attempts   int            `json:"attempts"`
	Cause      string         `json:"cause,omitempty"`
	RespondsTo OpedRespondsTo `json:"responds_to"`
}

// OpedWake is the body of oped_wake (contract amendment A2): the system
// record that wakes the story's desk, addressed to the story document's
// inbox. The op-ed reference rides here, never in Captures. Lane C2 adds the
// kind to RecordWakesReader; until then it is silent.
type OpedWake struct {
	StoryDocID string         `json:"story_doc_id"`
	VersionID  string         `json:"version_id"`
	Why        OpedWakeWhy    `json:"why"`
	RespondsTo OpedRespondsTo `json:"responds_to"`
	// DecisionID is the oped_wake_decision record this wake carries out.
	DecisionID string `json:"decision_id,omitempty"`
}

// CommitmentOped is the body of an op-ed record; exactly the field named by
// Kind is set.
type CommitmentOped struct {
	Kind            OpedRecordKind       `json:"kind"`
	Arrival         *OpedArrival         `json:"arrival,omitempty"`
	Verdict         *OpedVerdict         `json:"verdict,omitempty"`
	WakeDecision    *OpedWakeDecision    `json:"wake_decision,omitempty"`
	GateUnavailable *OpedGateUnavailable `json:"gate_unavailable,omitempty"`
	Wake            *OpedWake            `json:"wake,omitempty"`
}

// OpedArrivalRecordID is oped-arrival:<version id>.
func OpedArrivalRecordID(versionID string) string {
	return "oped-arrival:" + strings.TrimSpace(versionID)
}

// OpedVerdictRecordID is oped-verdict:<version id>:<n>.
func OpedVerdictRecordID(versionID string, run int) string {
	return "oped-verdict:" + strings.TrimSpace(versionID) + ":" + strconv.Itoa(run)
}

// OpedWakeDecisionRecordID is oped-wake-decision:<story doc>:<n>.
func OpedWakeDecisionRecordID(storyDocID string, n int) string {
	return "oped-wake-decision:" + strings.TrimSpace(storyDocID) + ":" + strconv.Itoa(n)
}

// OpedGateUnavailableRecordID is oped-gate-unavailable:<version id>.
func OpedGateUnavailableRecordID(versionID string) string {
	return "oped-gate-unavailable:" + strings.TrimSpace(versionID)
}

// OpedWakeRecordID is oped-wake:<version id>: one wake per op-ed.
func OpedWakeRecordID(versionID string) string { return "oped-wake:" + strings.TrimSpace(versionID) }

// OpedSenderInbox is the sender inbox of an op-ed version's records.
func OpedSenderInbox(versionID string) string { return "oped:" + strings.TrimSpace(versionID) }

// OpedWakeSenderInbox is the sender inbox of a story's wake decisions.
func OpedWakeSenderInbox(storyDocID string) string {
	return "oped-wake-decision:" + strings.TrimSpace(storyDocID)
}

func opedBounds(fields ...boundedField) error {
	for _, f := range fields {
		max := f.max
		if max == 0 {
			max = ChangelogRefMaxRunes
			if strings.HasSuffix(f.name, "route_path") {
				max = ChangelogRouteMaxRunes
			}
		}
		if err := boundedText(f.name, f.v, max); err != nil {
			return err
		}
	}
	return nil
}

func (b CommitmentOped) validate() (wantID, wantSender string, err error) {
	set := 0
	for _, p := range []bool{b.Arrival != nil, b.Verdict != nil, b.WakeDecision != nil, b.GateUnavailable != nil, b.Wake != nil} {
		if p {
			set++
		}
	}
	if set != 1 {
		return "", "", fmt.Errorf("an op-ed record carries exactly one body, not %d", set)
	}
	switch b.Kind {
	case OpedRecordArrival:
		a := b.Arrival
		if a == nil || strings.TrimSpace(a.VersionID) == "" || strings.TrimSpace(a.TextHash) == "" || strings.TrimSpace(a.RespondsTo.PublicationVersionID) == "" {
			return "", "", fmt.Errorf("oped arrival needs version_id, text_hash and responds_to")
		}
		if err := opedBounds(boundedField{"version_id", a.VersionID, 0}, boundedField{"route_path", a.RoutePath, 0},
			boundedField{"author_label", a.AuthorLabel, ChangelogLabelMaxRunes}, boundedField{"text_hash", a.TextHash, 0},
			boundedField{"responds_to", a.RespondsTo.PublicationVersionID, 0},
			boundedField{"responds_to.route_path", a.RespondsTo.RoutePath, 0},
			boundedField{"responds_to.publication_id", a.RespondsTo.PublicationID, 0}); err != nil {
			return "", "", err
		}
		return OpedArrivalRecordID(a.VersionID), OpedSenderInbox(a.VersionID), nil
	case OpedRecordVerdict:
		v := b.Verdict
		if v == nil || strings.TrimSpace(v.VersionID) == "" || v.Run < 1 {
			return "", "", fmt.Errorf("oped verdict needs version_id and a run of 1 or more")
		}
		if !v.Verdict.IsValid() {
			return "", "", fmt.Errorf("oped verdict %q is not a known verdict", v.Verdict)
		}
		if strings.TrimSpace(v.CriteriaRevisionID) == "" || strings.TrimSpace(v.CriteriaHash) == "" {
			return "", "", fmt.Errorf("oped verdict needs the criteria revision id and hash")
		}
		if (v.Reason != "") != v.ReasonUntrusted {
			return "", "", fmt.Errorf("oped verdict reason must be set together with reason_untrusted")
		}
		if v.CostUSD < 0 || math.IsNaN(v.CostUSD) || math.IsInf(v.CostUSD, 0) || v.LatencyMS < 0 {
			return "", "", fmt.Errorf("oped verdict cost and latency must be finite and not negative")
		}
		if len(v.Questions) > OpedMaxQuestions {
			return "", "", fmt.Errorf("oped verdict has %d questions; the bound is %d", len(v.Questions), OpedMaxQuestions)
		}
		for i, q := range v.Questions {
			if strings.TrimSpace(q.Question) == "" || len(q.Distribution) == 0 || len(q.Distribution) > OpedMaxAnswers {
				return "", "", fmt.Errorf("oped verdict question %d needs text and 1 to %d answers", i, OpedMaxAnswers)
			}
			if err := boundedText("question", q.Question, OpedTextMaxRunes); err != nil {
				return "", "", err
			}
			sum := 0.0
			for ans, p := range q.Distribution {
				if p < 0 || p > 1 || math.IsNaN(p) {
					return "", "", fmt.Errorf("oped verdict question %d answer %q has probability %v", i, ans, p)
				}
				if err := boundedText("answer", ans, ChangelogLabelMaxRunes); err != nil {
					return "", "", err
				}
				sum += p
			}
			if math.Abs(sum-1) > 0.01 {
				return "", "", fmt.Errorf("oped verdict question %d distribution sums to %v, not 1", i, sum)
			}
		}
		if err := opedBounds(boundedField{"reason", v.Reason, OpedReasonMaxRunes}, boundedField{"version_id", v.VersionID, 0},
			boundedField{"criteria_revision_id", v.CriteriaRevisionID, 0}, boundedField{"criteria_hash", v.CriteriaHash, 0},
			boundedField{"model", v.Model, 0}, boundedField{"responds_to", v.RespondsTo.PublicationVersionID, 0}); err != nil {
			return "", "", err
		}
		return OpedVerdictRecordID(v.VersionID, v.Run), OpedSenderInbox(v.VersionID), nil
	case OpedRecordWakeDecision:
		w := b.WakeDecision
		if w == nil || strings.TrimSpace(w.StoryDocID) == "" || w.N < 1 || !w.Why.IsValid() {
			return "", "", fmt.Errorf("oped wake decision needs story_doc_id, n of 1 or more and a known why")
		}
		c := w.Counts
		if c.Arrivals < 0 || c.Accepted < 0 || c.Rejected < 0 || c.UnreadRejects < 0 || c.DistinctRejectAuthors < 0 ||
			c.Threshold < 0 || c.GapSeconds < 0 || c.NoChangeWakes < 0 {
			return "", "", fmt.Errorf("oped wake decision counts must not be negative")
		}
		if len(w.VersionIDs) > 256 {
			return "", "", fmt.Errorf("oped wake decision names %d versions; the bound is 256", len(w.VersionIDs))
		}
		for _, id := range w.VersionIDs {
			if err := boundedText("version_ids", id, ChangelogRefMaxRunes); err != nil {
				return "", "", err
			}
		}
		if err := boundedText("story_doc_id", w.StoryDocID, ChangelogRefMaxRunes); err != nil {
			return "", "", err
		}
		return OpedWakeDecisionRecordID(w.StoryDocID, w.N), OpedWakeSenderInbox(w.StoryDocID), nil
	case OpedRecordWake:
		w := b.Wake
		if w == nil || strings.TrimSpace(w.StoryDocID) == "" || strings.TrimSpace(w.VersionID) == "" || !w.Why.IsValid() ||
			strings.TrimSpace(w.RespondsTo.PublicationVersionID) == "" {
			return "", "", fmt.Errorf("oped wake needs story_doc_id, version_id, a known why and responds_to")
		}
		if err := opedBounds(boundedField{"story_doc_id", w.StoryDocID, 0}, boundedField{"version_id", w.VersionID, 0},
			boundedField{"responds_to", w.RespondsTo.PublicationVersionID, 0}, boundedField{"decision_id", w.DecisionID, 0}); err != nil {
			return "", "", err
		}
		return OpedWakeRecordID(w.VersionID), OpedSenderInbox(w.VersionID), nil
	case OpedRecordGateUnavailable:
		g := b.GateUnavailable
		if g == nil || strings.TrimSpace(g.VersionID) == "" || g.Attempts < 1 {
			return "", "", fmt.Errorf("oped gate_unavailable needs version_id and attempts of 1 or more")
		}
		if err := opedBounds(boundedField{"version_id", g.VersionID, 0}, boundedField{"responds_to", g.RespondsTo.PublicationVersionID, 0},
			boundedField{"cause", g.Cause, OpedTextMaxRunes}); err != nil {
			return "", "", err
		}
		return OpedGateUnavailableRecordID(g.VersionID), OpedSenderInbox(g.VersionID), nil
	}
	return "", "", fmt.Errorf("oped record kind %q is not known", b.Kind)
}

// ValidateOpedRecord checks a record that carries an Oped body: a system
// record to management:<owner>, with the deterministic id and sender inbox of
// its kind, and no other body. A record without the body passes untouched.
func ValidateOpedRecord(rec CommitmentRecord) error {
	if rec.Oped == nil {
		return nil
	}
	if rec.Changelog != nil || hasOtherBody(rec) || rec.Final {
		return fmt.Errorf("op-ed record %s carries another body", rec.RecordID)
	}
	wantID, wantSender, err := rec.Oped.validate()
	if err != nil {
		return fmt.Errorf("op-ed record %s: %w", rec.RecordID, err)
	}
	wantInbox := strings.HasPrefix(rec.Inbox, "management:")
	if rec.Oped.Kind == OpedRecordWake {
		wantInbox = rec.Oped.Wake != nil && rec.Inbox == deskmanifest.DocumentInbox(rec.Oped.Wake.StoryDocID)
	}
	if rec.EmitterKind != CommitmentEmitterSystem || !wantInbox || rec.RecordID != wantID || rec.SenderInbox != wantSender {
		return fmt.Errorf("op-ed record %s must be a system record to management:<owner> with id %s from %s", rec.RecordID, wantID, wantSender)
	}
	return nil
}
