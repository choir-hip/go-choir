package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// F11 (docs/design/choir-event-table-f-c-2026-10-22.md section 3.6): the
// vocabulary fences must see choir_events. The full pass reads both
// object-row tables; the narrowed replay fence re-reads a replayed stamped op
// from choir_events by tape position and never skips one it cannot find.

// A V1 role leaf in a choir_events row makes the full fence pass fail
// closed.
func TestEventTableFullFenceReadsTheTable(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "full-fence")
	if _, err := s.db.Exec(`INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at)
		VALUES (9, 0, 'obj:choir.event:owner:v1', 'choir.event', 'owner', 'h', '{"actor_profile":"super"}', '{"actor_profile":"super"}', NOW(), NOW())`); err != nil {
		t.Fatal(err)
	}
	_, err := s.MigrateAndFenceServingVocabulary(context.Background(), true, nil)
	if err == nil || !strings.Contains(err.Error(), "vocab serving fence") {
		t.Fatalf("a V1 role leaf in choir_events passed the full fence: %v", err)
	}

	clean, _ := vocabUpcastOpenStore(t, "full-fence-clean")
	if _, err := clean.db.Exec(`INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at)
		VALUES (9, 0, 'obj:choir.event:owner:v2', 'choir.event', 'owner', 'h', '{"actor_profile":"management"}', '{"actor_profile":"management"}', NOW(), NOW())`); err != nil {
		t.Fatal(err)
	}
	if _, err := clean.MigrateAndFenceServingVocabulary(context.Background(), true, nil); err != nil {
		t.Fatalf("test premise: a V2 row passes the fence: %v", err)
	}
}

// A clean replayed stamped op keeps the narrowed path.
func TestEventTableNarrowedFenceTrustsACleanStampedRow(t *testing.T) {
	at := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	tail := []computerevent.ProjectionOp{eventTableOp(t, eventTableRecord("fenced-1", "run-f", vocabUpcastOwnerID, "traj-f", types.EventToolResult, 1, 1, at), true)}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "narrow-stamped")
	vocabReplayFenceCutOver(t, path, tape)

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if n := tableCount(t, s, "choir_events", ""); n != 1 {
		t.Fatalf("fixture: %d stamped rows", n)
	}
	if _, ticks := vocabReplayFenceRun(t, s, true); ticks >= vocabReplayFenceTicksFull {
		t.Fatalf("a clean stamped row cost the full pass (%d ticks)", ticks)
	}
}

// A replayed stamped op whose reference names an ID the cutover renamed
// must take the full pass, as an og_objects row does. Today's fence re-reads
// og_objects only and skips a row it cannot find: it would fail open.
func TestEventTableNarrowedFenceRereadsStampedRows(t *testing.T) {
	at := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	agentV1 := vocabUpcastAgentID(t, "super:root")
	base := []computerevent.ProjectionOp{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "super:root", "super", "a", false, at, at))}
	tail := []computerevent.ProjectionOp{eventTableObjectOp(t, vocabUpcastEventObject(t, agentV1, "management", at.Add(time.Hour)), true)}
	tape := vocabReplayFenceTape(t, base, tail)
	path := vocabUpcastStorePath(t, "narrow-stamped-legacy")
	cut := vocabReplayFenceCutOver(t, path, tape)
	if len(cut.OGObjects) == 0 {
		t.Fatal("fixture: the cutover renamed nothing")
	}

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if n := tableCount(t, s, "choir_events", ""); n != 1 {
		t.Fatalf("fixture: %d stamped rows", n)
	}
	if _, ticks := vocabReplayFenceRun(t, s, true); ticks < vocabReplayFenceTicksFull {
		t.Fatalf("a stamped row with a legacy reference took the narrowed fence (%d ticks)", ticks)
	}
}

// A tracked stamped position whose row is missing returns false: the full
// pass decides, it is never skipped.
func TestEventTableNarrowedFenceNeverSkipsAMissingStampedRow(t *testing.T) {
	at := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	tail := []computerevent.ProjectionOp{eventTableOp(t, eventTableRecord("fenced-2", "run-f", vocabUpcastOwnerID, "traj-f", types.EventToolResult, 1, 1, at), true)}
	tape := vocabReplayFenceTape(t, nil, tail)
	path := vocabUpcastStorePath(t, "narrow-missing")
	vocabReplayFenceCutOver(t, path, tape)

	s := vocabReplayFenceReopen(t, path)
	t.Cleanup(func() { _ = s.Close() })
	vocabUpcastApplyTapeEvent(t, s, tape[2])
	if _, err := s.db.Exec(`DELETE FROM choir_events`); err != nil {
		t.Fatal(err)
	}
	if _, ticks := vocabReplayFenceRun(t, s, true); ticks < vocabReplayFenceTicksFull {
		t.Fatalf("a missing stamped row was skipped (%d ticks)", ticks)
	}
}
