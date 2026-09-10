package store

// Object-graph vocabulary migration for mission-2 repair (acceptance item 10).
//
// The object graph is the live carrier: agents, runs, events, assignments,
// and channel state all serve from og_objects/og_edges. This file extends the
// cutover to that carrier in place — tombstoned copies would still serve
// through GetObject and metadata lookups, so migration rewrites rows:
//
//   1. Every string leaf in metadata and body is classified: exact V1 desk
//      tokens map through the frozen ForwardV1ToV2; desk-bearing IDs
//      (super:/co-super:/cosuper:/researcher: prefixes, -super-/-co-super-/
//      -cosuper-/-researcher- infixes) rewrite through migrateIDForward;
//      frozen protocol values (owner, trusted-core) never move; everything
//      else is untouched. Prose is safe: both rules require the whole leaf
//      to be a token or a whitespace-free ID shape.
//   2. content_hash is recomputed over migrated body+metadata.
//   3. canonical_id is re-derived: content-suffixed objects take the new
//      hash; key-suffixed objects re-derive through the kind's identity-key
//      formula, verified against the old suffix before use.
//   4. og_edges endpoints rewrite through the old→new ID map and edge_id is
//      recomputed via BuildEdgeID.
//   5. Body/metadata leaves embedding obj:/edge: references rewrite to the
//      new IDs. Because a referencing object's own content-derived ID can
//      change when its embedded refs rewrite, steps 3-5 iterate to a bounded
//      fixpoint; a reference cycle that cannot converge fails loudly.
//
// Provenance: every changed leaf's original value is recorded per object so
// revert restores exact bytes, then recomputes hashes and IDs symmetrically.
// The report is persisted before any row mutates (crash-atomicity), and a
// re-run seeds its ID map from the persisted report so a crash mid-apply
// still converges dangling references.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// ogObjectRow is one og_objects row under migration.
type ogObjectRow struct {
	canonicalID string
	kind        string
	ownerID     string
	computerID  string
	versionID   string
	body        []byte
	metadata    []byte
	tombstone   bool
	supersededBy string

	bodyJSON    any // decoded body (nil when not JSON)
	metaJSON    map[string]any
	fields      map[string]string // json path -> original leaf value (changed only)
	newID       string
	newHash     string
	dirty       bool
}

// ogEdgeRow is one og_edges row under migration.
type ogEdgeRow struct {
	edgeID    string
	fromID    string
	toID      string
	kind      string
	metadata  []byte
	tombstone bool
	newEdgeID string
	newFrom   string
	newTo     string
	dirty     bool
}

// ogIdentityFormula re-derives a key-suffixed object's identity key from its
// migrated fields. Each candidate is verified against the row's existing
// canonical suffix before use, so a wrong guess can never rename an object.
type ogIdentityFormula struct {
	kind string
	// fields are metadata keys (preferred) or body top-level keys consulted
	// in order; join is the separator between them.
	fields []string
	// scoped wraps the joined key with computerID\x00 when true.
	scoped bool
}

