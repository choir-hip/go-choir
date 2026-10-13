package store

// Scratch migrate/revert/migrate drill for mission-2 (landing step 5). It
// exercises production migration code (not copies) on a scratch store: seed
// V1 object-graph rows, migrate, verify the fence, revert, verify
// byte-identity. The SQL role-column classes are gone with their tables
// (Cut 1 L2, 6c).
//
// Out of drill round 1 (cutover obligations, also listed on the production
// functions): prompts, TOML overlays, grants, digests, bare-compound ID
// suffixes.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func drillExec(t *testing.T, s *Store, query string, args ...any) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("drill seed: %v", err)
	}
}

// TestMigrateAndFenceRefusesUnknown proves the serving fence stays closed:
// an unknown role token survives migration untouched and fails the fence.
func TestMigrateAndFenceRefusesUnknown(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	body := []byte(`{"agent_id":"alias:one","profile":"boss"}`)
	metaNorm, err := objectgraph.NormalizeMetadata([]byte(`{"agent_id":"alias:one","profile":"boss"}`))
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
		objectgraph.StableSuffixFromKey("computer\x00alias:one"))
	if err != nil {
		t.Fatal(err)
	}
	hash := objectgraph.ContentHash("choir.agent", body, metaNorm)
	drillExec(t, s, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
		id, "choir.agent", "owner", "computer", "v1", hash, body, string(metaNorm), now, now)
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil); err == nil {
		t.Fatal("fence passed with unknown role token serving")
	}
	var got string
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM og_objects WHERE object_kind='choir.agent'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"profile":"boss"`) {
		t.Fatalf("unknown token rewritten: %s", got)
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

// TestVocabDrillOGConvergenceDedup reproduces the 2026-10-07 rebuild
// failure: two V1 choir.agent objects whose agent_id spellings differ
// ("co-super:impl" vs "cosuper:impl") both migrate to "engineering:impl",
// producing the same canonical_id. Identical post-migration rows dedupe
// (one row survives, one is deleted); divergent rows would fail loudly.
func TestVocabDrillOGConvergenceDedup(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	body1 := []byte(`{"agent_id":"co-super:impl","profile":"co-super"}`)
	meta1 := []byte(`{"agent_id":"co-super:impl","profile":"co-super"}`)
	body2 := []byte(`{"agent_id":"cosuper:impl","profile":"cosuper"}`)
	meta2 := []byte(`{"agent_id":"cosuper:impl","profile":"cosuper"}`)
	m1, err := objectgraph.NormalizeMetadata(meta1)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := objectgraph.NormalizeMetadata(meta2)
	if err != nil {
		t.Fatal(err)
	}
	id1, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
		objectgraph.StableSuffixFromKey("computer\x00co-super:impl"))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
		objectgraph.StableSuffixFromKey("computer\x00cosuper:impl"))
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("test requires distinct V1 canonical IDs")
	}
	for i, args := range [][]any{
		{id1, "choir.agent", "owner", "computer", "v1", objectgraph.ContentHash("choir.agent", body1, m1), body1, string(m1)},
		{id2, "choir.agent", "owner", "computer", "v1", objectgraph.ContentHash("choir.agent", body2, m2), body2, string(m2)},
	} {
		drillExec(t, s, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
			args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], now.Add(time.Duration(i)*time.Second), now)
	}

	rep, err := s.MigrateVocabularyToV2(ctx)
	if err != nil {
		t.Fatalf("og migrate with convergence: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.agent'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("og_objects after dedup: %d rows, want 1", count)
	}
	if len(rep.OGDropped) != 1 {
		t.Fatalf("OGDropped: got %v, want 1 entry", rep.OGDropped)
	}

	var finalID string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE object_kind='choir.agent'`).Scan(&finalID); err != nil {
		t.Fatal(err)
	}
	wantID, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
		objectgraph.StableSuffixFromKey("computer\x00engineering:impl"))
	if err != nil {
		t.Fatal(err)
	}
	if finalID != wantID {
		t.Fatalf("survivor canonical_id = %q, want %q", finalID, wantID)
	}

	// Survivor is the earlier-created row (co-super, first inserted).
	if rep.OGDropped[0] != id2 {
		t.Fatalf("dropped canonical_id = %q, want %q (cosuper)", rep.OGDropped[0], id2)
	}
}

