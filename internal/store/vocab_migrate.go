package store

// SQL row migration for mission-2 (drill and cutover machinery).
//
// MigrateVocabularyToV2 rewrites every inventoried SQL role column,
// desk-bearing ID, and metadata role key under the frozen §3 map;
// RevertVocabularyToV1 restores them via retained provenance (INV-PROV
// exact) or the canonical representative (INV-CANON). The drill test
// exercises migrate→verify→revert→verify→re-migrate→verify→revert ending
// on V1 rows; the cutover calls the same functions once, forward only.
//
// Everything runs row-by-row in Go: value columns match case-insensitively
// (raw persisted roles are not guaranteed lowercase); ID rewrites avoid SQL
// string surgery because neither `||` concatenation nor vendor functions
// are portable across SQLite and Dolt. JSON columns rewrite through the
// frozen applier, never string surgery.
//
// Deliberately out of drill round 1 (recorded cutover obligations):
// object-graph objects/edges, prompt/profile registry tags, computer-owned
// TOML overlays, grant attestation rows, digest-domain succession (§6),
// and bare-compound ID suffixes (e.g. run-cosuper style) beyond the
// prefix/infix rules below.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// provenanceSpellings are V1 spellings whose canonical inverse differs from
// the string itself (many-to-one collapse plus the engineering/research
// version collisions). Rows carrying these spellings get exact-token
// provenance so revert restores bytes, not just semantics.
var provenanceSpellings = map[string]bool{
	"cosuper": true, "coagent": true, "co-agent": true,
	"co_super": true, "cosuper_coding": true, "co-super-coding": true,
	"engineering": true,
	"researchers": true, "research": true, "research-agent": true,
	"web-research": true, "web-researcher": true,
}

// ProvEntry retains one exact V1 token for INV-PROV revert. Column names a
// table column, or "jsoncol.jsonkey" for metadata/detail JSON role keys.
type ProvEntry struct {
	Table  string            `json:"table"`
	Keys   map[string]string `json:"keys"`
	Column string            `json:"column"`
	Exact  string            `json:"exact"`
}

// MigrationReport retains per-row source tokens for INV-PROV revert plus
// per-table migration counts. It is JSON-serializable so the cutover can
// persist it durably across restarts.
type MigrationReport struct {
	Provenance map[string]ProvEntry `json:"provenance"`
	Counts     map[string]int64     `json:"counts"`
}