// ogIdentityFormulas enumerates the identity-key formulas per kind from the
// write census. Kinds absent here fall back to generic candidates: bare
// <kind>_id / *_id metadata fields, then their computer-scoped forms.
var ogIdentityFormulas = []ogIdentityFormula{
	{kind: "choir.agent", fields: []string{"agent_id"}, scoped: true},
	{kind: "choir.agent", fields: []string{"agent_id"}},
	{kind: "choir.run", fields: []string{"run_id"}, scoped: true},
	{kind: "choir.run", fields: []string{"run_id"}},
	{kind: "choir.trajectory", fields: []string{"trajectory_id"}, scoped: true},
	{kind: "choir.trajectory", fields: []string{"trajectory_id"}},
	{kind: "choir.work_item", fields: []string{"work_item_id"}, scoped: true},
	{kind: "choir.work_item", fields: []string{"work_item_id"}},
	{kind: "choir.worker_update", fields: []string{"trajectory_id", "target_agent_id", "agent_id", "producer_update_id"}, scoped: true},
	{kind: "choir.worker_update", fields: []string{"update_id"}, scoped: true},
	{kind: "choir.worker_update", fields: []string{"update_id"}},
	{kind: "choir.lifecycle_event", fields: []string{"event_id"}, scoped: true},
	{kind: "choir.lifecycle_command", fields: []string{"command_id"}, scoped: true},
	{kind: "choir.lifecycle_cancel_intent", fields: []string{"trajectory_id"}, scoped: true},
	{kind: "choir.lifecycle_sequence", fields: []string{"trajectory_id"}, scoped: true},
	{kind: "choir.owner_instruction", fields: []string{"trajectory_id", "instruction_id"}, scoped: true},
	{kind: "choir.inbox_delivery", fields: []string{"delivery_id"}},
	{kind: "choir.run_memory_entry", fields: []string{"entry_id"}},
	{kind: "choir.run_acceptance", fields: []string{"acceptance_id"}},
	{kind: "choir.run_continuation", fields: []string{"continuation_id"}},
	{kind: "choir.texture_document", fields: []string{"doc_id"}, scoped: true},
	{kind: "choir.texture_document", fields: []string{"doc_id"}},
	{kind: "choir.texture_revision", fields: []string{"revision_id"}, scoped: true},
	{kind: "choir.texture_revision", fields: []string{"revision_id"}},
	{kind: "choir.texture_decision", fields: []string{"decision_id"}},
	{kind: "choir.agent_evidence", fields: []string{"evidence_id"}},
	{kind: "choir.content_item", fields: []string{"content_id"}},
	{kind: "choir.podcast_subscription", fields: []string{"subscription_id"}},
	{kind: "choir.browser_session", fields: []string{"session_id"}},
	{kind: "choir.coagent_mailbox", fields: []string{"agent_id"}},
	{kind: "choir.desktop_session", fields: []string{"desktop_id"}},
	{kind: "choir.co_super_assignment", fields: []string{"assignment_id", "attempt"}, scoped: true},
	{kind: "choir.co_super_assignment_report", fields: []string{"report_id"}, scoped: true},
	{kind: "choir.co_super_run_claim", fields: []string{"run_id"}, scoped: true},
	{kind: "choir.co_super_capability_claim", fields: []string{"capability_digest"}, scoped: true},
	{kind: "choir.co_super_capsule_claim", fields: []string{"capsule_id"}, scoped: true},
	{kind: "choir.source_entity", fields: []string{"canonical_id", "version_id"}, scoped: true},
	{kind: "choir.source_entity", fields: []string{"canonical_id", "version_id"}},
	{kind: "choir.source_ref", fields: []string{"canonical_id", "version_id"}, scoped: true},
	{kind: "choir.source_ref", fields: []string{"canonical_id", "version_id"}},
}

// ogLeafMigrate applies the frozen vocabulary rules to one string leaf.
// Returns the migrated value and whether it changed. Frozen protocol values
// and unknown tokens pass through unchanged (the fence owns refusal).
func ogLeafMigrate(v string) (string, bool) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" || trimmed != v {
		// Whitespace-bearing leaves are prose, never IDs or tokens.
		return v, false
	}
	if vocabmigrate.IsFrozenProtocol(v) {
		return v, false
	}
	if v2, ok := vocabmigrate.ForwardV1ToV2(v); ok {
		return v2, v2 != v
	}
	if migrated, changed := migrateIDForward(v); changed {
		return migrated, true
	}
	return v, false
}

// ogWalkStrings visits every string leaf in a decoded JSON value, applying fn
// in place. path is the dotted key path for provenance.
func ogWalkStrings(node any, path string, fn func(path string, v string) string) {
	switch t := node.(type) {
	case map[string]any:
		for k, child := range t {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			if s, ok := child.(string); ok {
				t[k] = fn(childPath, s)
				continue
			}
			ogWalkStrings(child, childPath, fn)
		}
	case []any:
		for i, child := range t {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if s, ok := child.(string); ok {
				t[i] = fn(childPath, s)
				continue
			}
			ogWalkStrings(child, childPath, fn)
		}
	}
}

// ogLookupField resolves a metadata key first, then a body top-level key.
// Numeric values stringify (identity keys embed decimal attempts).
func (o *ogObjectRow) field(name string) (string, bool) {
	stringify := func(v any) (string, bool) {
		switch t := v.(type) {
		case string:
			return t, t != ""
		case float64:
			return fmt.Sprintf("%v", t), true
		}
		return "", false
	}
	if o.metaJSON != nil {
		if v, ok := stringify(o.metaJSON[name]); ok {
			return v, true
		}
	}
	if m, ok := o.bodyJSON.(map[string]any); ok {
		if v, ok := stringify(m[name]); ok {
			return v, true
		}
	}
	return "", false
}