// seedOGAgentPair inserts a V1 choir.agent row (agent_id "super:x") and a
// live V2 row for the same agent (agent_id "management:x", the V1 row's
// migration target), both created at the same instant, with the given
// updated_at stamps. It returns the V1 and V2 canonical IDs.
func seedOGAgentPair(t *testing.T, s *Store, v1Updated, v2Updated time.Time) (string, string) {
	t.Helper()
	created := v1Updated.Add(-24 * time.Hour)
	insert := func(agentID, extra string, updated time.Time) string {
		body := []byte(`{"agent_id":"` + agentID + `"` + extra + `}`)
		meta, err := objectgraph.NormalizeMetadata([]byte(`{"agent_id":"` + agentID + `"}`))
		if err != nil {
			t.Fatal(err)
		}
		id, err := objectgraph.BuildCanonicalID("choir.agent", "owner",
			objectgraph.StableSuffixFromKey("computer\x00"+agentID))
		if err != nil {
			t.Fatal(err)
		}
		drillExec(t, s, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
			id, "choir.agent", "owner", "computer", "", objectgraph.ContentHash("choir.agent", body, meta), body, string(meta), created, updated)
		return id
	}
	v1 := insert("super:x", `,"state":"stale"`, v1Updated)
	v2 := insert("management:x", `,"state":"current"`, v2Updated)
	return v1, v2
}

// TestVocabDrillOGStaleV1CollidesWithLaterV2 reproduces the fourth
// 2026-10-07 rebuild failure: replay deposits a pre-cutover V1 agent row
// next to the post-cutover V2 row for the same agent. The V1 row migrates
// onto the V2 row's canonical ID with divergent content. The live V2 row
// is newer, so it survives unchanged; the stale V1 row is removed and its
// original content kept in the report.
func TestVocabDrillOGStaleV1CollidesWithLaterV2(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 6, 4, 4, 22, 0, time.UTC)
	v1ID, v2ID := seedOGAgentPair(t, s, base, base.Add(31*24*time.Hour))

	rep, err := s.MigrateVocabularyToV2(ctx)
	if err != nil {
		t.Fatalf("og migrate with stale V1 collision: %v", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE object_kind='choir.agent'`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var id, body string
		if err := rows.Scan(&id, &body); err != nil {
			t.Fatal(err)
		}
		got = append(got, id+" "+body)
	}
	rows.Close()
	if len(got) != 1 || !strings.HasPrefix(got[0], v2ID+" ") || !strings.Contains(got[0], `"state":"current"`) {
		t.Fatalf("og_objects after migrate = %v, want only live V2 row %s with current state", got, v2ID)
	}
	if len(rep.OGSuperseded) != 1 {
		t.Fatalf("OGSuperseded = %+v, want one entry", rep.OGSuperseded)
	}
	sup := rep.OGSuperseded[0]
	if sup.OldCanonicalID != v1ID || sup.SurvivorCanonicalID != v2ID {
		t.Fatalf("OGSuperseded ids = %s -> %s, want %s -> %s", sup.OldCanonicalID, sup.SurvivorCanonicalID, v1ID, v2ID)
	}
	if !strings.Contains(string(sup.Body), `"agent_id":"super:x"`) || !strings.Contains(string(sup.Body), `"state":"stale"`) {
		t.Fatalf("OGSuperseded body lost original V1 content: %s", sup.Body)
	}
}

// TestVocabDrillOGNewerV1CollisionRefused: a migrating V1 row strictly
// newer than the live V2 row it collides with means a V1 write after the
// cutover. That is not a replay artifact; the migration must refuse it.
func TestVocabDrillOGNewerV1CollisionRefused(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 6, 4, 4, 22, 0, time.UTC)
	seedOGAgentPair(t, s, base.Add(time.Hour), base)

	if _, err := s.MigrateVocabularyToV2(ctx); err == nil || !strings.Contains(err.Error(), "convergence conflict") {
		t.Fatalf("migrate with newer V1 collision: err = %v, want convergence conflict", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.agent'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("refused migration mutated rows: %d agent rows, want 2", count)
	}
}
