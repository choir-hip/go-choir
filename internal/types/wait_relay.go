package types

import (
	"fmt"
	"strings"
	"time"
)

// An engineering wait relay is the progress relay's shape for a wait
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md): the
// platform telling the Texture an assignment serves that the assignment is
// waiting, and later that the wait ended, with the minutes waited by
// cause. It is a system Note from progress:<assignment>, like the relay of
// a Report, so it wakes Texture, opens and closes no round, and is no
// desk's speech; Texture stays the only writer of the document.
//
// One record per wait episode and phase: the id is derived from the
// assignment, its attempt, the episode's ordinal and the phase, so the
// re-checks of one wait append nothing (the line stays one line), and the
// end is its own record.

// Wait relay phases.
const (
	WaitRelayStart = "start"
	WaitRelayEnd   = "end"
)

// WaitRelayRecordID is the id of one wait episode's relay phase.
//
// episode names the wait within the attempt; the runtime makes it from the
// wait's start time as well as its ordinal, so a tracker that a planned
// restart reset never reuses an id already committed with another body.
func WaitRelayRecordID(engineeringInbox string, attempt uint64, episode, phase string) string {
	assignment := strings.TrimPrefix(strings.TrimSpace(engineeringInbox), "engineering:")
	return fmt.Sprintf("%swait:%s:%d:%s:%s", progressRecordPrefix, assignment, attempt, strings.TrimSpace(episode), phase)
}

// NewEngineeringWaitRelay builds one wait relay. refs are added to its
// subject refs (wait_kind, waited seconds by kind). It returns false when
// the inbox is not a Texture's, the sender is not an engineering
// assignment, or the body is empty.
func NewEngineeringWaitRelay(engineeringInbox, textureInbox, computerID string, attempt uint64, episode, phase, body string,
	at time.Time, refs map[string]string) (CommitmentRecord, bool) {
	engineeringInbox, textureInbox, body = strings.TrimSpace(engineeringInbox), strings.TrimSpace(textureInbox), strings.TrimSpace(body)
	if body == "" || !strings.HasPrefix(textureInbox, "texture:") || !strings.HasPrefix(engineeringInbox, "engineering:") ||
		engineeringInbox == "engineering:" || strings.TrimSpace(episode) == "" || (phase != WaitRelayStart && phase != WaitRelayEnd) {
		return CommitmentRecord{}, false
	}
	subject := map[string]string{
		"assignment_inbox": engineeringInbox,
		"wait_episode":     fmt.Sprintf("%d:%s", attempt, strings.TrimSpace(episode)),
		"wait_phase":       phase,
	}
	for k, v := range refs {
		if _, taken := subject[k]; !taken && strings.TrimSpace(k) != "" {
			subject[k] = v
		}
	}
	stamp := at.UTC().Format(time.RFC3339Nano)
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    WaitRelayRecordID(engineeringInbox, attempt, episode, phase),
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: ProgressRelayAgentID, CommittedAt: stamp},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  computerID,
		Inbox:       textureInbox, SenderInbox: ProgressSenderInbox(engineeringInbox), Addressee: textureInbox,
		Directive:   &CommitmentDirective{Subtype: CommitmentDirectiveNote, Body: body},
		SubjectRefs: subject,
	}, true
}

// AssignmentWaitView is management's view of one open engineering
// assignment's waiting: its current wait (nil when it is not waiting) and
// the time it has waited this attempt, by kind (ActingPack.Waits).
type AssignmentWaitView struct {
	AssignmentID string     `json:"assignment_id"`
	Attempt      uint64     `json:"attempt"`
	Wait         *WaitState `json:"wait,omitempty"`
	// WaitedMinutes is the attempt's minutes waited by kind; WaitedSummary
	// says it in words ("waited 13 min: host memory 10 min, ...").
	WaitedMinutes map[string]int64 `json:"waited_minutes,omitempty"`
	WaitedSummary string           `json:"waited_summary,omitempty"`
}