// ogRekey recomputes the row's canonical_id from migrated content.
// sha256-suffixed objects take the new content hash; key-suffixed objects
// re-derive through the kind's identity formulas, each verified against the
// existing suffix. A key-suffixed object whose identity-bearing fields
// migrated but whose formula cannot be verified fails loudly rather than
// leaving a stale lookup key.
func (o *ogObjectRow) ogRekey() error {
	kind, owner, suffix, err := objectgraph.ParseCanonicalID(o.canonicalID)
	if err != nil {
		return fmt.Errorf("og migrate: parse canonical_id %q: %w", o.canonicalID, err)
	}
	if strings.HasPrefix(suffix, "sha256-") {
		id, err := objectgraph.BuildCanonicalID(kind, owner, objectgraph.StableSuffixFromContent(o.newHash))
		if err != nil {
			return err
		}
		o.newID = id
		return nil
	}
	if !strings.HasPrefix(suffix, "key-") {
		return fmt.Errorf("og migrate: unrecognized canonical suffix %q in %s", suffix, o.canonicalID)
	}
	// originalField resolves a field's pre-migration value: provenance holds
	// every changed leaf, so a recorded entry is the original; anything else
	// is unchanged and reads from the migrated copy.
	originalField := func(name string) (string, bool) {
		if v, ok := o.fields["metadata."+name]; ok {
			return v, true
		}
		if v, ok := o.fields["body."+name]; ok {
			return v, true
		}
		return o.field(name)
	}
	for _, f := range ogIdentityFormulas {
		if f.kind != o.kind {
			continue
		}
		// Verify: the formula over ORIGINAL values must reproduce the old
		// suffix. Only then is it this object's identity formula.
		origParts := make([]string, 0, len(f.fields))
		newParts := make([]string, 0, len(f.fields))
		missing := false
		for _, name := range f.fields {
			ov, ok := originalField(name)
			if !ok {
				missing = true
				break
			}
			nv, _ := o.field(name)
			origParts = append(origParts, ov)
			newParts = append(newParts, nv)
		}
		if missing {
			continue
		}
		origKey := strings.Join(origParts, "\x00")
		newKey := strings.Join(newParts, "\x00")
		if f.scoped {
			origKey = o.computerID + "\x00" + origKey
			newKey = o.computerID + "\x00" + newKey
		}
		if objectgraph.StableSuffixFromKey(origKey) != suffix {
			continue // formula does not describe this object
		}
		id, err := objectgraph.BuildCanonicalID(kind, owner, objectgraph.StableSuffixFromKey(newKey))
		if err != nil {
			return err
		}
		o.newID = id
		return nil
	}
	// No formula verified. If no ID-shaped field changed, the key is stable.
	if !o.identityFieldMigrated() {
		o.newID = o.canonicalID
		return nil
	}
	return fmt.Errorf("og migrate: %s %s has migrated identity fields but no verified identity formula", o.kind, o.canonicalID)
}

// identityFieldMigrated reports whether any changed leaf looks like an ID
// (desk-prefixed or desk-infixed), meaning it could feed the identity key.
func (o *ogObjectRow) identityFieldMigrated() bool {
	for _, orig := range o.fields {
		if strings.Contains(orig, ":") || strings.Contains(orig, "-") {
			if _, changed := migrateIDForward(orig); changed {
				return true
			}
		}
	}
	return false
}


