package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A boot that replays a short tail onto a fenced store checks the replayed
// rows, not the whole table
// (problems/s0-vocab-rescan-fires-on-any-replay-2026-10-01.md, 2026-10-16:
// one replayed row cost the paper a 3 min 44 s whole-table pass). Ways the
// narrowed fence can fail, written before the code:
//
//   - it still scans every row (no gain);
//   - it trusts a replayed row whose reference names an ID the cutover
//     renamed, which the full pass would rewrite;
//   - it trusts a replay whose upcast renamed a row, so rows outside the
//     replay that reference the old ID are never rewritten;
//   - it trusts rows a crashed or resumed earlier process replayed, because
//     nothing durable recorded that a replay opened after the fence (this
//     gap exists on today's zero-replay fast path too);
//   - it leaves the report unstamped or the marker set, so every later boot
//     pays the full pass.

const vocabReplayFenceFiller = 40

// vocabReplayFenceTicksFull is the least progress a whole-table pass ticks
// over the filler rows (one per scanned object, plus the fence scan).
const vocabReplayFenceTicksFull = vocabReplayFenceFiller

// vocabReplayFenceTape builds genesis, a base batch of filler V2 agents plus
// extra base ops, then one event per tail batch.
func vocabReplayFenceTape(t *testing.T, base []computerevent.ProjectionOp, tail ...[]computerevent.ProjectionOp) []vocabUpcastTapeEvent {
	t.Helper()
	at := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	ops := make([]computerevent.ProjectionOp, 0, vocabReplayFenceFiller+len(base))
	for i := range vocabReplayFenceFiller {
		agent := fmt.Sprintf("management:filler-%d", i)
		ops = append(ops, vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, agent, "management", "idle", false, at, at)))
	}
	ops = append(ops, base...)
	return vocabUpcastTapeFromBatches(t, append([][]computerevent.ProjectionOp{ops}, tail...))
}

// vocabReplayFenceCutOver writes the base live (pre-cutover), runs the
// one-time cutover, and closes the store: the next Open is a new process.
func vocabReplayFenceCutOver(t *testing.T, path string, tape []vocabUpcastTapeEvent) *MigrationReport {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	vocabUpcastApplyLiveEvent(t, s, tape[0])
	vocabUpcastApplyLiveEvent(t, s, tape[1])
	rep, err := s.MigrateAndFenceServingVocabulary(context.Background(), true, nil)
	if err != nil {
		t.Fatalf("cutover: %v", err)
	}
	if rep.FencedAt == "" {
		t.Fatal("cutover did not stamp the fence")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return rep
}

func vocabReplayFenceReopen(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return s
}

func vocabReplayFenceRun(t *testing.T, s *Store, replayed bool) (*MigrationReport, int) {
	t.Helper()
	ticks := 0
	rep, err := s.MigrateAndFenceServingVocabulary(context.Background(), replayed, func() { ticks++ })
	if err != nil {
		t.Fatalf("fence: %v", err)
	}
	return rep, ticks
}

func vocabReplayFenceOnDisk(t *testing.T, s *Store) *MigrationReport {
	t.Helper()
	rep, err := s.loadVocabMigrationReport()
	if err != nil || rep == nil {
		t.Fatalf("load report: %v (nil=%v)", err, rep == nil)
	}
	return rep
}

func vocabReplayFenceBodies(t *testing.T, s *Store) string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), `SELECT canonical_id, body FROM og_objects`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var all strings.Builder
	for rows.Next() {
		var id, body string
		if err := rows.Scan(&id, &body); err != nil {
			t.Fatal(err)
		}
		all.WriteString(id + " " + body + "\n")
	}
	return all.String()
}

func TestVocabReplayFenceChecksOnlyReplayedRows(t *testing.T) {
	at := time.Date(2026, 10, 16, 16, 29, 59, 0, time.UTC)
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "management:tail", "management", "running", false, at, at))}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "narrow")
	cut := vocabReplayFenceCutOver(t, path, tape)

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if vocabReplayFenceOnDisk(t, s).ReplayOpenedAt != "" {
		t.Fatal("a replayed deposit wrote a durable marker (station V deleted it)")
	}
	rep, ticks := vocabReplayFenceRun(t, s, true)
	if ticks >= vocabReplayFenceTicksFull {
		t.Fatalf("one replayed row cost a whole-table pass (%d ticks over %d filler rows)", ticks, vocabReplayFenceFiller)
	}
	disk := vocabReplayFenceOnDisk(t, s)
	if disk.ReplayOpenedAt != "" || rep.ReplayOpenedAt != "" {
		t.Fatal("the narrowed fence left the replay marker set")
	}
	if disk.FencedAt == "" || disk.FencedAt == cut.FencedAt {
		t.Fatalf("the narrowed fence did not restamp the report (was %q, now %q)", cut.FencedAt, disk.FencedAt)
	}
	if !strings.Contains(vocabReplayFenceBodies(t, s), "management:tail") {
		t.Fatal("the replayed row is missing")
	}
	if !s.vocabCutover.Load() {
		t.Fatal("the write guard is not active after the narrowed fence")
	}
}

