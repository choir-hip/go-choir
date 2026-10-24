package agentcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// These tests pin docs/problems/data-has-more-than-one-home-2026-10-24.md,
// step 3. agent_evidence (research SaveEvidence) and co_super_slots
// (engineering slot claims) were written by direct SQL, off the tape, and the
// replay manifest admits them only while empty, so one evidence row or one
// claimed slot made the computer checkpoint-ineligible. With the computer
// setting on, both write graph objects (on the tape) and the tables are
// emptied by a one-time import.

const graphHomeOwner = "owner-graph-home"

func turnOn(t *testing.T, rt *Runtime, computerID, key string) {
	t.Helper()
	if err := rt.store.PutComputerSetting(context.Background(), choirstore.ComputerSetting{
		OwnerID: graphHomeOwner, ComputerID: computerID, Key: key, Value: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func tableCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func evidenceRec(id, agent string, at time.Time) types.EvidenceRecord {
	return types.EvidenceRecord{
		EvidenceID: id, OwnerID: graphHomeOwner, AgentID: agent, Kind: "web_page",
		SourceURI: "https://example.test/" + id, Title: "T " + id, Content: "body " + id,
		Metadata: []byte(`{"k":"v"}`), CreatedAt: at,
	}
}

func seedLegacyEvidence(t *testing.T, db *sql.DB, id, agent string, at time.Time) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO agent_evidence (evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at)
		VALUES (?, ?, ?, 'web_page', 'https://old.test/x', 'Old', 'old body', '{}', ?)`,
		id, graphHomeOwner, agent, at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
}

func requireEligible(t *testing.T, rt *Runtime, computerID string) {
	t.Helper()
	requireMediaStateReplayEligible(t, rt, computerID)
}

func requireIneligible(t *testing.T, rt *Runtime, computerID, table string) {
	t.Helper()
	report, err := rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Eligibility.Eligible {
		t.Fatalf("control: replay eligible with rows in %s", table)
	}
	found := false
	for _, u := range report.Eligibility.UnsupportedTables {
		if u == table {
			found = true
		}
	}
	if !found {
		t.Fatalf("unsupported tables = %v, want %s", report.Eligibility.UnsupportedTables, table)
	}
}

func TestEvidenceDefaultsOffAndStaysInTheTable(t *testing.T) {
	computerID := "computer-evidence-off"
	rt, _ := selfdevReplayRuntime(t, computerID)
	if err := rt.store.CreateEvidence(context.Background(), evidenceRec("ev-1", "researcher", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, rt.store.DB(), "agent_evidence"); n != 1 {
		t.Fatalf("setting off: table rows = %d, want 1 (the old home until turned on)", n)
	}
	requireIneligible(t, rt, computerID, "agent_evidence")
}

func TestEvidenceGraphHomeReplaysEligibleAndEquivalent(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-evidence-graph"
	rt, _ := selfdevReplayRuntime(t, computerID)
	turnOn(t, rt, computerID, choirstore.SettingEvidenceGraph)

	base := time.Now().UTC().Truncate(time.Millisecond)
	for i, agent := range []string{"researcher", "researcher", "other"} {
		if err := rt.store.CreateEvidence(ctx, evidenceRec(fmt.Sprintf("ev-%d", i+1), agent, base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if n := tableCount(t, rt.store.DB(), "agent_evidence"); n != 0 {
		t.Fatalf("setting on: table rows = %d, want 0", n)
	}
	requireEligible(t, rt, computerID)

	got, err := rt.store.GetEvidence(ctx, "ev-2", graphHomeOwner)
	if err != nil || got.Content != "body ev-2" || string(got.Metadata) != `{"k":"v"}` || got.AgentID != "researcher" {
		t.Fatalf("GetEvidence = %+v, %v", got, err)
	}
	if _, err := rt.store.GetEvidence(ctx, "ev-2", "someone-else"); err == nil {
		t.Fatal("evidence read as another owner")
	}
	list, err := rt.store.ListEvidenceByAgent(ctx, graphHomeOwner, "researcher", 10)
	if err != nil || len(list) != 2 || list[0].EvidenceID != "ev-2" || list[1].EvidenceID != "ev-1" {
		t.Fatalf("ListEvidenceByAgent = %+v, %v", list, err)
	}
	all, err := rt.store.ListEvidenceByAgent(ctx, graphHomeOwner, "", 2)
	if err != nil || len(all) != 2 || all[0].EvidenceID != "ev-3" || all[1].EvidenceID != "ev-2" {
		t.Fatalf("ListEvidenceByAgent(all, limit 2) = %+v, %v", all, err)
	}
}

func TestEvidenceRowsFromBeforeTheChangeAreImportedAndStillRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		move func(t *testing.T, rt *Runtime, computerID string)
	}{
		{"the first write imports", func(t *testing.T, rt *Runtime, _ string) {
			if err := rt.store.CreateEvidence(context.Background(), evidenceRec("ev-new", "researcher", time.Now().UTC())); err != nil {
				t.Fatal(err)
			}
		}},
		{"the explicit import", func(t *testing.T, rt *Runtime, _ string) {
			moved, err := rt.store.ImportEvidenceToGraph(context.Background())
			if err != nil || moved != 1 {
				t.Fatalf("import = %d, %v", moved, err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computerID := "computer-evidence-legacy"
			rt, _ := selfdevReplayRuntime(t, computerID)
			seedLegacyEvidence(t, rt.store.DB(), "ev-old", "researcher", time.Now().UTC().Add(-time.Hour))
			requireIneligible(t, rt, computerID, "agent_evidence")

			// Switch off: nothing moves, the old row still reads.
			if _, err := rt.store.GetEvidence(context.Background(), "ev-old", graphHomeOwner); err != nil {
				t.Fatalf("legacy read: %v", err)
			}
			turnOn(t, rt, computerID, choirstore.SettingEvidenceGraph)
			tc.move(t, rt, computerID)
			if n := tableCount(t, rt.store.DB(), "agent_evidence"); n != 0 {
				t.Fatalf("table rows after import = %d, want 0", n)
			}
			requireEligible(t, rt, computerID)
			old, err := rt.store.GetEvidence(context.Background(), "ev-old", graphHomeOwner)
			if err != nil || old.Content != "old body" || old.SourceURI != "https://old.test/x" {
				t.Fatalf("imported evidence = %+v, %v", old, err)
			}
			// An import run again moves nothing and duplicates nothing.
			if moved, err := rt.store.ImportEvidenceToGraph(context.Background()); err != nil || moved != 0 {
				t.Fatalf("second import = %d, %v", moved, err)
			}
			list, _ := rt.store.ListEvidenceByAgent(context.Background(), graphHomeOwner, "researcher", 10)
			ids := map[string]int{}
			for _, r := range list {
				ids[r.EvidenceID]++
			}
			if ids["ev-old"] != 1 {
				t.Fatalf("ev-old listed %d times: %+v", ids["ev-old"], list)
			}
		})
	}
}

func TestEvidenceRollbackReadsBothHomesAndReimports(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-evidence-rollback"
	rt, _ := selfdevReplayRuntime(t, computerID)
	turnOn(t, rt, computerID, choirstore.SettingEvidenceGraph)
	if err := rt.store.CreateEvidence(ctx, evidenceRec("ev-graph", "r", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	// Off again: the next write goes to the table; both read.
	if err := rt.store.PutComputerSetting(ctx, choirstore.ComputerSetting{OwnerID: graphHomeOwner, ComputerID: computerID, Key: choirstore.SettingEvidenceGraph, Value: false}); err != nil {
		t.Fatal(err)
	}
	if err := rt.store.CreateEvidence(ctx, evidenceRec("ev-table", "r", time.Now().UTC().Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, rt.store.DB(), "agent_evidence"); n != 1 {
		t.Fatalf("rollback write: table rows = %d, want 1", n)
	}
	list, err := rt.store.ListEvidenceByAgent(ctx, graphHomeOwner, "r", 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("both homes must read: %+v, %v", list, err)
	}
	// On again: the next write folds the table row in.
	turnOn(t, rt, computerID, choirstore.SettingEvidenceGraph)
	if err := rt.store.CreateEvidence(ctx, evidenceRec("ev-again", "r", time.Now().UTC().Add(2*time.Second))); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, rt.store.DB(), "agent_evidence"); n != 0 {
		t.Fatalf("after turning on again: table rows = %d, want 0", n)
	}
	requireEligible(t, rt, computerID)
}

func TestEvidenceGraphWriteFailureDoesNotFallBackToTheTable(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-evidence-fail"
	rt, _ := selfdevReplayRuntime(t, computerID)
	turnOn(t, rt, computerID, choirstore.SettingEvidenceGraph)
	bad := evidenceRec("", "r", time.Now().UTC()) // no identity: the graph refuses
	if err := rt.store.CreateEvidence(ctx, bad); err == nil {
		t.Fatal("an evidence record with no id was accepted")
	}
	if n := tableCount(t, rt.store.DB(), "agent_evidence"); n != 0 {
		t.Fatalf("a failed graph write left %d table rows", n)
	}
}

func claimSlot(t *testing.T, rt *Runtime, traj, slot, runID, agentID string) (types.RunRecord, bool) {
	t.Helper()
	rec, claimed, err := rt.store.ClaimEngineeringSlot(context.Background(), graphHomeOwner, traj, slot, runID, agentID, "requester")
	if err != nil {
		t.Fatalf("claim %s/%s by %s: %v", traj, slot, runID, err)
	}
	return rec, claimed
}

func makeRun(t *testing.T, rt *Runtime, runID, agentID, traj string, state types.RunState) types.RunRecord {
	t.Helper()
	now := time.Now().UTC()
	rec := types.RunRecord{RunID: runID, AgentID: agentID, OwnerID: graphHomeOwner, ComputerID: "c", State: state,
		Prompt: "p", TrajectoryID: traj, CreatedAt: now, UpdatedAt: now}
	if err := rt.store.CreateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestSlotsDefaultOffAndStayInTheTable(t *testing.T) {
	computerID := "computer-slots-off"
	rt, _ := selfdevReplayRuntime(t, computerID)
	if _, claimed := claimSlot(t, rt, "traj-1", "implementation", "run-1", "agent-1"); !claimed {
		t.Fatal("claim refused")
	}
	if n := tableCount(t, rt.store.DB(), "co_super_slots"); n != 1 {
		t.Fatalf("setting off: table rows = %d, want 1", n)
	}
	requireIneligible(t, rt, computerID, "co_super_slots")
}

func TestSlotsGraphHomeSemanticsAndReplay(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-slots-graph"
	rt, _ := selfdevReplayRuntime(t, computerID)
	turnOn(t, rt, computerID, choirstore.SettingEngineeringSlotsGraph)

	run1 := makeRun(t, rt, "run-1", "agent-1", "traj-1", types.RunRunning)
	makeRun(t, rt, "run-2", "agent-2", "traj-1", types.RunRunning)

	if _, claimed := claimSlot(t, rt, "traj-1", "implementation", "run-1", "agent-1"); !claimed {
		t.Fatal("first claim refused")
	}
	// A live owner holds the slot: the second claim returns it, unclaimed.
	existing, claimed := claimSlot(t, rt, "traj-1", "implementation", "run-2", "agent-2")
	if claimed || existing.RunID != "run-1" {
		t.Fatalf("second claim = %+v claimed=%v, want run-1 unclaimed", existing, claimed)
	}
	if _, claimed := claimSlot(t, rt, "traj-1", "verifier", "run-2", "agent-2"); !claimed {
		t.Fatal("verifier claim refused")
	}
	if n := tableCount(t, rt.store.DB(), "co_super_slots"); n != 0 {
		t.Fatalf("setting on: table rows = %d, want 0", n)
	}
	requireEligible(t, rt, computerID)

	rec, found, err := rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-1", "implementation")
	if err != nil || !found || rec.RunID != "run-1" {
		t.Fatalf("EngineeringSlotRun = %+v %v %v", rec, found, err)
	}
	slot, found, err := rt.store.EngineeringSlotByAgentAndTrajectory(ctx, graphHomeOwner, "traj-1", "agent-2")
	if err != nil || !found || slot.Slot != "verifier" || slot.RequestedByRunID != "requester" {
		t.Fatalf("by agent+trajectory = %+v %v %v", slot, found, err)
	}
	slot, found, err = rt.store.EngineeringSlotByAgent(ctx, graphHomeOwner, "agent-1")
	if err != nil || !found || slot.RunID != "run-1" {
		t.Fatalf("by agent = %+v %v %v", slot, found, err)
	}
	if n, err := rt.store.CountActiveEngineeringSlots(ctx, graphHomeOwner, "traj-1"); err != nil || n != 2 {
		t.Fatalf("active slots = %d %v, want 2", n, err)
	}

	// The owner goes terminal: the next claim advances the slot.
	fin := time.Now().UTC()
	run1.State, run1.FinishedAt, run1.UpdatedAt = types.RunCompleted, &fin, fin
	if err := rt.store.UpdateRun(ctx, run1); err != nil {
		t.Fatal(err)
	}
	makeRun(t, rt, "run-3", "agent-3", "traj-1", types.RunRunning)
	if _, claimed := claimSlot(t, rt, "traj-1", "implementation", "run-3", "agent-3"); !claimed {
		t.Fatal("advance over a terminal owner refused")
	}
	rec, _, _ = rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-1", "implementation")
	if rec.RunID != "run-3" {
		t.Fatalf("slot owner after advance = %q, want run-3", rec.RunID)
	}

	// Release clears only the matching run.
	if err := rt.store.ReleaseEngineeringSlotClaim(ctx, graphHomeOwner, "traj-1", "implementation", "run-1"); err != nil {
		t.Fatal(err)
	}
	if rec, found, _ := rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-1", "implementation"); !found || rec.RunID != "run-3" {
		t.Fatalf("a non-matching release cleared the slot: %+v %v", rec, found)
	}
	if err := rt.store.ReleaseEngineeringSlotClaim(ctx, graphHomeOwner, "traj-1", "implementation", "run-3"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-1", "implementation"); found {
		t.Fatal("slot still held after the matching release")
	}
	requireEligible(t, rt, computerID)
}

func TestSlotsConcurrentClaimsHaveOneWinner(t *testing.T) {
	computerID := "computer-slots-race"
	rt, _ := selfdevReplayRuntime(t, computerID)
	turnOn(t, rt, computerID, choirstore.SettingEngineeringSlotsGraph)
	const n = 6
	for i := 0; i < n; i++ {
		makeRun(t, rt, fmt.Sprintf("race-%d", i), fmt.Sprintf("agent-%d", i), "traj-race", types.RunRunning)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, claimed, err := rt.store.ClaimEngineeringSlot(context.Background(), graphHomeOwner, "traj-race", "implementation",
				fmt.Sprintf("race-%d", i), fmt.Sprintf("agent-%d", i), "requester")
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if claimed {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if winners != 1 {
		t.Fatalf("winners = %d, want 1", winners)
	}
}

func TestSlotRowsFromBeforeTheChangeAreImportedAndStillRead(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		move func(t *testing.T, rt *Runtime)
	}{
		{"the first claim imports", func(t *testing.T, rt *Runtime) {
			makeRun(t, rt, "run-new", "agent-new", "traj-new", types.RunRunning)
			if _, claimed := claimSlot(t, rt, "traj-new", "implementation", "run-new", "agent-new"); !claimed {
				t.Fatal("claim refused")
			}
		}},
		{"the explicit import", func(t *testing.T, rt *Runtime) {
			moved, err := rt.store.ImportEngineeringSlotsToGraph(ctx)
			if err != nil || moved != 1 {
				t.Fatalf("import = %d, %v", moved, err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computerID := "computer-slots-legacy"
			rt, _ := selfdevReplayRuntime(t, computerID)
			makeRun(t, rt, "run-old", "agent-old", "traj-old", types.RunRunning)
			// Written as the binary before this change did: direct SQL.
			if _, claimed := claimSlot(t, rt, "traj-old", "implementation", "run-old", "agent-old"); !claimed {
				t.Fatal("legacy claim refused")
			}
			requireIneligible(t, rt, computerID, "co_super_slots")
			turnOn(t, rt, computerID, choirstore.SettingEngineeringSlotsGraph)
			tc.move(t, rt)
			if n := tableCount(t, rt.store.DB(), "co_super_slots"); n != 0 {
				t.Fatalf("table rows after import = %d, want 0", n)
			}
			requireEligible(t, rt, computerID)
			rec, found, err := rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-old", "implementation")
			if err != nil || !found || rec.RunID != "run-old" {
				t.Fatalf("imported slot = %+v %v %v", rec, found, err)
			}
			slot, found, _ := rt.store.EngineeringSlotByAgent(ctx, graphHomeOwner, "agent-old")
			if !found || slot.TrajectoryID != "traj-old" || slot.RequestedByRunID != "requester" || slot.ClaimedAt.IsZero() {
				t.Fatalf("imported slot record = %+v", slot)
			}
		})
	}
}

func TestSlotRollbackReadsBothHomesAndNewerRowWinsTheImport(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-slots-rollback"
	rt, _ := selfdevReplayRuntime(t, computerID)
	makeRun(t, rt, "run-a", "agent-a", "traj-r", types.RunRunning)
	makeRun(t, rt, "run-b", "agent-b", "traj-r", types.RunRunning)
	turnOn(t, rt, computerID, choirstore.SettingEngineeringSlotsGraph)
	if _, claimed := claimSlot(t, rt, "traj-r", "implementation", "run-a", "agent-a"); !claimed {
		t.Fatal("claim refused")
	}
	// Rollback: off. run-a finishes; a new claim lands in the table.
	if err := rt.store.PutComputerSetting(ctx, choirstore.ComputerSetting{OwnerID: graphHomeOwner, ComputerID: computerID, Key: choirstore.SettingEngineeringSlotsGraph, Value: false}); err != nil {
		t.Fatal(err)
	}
	ra, _ := rt.store.GetRun(ctx, "run-a")
	fin := time.Now().UTC()
	ra.State, ra.FinishedAt, ra.UpdatedAt = types.RunCompleted, &fin, fin
	if err := rt.store.UpdateRun(ctx, ra); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, claimed := claimSlot(t, rt, "traj-r", "implementation", "run-b", "agent-b"); !claimed {
		t.Fatal("advance over the graph-held terminal slot refused")
	}
	if n := tableCount(t, rt.store.DB(), "co_super_slots"); n != 1 {
		t.Fatalf("rollback claim: table rows = %d, want 1", n)
	}
	if rec, found, _ := rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-r", "implementation"); !found || rec.RunID != "run-b" {
		t.Fatalf("the newer table row must win the read: %+v %v", rec, found)
	}
	// On again: the import folds the newer row in over the graph object.
	turnOn(t, rt, computerID, choirstore.SettingEngineeringSlotsGraph)
	if moved, err := rt.store.ImportEngineeringSlotsToGraph(ctx); err != nil || moved != 1 {
		t.Fatalf("import = %d, %v", moved, err)
	}
	if n := tableCount(t, rt.store.DB(), "co_super_slots"); n != 0 {
		t.Fatalf("table rows after re-import = %d", n)
	}
	if rec, found, _ := rt.store.EngineeringSlotRun(ctx, graphHomeOwner, "traj-r", "implementation"); !found || rec.RunID != "run-b" {
		t.Fatalf("after re-import: %+v %v", rec, found)
	}
	requireEligible(t, rt, computerID)
}

func TestUnboundStoreKeepsTheTableWhateverTheSettingSays(t *testing.T) {
	ctx := context.Background()
	s, err := choirstore.Open(t.TempDir() + "/unbound.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, key := range []string{choirstore.SettingEvidenceGraph, choirstore.SettingEngineeringSlotsGraph} {
		if err := s.PutComputerSetting(ctx, choirstore.ComputerSetting{OwnerID: graphHomeOwner, ComputerID: "c", Key: key, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateEvidence(ctx, evidenceRec("ev-1", "r", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ClaimEngineeringSlot(ctx, graphHomeOwner, "t", "s", "r", "a", ""); err != nil {
		t.Fatal(err)
	}
	if tableCount(t, s.DB(), "agent_evidence") != 1 || tableCount(t, s.DB(), "co_super_slots") != 1 {
		t.Fatal("an unbound store must keep the tables")
	}
}

func TestEvidenceAndSlotKindsAreGraphClassified(t *testing.T) {
	// The tables stay in the manifest as empty-until-supported: with the
	// settings on they are empty, and the graph kinds replay as og_objects.
	m := CurrentReplayAirworthinessManifest()
	for _, table := range []string{"agent_evidence", "co_super_slots", "og_objects"} {
		if _, ok := m.Entries[table]; !ok {
			t.Fatalf("manifest lost %s", table)
		}
	}
}

// Turning a setting on through the product API runs the import, so a
// computer nobody writes becomes eligible; the response reports it.
func TestTurningTheSettingOnThroughTheAPIImportsTheTables(t *testing.T) {
	computerID := "computer-graph-home-api"
	rt, _ := selfdevReplayRuntime(t, computerID)
	seedLegacyEvidence(t, rt.store.DB(), "ev-api", "researcher", time.Now().UTC().Add(-time.Hour))
	makeRun(t, rt, "run-api", "agent-api", "traj-api", types.RunRunning)
	claimSlot(t, rt, "traj-api", "implementation", "run-api", "agent-api")
	requireIneligible(t, rt, computerID, "agent_evidence")

	post := func(key string, value bool) map[string]any {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":%q,"value":%v}`, key, value)))
		req.Header.Set("X-Authenticated-User", graphHomeOwner)
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: %d %s", key, rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	// Off first: nothing moves.
	post(choirstore.SettingEvidenceGraph, false)
	if tableCount(t, rt.store.DB(), "agent_evidence") != 1 {
		t.Fatal("turning off moved rows")
	}
	out := post(choirstore.SettingEvidenceGraph, true)
	if imp, _ := out["import"].(map[string]any); imp == nil || imp["moved"] != float64(1) || imp["error"] != nil {
		t.Fatalf("evidence import report = %v", out["import"])
	}
	out = post(choirstore.SettingEngineeringSlotsGraph, true)
	if imp, _ := out["import"].(map[string]any); imp == nil || imp["moved"] != float64(1) {
		t.Fatalf("slots import report = %v", out["import"])
	}
	requireEligible(t, rt, computerID)
}
