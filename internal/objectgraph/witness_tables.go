package objectgraph

import "github.com/yusefmosiah/go-choir/internal/computerversion"

// ObjectsWitnessQuery reads og_objects as the one table it was before the
// header/body split: exactly its stored pre-split columns in declaration
// order, body in place, in primary-key (canonical_id) order. obj_id,
// og_rec_claim, event_phase and the VIRTUAL columns are not state and are
// left out, so a split store's og_objects digest equals the digest of the
// same rows in the pre-split layout.
const ObjectsWitnessQuery = `SELECT h.canonical_id, h.object_kind, h.owner_id, h.computer_id, h.version_id, h.content_hash, b.body, h.metadata, h.created_at, h.updated_at, h.tombstone, h.superseded_by
	FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id
	ORDER BY h.canonical_id`

// WitnessLogicalTables are the object graph's logical tables for the state
// witness (computerversion.SQLiteStateExtractor.LogicalTables).
func WitnessLogicalTables() map[string]computerversion.LogicalTable {
	return map[string]computerversion.LogicalTable{
		"og_objects": {Query: ObjectsWitnessQuery, Members: []string{"og_objects", "og_bodies"}},
	}
}