// planOGMigration computes the migrated state of every og_objects/og_edges
// row without mutating the store. seed maps historical canonical/edge IDs to
// their current forms (from a persisted prior report) so a re-run after a
// mid-apply crash still resolves references.
func (s *Store) planOGMigration(ctx context.Context, rep *MigrationReport, seed map[string]string) (objs []*ogObjectRow, edges []*ogEdgeRow, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, body, metadata, tombstone, superseded_by FROM og_objects`)
	if err != nil {
		return nil, nil, fmt.Errorf("og migrate scan: %w", err)
	}
	objs = []*ogObjectRow{}
	for rows.Next() {
		o := &ogObjectRow{fields: map[string]string{}}
		var tomb int
		if err := rows.Scan(&o.canonicalID, &o.kind, &o.ownerID, &o.computerID, &o.versionID, &o.body, &o.metadata, &tomb, &o.supersededBy); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("og migrate scan: %w", err)
		}
		o.tombstone = tomb != 0
		objs = append(objs, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	edgeRows, err := s.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, tombstone FROM og_edges`)
	if err != nil {
		return nil, nil, fmt.Errorf("og migrate edge scan: %w", err)
	}
	edges = []*ogEdgeRow{}
	for edgeRows.Next() {
		e := &ogEdgeRow{}
		var tomb int
		if err := edgeRows.Scan(&e.edgeID, &e.fromID, &e.toID, &e.kind, &e.metadata, &tomb); err != nil {
			edgeRows.Close()
			return nil, nil, fmt.Errorf("og migrate edge scan: %w", err)
		}
		e.tombstone = tomb != 0
		edges = append(edges, e)
	}
	edgeRows.Close()
	if err := edgeRows.Err(); err != nil {
		return nil, nil, err
	}

	// Pass 1: token and ID migration on every leaf.
	for _, o := range objs {
		if err := json.Unmarshal(o.metadata, &o.metaJSON); err != nil {
			o.metaJSON = nil
		}
		var body any
		if err := json.Unmarshal(o.body, &body); err == nil {
			o.bodyJSON = body
		}
		record := func(prefix string) func(string, string) string {
			return func(path, v string) string {
				migrated, changed := ogLeafMigrate(v)
				if !changed {
					return v
				}
				o.fields[prefix+path] = v
				o.dirty = true
				return migrated
			}
		}
		if o.metaJSON != nil {
			ogWalkStrings(o.metaJSON, "", record("metadata."))
		}
		if o.bodyJSON != nil {
			ogWalkStrings(o.bodyJSON, "", record("body."))
		}
		if !o.dirty {
			o.newID = o.canonicalID
			continue
		}
		if o.bodyJSON != nil {
			newBody, err := json.Marshal(o.bodyJSON)
			if err != nil {
				return nil, nil, fmt.Errorf("og migrate %s body encode: %w", o.canonicalID, err)
			}
			o.body = newBody
		}
		newMeta, err := objectgraph.NormalizeMetadata(o.metaJSON)
		if err != nil {
			return nil, nil, fmt.Errorf("og migrate %s metadata encode: %w", o.canonicalID, err)
		}
		o.metadata = newMeta
		o.newHash = objectgraph.ContentHash(objectgraph.ObjectKind(o.kind), o.body, o.metadata)
		if err := o.ogRekey(); err != nil {
			return nil, nil, err
		}
	}

	// ID map: every historical ID resolves to the current one. Seed first so
	// a crash-recovery re-run still resolves references to already-migrated
	// objects.
	idMap := map[string]string{}
	for old, cur := range seed {
		idMap[old] = cur
	}
	for _, o := range objs {
		if o.newID != o.canonicalID {
			idMap[o.canonicalID] = o.newID
		}
	}

	// Fixpoint: rewrite embedded obj:/edge: references, re-derive IDs for
	// content-suffixed objects whose content changed, repeat until stable.
	for pass := range 8 {
		changed := false
		rewriteRefs := func(o *ogObjectRow) {
			apply := func(prefix string) func(string, string) string {
				return func(path, v string) string {
					if !strings.Contains(v, "obj:") && !strings.Contains(v, "edge:") {
						return v
					}
					out := v
					for old, cur := range idMap {
						if strings.Contains(out, old) {
							out = strings.ReplaceAll(out, old, cur)
						}
					}
					if out != v {
						// Record the pre-rewrite value only when pass 1 did
						// not already record this leaf's original.
						if _, seen := o.fields[prefix+path]; !seen {
							o.fields[prefix+path] = v
						}
						o.dirty = true
					}
					return out
				}
			}
			if o.metaJSON != nil {
				ogWalkStrings(o.metaJSON, "", apply("metadata."))
			}
			if o.bodyJSON != nil {
				ogWalkStrings(o.bodyJSON, "", apply("body."))
			}
		}
		for _, o := range objs {
			before := o.newID
			rewriteRefs(o)
			if o.dirty {
				if o.bodyJSON != nil {
					newBody, err := json.Marshal(o.bodyJSON)
					if err != nil {
						return nil, nil, fmt.Errorf("og migrate %s body re-encode: %w", o.canonicalID, err)
					}
					o.body = newBody
				}
				newMeta, err := objectgraph.NormalizeMetadata(o.metaJSON)
				if err != nil {
					return nil, nil, fmt.Errorf("og migrate %s metadata re-encode: %w", o.canonicalID, err)
				}
				o.metadata = newMeta
				o.newHash = objectgraph.ContentHash(objectgraph.ObjectKind(o.kind), o.body, o.metadata)
				if _, _, suffix, perr := objectgraph.ParseCanonicalID(o.canonicalID); perr == nil && strings.HasPrefix(suffix, "sha256-") {
					id, berr := objectgraph.BuildCanonicalID(objectgraph.ObjectKind(o.kind), o.ownerID, objectgraph.StableSuffixFromContent(o.newHash))
					if berr != nil {
						return nil, nil, fmt.Errorf("og migrate %s rekey: %w", o.canonicalID, berr)
					}
					o.newID = id
				}
			}
			if o.newID != before {
				idMap[before] = o.newID
				idMap[o.canonicalID] = o.newID
				changed = true
			}
		}
		// Edges: rewrite endpoints, recompute edge_id.
		for _, e := range edges {
			from, to := e.newFrom, e.newTo
			if from == "" {
				from = e.fromID
			}
			if to == "" {
				to = e.toID
			}
			if cur, ok := idMap[from]; ok {
				from = cur
			}
			if cur, ok := idMap[to]; ok {
				to = cur
			}
			if from != e.fromID || to != e.toID || e.newFrom != "" {
				newID, err := objectgraph.BuildEdgeID(from, to, objectgraph.EdgeKind(e.kind), e.metadata)
				if err != nil {
					return nil, nil, fmt.Errorf("og migrate edge %s rebuild: %w", e.edgeID, err)
				}
				if newID != e.edgeID && newID != e.newEdgeID {
					if e.newEdgeID != "" {
						idMap[e.newEdgeID] = newID
					}
					idMap[e.edgeID] = newID
					e.newEdgeID = newID
					changed = true
				}
				e.newFrom, e.newTo = from, to
				e.dirty = true
			}
		}
		if !changed {
			break
		}
		if pass == 7 {
			return nil, nil, fmt.Errorf("og migrate: canonical ID fixpoint did not converge in 8 passes (reference cycle)")
		}
	}

	// version_id / superseded_by columns carry canonical IDs; rewrite through
	// the map. Recorded as column provenance so revert restores exactly.
	for _, o := range objs {
		if cur, ok := idMap[o.versionID]; ok && cur != o.versionID {
			o.fields["col.version_id"] = o.versionID
			o.versionID = cur
			o.dirty = true
		}
		if cur, ok := idMap[o.supersededBy]; ok && cur != o.supersededBy {
			o.fields["col.superseded_by"] = o.supersededBy
			o.supersededBy = cur
			o.dirty = true
		}
	}

	// Record provenance.
	for _, o := range objs {
		if !o.dirty {
			continue
		}
		rep.OGObjects = append(rep.OGObjects, OGProvEntry{
			OldCanonicalID: o.canonicalID,
			NewCanonicalID: o.newID,
			Fields:         o.fields,
		})
		rep.Counts["og_objects."+o.kind]++
	}
	for _, e := range edges {
		if !e.dirty {
			continue
		}
		rep.OGEdges = append(rep.OGEdges, OGEdgeProvEntry{
			OldEdgeID: e.edgeID, NewEdgeID: e.newEdgeID,
			OldFrom: e.fromID, OldTo: e.toID,
		})
		rep.Counts["og_edges."+e.kind]++
	}

	return objs, edges, nil
}

