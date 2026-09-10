package store

// Scratch migrate/revert/migrate drill for mission-2 (landing step 5,
// first half). It exercises production migration code (not copies) on a
// scratch store: seed V1 rows across SQL classes, migrate, verify fence +
// joins + authorization equivalence, revert, verify byte-identity, repeat,
// ending on V1 serving rows. The drill never leaves mixed vocabulary
// serving: every phase ends verified before the next begins.
//
// Out of drill round 1 (cutover obligations, also listed on the production
// functions): object-graph, prompts, TOML overlays, grants, digests,
// bare-compound ID suffixes.

import (
	"encoding/json"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

func drillExec(t *testing.T, s *Store, query string, args ...any) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("drill seed: %v", err)
	}
}

func drillSeedV1(t *testing.T, s *Store) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	drillExec(t, s, `INSERT INTO agents (agent_id, owner_id, computer_id, profile, role, channel_id, created_at, updated_at) VALUES
		('super:root', 'owner', 'computer', 'super', 'super', 'super:root', ?, ?),
		('co-super:impl', 'owner', 'computer', 'co-super', 'co-super', 'chan-1', ?, ?),
		('researcher:doc', 'owner', 'computer', 'researcher', 'researcher', 'doc-1', ?, ?),
		('texture:doc', 'owner', 'computer', 'texture', 'texture', 'doc-1', ?, ?),
		('alias:one', 'owner', 'computer', 'cosuper', 'CoAgent', 'chan-1', ?, ?)`,
		now, now, now, now, now, now, now, now, now, now)
	drillExec(t, s, `INSERT INTO runs (loop_id, agent_id, channel_id, requested_by_run_id, trajectory_id, agent_profile, agent_role, owner_id, computer_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json) VALUES
		('run-super-1', 'super:root', 'super:root', '', 'traj-1', 'super', 'super', 'owner', 'computer', 'completed', 'p', '', '', ?, ?, NULL, '{"agent_profile":"super","requested_by_profile":"super"}'),
		('run-cosuper-1', 'co-super:impl', 'chan-1', 'run-super-1', 'traj-1', 'cosuper', 'researcher', 'owner', 'computer', 'running', 'p', '', '', ?, ?, NULL, '{"agent_profile":"cosuper","requested_by_profile":"co_super"}'),
		('run-research-1', 'researcher:doc', 'doc-1', 'run-super-1', 'traj-1', 'researchers', 'research', 'owner', 'computer', 'running', 'p', '', '', ?, ?, NULL, '{}')`,
		now, now, now, now, now, now)
	drillExec(t, s, `INSERT INTO channel_messages (channel_id, seq, owner_id, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at) VALUES
		('chan-1', 1, 'owner', 'co-super:impl', 'run-cosuper-1', 'super:root', 'run-super-1', 'traj-1', '', 'co-super', 'packet', ?),
		('doc-1', 2, 'owner', 'researcher:doc', 'run-research-1', 'texture:doc', '', 'traj-1', '', 'researcher', 'evidence', ?),
		('chan-9', 3, 'owner', 'alias:one', '', '', '', 'traj-9', '', 'boss', 'mystery', ?)`,
		now, now, now)
	drillExec(t, s, `INSERT INTO inbox_deliveries (delivery_id, owner_id, to_agent_id, to_loop_id, from_agent_id, from_loop_id, channel_id, role, content, trajectory_id, created_at) VALUES
		('del-1', 'owner', 'super:root', 'run-super-1', 'co-super:impl', 'run-cosuper-1', 'chan-1', 'super', 'wake', 'traj-1', ?)`,
		now)
	drillExec(t, s, `INSERT INTO work_items (work_item_id, trajectory_id, owner_id, objective, reason, authority_profile, step_budget, token_budget, objective_fingerprint, status, assigned_agent_id, created_by_loop_id, details_json, created_at, updated_at) VALUES
		('work-super-assignment', 'traj-1', 'owner', 'coordinate', '', 'super', 0, 0, 'fp', 'open', 'super:root', 'run-super-1', '{"requested_by_profile":"super"}', ?, ?),
		('work-cosuper-1', 'traj-1', 'owner', 'implement', '', 'co-super', 0, 0, 'fp', 'open', 'co-super:impl', 'run-cosuper-1', '{}', ?, ?),
		('work-co-super-2', 'traj-1', 'owner', 'review', '', 'co-super', 0, 0, 'fp', 'open', 'co-super:impl', 'run-cosuper-1', '{}', ?, ?)`,
		now, now, now, now, now, now)
	drillExec(t, s, `INSERT INTO worker_updates (owner_id, update_id, agent_id, target_agent_id, channel_id, message_seq, trajectory_id, role, kind, summary, packet_json, content, created_at, delivered_to_loop_id) VALUES
		('owner', 'upd-1', 'researcher:doc', 'super:root', 'doc-1', 1, 'traj-1', 'researcher', 'evidence_update', 's', '{}', 'c', ?, '')`,
		now)
	drillExec(t, s, `INSERT INTO coagent_mailboxes (owner_id, agent_id, channel_id, processed_message_seq, updated_at) VALUES
		('owner', 'super:root', 'super:root', 0, ?)`,
		now)
}