func TestVocabReplayFenceFallsBackOnLegacyReference(t *testing.T) {
	at := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	agentV1 := vocabUpcastAgentID(t, "super:root")
	agentV2 := vocabUpcastAgentID(t, "management:root")
	base := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "super:root", "super", "a", false, at, at))}
	// A V2-spelled event whose reference still names the agent's pre-cutover
	// ID: the cutover renamed it in its report, not in the upcast ledger.
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastEventObject(t, agentV1, "management", at.Add(time.Hour)))}
	tape := vocabReplayFenceTape(t, base, tail)
	path := vocabUpcastStorePath(t, "legacy-ref")
	cut := vocabReplayFenceCutOver(t, path, tape)
	if len(cut.OGObjects) == 0 {
		t.Fatal("fixture: the cutover renamed nothing")
	}

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if !strings.Contains(vocabReplayFenceBodies(t, s), agentV1) {
		t.Fatal("fixture: the upcast already rewrote the legacy reference")
	}
	_, ticks := vocabReplayFenceRun(t, s, true)
	if ticks < vocabReplayFenceTicksFull {
		t.Fatalf("a replayed legacy reference took the narrowed fence (%d ticks)", ticks)
	}
	bodies := vocabReplayFenceBodies(t, s)
	if strings.Contains(bodies, agentV1) || !strings.Contains(bodies, agentV2) {
		t.Fatalf("the legacy reference was not rewritten:\n%s", bodies)
	}
	if vocabReplayFenceOnDisk(t, s).ReplayOpenedAt != "" {
		t.Fatal("the full fence left the replay marker set")
	}
}

func TestVocabReplayFenceFallsBackWhenReplayRenames(t *testing.T) {
	at := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "super:late", "super", "a", false, at, at))}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "rename")
	vocabReplayFenceCutOver(t, path, tape)

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	_, ticks := vocabReplayFenceRun(t, s, true)
	if ticks < vocabReplayFenceTicksFull {
		t.Fatalf("a replay whose upcast renamed a row took the narrowed fence (%d ticks)", ticks)
	}
	if !strings.Contains(vocabReplayFenceBodies(t, s), vocabUpcastAgentID(t, "management:late")) {
		t.Fatal("the renamed agent is missing")
	}
}

// TestVocabReplayFenceDistrustsAReplayFromAnotherProcess pinned the durable
// marker that sent an earlier process's replay to the full pass; station V
// deleted it (vocab_boot_fence_test.go pins the boot without it).

// An alias can reach the durable upcast ledger without a replay marker: a
// process that crashed between recording the alias and marking the report,
// or a batch that rolled back after an earlier op recorded one. Rows outside
// the replay may still reference the old ID, so the narrowed fence must
// notice the ledger grew since the fence (panel review 2026-10-16).
func TestVocabReplayFenceFallsBackWhenTheLedgerGrewWithoutAMarker(t *testing.T) {
	at := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "management:tail", "management", "running", false, at, at))}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "orphan-alias")
	vocabReplayFenceCutOver(t, path, tape)

	crashed := vocabReplayFenceReopen(t, path)
	upcaster, err := crashed.depositUpcastForBatch(true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := upcaster.recordAlias(vocabUpcastAgentID(t, "super:orphan"), vocabUpcastAgentID(t, "management:orphan"), false); err != nil {
		t.Fatal(err)
	}
	if err := crashed.Close(); err != nil {
		t.Fatal(err)
	}

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	if vocabReplayFenceOnDisk(t, s).ReplayOpenedAt != "" {
		t.Fatal("fixture: the orphan alias left a marker")
	}
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if _, ticks := vocabReplayFenceRun(t, s, true); ticks < vocabReplayFenceTicksFull {
		t.Fatalf("a ledger that grew without a marker took the narrowed fence (%d ticks)", ticks)
	}
}