// applyOGMigration writes the planned object/edge state in one transaction.
// The report is already durable at this point (crash-atomicity ordering).
func (s *Store) applyOGMigration(ctx context.Context, objs []*ogObjectRow, edges []*ogEdgeRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("og migrate tx: %w", err)
	}
	for _, o := range objs {
		if !o.dirty {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE og_objects SET canonical_id = ?, content_hash = ?, body = ?, metadata = ?, version_id = ?, superseded_by = ? WHERE canonical_id = ?`,
			o.newID, o.newHash, o.body, o.metadata, o.versionID, o.supersededBy, o.canonicalID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og migrate write %s: %w", o.canonicalID, err)
		}
	}
	for _, e := range edges {
		if !e.dirty {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE og_edges SET edge_id = ?, from_id = ?, to_id = ? WHERE edge_id = ?`,
			e.newEdgeID, e.newFrom, e.newTo, e.edgeID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og migrate edge write %s: %w", e.edgeID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("og migrate commit: %w", err)
	}
	return nil
}

// ogSeedFromReport builds the historical→current ID map from a persisted
// report so a re-run resolves references to already-migrated rows.
func ogSeedFromReport(rep *MigrationReport) map[string]string {
	seed := map[string]string{}
	if rep == nil {
		return seed
	}
	for _, e := range rep.OGObjects {
		seed[e.OldCanonicalID] = e.NewCanonicalID
	}
	for _, e := range rep.OGEdges {
		seed[e.OldEdgeID] = e.NewEdgeID
	}
	return seed
}

