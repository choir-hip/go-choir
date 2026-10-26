package types

import "sort"

// EventDurability says how a runtime event kind reaches the tape.
//
// It belongs to the kind, never to the caller: a call site has no parameter
// to opt in or out (docs/reports/telemetry-batching-design-2026-10-20.md,
// principle 2). The default for anything not named below is acknowledged.
type EventDurability string

const (
	// DurabilityAcknowledged is one acknowledged tape write per event, exactly
	// as before telemetry batching existed.
	DurabilityAcknowledged EventDurability = "acknowledged"
	// DurabilityBatched marks high-volume, low-authority run telemetry that the
	// standalone Store.AppendEvent path may seal with other telemetry into one
	// tape event, when the batching flag is on. A crash may lose the open
	// window of it (owner ruling 2026-10-14, notes section 27 item 18).
	DurabilityBatched EventDurability = "batched"
)

// eventDurability is the closed classification of every EventKind. A kind is
// batched only if it is named here with DurabilityBatched. The parser test in
// event_durability_test.go fails if an EventKind constant has no entry, so a
// new kind cannot ship unclassified.
//
// types.LiveOnly (event_live_only.go) names the lines the runtime no longer
// appends at all (tool.invoked, the Texture progress copy, the request-side
// loop.progress); they stay classified here because old tapes hold them and
// another caller may still append them.
//
// Ten kinds batch: the per-step trace lines. Every other kind is a run state
// transition, an owner act, evidence, or a Texture canonical-revision event,
// and stays one acknowledged write (design section 3.1).
var eventDurability = map[EventKind]EventDurability{
	// Batched: trace lines. The cost lines of loop.progress feed an estimate
	// endpoint; tool.result is read as evidence by run acceptance and the
	// Texture workflow verifier, which read through the sealing (Consistent)
	// list API.
	EventRunDelta:                     DurabilityBatched,
	EventRunProgress:                  DurabilityBatched,
	EventRunRetry:                     DurabilityBatched,
	EventRunCompactionStarted:         DurabilityBatched,
	EventRunCompactionCompleted:       DurabilityBatched,
	EventToolInvoked:                  DurabilityBatched,
	EventToolResult:                   DurabilityBatched,
	EventDeskCell:                     DurabilityBatched,
	EventDeskEgress:                   DurabilityBatched,
	EventTextureAgentRevisionProgress: DurabilityBatched,

	// Acknowledged: run state transitions. A few per run; terminal events are
	// what a boundary reads.
	EventRunSubmitted:  DurabilityAcknowledged,
	EventRunStarted:    DurabilityAcknowledged,
	EventRunCompleted:  DurabilityAcknowledged,
	EventRunFailed:     DurabilityAcknowledged,
	EventRunBlocked:    DurabilityAcknowledged,
	EventRunCancelled:  DurabilityAcknowledged,
	EventRunPassivated: DurabilityAcknowledged,

	// Acknowledged: operational incident signal, ownerless and rare.
	EventRuntimeHealth:   DurabilityAcknowledged,
	EventRuntimeDegraded: DurabilityAcknowledged,

	// Acknowledged: inter-agent coordination record.
	EventChannelMessage: DurabilityAcknowledged,

	// Acknowledged: a System1 call is a paid, replayed-from-the-tape fact. A
	// lost record would be a second spend on replay.
	EventSystem1Judgment: DurabilityAcknowledged,

	// Acknowledged: a judged pair relation is the authority for a public
	// graph edge; a lost record would leave an edge with no record behind it.
	EventStoryRelation: DurabilityAcknowledged,

	// Acknowledged: a story's entity attachments are the authority for public
	// graph rows and edges, as a judged pair is.
	EventStoryEntities: DurabilityAcknowledged,

	// Acknowledged: the owner's approval and the send are acts and evidence.
	EventEmailDraftApprovalRecorded: DurabilityAcknowledged,
	EventEmailDraftBlocked:          DurabilityAcknowledged,
	EventEmailDraftSent:             DurabilityAcknowledged,

	// Acknowledged: owner-visible browser session acts.
	EventBrowserSessionCreated:      DurabilityAcknowledged,
	EventBrowserSessionClosed:       DurabilityAcknowledged,
	EventBrowserNavigationCompleted: DurabilityAcknowledged,
	EventBrowserNavigationFailed:    DurabilityAcknowledged,
	EventBrowserControlCompleted:    DurabilityAcknowledged,
	EventBrowserControlFailed:       DurabilityAcknowledged,

	// Acknowledged: the Texture desk's canonical-revision and decision events.
	// Texture is the sole writer of canonical revisions; editors follow these.
	EventTextureAgentRevisionStarted:    DurabilityAcknowledged,
	EventTextureAgentRevisionCompleted:  DurabilityAcknowledged,
	EventTextureAgentRevisionFailed:     DurabilityAcknowledged,
	EventTextureDocumentRevisionCreated: DurabilityAcknowledged,
	EventTextureDecisionRecorded:        DurabilityAcknowledged,

	// Acknowledged: product state events (owner acts), low volume.
	EventDesktopDriverLeaseUpdated:     DurabilityAcknowledged,
	EventDesktopAppInstancesUpdated:    DurabilityAcknowledged,
	EventDesktopWindowPlacementUpdated: DurabilityAcknowledged,
	EventContentItemCreated:            DurabilityAcknowledged,
	EventMediaProgressUpdated:          DurabilityAcknowledged,
	EventMediaRecentUpdated:            DurabilityAcknowledged,
	EventThemeUpdated:                  DurabilityAcknowledged,
	EventFileChanged:                   DurabilityAcknowledged,
}

// Durability returns the kind's class. A kind the table does not name is
// acknowledged, so an unknown or mistyped kind can never batch.
func (k EventKind) Durability() EventDurability {
	if d, ok := eventDurability[k]; ok {
		return d
	}
	return DurabilityAcknowledged
}

// Batchable reports whether the kind may be sealed in a telemetry window.
func (k EventKind) Batchable() bool {
	return k.Durability() == DurabilityBatched
}

// EventKinds returns every classified runtime event kind, sorted. Tests use it
// to exercise each kind; the parser test keeps it equal to the constants.
func EventKinds() []EventKind {
	kinds := make([]EventKind, 0, len(eventDurability))
	for k := range eventDurability {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return kinds
}
