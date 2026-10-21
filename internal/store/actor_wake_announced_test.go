package store

import (
	"context"
	"strings"
	"testing"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: every
// committed wake signals the outbox projector, which then lists unprojected
// wakes by a JSON match on $.projected over every outbox row ever written
// (2 to 4 s on the paper, before each wake). The store knows which rows each
// commit wrote, so a signalled sweep can read those by key. Failure modes:
//  1. an announced wake is not returned, or is returned by a metadata scan;
//  2. a wake already projected is returned again;
//  3. a signal that named no rows (a commit path that does not pass its
//     objects) is lost instead of asking for a full sweep;
//  4. taking the announced wakes twice returns them twice.

func metadataScanCalls(s *Store, field string) uint64 {
	var n uint64
	for op, st := range s.EngineMutexStats() {
		if strings.Contains(op, "ListObjectsByMetadata") && strings.Contains(op, field) {
			n += st.Calls
		}
	}
	return n
}

func TestAnnouncedActorWakesAreReadByKey(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	docID, head := storyResearchFixture(t, s, "aw-1")
	// The fixture's own commits announced wakes; start from a drained set.
	if _, _, err := s.TakeAnnouncedActorWakes(ctx); err != nil {
		t.Fatal(err)
	}
	receipt, err := askStoryResearch(s, docID, "expand:aw-1", head)
	if err != nil {
		t.Fatal(err)
	}
	scans := metadataScanCalls(s, "$.projected")
	wakes, full, err := s.TakeAnnouncedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if full {
		t.Fatal("a commit that passed its objects asked for a full sweep")
	}
	if len(wakes) != 1 || wakes[0].TargetAgentID != "research:"+docID || wakes[0].CanonicalID == "" {
		t.Fatalf("announced wakes = %+v; want research's one wake (receipt wake %s)", wakes, receipt.WakeID)
	}
	if got := metadataScanCalls(s, "$.projected") - scans; got != 0 {
		t.Fatalf("announced wakes read by %d metadata scans", got)
	}
	// 4: taken once.
	if again, _, _ := s.TakeAnnouncedActorWakes(ctx); len(again) != 0 {
		t.Fatalf("announced wakes taken twice: %+v", again)
	}
	// 2: a projected wake is not returned.
	s.noteAnnouncedActorWake(wakes[0].CanonicalID)
	if err := s.MarkActorWakeProjected(ctx, wakes[0].CanonicalID); err != nil {
		t.Fatal(err)
	}
	if after, _, _ := s.TakeAnnouncedActorWakes(ctx); len(after) != 0 {
		t.Fatalf("projected wake returned: %+v", after)
	}
	// 3: a signal with no rows asks for a full sweep, once.
	s.signalActorWakes()
	if _, full, _ := s.TakeAnnouncedActorWakes(ctx); !full {
		t.Fatal("a signal that named no rows did not ask for a full sweep")
	}
	if _, full, _ := s.TakeAnnouncedActorWakes(ctx); full {
		t.Fatal("the full-sweep request was not consumed")
	}
}
