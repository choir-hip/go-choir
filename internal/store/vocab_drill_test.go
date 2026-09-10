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
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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
		('work-cosuper-1', 'traj-1', 'owner', 'implement', '', 'co-super', 0, 0, 'fp', 'open', 'co-super:impl', 'run-cosuper-1', '{}', ?, ?)`,
		now, now, now, now)
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
			if agentprofile.CanSpawn(cRest[0], tRest[0]) != agentprofile.CanSpawn(cOrig[0], tOrig[0]) {
				t.Fatalf("drill authz: CanSpawn(%q,%q) mismatch after restore (orig %q,%q)",
					cRest[0], tRest[0], cOrig[0], tOrig[0])
			}
			if agentprofile.CanMessage(cRest[0], tRest[0]) != agentprofile.CanMessage(cOrig[0], tOrig[0]) {
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
