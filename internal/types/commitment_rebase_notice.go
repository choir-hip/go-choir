package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A stack rebase notice tells the root Texture RLM that the stack of applied
// self-development changes sits on a base the computer no longer booted
// (docs/design/u4-stack-rebase-build-2026-10-26.md, route D; owner, 2026-10-26:
// "we need to rebase not reject security updates", "it shouldn't be a
// platform-minted assignment. Use the root texture rlm"). It has the owner-act
// record's shape: a system Note from a non-desk sender, selfdev-rebase:<op>,
// so it opens and closes no round and is no desk's speech, and it wakes its
// reader. It is minted only by the runtime's live timer, never at a boot, and
// it asks the root for one Cast to management that names the notice.
//
// Attempt n of the pair (operation, booted base) has the record id
// rebase-notice:<op>:<base>:<n>, n in 1..RebaseNoticeMaxAttempts. The pair is
// spent once an assignment claims a notice (StackRebaseClaim).

const (
	// RebaseNoticeAgentID is a rebase notice's provenance.
	RebaseNoticeAgentID = "system:rebase-notice"
	// RebaseOwnerRetryAgentID is the provenance of a notice the owner's retry
	// act minted (slice 8): attempts above the cap, each one the owner's.
	RebaseOwnerRetryAgentID = "system:rebase-owner-retry"
	// RebaseNoticeMaxAttempts bounds the automatic attempts per pair. It
	// mirrors the source schedule's crash-loop limit (two crashes close it).
	RebaseNoticeMaxAttempts = 3

	rebaseNoticeSenderDesk   = "selfdev-rebase"
	rebaseNoticeRecordPrefix = "rebase-notice:"
	rebasePairKeyPrefix      = "rebase-pair:"
	// RebaseNoticeCastField is the Cast spec/packet field that names a notice.
	RebaseNoticeCastField = "rebase_notice"
)

// StackRebaseSenderInbox is the sender inbox of a pair's notices:
// selfdev-rebase:<operation id>.
func StackRebaseSenderInbox(operationID string) string {
	return rebaseNoticeSenderDesk + ":" + strings.TrimSpace(operationID)
}

// StackRebaseNoticeID is the record id of attempt n of the pair (operation,
// base). The runtime commits it with the command id equal to the record id.
func StackRebaseNoticeID(operationID, bootedBase string, attempt int) string {
	return rebaseNoticeRecordPrefix + strings.TrimSpace(operationID) + ":" + strings.TrimSpace(bootedBase) + ":" + strconv.Itoa(attempt)
}

// StackRebasePairKey keys the pair (operation, booted base): the claim and the
// attempt count belong to it.
func StackRebasePairKey(operationID, bootedBase string) string {
	return rebasePairKeyPrefix + strings.TrimSpace(operationID) + ":" + strings.TrimSpace(bootedBase)
}

// ParseStackRebaseNoticeID splits a notice id into its operation, base and
// attempt. The operation id may not contain ':'; the attempt is a positive
// integer.
func ParseStackRebaseNoticeID(id string) (operationID, bootedBase string, attempt int, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(id), rebaseNoticeRecordPrefix)
	if !found {
		return "", "", 0, false
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", 0, false
	}
	n, err := strconv.Atoi(parts[2])
	if err != nil || n < 1 {
		return "", "", 0, false
	}
	return parts[0], parts[1], n, true
}

// StackRebaseClaimKey is the key of the claim of attempt n of a pair. The
// automatic attempts (1..RebaseNoticeMaxAttempts) share the pair's one claim;
// each owner retry (an attempt above the cap) is its own generation with its
// own claim, so a spent pair can be re-opened by the owner's act.
func StackRebaseClaimKey(pairKey string, attempt int) string {
	if attempt <= RebaseNoticeMaxAttempts {
		return pairKey
	}
	return pairKey + "#" + strconv.Itoa(attempt)
}

// StackRebaseNewerAttemptIDs are the record ids that would supersede attempt n:
// every number above n up to the first owner retry, then n+1. An owner retry
// is always numbered above the cap, so it can follow attempt 2 with a gap.
func StackRebaseNewerAttemptIDs(operationID, bootedBase string, attempt int) []string {
	last := attempt + 1
	if attempt <= RebaseNoticeMaxAttempts {
		last = RebaseNoticeMaxAttempts + 1
	}
	var out []string
	for n := attempt + 1; n <= last; n++ {
		out = append(out, StackRebaseNoticeID(operationID, bootedBase, n))
	}
	return out
}

