package store

import (
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// choirEventsWitnessQuery reads choir_events as the one table it was before
// the header/body split: its 19 stored pre-split columns in declaration
// order (body in position 9, event_kind included; ev_id and event_phase are
// not state), in primary-key (tape_seq, op_index) order.
const choirEventsWitnessQuery = `SELECT h.tape_seq, h.op_index, h.canonical_id, h.object_kind, h.owner_id, h.computer_id, h.version_id, h.content_hash, b.body, h.metadata, h.created_at, h.updated_at, h.tombstone, h.superseded_by, h.run_id, h.seq, h.stream_seq, h.trajectory_id, h.event_kind
	FROM choir_events h LEFT JOIN choir_event_bodies b ON b.ev_id = h.ev_id
	ORDER BY h.tape_seq, h.op_index`

// WitnessLogicalTables are the guest store's logical tables for the state
// witness (computerversion.SQLiteStateExtractor.LogicalTables): og_objects
// (objectgraph.WitnessLogicalTables) and choir_events. With them every
// table digest of a split store equals the digest of the same rows in the
// pre-split layout.
func WitnessLogicalTables() map[string]computerversion.LogicalTable {
	out := objectgraph.WitnessLogicalTables()
	out["choir_events"] = computerversion.LogicalTable{Query: choirEventsWitnessQuery, Members: []string{"choir_events", "choir_event_bodies"}}
	return out
}