func provKey(table string, keys map[string]string, col string) string {
	cols := make([]string, 0, len(keys))
	for k := range keys {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	parts := []string{table, col}
	for _, k := range cols {
		parts = append(parts, k+"="+keys[k])
	}
	return strings.Join(parts, ":")
}

// migrateIDForward rewrites desk-bearing ID prefixes and hyphen-delimited
// desk infixes (run-super-assignment / work-super-assignment style per the
// charter tie-break). Command identities are never passed in: callers only
// supply agent/address/constructed-ID columns.
func migrateIDForward(id string) (string, bool) {
	for _, prefix := range []string{"super:", "co-super:", "cosuper:", "researcher:"} {
		if strings.HasPrefix(id, prefix) {
			v1tok := strings.TrimSuffix(prefix, ":")
			if v2, ok := vocabmigrate.ForwardV1ToV2(v1tok); ok {
				return v2 + ":" + strings.TrimPrefix(id, prefix), true
			}
			return id, false
		}
	}
	// Longest-token-first: "-super-" is a substring of "-co-super-", so a
	// shorter infix must never match before a longer one containing it.
	for _, pair := range [][2]string{
		{"-co-super-", "-engineering-"}, {"-cosuper-", "-engineering-"},
		{"-researcher-", "-research-"}, {"-super-", "-management-"},
	} {
		if strings.Contains(id, pair[0]) {
			return strings.ReplaceAll(id, pair[0], pair[1]), true
		}
	}
	return id, false
}

// migrateIDBackward inverts migrateIDForward through canonical
// representatives. Exact non-canonical spellings (cosuper:) restore through
// provenance, never here.
func migrateIDBackward(id string) (string, bool) {
	for _, prefix := range []string{"management:", "engineering:", "research:"} {
		if strings.HasPrefix(id, prefix) {
			v1tok := map[string]string{
				"management:": "super:", "engineering:": "co-super:", "research:": "researcher:",
			}[prefix]
			return v1tok + strings.TrimPrefix(id, prefix), true
		}
	}
	// Longest-first: "-co-management-" (the mislabeled engineering form)
	// contains "-management-" and must invert before it.
	for _, pair := range [][2]string{
		{"-co-management-", "-co-super-"}, {"-engineering-", "-co-super-"},
		{"-research-", "-researcher-"}, {"-management-", "-super-"},
	} {
		if strings.Contains(id, pair[0]) {
			return strings.ReplaceAll(id, pair[0], pair[1]), true
		}
	}
	return id, false
}

// needsProvenance reports whether reverting the migrated value through the
// canonical inverse would lose the original spelling.
func needsProvenance(original, migrated string) bool {
	if back, ok := migrateIDBackward(migrated); ok && back == original {
		return false
	}
	if v1, ok := vocabmigrate.InverseV2ToV1Canonical(migrated); ok && v1 == original {
		return false
	}
	return migrated != original
}

// valueTarget describes one role-bearing column migration.
type valueTarget struct {
	table   string
	keyCols []string
	col     string
}

var vocabValueTargets = []valueTarget{
	{"agents", []string{"agent_id"}, "profile"},
	{"agents", []string{"agent_id"}, "role"},
	{"runs", []string{"loop_id"}, "agent_profile"},
	{"runs", []string{"loop_id"}, "agent_role"},
	{"channel_messages", []string{"channel_id", "seq"}, "role"},
	{"inbox_deliveries", []string{"delivery_id"}, "role"},
	{"work_items", []string{"work_item_id"}, "authority_profile"},
	{"worker_updates", []string{"owner_id", "update_id"}, "role"},
}

// idTarget describes one ID/address column migration.
type idTarget struct {
	table   string
	keyCols []string
	col     string
}

var vocabIDTargets = []idTarget{
	{"agents", []string{"agent_id"}, "agent_id"},
	{"runs", []string{"loop_id"}, "agent_id"},
	{"channel_messages", []string{"channel_id", "seq"}, "from_agent_id"},
	{"channel_messages", []string{"channel_id", "seq"}, "to_agent_id"},
	{"inbox_deliveries", []string{"delivery_id"}, "to_agent_id"},
	{"inbox_deliveries", []string{"delivery_id"}, "from_agent_id"},
	{"work_items", []string{"work_item_id"}, "assigned_agent_id"},
	{"work_items", []string{"work_item_id"}, "work_item_id"},
	{"worker_updates", []string{"owner_id", "update_id"}, "agent_id"},
	{"worker_updates", []string{"owner_id", "update_id"}, "target_agent_id"},
	{"coagent_mailboxes", []string{"owner_id", "agent_id"}, "agent_id"},
	{"run_memory_entries", []string{"entry_id"}, "agent_id"},
}

// scanRows reads key columns plus one value column for every row.
func (s *Store) scanRows(ctx context.Context, table string, keyCols []string, col string) (keys []map[string]string, vals []string, err error) {
	cols := append(append([]string{}, keyCols...), col)
	q := fmt.Sprintf(`SELECT %s FROM %s`, strings.Join(cols, ", "), table)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		raw := make([]string, len(cols))
		ptrs := make([]any, len(raw))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		km := map[string]string{}
		for i, k := range keyCols {
			km[k] = raw[i]
		}
		keys = append(keys, km)
		vals = append(vals, raw[len(raw)-1])
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return keys, vals, nil
}

func updateRow(ctx context.Context, db *sql.DB, table, col string, value string, keys map[string]string) error {
	cols := make([]string, 0, len(keys))
	for k := range keys {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	where := make([]string, 0, len(cols))
	args := []any{value}
	for _, k := range cols {
		where = append(where, k+" = ?")
		args = append(args, keys[k])
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET %s = ? WHERE %s`, table, col, strings.Join(where, " AND ")), args...); err != nil {
		return err
	}
	return nil
}

// vocabWrite is one planned row update. Planning computes the full report
// and write set before any mutation so the provenance report can be made
// durable first: a crash between report persistence and row writes leaves
// correct provenance for every planned change, and re-running the plan is
// deterministic over the same input rows.
type vocabWrite struct {
	table string
	col   string
	value string
	keys  map[string]string
	// keyCol marks writes that mutate a column named in keys. They must apply
	// after every other write to the same table, since sibling writes address
	// the row by its pre-migration key values.
	keyCol bool
}

// planVocabularyMigration scans every inventoried target and returns the
// provenance report plus the pending write set without mutating any row.
// Unknown tokens are left in place: the fence (not this function) refuses
// them, and the drill asserts zero unknowns.
func (s *Store) planVocabularyMigration(ctx context.Context) (*MigrationReport, []vocabWrite, error) {
	rep := &MigrationReport{Provenance: map[string]ProvEntry{}, Counts: map[string]int64{}}
	var writes []vocabWrite
	record := func(table string, keys map[string]string, col, exact string) {
		rep.Provenance[provKey(table, keys, col)] = ProvEntry{
			Table: table, Keys: keys, Column: col, Exact: exact,
		}
	}
	for _, t := range vocabValueTargets {
		keys, vals, err := s.scanRows(ctx, t.table, t.keyCols, t.col)
		if err != nil {
			return nil, nil, fmt.Errorf("vocab migrate %s.%s scan: %w", t.table, t.col, err)
		}
		for i, v := range keys {
			raw := vals[i]
			if strings.TrimSpace(raw) == "" || vocabmigrate.IsFrozenProtocol(raw) {
				continue
			}
			v2, ok := vocabmigrate.ForwardV1ToV2(raw)
			if !ok {
				continue // unknown: left for the fence; drill asserts none
			}
			// Provenance first: every changed row records its exact V1 token,
			// not just non-canonical spellings. A later migration pass sees the
			// migrated V2 value; without the original entry the merge cannot
			// tell it from a V1-authored V2-spelling row and would record a
			// spurious provenance entry that hijacks INV-PROV revert.
			if provenanceSpellings[strings.ToLower(raw)] || v2 != raw {
				record(t.table, v, t.col, raw)
			}
			if v2 == raw {
				continue
			}
			writes = append(writes, vocabWrite{table: t.table, col: t.col, value: v2, keys: v})
			rep.Counts[t.table+"."+t.col]++
		}
	}
	for _, t := range vocabIDTargets {
		keys, vals, err := s.scanRows(ctx, t.table, t.keyCols, t.col)
		if err != nil {
			return nil, nil, fmt.Errorf("vocab migrate %s.%s id scan: %w", t.table, t.col, err)
		}
		for i, v := range keys {
			migrated, changed := migrateIDForward(vals[i])
			if !changed {
				continue
			}
			if needsProvenance(vals[i], migrated) {
				record(t.table, v, t.col, vals[i])
			}
			writes = append(writes, vocabWrite{table: t.table, col: t.col, value: migrated, keys: v, keyCol: isKeyColumn(t, t.col)})
			rep.Counts[t.table+"."+t.col]++
		}
	}
	if err := s.planMetadataJSON(ctx, rep, &writes, "runs", "loop_id", "metadata_json",
		[]string{"agent_profile", "agent_role", "requested_by_profile"}); err != nil {
		return nil, nil, err
	}
	if err := s.planMetadataJSON(ctx, rep, &writes, "work_items", "work_item_id", "details_json",
		[]string{"requested_by_profile"}); err != nil {
		return nil, nil, err
	}
	return rep, writes, nil
}

// applyVocabularyMigration executes a planned write set. Writes that mutate
// a key column apply last within their table so sibling writes still address
// rows by pre-migration key values.
func (s *Store) applyVocabularyMigration(ctx context.Context, writes []vocabWrite) error {
	var deferred []vocabWrite
	for _, w := range writes {
		if w.keyCol {
			deferred = append(deferred, w)
			continue
		}
		if err := updateRow(ctx, s.db, w.table, w.col, w.value, w.keys); err != nil {
			return fmt.Errorf("vocab migrate %s.%s write: %w", w.table, w.col, err)
		}
	}
	for _, w := range deferred {
		if err := updateRow(ctx, s.db, w.table, w.col, w.value, w.keys); err != nil {
			return fmt.Errorf("vocab migrate %s.%s write: %w", w.table, w.col, err)
		}
	}
	return nil
}

// isKeyColumn reports whether col is one of the target's key columns.
func isKeyColumn(t idTarget, col string) bool {
	for _, k := range t.keyCols {
		if k == col {
			return true
		}
	}
	return false
}

// MigrateVocabularyToV2 forward-migrates every inventoried SQL role column,
// desk-bearing ID, and metadata role key under the frozen map. Callers that
// need provenance durable before mutation (the cutover path) use the
// plan/persist/apply sequence inside MigrateAndFenceServingVocabulary.
func (s *Store) MigrateVocabularyToV2(ctx context.Context) (*MigrationReport, error) {
	rep, writes, err := s.planVocabularyMigration(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.applyVocabularyMigration(ctx, writes); err != nil {
		return nil, err
	}
	return rep, nil
}

// migrateMetadataJSON rewrites desk-bearing role keys inside a JSON text
// column row by row through the frozen applier (never string surgery),
// retaining provenance for non-canonical spellings.
func (s *Store) planMetadataJSON(ctx context.Context, rep *MigrationReport, writes *[]vocabWrite, table, key, col string, roleKeys []string) error {
	keys, vals, err := s.scanRows(ctx, table, []string{key}, col)
	if err != nil {
		return fmt.Errorf("vocab migrate %s.%s metadata scan: %w", table, col, err)
	}
	for i, v := range keys {
		var meta map[string]any
		if err := json.Unmarshal([]byte(vals[i]), &meta); err != nil || meta == nil {
			continue
		}
		changed := false
		for _, rk := range roleKeys {
			raw, ok := meta[rk].(string)
			if !ok || strings.TrimSpace(raw) == "" {
				continue
			}
			if vocabmigrate.IsFrozenProtocol(raw) {
				continue
			}
			v2, ok := vocabmigrate.ForwardV1ToV2(raw)
			if !ok {
				continue
			}
			if provenanceSpellings[strings.ToLower(raw)] || v2 != raw {
				rep.Provenance[provKey(table, v, col+"."+rk)] = ProvEntry{
					Table: table, Keys: v, Column: col + "." + rk, Exact: raw,
				}
			}
			if v2 == raw {
				continue
			}
			meta[rk] = v2
			changed = true
		}
		if !changed {
			continue
		}
		encoded, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("vocab migrate %s.%s metadata encode: %w", table, col, err)
		}
		*writes = append(*writes, vocabWrite{table: table, col: col, value: string(encoded), keys: v})
		rep.Counts[table+"."+col]++
	}
	return nil
}

// RevertVocabularyToV1 restores provenance-recorded rows to their exact
// original tokens first (INV-PROV, while migrated keys still address them),
// then canonical V1 representatives for the rest (INV-CANON). With a
// complete provenance log the revert is byte-identical; without it,
// many-to-one spellings revert canonically.
func (s *Store) RevertVocabularyToV1(ctx context.Context, rep *MigrationReport) error {
	inverse := map[string]string{"management": "super", "engineering": "co-super", "research": "researcher"}
	// restored marks cells the provenance pass already restored exactly;
	// canonical sweeps below must skip them (a restored V1 alias like
	// 'research' is indistinguishable by value from a V2 name, but only
	// the latter needs canonical revert).
	restored := map[string]bool{}
	markRestored := func(table, col string, keys map[string]string) {
		cols := make([]string, 0, len(keys))
		for k := range keys {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		key := table + ":" + col + ":"
		for _, k := range cols {
			key += k + "=" + keys[k] + ","
		}
		restored[key] = true
	}
	isRestored := func(table, col string, keys map[string]string) bool {
		cols := make([]string, 0, len(keys))
		for k := range keys {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		key := table + ":" + col + ":"
		for _, k := range cols {
			key += k + "=" + keys[k] + ","
		}
		return restored[key]
	}
	if rep != nil {
		for _, entry := range rep.Provenance {
			if strings.Contains(entry.Column, ".") {
				// JSON subkey: reload, restore exact token, rewrite.
				parts := strings.SplitN(entry.Column, ".", 2)
				jsonCol, jsonKey := parts[0], parts[1]
				keyCols := make([]string, 0, len(entry.Keys))
				for k := range entry.Keys {
					keyCols = append(keyCols, k)
				}
				sort.Strings(keyCols)
				keys, vals, err := s.scanRows(ctx, entry.Table, keyCols, jsonCol)
				if err != nil {
					return fmt.Errorf("vocab revert provenance %s scan: %w", entry.Table, err)
				}
				for i, v := range keys {
					match := true
					for k, want := range entry.Keys {
						if v[k] != want {
							match = false
							break
						}
					}
					if !match {
						continue
					}
					var meta map[string]any
					if err := json.Unmarshal([]byte(vals[i]), &meta); err != nil || meta == nil {
						continue
					}
					meta[jsonKey] = entry.Exact
					encoded, err := json.Marshal(meta)
					if err != nil {
						return fmt.Errorf("vocab revert provenance %s encode: %w", entry.Table, err)
					}
					if err := updateRow(ctx, s.db, entry.Table, jsonCol, string(encoded), v); err != nil {
						return fmt.Errorf("vocab revert provenance %s write: %w", entry.Table, err)
					}
					markRestored(entry.Table, jsonCol+"."+jsonKey, v)
				}
				continue
			}
			cols := make([]string, 0, len(entry.Keys))
			for k := range entry.Keys {
				cols = append(cols, k)
			}
			sort.Strings(cols)
			// Keys were captured pre-migration; key columns that were
			// themselves migrated no longer match, so the migrated form
			// is tried first (see tryKeys below).
			// Try migrated key form first, then the as-recorded form.
			tryKeys := []map[string]string{migratedKeysForRevert(entry), entry.Keys}
			done := false
			for _, tk := range tryKeys {
				keys, vals, err := s.scanRows(ctx, entry.Table, cols, entry.Column)
				if err != nil {
					return fmt.Errorf("vocab revert provenance %s scan: %w", entry.Table, err)
				}
				for _, v := range keys {
					match := true
					for _, k := range cols {
						if v[k] != tk[k] {
							match = false
							break
						}
					}
					if !match {
						continue
					}
					_ = vals
					if err := updateRow(ctx, s.db, entry.Table, entry.Column, entry.Exact, v); err != nil {
						return fmt.Errorf("vocab revert provenance %s write: %w", entry.Table, err)
					}
					markRestored(entry.Table, entry.Column, v)
					done = true
				}
				if done {
					break
				}
			}
			if !done {
				return fmt.Errorf("vocab revert provenance %s: row not found", provKey(entry.Table, entry.Keys, entry.Column))
			}
		}
	}
	for _, t := range vocabValueTargets {
		keys, vals, err := s.scanRows(ctx, t.table, t.keyCols, t.col)
		if err != nil {
			return fmt.Errorf("vocab revert %s.%s scan: %w", t.table, t.col, err)
		}
		for i, v := range keys {
			if isRestored(t.table, t.col, v) {
				continue
			}
			if v1, ok := inverse[strings.ToLower(vals[i])]; ok && v1 != vals[i] {
				if err := updateRow(ctx, s.db, t.table, t.col, v1, v); err != nil {
					return fmt.Errorf("vocab revert %s.%s write: %w", t.table, t.col, err)
				}
			}
		}
	}
	for _, t := range vocabIDTargets {
		keys, vals, err := s.scanRows(ctx, t.table, t.keyCols, t.col)
		if err != nil {
			return fmt.Errorf("vocab revert %s.%s id scan: %w", t.table, t.col, err)
		}
		for i, v := range keys {
			if isRestored(t.table, t.col, v) {
				continue
			}
			if back, changed := migrateIDBackward(vals[i]); changed {
				if err := updateRow(ctx, s.db, t.table, t.col, back, v); err != nil {
					return fmt.Errorf("vocab revert %s.%s id write: %w", t.table, t.col, err)
				}
			}
		}
	}
	for _, t := range []struct{ table, key, col string }{
		{"runs", "loop_id", "metadata_json"},
		{"work_items", "work_item_id", "details_json"},
	} {
		keys, vals, err := s.scanRows(ctx, t.table, []string{t.key}, t.col)
		if err != nil {
			return fmt.Errorf("vocab revert %s.%s metadata scan: %w", t.table, t.col, err)
		}
		for i, v := range keys {
			var meta map[string]any
			if err := json.Unmarshal([]byte(vals[i]), &meta); err != nil || meta == nil {
				continue
			}
			changed := false
			for rk, rv := range meta {
				sv, ok := rv.(string)
				if !ok {
					continue
				}
				switch rk {
				case "agent_profile", "agent_role", "requested_by_profile":
					if isRestored(t.table, t.col+"."+rk, v) {
						continue
					}
					if v1, ok := inverse[strings.ToLower(sv)]; ok && v1 != sv {
						meta[rk] = v1
						changed = true
					}
				}
			}
			if !changed {
				continue
			}
			encoded, err := json.Marshal(meta)
			if err != nil {
				return fmt.Errorf("vocab revert %s.%s metadata encode: %w", t.table, t.col, err)
			}
			if err := updateRow(ctx, s.db, t.table, t.col, string(encoded), v); err != nil {
				return fmt.Errorf("vocab revert %s.%s metadata write: %w", t.table, t.col, err)
			}
		}
	}
	return nil
}

// migratedKeysForRevert maps provenance-captured keys to their migrated form
// so exact overwrites address post-migration rows.
func migratedKeysForRevert(entry ProvEntry) map[string]string {
	out := map[string]string{}
	for k, v := range entry.Keys {
		if migrated, changed := migrateIDForward(v); changed {
			out[k] = migrated
		} else {
			out[k] = v
		}
	}
	return out
}

// servingRoleQueries enumerates every role-bearing value the serving fence
// must verify. It mirrors the drill's drillRoleColumns set plus the
// metadata_json.agent_role key the migrator already rewrites. Desk-bearing ID
// columns are excluded: they are compound tokens (management:owner), not
// fence-checkable role values.
var servingRoleQueries = map[string]string{
	"agents.profile":               `SELECT DISTINCT profile FROM agents`,
	"agents.role":                  `SELECT DISTINCT role FROM agents`,
	"runs.agent_profile":           `SELECT DISTINCT agent_profile FROM runs`,
	"runs.agent_role":              `SELECT DISTINCT agent_role FROM runs`,
	"channel_messages.role":        `SELECT DISTINCT role FROM channel_messages`,
	"inbox_deliveries.role":        `SELECT DISTINCT role FROM inbox_deliveries`,
	"work_items.authority":         `SELECT DISTINCT authority_profile FROM work_items`,
	"worker_updates.role":          `SELECT DISTINCT role FROM worker_updates`,
	"runs.meta_profile":            `SELECT DISTINCT metadata_json->>'$.agent_profile' FROM runs WHERE metadata_json->>'$.agent_profile' IS NOT NULL`,
	"runs.meta_role":               `SELECT DISTINCT metadata_json->>'$.agent_role' FROM runs WHERE metadata_json->>'$.agent_role' IS NOT NULL`,
	"runs.meta_requested_by":       `SELECT DISTINCT metadata_json->>'$.requested_by_profile' FROM runs WHERE metadata_json->>'$.requested_by_profile' IS NOT NULL`,
	"work_items.meta_requested_by": `SELECT DISTINCT details_json->>'$.requested_by_profile' FROM work_items WHERE details_json->>'$.requested_by_profile' IS NOT NULL`,
}

// servingRoleFields collects every distinct role-bearing value in the store
// for the serving fence.
func (s *Store) servingRoleFields(ctx context.Context) ([]vocabmigrate.Field, error) {
	names := make([]string, 0, len(servingRoleQueries))
	for name := range servingRoleQueries {
		names = append(names, name)
	}
	sort.Strings(names)
	var fields []vocabmigrate.Field
	for _, name := range names {
		rows, err := s.db.QueryContext(ctx, servingRoleQueries[name])
		if err != nil {
			return nil, fmt.Errorf("vocab fence scan %s: %w", name, err)
		}
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return nil, fmt.Errorf("vocab fence scan %s: %w", name, err)
			}
			fields = append(fields, vocabmigrate.Field{Key: name, Value: v})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("vocab fence scan %s: %w", name, err)
		}
	}
	return fields, nil
}

// VerifyServingVocabularyV2 refuses when any role-bearing value in the store
// falls outside the active V2 vocabulary (frozen protocol always passes).
func (s *Store) VerifyServingVocabularyV2(ctx context.Context) error {
	fields, err := s.servingRoleFields(ctx)
	if err != nil {
		return err
	}
	return vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV2, fields...)
}

// MigrateAndFenceServingVocabulary is the cutover transition: plan the
// forward migration, persist the merged provenance report BEFORE any row
// mutates (crash-atomicity: an interrupted run leaves correct provenance
// for every planned change, and re-planning over the same input rows is
// deterministic so a restart converges), apply the writes, then hold the
// serving fence closed until every row class agrees with the active V2
// vocabulary. It is idempotent and safe to run at every serving transition
// (post-replay boot, rematerialize flip, base publish); a second run only
// picks up rows written since the last pass.
func (s *Store) MigrateAndFenceServingVocabulary(ctx context.Context) (*MigrationReport, error) {
	prior, err := s.loadVocabMigrationReport()
	if err != nil {
		return nil, err
	}
	rep, writes, err := s.planVocabularyMigration(ctx)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		mergeVocabReports(prior, rep)
		rep = prior
	}
	// Provenance is durable before the first mutation: a crash after this
	// point can never lose or counterfeit the inverse record. A crash before
	// it leaves rows unmigrated and the prior report intact.
	if err := s.persistVocabMigrationReport(rep); err != nil {
		return nil, err
	}
	if err := s.applyVocabularyMigration(ctx, writes); err != nil {
		return nil, err
	}
	if err := s.VerifyServingVocabularyV2(ctx); err != nil {
		return nil, fmt.Errorf("vocab serving fence: %w", err)
	}
	return rep, nil
}

// mergeVocabReports folds a fresh migration report into the persisted one.
// Provenance entries whose post-migration row identity collides with a prior
// entry are dropped: the prior entry retains the original V1 token, which is
// the only correct INV-PROV source for that row.
func mergeVocabReports(dst, src *MigrationReport) {
	if dst.Provenance == nil {
		dst.Provenance = map[string]ProvEntry{}
	}
	if dst.Counts == nil {
		dst.Counts = map[string]int64{}
	}
	claimed := map[string]bool{}
	for key, entry := range dst.Provenance {
		claimed[key] = true
		claimed[provKey(entry.Table, migratedKeysForRevert(entry), entry.Column)] = true
	}
	for key, entry := range src.Provenance {
		if claimed[key] {
			continue
		}
		dst.Provenance[key] = entry
	}
	for key, n := range src.Counts {
		dst.Counts[key] += n
	}
}

// vocabReportFileName is the sidecar carrying the migration report inside the
// Dolt workspace directory. It lives outside the database so the witness
// extractor never observes it (a report table would change the schema hash
// and break live↔replay witness equality), while still traveling with the
// store through rematerialize flips and projection-base blobs.
const vocabReportFileName = "vocab-migration-report.json"

func (s *Store) vocabReportPath() string {
	return filepath.Join(s.texturePath, vocabReportFileName)
}

func (s *Store) persistVocabMigrationReport(rep *MigrationReport) error {
	raw, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("vocab migrate report encode: %w", err)
	}
	path := s.vocabReportPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	return nil
}

func (s *Store) loadVocabMigrationReport() (*MigrationReport, error) {
	raw, err := os.ReadFile(s.vocabReportPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vocab migrate report load: %w", err)
	}
	rep := &MigrationReport{}
	if err := json.Unmarshal(raw, rep); err != nil {
		return nil, fmt.Errorf("vocab migrate report decode: %w", err)
	}
	return rep, nil
}
