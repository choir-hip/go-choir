package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Station V (docs/definitions/choir-v-boot-vocab-pass-removal-2026-10-27.md):
// a boot never pays the whole-store vocabulary pass on a store the cutover
// already fenced. The paper was held down 17:48 to 19:19 on 2026-10-27 by
// that pass, OOM-killed on every boot
// (docs/problems/paper-down-on-full-vocabulary-rescan-2026-10-27.md).

func vocabBootFenceRun(t *testing.T, s *Store, replayed bool) (*MigrationReport, int) {
	t.Helper()
	ticks := 0
	rep, err := s.FenceServingVocabularyAtBoot(context.Background(), replayed, func() { ticks++ })
	if err != nil {
		t.Fatalf("boot fence: %v", err)
	}
	return rep, ticks
}

// The paper's loop: a process replays onto a fenced store and dies before
// its fence; the next boot replays nothing. No marker is written, and the
// boot pays nothing.
func TestBootVocabFenceSkipsAfterAnEarlierProcessReplayed(t *testing.T) {
	at := time.Date(2026, 10, 27, 17, 48, 0, 0, time.UTC)
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "management:tail", "management", "running", false, at, at))}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "boot-earlier")
	vocabReplayFenceCutOver(t, path, tape)

	crashed := vocabReplayFenceReopen(t, path)
	vocabUpcastApplyTapeEvent(t, crashed, tape[2])
	if vocabReplayFenceOnDisk(t, crashed).ReplayOpenedAt != "" {
		t.Fatal("a replay wrote a durable marker")
	}
	if err := crashed.Close(); err != nil {
		t.Fatal(err)
	}

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	if _, ticks := vocabBootFenceRun(t, s, false); ticks != 0 {
		t.Fatalf("a boot after an earlier process's replay paid a pass (%d ticks)", ticks)
	}
	if !s.vocabCutover.Load() {
		t.Fatal("the write guard is not armed")
	}
}

// A report an older build left with its marker set is not a reason to scan.
func TestBootVocabFenceIgnoresAMarkerFromAnOlderBuild(t *testing.T) {
	at := time.Date(2026, 10, 27, 17, 48, 0, 0, time.UTC)
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "management:tail", "management", "running", false, at, at))}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "boot-old-marker")
	vocabReplayFenceCutOver(t, path, tape)

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	rep := vocabReplayFenceOnDisk(t, s)
	rep.ReplayOpenedAt = at.Format(time.RFC3339Nano)
	if err := s.persistVocabMigrationReport(rep); err != nil {
		t.Fatal(err)
	}
	if _, ticks := vocabBootFenceRun(t, s, false); ticks != 0 {
		t.Fatalf("an older build's marker cost a pass (%d ticks)", ticks)
	}
	if vocabReplayFenceOnDisk(t, s).ReplayOpenedAt != "" {
		t.Fatal("the boot left the older build's marker set")
	}
}

// This boot replayed a row the narrowed check cannot prove (a reference to
// an ID the cutover renamed): the boot serves, with the write guard armed,
// and does not scan the store. A rebuild migrates such rows.
func TestBootVocabFenceServesWithoutAPassOnAnUnprovenReplayedRow(t *testing.T) {
	at := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	agentV1 := vocabUpcastAgentID(t, "super:root")
	base := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "super:root", "super", "a", false, at, at))}
	tail := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastEventObject(t, agentV1, "management", at.Add(time.Hour)))}
	tape := vocabReplayFenceTape(t, base, tail)
	path := vocabUpcastStorePath(t, "boot-unproven")
	vocabReplayFenceCutOver(t, path, tape)

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if _, ticks := vocabBootFenceRun(t, s, true); ticks >= vocabReplayFenceTicksFull {
		t.Fatalf("a boot scanned the store for one unproven replayed row (%d ticks)", ticks)
	}
	if !s.vocabCutover.Load() {
		t.Fatal("the write guard is not armed")
	}
	if vocabReplayFenceOnDisk(t, s).FencedAt == "" {
		t.Fatal("the boot unstamped the fence")
	}
}

// A store the cutover never fenced (an old local store, a fresh realization
// of V1 tape) still gets the one-time migration at boot.
func TestBootVocabFenceMigratesAnUnfencedStore(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	base := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "super:root", "super", "a", false, at, at))}
	tape := vocabReplayFenceTape(t, base)
	path := vocabUpcastStorePath(t, "boot-unfenced")
	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyLiveEvent(t, s, tape[0])
	vocabUpcastApplyLiveEvent(t, s, tape[1])
	rep, _ := vocabBootFenceRun(t, s, true)
	if rep == nil || rep.FencedAt == "" || len(rep.OGObjects) == 0 {
		t.Fatalf("an unfenced store was not migrated at boot: %+v", rep)
	}
	if !strings.Contains(vocabReplayFenceBodies(t, s), vocabUpcastAgentID(t, "management:root")) {
		t.Fatal("the V1 agent was not renamed")
	}
}