// ogRoleLeafKeys are the JSON key names whose values are desk vocabulary on
// the live carrier. Message-role fields (run_memory_entry.role =
// assistant/user) are excluded by kind below.
var ogRoleLeafKeys = map[string]bool{
	"role": true, "profile": true, "agent_profile": true, "agent_role": true,
	"authority_profile": true, "requested_by_profile": true, "actor_profile": true,
	"author_label": true, "target_profile": true,
}

// ogRoleSkipKinds lists kinds whose same-named fields are not desk
// vocabulary (message roles, UI labels).
var ogRoleSkipKinds = map[string]map[string]bool{
	"choir.run_memory_entry": {"role": true},
	"choir.event":            {"kind": true, "phase": true},
}

// ogRoleFieldsFromJSON collects declared role-bearing leaves from one JSON
// document for kind, excluding non-desk same-named fields.
func ogRoleFieldsFromJSON(kind string, raw []byte, prefix string, out *[]vocabmigrate.Field) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return
	}
	skip := ogRoleSkipKinds[kind]
	ogWalkStrings(decoded, "", func(path, v string) string {
		key := path
		if i := strings.LastIndex(key, "."); i >= 0 {
			key = key[i+1:]
		}
		if i := strings.Index(key, "["); i >= 0 {
			key = key[:i]
		}
		if ogRoleLeafKeys[key] && !skip[key] {
			*out = append(*out, vocabmigrate.Field{
				Key:   "og:" + kind + ":" + prefix + path,
				Value: v,
			})
		}
		return v
	})
}

