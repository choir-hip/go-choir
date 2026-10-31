package types

// EventStorage says where a choir.event is stored once the writer stamps it:
// which table (docs/archive/design/choir-event-table-f-c-2026-10-22.md section 2.1).
// There is no trace log class: the R7 trace segment log was deleted 2026-10-31
// (docs/design/one-tape-durability-2026-10-31.md) and traces stay on the tape.
//
// It is a decision of its own, beside durability: a change to the batching
// table must never move rows between tables without someone deciding it. The
// two are pinned together today by a test (a batched kind is in the event
// table, and nothing else is), so a change to either fails
// until both are decided. The default for anything not named below is the
// object graph, so an unknown or mistyped kind stays where every reader
// expects it.
//
// The class decides only what the writer stamps. The projector routes on the
// stamp on the tape, never on this class, so a later binary may add a kind
// here without making an earlier binary refuse its tape.
type EventStorage string

const (
	// StorageObjectGraph keeps the event in og_objects, as before F-c.
	StorageObjectGraph EventStorage = "object_graph"
	// StorageEventTable moves the event to choir_events (4 prolly trees per
	// insert instead of 13).
	StorageEventTable EventStorage = "event_table"
)

// eventStorage is the closed storage classification of every EventKind. The
// parser test in event_durability_test.go fails if a constant has no entry.
var eventStorage = map[EventKind]EventStorage{
	// Trace lines: the ten per-step lines, about 94% of appends, all in the
	// event table.
	EventRunDelta:                     StorageEventTable,
	EventRunProgress:                  StorageEventTable,
	EventRunRetry:                     StorageEventTable,
	EventRunCompactionStarted:         StorageEventTable,
	EventRunCompactionCompleted:       StorageEventTable,
	EventToolInvoked:                  StorageEventTable,
	EventToolResult:                   StorageEventTable,
	EventDeskCell:                     StorageEventTable,
	EventDeskEgress:                   StorageEventTable,
	EventTextureAgentRevisionProgress: StorageEventTable,

	// Object graph: run state transitions, which the lifecycle fold writes
	// under an absent-condition on og_objects.
	EventRunSubmitted:  StorageObjectGraph,
	EventRunStarted:    StorageObjectGraph,
	EventRunCompleted:  StorageObjectGraph,
	EventRunFailed:     StorageObjectGraph,
	EventRunBlocked:    StorageObjectGraph,
	EventRunCancelled:  StorageObjectGraph,
	EventRunPassivated: StorageObjectGraph,

	// Object graph: incident signals, coordination, and the System1
	// judgment, which the acknowledged channel read serves.
	EventRuntimeHealth:   StorageObjectGraph,
	EventRuntimeDegraded: StorageObjectGraph,
	EventChannelMessage:  StorageObjectGraph,
	EventSystem1Judgment: StorageObjectGraph,
	EventStoryRelation:   StorageObjectGraph,
	EventStoryEntities:   StorageObjectGraph,

	// Object graph: owner acts and evidence.
	EventEmailDraftApprovalRecorded:     StorageObjectGraph,
	EventEmailDraftBlocked:              StorageObjectGraph,
	EventEmailDraftSent:                 StorageObjectGraph,
	EventBrowserSessionCreated:          StorageObjectGraph,
	EventBrowserSessionClosed:           StorageObjectGraph,
	EventBrowserNavigationCompleted:     StorageObjectGraph,
	EventBrowserNavigationFailed:        StorageObjectGraph,
	EventBrowserControlCompleted:        StorageObjectGraph,
	EventBrowserControlFailed:           StorageObjectGraph,
	EventTextureAgentRevisionStarted:    StorageObjectGraph,
	EventTextureAgentRevisionCompleted:  StorageObjectGraph,
	EventTextureAgentRevisionFailed:     StorageObjectGraph,
	EventTextureDocumentRevisionCreated: StorageObjectGraph,
	EventTextureDecisionRecorded:        StorageObjectGraph,
	EventDesktopDriverLeaseUpdated:      StorageObjectGraph,
	EventDesktopAppInstancesUpdated:     StorageObjectGraph,
	EventDesktopWindowPlacementUpdated:  StorageObjectGraph,
	EventContentItemCreated:             StorageObjectGraph,
	EventMediaProgressUpdated:           StorageObjectGraph,
	EventMediaRecentUpdated:             StorageObjectGraph,
	EventThemeUpdated:                   StorageObjectGraph,
	EventFileChanged:                    StorageObjectGraph,
}

// Storage returns the kind's storage class. A kind the table does not name
// stays in the object graph.
func (k EventKind) Storage() EventStorage {
	if s, ok := eventStorage[k]; ok {
		return s
	}
	return StorageObjectGraph
}

// EventTableEligible reports whether the F-c writer may stamp the kind for
// choir_events: the kinds of the event-table class. It is the old ten.
func (k EventKind) EventTableEligible() bool {
	switch k.Storage() {
	case StorageEventTable:
		return true
	}
	return false
}
