package types

// EventStorage says where a choir.event is stored once the writer stamps it:
// which table (docs/design/choir-event-table-f-c-2026-10-22.md section 2.1), or
// the guest-local trace log (docs/design/trace-segment-log-2026-10-22.md
// section 2.2).
//
// It is a decision of its own, beside durability: a change to the batching
// table must never move rows between tables without someone deciding it. The
// two are pinned together today by a test (a batched kind is in the event
// table or the trace log, and nothing else is), so a change to either fails
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
	// StorageTraceLog is the guest-local trace segment log, off the tape: the
	// nine trace kinds, once the log writes (R7 slice S4). Until then nothing
	// writes the class, and these kinds go where StorageEventTable kinds go
	// (EventTableEligible). loop.compaction.completed stays StorageEventTable:
	// run acceptance reads it as a checkpoint.
	StorageTraceLog EventStorage = "trace_log"
)

// eventStorage is the closed storage classification of every EventKind. The
// parser test in event_durability_test.go fails if a constant has no entry.
var eventStorage = map[EventKind]EventStorage{
	// Trace lines: the ten per-step lines, about 94% of appends. Nine are the
	// trace log's (R7); loop.compaction.completed is the tape's, in the event
	// table.
	EventRunDelta:                     StorageTraceLog,
	EventRunProgress:                  StorageTraceLog,
	EventRunRetry:                     StorageTraceLog,
	EventRunCompactionStarted:         StorageTraceLog,
	EventRunCompactionCompleted:       StorageEventTable,
	EventToolInvoked:                  StorageTraceLog,
	EventToolResult:                   StorageTraceLog,
	EventDeskCell:                     StorageTraceLog,
	EventDeskEgress:                   StorageTraceLog,
	EventTextureAgentRevisionProgress: StorageTraceLog,

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
// choir_events: the kinds of the event-table class, and the trace-log kinds,
// whose fallback sink the event table is until the log writes (and whenever it
// falls back, design 4.3). It is the old ten.
func (k EventKind) EventTableEligible() bool {
	switch k.Storage() {
	case StorageEventTable, StorageTraceLog:
		return true
	}
	return false
}