func drillSnapshot(t *testing.T, s *Store) map[string][]string {
	t.Helper()
	ctx := context.Background()
	tables := map[string]string{
		"agents":            "agent_id",
		"runs":              "loop_id",
		"channel_messages":  "channel_id, seq",
		"inbox_deliveries":  "delivery_id",
		"work_items":        "work_item_id",
		"worker_updates":    "owner_id, update_id",
		"coagent_mailboxes": "owner_id, agent_id",
	}
	snap := map[string][]string{}
	for table, order := range tables {
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT * FROM %s ORDER BY %s`, table, order))
		if err != nil {
			t.Fatalf("drill snapshot %s: %v", table, err)
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatal(err)
		}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(vals))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			var b strings.Builder
			for i, c := range cols {
				fmt.Fprintf(&b, "%s=%v|", c, vals[i])
			}
			snap[table] = append(snap[table], b.String())
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		sort.Strings(snap[table])
	}
	return snap
}

func drillAssertSnapshotsEqual(t *testing.T, want, got map[string][]string) {
	t.Helper()
	for table, wantRows := range want {
		gotRows := got[table]
		if len(gotRows) != len(wantRows) {
			t.Fatalf("drill table %s row count %d, want %d", table, len(gotRows), len(wantRows))
		}
		for i := range wantRows {
			if gotRows[i] != wantRows[i] {
				t.Fatalf("drill table %s row %d differs:\n got: %s\nwant: %s", table, i, gotRows[i], wantRows[i])
			}
		}
	}
}

func drillRoleColumns(t *testing.T, s *Store) map[string][]string {
	t.Helper()
	ctx := context.Background()
	queries := map[string]string{
		"agents.profile":               `SELECT DISTINCT profile FROM agents`,
		"agents.role":                  `SELECT DISTINCT role FROM agents`,
		"runs.agent_profile":           `SELECT DISTINCT agent_profile FROM runs`,
		"runs.agent_role":              `SELECT DISTINCT agent_role FROM runs`,
		"channel_messages.role":        `SELECT DISTINCT role FROM channel_messages`,
		"inbox_deliveries.role":        `SELECT DISTINCT role FROM inbox_deliveries`,
		"work_items.authority":         `SELECT DISTINCT authority_profile FROM work_items`,
		"worker_updates.role":          `SELECT DISTINCT role FROM worker_updates`,
		"runs.meta_profile":            `SELECT DISTINCT metadata_json->>'$.agent_profile' FROM runs WHERE metadata_json->>'$.agent_profile' IS NOT NULL`,
		"runs.meta_requested_by":       `SELECT DISTINCT metadata_json->>'$.requested_by_profile' FROM runs WHERE metadata_json->>'$.requested_by_profile' IS NOT NULL`,
		"work_items.meta_requested_by": `SELECT DISTINCT details_json->>'$.requested_by_profile' FROM work_items WHERE details_json->>'$.requested_by_profile' IS NOT NULL`,
	}
	out := map[string][]string{}
	for name, q := range queries {
		rows, err := s.db.QueryContext(ctx, q)
		if err != nil {
			t.Fatalf("drill role scan %s: %v", name, err)
		}
		var vals []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			vals = append(vals, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		out[name] = vals
	}
	return out
}

func drillAssertFence(t *testing.T, active string, cols map[string][]string) {
	t.Helper()
	for name, vals := range cols {
		for _, v := range vals {
			// The boss control row carries an unknown token by design: it
			// is asserted separately (left in place, fence refuses it).
			if v == "boss" {
				continue
			}
			if err := vocabmigrate.VerifyServingVocabulary(active, vocabmigrate.Field{Key: name, Value: v}); err != nil {
				t.Fatalf("drill fence %s: %v", active, err)
			}
		}
	}
}

func drillAssertJoins(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`SELECT COUNT(*) FROM channel_messages m LEFT JOIN agents a ON a.agent_id = m.to_agent_id WHERE m.to_agent_id != '' AND a.agent_id IS NULL`,
		`SELECT COUNT(*) FROM runs r LEFT JOIN agents a ON a.agent_id = r.agent_id WHERE a.agent_id IS NULL`,
		`SELECT COUNT(*) FROM work_items w LEFT JOIN agents a ON a.agent_id = w.assigned_agent_id WHERE a.agent_id IS NULL`,
		`SELECT COUNT(*) FROM inbox_deliveries d LEFT JOIN agents a ON a.agent_id = d.to_agent_id WHERE d.to_agent_id != '' AND a.agent_id IS NULL`,
	} {
		var n int
		if err := s.db.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatalf("drill join: %v", err)
		}
		if n != 0 {
			t.Fatalf("drill join broken: %d orphans: %s", n, q)
		}
	}
}

// drillAssertAuthzEquivalence proves the migration preserves authorization
// topology: for every caller/target agent pair, V1 policy outcomes on the
// inverse-restored profiles equal outcomes on the original V1 profiles.
// Restoration uses provenance-exact tokens where retained, else canonical.
func drillAssertAuthzEquivalence(t *testing.T, s *Store, rep *MigrationReport, seeds map[string][2]string) {
	t.Helper()
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, `SELECT agent_id, profile, role FROM agents`)
	if err != nil {
		t.Fatal(err)
	}
	type rec struct{ id, profile, role string }
	var current []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.profile, &r.role); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		current = append(current, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	restored := map[string][2]string{}
	for _, r := range current {
		prof, role := r.profile, r.role
		if entry, ok := rep.Provenance["agents:"+r.id+":profile"]; ok {
			prof = entry.Exact
		} else if canon, ok := vocabmigrate.InverseV2ToV1Canonical(r.profile); ok {
			prof = canon
		}
		if entry, ok := rep.Provenance["agents:"+r.id+":role"]; ok {
			role = entry.Exact
		} else if canon, ok := vocabmigrate.InverseV2ToV1Canonical(r.role); ok {
			role = canon
		}
		restored[r.id] = [2]string{prof, role}
	}
	for _, caller := range current {
		for _, target := range current {
			cOrig, okC := seeds[drillOriginalID(caller.id)]
			tOrig, okT := seeds[drillOriginalID(target.id)]
			if !okC || !okT {
				continue
			}
			cRest := restored[caller.id]
			tRest := restored[target.id]
			spawnRest, spawnRestErr := agentprofile.CanSpawn(cRest[0], tRest[0])
			spawnOrig, spawnOrigErr := agentprofile.CanSpawn(cOrig[0], tOrig[0])
			if spawnRest != spawnOrig || (spawnRestErr == nil) != (spawnOrigErr == nil) {
				t.Fatalf("drill authz: CanSpawn(%q,%q) mismatch after restore (orig %q,%q)",
					cRest[0], tRest[0], cOrig[0], tOrig[0])
			}
			msgRest, msgRestErr := agentprofile.CanMessage(cRest[0], tRest[0])
			msgOrig, msgOrigErr := agentprofile.CanMessage(cOrig[0], tOrig[0])
			if msgRest != msgOrig || (msgRestErr == nil) != (msgOrigErr == nil) {
				t.Fatalf("drill authz: CanMessage mismatch for (%q,%q)", cRest[0], tRest[0])
			}
		}
	}
}

// drillOriginalID maps a possibly-migrated agent ID back to its seed ID
// through the canonical inverse (alias:one keeps its ID).
func drillOriginalID(id string) string {
	for _, prefix := range []string{"management:", "engineering:", "research:"} {
		if strings.HasPrefix(id, prefix) {
			back := map[string]string{
				"management:": "super:", "engineering:": "co-super:", "research:": "researcher:",
			}[prefix]
			return back + strings.TrimPrefix(id, prefix)
		}
	}
	return id
}

func TestVocabDrillMigrateRevertMigrate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	drillSeedV1(t, s)
	original := drillSnapshot(t, s)

	// Original V1 rows fence clean under v1 (aliases included), and seed
	// profiles for authorization equivalence (all seeded agents).
	originals := map[string][2]string{
		"super:root":     {"super", "super"},
		"co-super:impl":  {"co-super", "co-super"},
		"researcher:doc": {"researcher", "researcher"},
		"texture:doc":    {"texture", "texture"},
		"alias:one":      {"cosuper", "CoAgent"},
	}

	// Unknown tokens are never silently migrated: the boss row stays put.
	assertBoss := func(where string) {
		t.Helper()
		var role string
		if err := s.db.QueryRowContext(ctx, `SELECT role FROM channel_messages WHERE channel_id = 'chan-9'`).Scan(&role); err != nil {
			t.Fatal(err)
		}
		if role != "boss" {
			t.Fatalf("drill %s: unknown token rewritten to %q", where, role)
		}
		if err := vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV1,
			vocabmigrate.Field{Key: "channel_messages.role", Value: role}); err == nil {
			t.Fatalf("drill %s: fence passed unknown token", where)
		}
	}

	// Round 1: migrate, verify fence-v2, joins, authz.
	rep, err := s.MigrateVocabularyToV2(ctx)
	if err != nil {
		t.Fatalf("drill migrate: %v", err)
	}
	if len(rep.Provenance) == 0 {
		t.Fatal("drill migrate recorded no provenance; alias coverage missing")
	}
	drillAssertFence(t, vocabmigrate.VocabularyV2, drillRoleColumns(t, s))
	drillAssertJoins(t, s)
	drillAssertAuthzEquivalence(t, s, rep, originals)
	// Longest-token-first: "work-co-super-2" must migrate to
	// "work-engineering-2", never "work-co-management-2" (the "-super-"
	// infix is a substring of "-co-super-"; regression pin for F3).
	var wid string
	if err := s.db.QueryRowContext(ctx, `SELECT work_item_id FROM work_items WHERE objective = 'review'`).Scan(&wid); err != nil {
		t.Fatalf("drill infix: %v", err)
	}
	if wid != "work-engineering-2" {
		t.Fatalf("drill infix: work-co-super-2 migrated to %q, want work-engineering-2", wid)
	}
	assertBoss("post-migrate")

	// Revert: byte-identical through provenance (aliases included).
	if err := s.RevertVocabularyToV1(ctx, rep); err != nil {
		t.Fatalf("drill revert: %v", err)
	}
	drillAssertSnapshotsEqual(t, original, drillSnapshot(t, s))
	drillAssertFence(t, vocabmigrate.VocabularyV1, drillRoleColumns(t, s))
	drillAssertJoins(t, s)

	// Round 2: re-migrate, verify, revert, end on V1 serving rows.
	rep2, err := s.MigrateVocabularyToV2(ctx)
	if err != nil {
		t.Fatalf("drill re-migrate: %v", err)
	}
	drillAssertFence(t, vocabmigrate.VocabularyV2, drillRoleColumns(t, s))
	drillAssertJoins(t, s)
	drillAssertAuthzEquivalence(t, s, rep2, originals)
	if err := s.RevertVocabularyToV1(ctx, rep2); err != nil {
		t.Fatalf("drill final revert: %v", err)
	}
	drillAssertSnapshotsEqual(t, original, drillSnapshot(t, s))

	// Terminal serving state: every role column V1, zero V2-only names.
	cols := drillRoleColumns(t, s)
	drillAssertFence(t, vocabmigrate.VocabularyV1, cols)
	for name, vals := range cols {
		for _, v := range vals {
			if v == "management" || v == "engineering" {
				t.Fatalf("drill terminal: V2-only token %q in %s", v, name)
			}
		}
	}
}

// TestMigrateAndFenceServingVocabulary exercises the production cutover
// orchestrator: migrate + persist + fence in one call, idempotent across
// repeated serving transitions, with provenance surviving a second run so
// revert still restores the original V1 bytes.
func TestMigrateAndFenceServingVocabulary(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	drillExec(t, s, `INSERT INTO agents (agent_id, owner_id, computer_id, profile, role, channel_id, created_at, updated_at) VALUES
		('super:root', 'owner', 'computer', 'super', 'super', 'super:root', ?, ?),
		('co-super:impl', 'owner', 'computer', 'co-super', 'co-super', 'chan-1', ?, ?),
		('alias:one', 'owner', 'computer', 'cosuper', 'CoAgent', 'chan-1', ?, ?)`,
		now, now, now, now, now, now)
	original := drillSnapshot(t, s)

	// First transition: migrates, persists the report, fence passes.
	if _, err := s.MigrateAndFenceServingVocabulary(ctx); err != nil {
		t.Fatalf("first migrate+fence: %v", err)
	}
	rep, err := s.loadVocabMigrationReport()
	if err != nil || rep == nil {
		t.Fatalf("report not persisted: %v", err)
	}
	if len(rep.Provenance) == 0 {
		t.Fatal("persisted report has no provenance")
	}

	// Second transition (e.g. next boot): must not overwrite the original
	// V1 provenance with the migrated V2 spellings.
	if _, err := s.MigrateAndFenceServingVocabulary(ctx); err != nil {
		t.Fatalf("second migrate+fence: %v", err)
	}
	rep, err = s.loadVocabMigrationReport()
	if err != nil {
		t.Fatalf("reload report: %v", err)
	}
	if err := s.RevertVocabularyToV1(ctx, rep); err != nil {
		t.Fatalf("revert after double migration: %v", err)
	}
	drillAssertSnapshotsEqual(t, original, drillSnapshot(t, s))
}

// TestMigrateAndFenceRefusesUnknown proves the serving fence stays closed:
// an unknown role token survives migration untouched and fails the fence.
func TestMigrateAndFenceRefusesUnknown(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	drillExec(t, s, `INSERT INTO channel_messages (channel_id, seq, owner_id, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at) VALUES
		('chan-9', 3, 'owner', 'alias:one', '', '', '', 'traj-9', '', 'boss', 'mystery', ?)`, now)
	if _, err := s.MigrateAndFenceServingVocabulary(ctx); err == nil {
		t.Fatal("fence passed with unknown role token serving")
	}
	var role string
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM channel_messages WHERE channel_id = 'chan-9'`).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "boss" {
		t.Fatalf("unknown token rewritten to %q", role)
	}
}

