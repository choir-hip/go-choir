package types

import (
	"fmt"
	"strings"
)

// Desk-to-desk Casts are settled by the judge (plan 4.4, amendment
// 2026-11-05; owner: "Yes, continue judging with system1 models").
//
// A desk-to-desk Cast (Texture to research, management to Texture or
// engineering) is an inbox record that states p_success and weight
// (MA1.3). With settlement "recipient" the recipient's answer in the round
// is the observation and the host's judge settles it, blind, once the round
// closes; the recipient never settles (it is a party to the Cast). The
// judge's records ride the sub-call ledger with plane "desk_cast":
//
//   - judge work, one per round, written by the store with the round's
//     first scorable Cast, so the work is durable before the outcome and
//     before any coalesce sleep;
//   - the round's outcome, written by the store in the batch that closes the
//     round, naming the closing record and its reason. The judge never runs
//     on a round without one;
//   - settlements per Cast, closes per work, as for sub-calls.

// SubCallPlaneDeskCast marks a sub-call-ledger record of a desk-to-desk
// Cast's judgment. A record with no plane is a sub-call's.
const SubCallPlaneDeskCast = "desk_cast"

// SubCallDeskOutcome is the kind of the round outcome record: the round of
// a desk-to-desk Cast closed, so its answer is known and judgment is owed.
const SubCallDeskOutcome = "desk_outcome"

// DeskCastRoundKey names one round of one conversation.
func DeskCastRoundKey(conversationID string, round int) string {
	return fmt.Sprintf("%s:r%d", strings.TrimSpace(conversationID), round)
}

// DeskCastJudgeWorkID is the judge's work on a round. attempt 0 is the
// store's, written with the round's first scorable Cast; a planned
// restart's one resume is attempt 1, and so on.
func DeskCastJudgeWorkID(roundKey string, attempt int) string {
	if attempt <= 0 {
		return "deskjudge:" + roundKey
	}
	return fmt.Sprintf("deskjudge:%s:attempt%d", roundKey, attempt)
}

// DeskCastOutcomeRecordID is the outcome record of a round.
func DeskCastOutcomeRecordID(roundKey string) string { return "deskjudge:" + roundKey + ":outcome" }

// IsDeskCastScorable reports a desk-to-desk Cast the judge settles: a Cast
// by a desk (not the system) that states p_success and settles "recipient",
// and is not a management task's Cast (those are scored as sub-calls,
// MA1.5) nor itself a sub-call record.
func IsDeskCastScorable(rec CommitmentRecord) bool {
	if !rec.IsCast() || rec.SubCall != nil || rec.EmitterKind == CommitmentEmitterSystem {
		return false
	}
	if rec.PSuccess == nil || rec.Settlement != SettlementRecipient {
		return false
	}
	if strings.TrimSpace(rec.Inbox) == "" || strings.TrimSpace(rec.SenderInbox) == "" {
		return false
	}
	if _, task := ManagementTaskIDOfInbox(rec.Inbox); task {
		return false
	}
	return true
}

// DeskCastJudgeWork builds the judge work record of a round.
func DeskCastJudgeWork(computerID, roundKey string, attempt int, committedAt string) CommitmentRecord {
	return CommitmentRecord{
		SchemaID: CommitmentRecordSchemaV1, RecordID: DeskCastJudgeWorkID(roundKey, attempt),
		Kind: CommitmentKindResolve, Discrepancy: DiscrepancyUnresolved,
		Observation: CommitmentObservation{ObservedAt: committedAt},
		Provenance:  CommitmentProvenance{AgentID: SubCallJudgeScorerID, CommittedAt: committedAt, Origin: OriginSystem},
		EmitterKind: CommitmentEmitterSystem, ComputerID: computerID, Addressee: SubCallLedgerAddressee,
		SubCall: &CommitmentSubCall{Kind: SubCallJudgeWork, Plane: SubCallPlaneDeskCast, FanOutID: roundKey, Phase: "requested"},
	}
}

// IsRestartRoundClosure reports whether the record is the host's boot
// closure of an open round: a system-emitted final Report under the
// deterministic restart closure id and the closure agent.
func IsRestartRoundClosure(rec CommitmentRecord) bool {
	return rec.EmitterKind == CommitmentEmitterSystem && rec.Provenance.AgentID == closureSystemAgentID &&
		rec.Final && rec.RecordID == RoundClosureRecordID(rec.ConversationID, rec.Round, ClosureReasonInterruptedByRestart)
}

// DeskCastOutcome builds the outcome record of a round that closing closed.
func DeskCastOutcome(computerID, roundKey string, closing CommitmentRecord, committedAt string) CommitmentRecord {
	// The outcome's Reason is host-minted: it names the restart fate only
	// for the store's own system closure record. A desk's Report.StopReason
	// is desk-supplied text and never reaches it.
	reason := ""
	if IsRestartRoundClosure(closing) {
		reason = ClosureReasonInterruptedByRestart
	}
	return CommitmentRecord{
		SchemaID: CommitmentRecordSchemaV1, RecordID: DeskCastOutcomeRecordID(roundKey),
		Kind: CommitmentKindResolve, Discrepancy: DiscrepancyUnresolved,
		Observation: CommitmentObservation{Excerpt: reason, SourceRef: closing.RecordID, ObservedAt: committedAt},
		Provenance:  CommitmentProvenance{AgentID: SubCallJudgeScorerID, CommittedAt: committedAt, Origin: OriginSystem},
		EmitterKind: CommitmentEmitterSystem, ComputerID: computerID, ParentID: DeskCastJudgeWorkID(roundKey, 0),
		Addressee: SubCallLedgerAddressee,
		SubCall: &CommitmentSubCall{Kind: SubCallDeskOutcome, Plane: SubCallPlaneDeskCast, FanOutID: roundKey,
			Reason: reason},
	}
}