// servingOGRoleFields collects every role-bearing leaf in og_objects for the
// serving fence: declared role keys in metadata and body (recursive, so
// nested attestation roles are covered), excluding non-desk same-named
// fields. computer_event_index.event_json is a tape mirror and is never
// scanned.
func (s *Store) servingOGRoleFields(ctx context.Context) ([]vocabmigrate.Field, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, object_kind, body, metadata FROM og_objects`)
	if err != nil {
		return nil, fmt.Errorf("vocab fence og scan: %w", err)
	}
	defer rows.Close()
	var fields []vocabmigrate.Field
	for rows.Next() {
		var id, kind string
		var body, meta []byte
		if err := rows.Scan(&id, &kind, &body, &meta); err != nil {
			return nil, fmt.Errorf("vocab fence og scan: %w", err)
		}
		ogRoleFieldsFromJSON(kind, meta, "metadata.", &fields)
		ogRoleFieldsFromJSON(kind, body, "body.", &fields)
	}
	return fields, rows.Err()
}

// vocabWriteGuard refuses durable OG writes carrying declared role fields
// outside the active V2 vocabulary. Installed on ogStore at open; activates
// only after the cutover report exists (vocabCutover), so pre-cutover V1
// writers and replay's direct-SQL path are unaffected.
func (s *Store) vocabWriteGuard(ctx context.Context, objects []objectgraph.Object, edges []objectgraph.Edge) error {
	if !s.vocabCutover.Load() {
		return nil
	}
	var fields []vocabmigrate.Field
	for _, obj := range objects {
		kind := string(obj.ObjectKind)
		ogRoleFieldsFromJSON(kind, obj.Metadata, "metadata.", &fields)
		ogRoleFieldsFromJSON(kind, obj.Body, "body.", &fields)
	}
	for _, e := range edges {
		ogRoleFieldsFromJSON("edge:"+string(e.Kind), e.Metadata, "metadata.", &fields)
	}
	if err := vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV2, fields...); err != nil {
		return fmt.Errorf("vocab write guard: %w", err)
	}
	return nil
}

// ogSetPath sets a dotted-path leaf inside a decoded JSON value, creating
// intermediate maps as needed. Array indices in the path ([N]) are supported.
func ogSetPath(node any, path, value string) {
	parts := strings.Split(path, ".")
	cur := node
	for i, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return
		}
		key := p
		var idx int = -1
		if j := strings.Index(p, "["); j >= 0 && strings.HasSuffix(p, "]") {
			key = p[:j]
			if n, err := fmt.Sscanf(p[j:], "[%d]", &idx); n != 1 || err != nil {
				idx = -1
			}
		}
		if i == len(parts)-1 {
			if idx >= 0 {
				if arr, ok := m[key].([]any); ok && idx < len(arr) {
					arr[idx] = value
				}
			} else {
				m[key] = value
			}
			return
		}
		next := m[key]
		if idx >= 0 {
			arr, ok := next.([]any)
			if !ok || idx >= len(arr) {
				return
			}
			cur = arr[idx]
			continue
		}
		cur = next
	}
}

// revertOGMigration restores every recorded og_objects/og_edges row to its
// pre-migration bytes, then recomputes hashes and IDs symmetrically. Because
// provenance records every changed leaf's original value, restore is exact;
// the recomputed canonical_id must equal OldCanonicalID (the drill asserts
// byte-identity, so a mismatch fails loudly rather than silently corrupting).
func (s *Store) revertOGMigration(ctx context.Context, rep *MigrationReport) error {
	if rep == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("og revert tx: %w", err)
	}
	for _, entry := range rep.OGObjects {
		var kind, ownerID, computerID, versionID, supersededBy string
		var body, meta []byte
		var tomb int
		err := tx.QueryRowContext(ctx,
			`SELECT object_kind, owner_id, computer_id, version_id, superseded_by, body, metadata, tombstone FROM og_objects WHERE canonical_id = ?`,
			entry.NewCanonicalID).Scan(&kind, &ownerID, &computerID, &versionID, &supersededBy, &body, &meta, &tomb)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og revert load %s: %w", entry.NewCanonicalID, err)
		}
		var bodyJSON any
		if err := json.Unmarshal(body, &bodyJSON); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og revert %s body decode: %w", entry.NewCanonicalID, err)
		}
		var metaJSON any
		if err := json.Unmarshal(meta, &metaJSON); err != nil {
			metaJSON = map[string]any{}
		}
		for path, orig := range entry.Fields {
			switch {
			case strings.HasPrefix(path, "body."):
				ogSetPath(bodyJSON, strings.TrimPrefix(path, "body."), orig)
			case strings.HasPrefix(path, "metadata."):
				ogSetPath(metaJSON, strings.TrimPrefix(path, "metadata."), orig)
			case path == "col.version_id":
				versionID = orig
			case path == "col.superseded_by":
				supersededBy = orig
			}
		}
		newBody, err := json.Marshal(bodyJSON)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og revert %s body encode: %w", entry.NewCanonicalID, err)
		}
		newMeta, err := objectgraph.NormalizeMetadata(metaJSON)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og revert %s metadata encode: %w", entry.NewCanonicalID, err)
		}
		hash := objectgraph.ContentHash(objectgraph.ObjectKind(kind), newBody, newMeta)
		// Content-suffixed objects must re-derive exactly the recorded old
		// ID; a mismatch means provenance was incomplete — fail loudly.
		if _, _, suffix, perr := objectgraph.ParseCanonicalID(entry.OldCanonicalID); perr == nil && strings.HasPrefix(suffix, "sha256-") {
			if objectgraph.StableSuffixFromContent(hash) != suffix {
				_ = tx.Rollback()
				return fmt.Errorf("og revert %s: restored content does not re-derive recorded ID", entry.NewCanonicalID)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE og_objects SET canonical_id = ?, content_hash = ?, body = ?, metadata = ?, version_id = ?, superseded_by = ? WHERE canonical_id = ?`,
			entry.OldCanonicalID, hash, newBody, newMeta, versionID, supersededBy, entry.NewCanonicalID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og revert write %s: %w", entry.NewCanonicalID, err)
		}
	}
	for _, entry := range rep.OGEdges {
		if _, err := tx.ExecContext(ctx,
			`UPDATE og_edges SET edge_id = ?, from_id = ?, to_id = ? WHERE edge_id = ?`,
			entry.OldEdgeID, entry.OldFrom, entry.OldTo, entry.NewEdgeID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("og revert edge %s: %w", entry.NewEdgeID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("og revert commit: %w", err)
	}
	return nil
}