// IsStackRebaseNotice reports whether rec is a rebase notice: a system Note
// from selfdev-rebase:<operation> to a Texture inbox, whose id and subject
// refs agree. An automatic attempt is in 1..RebaseNoticeMaxAttempts and has no
// owner stamp; an attempt above the cap is the owner's retry and carries the
// stamp and its provenance. An agent's record is always EmitterAgent from its
// own inbox, so none passes.
func IsStackRebaseNotice(rec CommitmentRecord) bool {
	operationID, base, attempt, ok := ParseStackRebaseNoticeID(rec.RecordID)
	if !ok {
		return false
	}
	ownerRetry := strings.TrimSpace(rec.SubjectRefs["owner_retry"]) == "true"
	agent := strings.TrimSpace(rec.Provenance.AgentID)
	switch {
	case attempt <= RebaseNoticeMaxAttempts && (ownerRetry || agent == RebaseOwnerRetryAgentID):
		return false
	case attempt > RebaseNoticeMaxAttempts && (!ownerRetry || agent != RebaseOwnerRetryAgentID):
		return false
	}
	refs := rec.SubjectRefs
	return rec.EmitterKind == CommitmentEmitterSystem && !rec.Final && rec.Round == 0 &&
		strings.HasPrefix(strings.TrimSpace(rec.Inbox), "texture:") && strings.TrimSpace(rec.Inbox) != "texture:" &&
		strings.TrimSpace(rec.SenderInbox) == StackRebaseSenderInbox(operationID) &&
		strings.TrimSpace(refs["operation_id"]) == operationID &&
		strings.TrimSpace(refs["booted_base"]) == base &&
		strings.TrimSpace(refs["attempt"]) == strconv.Itoa(attempt) &&
		rec.Directive != nil && rec.Directive.Subtype == CommitmentDirectiveNote
}

// StackRebaseNotice is one attempt's notice, addressed to the root Texture
// RLM of the current master document.
type StackRebaseNotice struct {
	TextureInbox string
	ComputerID   string
	OperationID  string
	// BootedBase is B', the base the computer now runs; StackBase is B, the
	// base the applied stack's patch was built on.
	BootedBase string
	StackBase  string
	Attempt    int
	DocID      string
	// OwnerRetry marks a notice the owner's retry act minted: required above
	// RebaseNoticeMaxAttempts, refused at or below it.
	OwnerRetry bool
}

// NewStackRebaseNotice builds the notice. It returns false when the notice
// does not bind: no Texture inbox, operation or base, or an attempt outside
// 1..RebaseNoticeMaxAttempts.
func NewStackRebaseNotice(n StackRebaseNotice) (CommitmentRecord, bool) {
	inbox := strings.TrimSpace(n.TextureInbox)
	op, booted, stack := strings.TrimSpace(n.OperationID), strings.TrimSpace(n.BootedBase), strings.TrimSpace(n.StackBase)
	if !strings.HasPrefix(inbox, "texture:") || inbox == "texture:" || op == "" || strings.Contains(op, ":") || booted == "" || strings.Contains(booted, ":") || stack == "" ||
		n.Attempt < 1 || (n.Attempt > RebaseNoticeMaxAttempts) != n.OwnerRetry {
		return CommitmentRecord{}, false
	}
	id := StackRebaseNoticeID(op, booted, n.Attempt)
	refs := map[string]string{
		"operation_id": op, "booted_base": booted, "stack_base": stack, "attempt": strconv.Itoa(n.Attempt),
	}
	if doc := strings.TrimSpace(n.DocID); doc != "" {
		refs["doc_id"] = doc
	}
	agent, body := RebaseNoticeAgentID, stackRebaseNoticeBody(op, booted, stack, id)
	if n.OwnerRetry {
		refs["owner_retry"] = "true"
		agent, body = RebaseOwnerRetryAgentID, "The owner asked for another try. "+body
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    id,
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: agent},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  strings.TrimSpace(n.ComputerID),
		Inbox:       inbox, SenderInbox: StackRebaseSenderInbox(op), Addressee: inbox,
		Directive:    &CommitmentDirective{Subtype: CommitmentDirectiveNote, Body: stackRebaseNoticeBody(op, booted, stack, id)},
		EvidenceRefs: []string{op},
		SubjectRefs:  refs,
	}, true
}

func stackRebaseNoticeBody(op, booted, stack, id string) string {
	packet := fmt.Sprintf(`map[string]any{"kind": "execution_request", "summary": "Rebase the applied self-development change %s onto the platform base %s.", "rebase_notice": %q}`, op, booted, id)
	return fmt.Sprintf("Rebase notice %s. The computer runs a newer platform base (%s) than the one its applied self-development change %s was built on (%s). "+
		"The applied change is kept, and new engineering casts are refused until it is rebased. This is the platform's request, not the owner's: it asks for a rebase of the change as it is, not new work. "+
		"Make this one Cast to management and nothing else, copying the notice id exactly:\n\nchoir.Cast(\"management\", \"Rebase the applied change %s onto base %s (rebase notice %s)\", %s)",
		id, booted, op, stack, op, booted, id, packet)
}

// CastRebaseNotice reads the rebase_notice field of a Cast's body (the spec a
// management Cast passes, or the packet a Texture Cast carries), or "" when the
// body is not a JSON object naming one.
func CastRebaseNotice(body string) string {
	body = strings.TrimSpace(body)
	if body == "" || body == "null" {
		return ""
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &probe) != nil {
		return ""
	}
	raw, ok := probe[RebaseNoticeCastField]
	if !ok {
		return ""
	}
	var id string
	if json.Unmarshal(raw, &id) != nil {
		return ""
	}
	return strings.TrimSpace(id)
}

// StackRebaseClaim is the claim of one notice by one assignment: written once
// per pair by the assignment open's own transaction. Its existence means the
// pair is spent (capsule work has begun on it).
type StackRebaseClaim struct {
	PairKey      string    `json:"pair_key"`
	OperationID  string    `json:"operation_id"`
	BootedBase   string    `json:"booted_base"`
	NoticeID     string    `json:"notice_id"`
	Attempt      int       `json:"attempt"`
	AssignmentID string    `json:"assignment_id"`
	ClaimedAt    time.Time `json:"claimed_at"`
}