// TestVocabDrillObjectGraph exercises the OG carrier migration: a
// key-suffixed agent object re-derives its canonical ID from migrated
// identity fields, a content-suffixed event re-derives from migrated
// content, an edge rewrites endpoints and edge_id, embedded obj: refs
// converge through the fixpoint, the write guard refuses V1 role writes
// post-cutover, and revert restores byte-identical rows.
func TestVocabDrillObjectGraph(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Seed a V1 agent object: canonical ID derives from the V1 agent_id.
	agentBody := []byte(`{"agent_id":"co-super:impl","profile":"co-super"}`)
	agentMeta := []byte(`{"agent_id":"co-super:impl","profile":"co-super"}`)
	agentMetaNorm, err := objectgraph.NormalizeMetadata(agentMeta)
	if err != nil {
		t.Fatal(err)
	}
	agentID, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
		objectgraph.StableSuffixFromKey("computer\x00co-super:impl"))
	if err != nil {
		t.Fatal(err)
	}
	agentHash := objectgraph.ContentHash("choir.agent", agentBody, agentMetaNorm)
	drillExec(t, s, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
		agentID, "choir.agent", "owner", "computer", "v1", agentHash, agentBody, string(agentMetaNorm), now, now)

	// Seed a V1 event embedding the agent's canonical ID plus a desk role.
	eventBody := []byte(`{"actor_profile":"co-super","ref":"` + agentID + `"}`)
	eventMeta := []byte(`{"actor_profile":"co-super"}`)
	eventMetaNorm, err := objectgraph.NormalizeMetadata(eventMeta)
	if err != nil {
		t.Fatal(err)
	}
	eventHash := objectgraph.ContentHash("choir.event", eventBody, eventMetaNorm)
	eventID, err := objectgraph.BuildCanonicalID("choir.event", "owner",
		objectgraph.StableSuffixFromContent(eventHash))
	if err != nil {
		t.Fatal(err)
	}
	drillExec(t, s, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
		eventID, "choir.event", "owner", "computer", "v1", eventHash, eventBody, string(eventMetaNorm), now, now)

	// Edge from agent to event.
	edgeID, err := objectgraph.BuildEdgeID(agentID, eventID, "produced", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	drillExec(t, s, `INSERT INTO og_edges (edge_id, from_id, to_id, kind, metadata, created_at, tombstone) VALUES (?,?,?,?,?,?,0)`,
		edgeID, agentID, eventID, "produced", "{}", now)

	snapOG := func() map[string][]string {
		out := map[string][]string{}
		for _, table := range []string{"og_objects", "og_edges"} {
			rows, err := s.db.QueryContext(ctx, `SELECT * FROM `+table)
			if err != nil {
				t.Fatal(err)
			}
			cols, _ := rows.Columns()
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					t.Fatal(err)
				}
				var line string
				for i, c := range cols {
					line += c + "=" + fmt.Sprintf("%v", vals[i]) + "|"
				}
				out[table] = append(out[table], line)
			}
			rows.Close()
			sort.Strings(out[table])
		}
		return out
	}
	original := snapOG()

	// Migrate.
	rep, err := s.MigrateVocabularyToV2(ctx)
	if err != nil {
		t.Fatalf("og migrate: %v", err)
	}
	if len(rep.OGObjects) != 2 || len(rep.OGEdges) != 1 {
		t.Fatalf("og provenance: got %d objects %d edges, want 2/1", len(rep.OGObjects), len(rep.OGEdges))
	}

	// Agent canonical ID re-derived from migrated identity key.
	newAgentID, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
		objectgraph.StableSuffixFromKey("computer\x00engineering:impl"))
	if err != nil {
		t.Fatal(err)
	}
	var gotAgent string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE object_kind='choir.agent'`).Scan(&gotAgent); err != nil {
		t.Fatal(err)
	}
	if gotAgent != newAgentID {
		t.Fatalf("agent canonical_id = %q, want %q", gotAgent, newAgentID)
	}

	// Event re-derived from migrated content; embedded ref converged.
	var evBody string
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM og_objects WHERE object_kind='choir.event'`).Scan(&evBody); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(evBody, `"actor_profile":"engineering"`) || !strings.Contains(evBody, newAgentID) {
		t.Fatalf("event body not migrated: %s", evBody)
	}

	// Edge endpoints and edge_id rewritten.
	var eFrom, eTo, eID string
	if err := s.db.QueryRowContext(ctx, `SELECT edge_id, from_id, to_id FROM og_edges`).Scan(&eID, &eFrom, &eTo); err != nil {
		t.Fatal(err)
	}
	if eFrom != newAgentID {
		t.Fatalf("edge from_id = %q, want %q", eFrom, newAgentID)
	}
	wantEdge, err := objectgraph.BuildEdgeID(newAgentID, eTo, "produced", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if eID != wantEdge {
		t.Fatalf("edge_id = %q, want %q", eID, wantEdge)
	}

	// Write guard: post-cutover V1 role write refuses.
	badObj := objectgraph.Object{
		CanonicalID: "obj:choir.agent:owner:key-x",
		ObjectKind:  "choir.agent",
		OwnerID:     "owner",
		Metadata:    []byte(`{"profile":"co-super"}`),
	}
	if err := s.ogStore.PutObject(ctx, badObj); err == nil {
		t.Fatal("write guard passed V1 role token post-cutover")
	}

	// Revert: byte-identical.
	if err := s.RevertVocabularyToV1(ctx, rep); err != nil {
		t.Fatalf("og revert: %v", err)
	}
	reverted := snapOG()
	for table := range original {
		if len(original[table]) != len(reverted[table]) {
			t.Fatalf("og revert %s: %d rows, want %d", table, len(reverted[table]), len(original[table]))
		}
		for i := range original[table] {
			if original[table][i] != reverted[table][i] {
				t.Fatalf("og revert %s row %d differs:\n got %s\nwant %s", table, i, reverted[table][i], original[table][i])
			}
		}
	}
}
