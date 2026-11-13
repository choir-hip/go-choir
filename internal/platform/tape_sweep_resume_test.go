package platform

import (
	"errors"
	"os"
	"sort"
	"testing"
)

// Tests for the durable reference walk (docs/definitions/
// choir-storage-solutions-2-2026-11-12.md, S7). Failure list, written before
// the code:
//
//	R1 a walk interrupted by a restart leaves its progress in the control
//	   database and the next dry-run resumes from it (fewer rows walked);
//	R2 the resumed run's manifest lists exactly the items an uninterrupted
//	   run lists;
//	R3 a chunk that fails any check (anchor, checksum, gap) is never used:
//	   that computer's walk restarts from sequence 0 and the manifest is
//	   still equal;
//	R4 a head below the stored progress, or progress that cannot serve an
//	   event-reading walk, is discarded;
//	R5 an unreadable event still refuses the payload sweep after a resume;
//	R6 progress is dropped when a dry-run completes (item 1 only; the
//	   watermark commit replaces this with retention);
//	R7 a destroyed computer's apply removes its stored progress.

var errTapeTestInterrupt = errors.New("test: corpusd restarted")

func resumeFixture(t *testing.T) (*tapeFixture, *tapeChain, *tapeChain) {
	t.Helper()
	f := newTapeFixture(t)
	f.live("live-1")
	f.svc.tapeSweep.refChunkEvents = 4
	dead := f.chain("dead-1", 9, tapeAged)
	live := f.chain("live-1", 11, tapeAged)
	f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
	return f, dead, live
}

func itemKeys(items []tapeSweepItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Sweep+"|"+it.Namespace+"|"+it.Digest)
	}
	sort.Strings(out)
	return out
}

func equalKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// interruptAfter makes the n-th persisted chunk the last thing the run does.
func interruptAfter(f *tapeFixture, n int) {
	count := 0
	f.svc.tapeSweep.afterRefChunk = func(string, uint64) error {
		count++
		if count == n {
			return errTapeTestInterrupt
		}
		return nil
	}
}

func (f *tapeFixture) interruptedRun(n int) {
	f.t.Helper()
	interruptAfter(f, n)
	_, err := f.svc.RunTapeSweep(f.ctx, TapeSweepRequest{Action: TapeSweepActionDryRun, AcceptUnrecordedForkLineage: true})
	f.svc.tapeSweep.afterRefChunk = nil
	if !errors.Is(err, errTapeTestInterrupt) {
		f.t.Fatalf("interrupted run error = %v, want the test interrupt", err)
	}
}

func (f *tapeFixture) resumeDry() TapeSweepReport {
	return f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true})
}

func (f *tapeFixture) chunkCount(id string) int64 { return f.rowCount("tape_sweep_ref_chunks", id) }

// R1, R2
func TestTapeSweepResumesAnInterruptedReferenceWalk(t *testing.T) {
	f, _, _ := resumeFixture(t)
	base := f.resumeDry()
	if base.ChainEventsWalked != 20 || base.ChainEventsResumed != 0 {
		t.Fatalf("uninterrupted run walked %d resumed %d, want 20 and 0", base.ChainEventsWalked, base.ChainEventsResumed)
	}
	want := itemKeys(f.manifestItems(base))
	if len(want) == 0 {
		t.Fatal("fixture lists nothing; the comparison would prove nothing")
	}

	// dead-1 persists chunks to 4, 8, 9 and live-1 to 4 on the fourth.
	f.interruptedRun(4)
	if f.chunkCount("dead-1") != 3 || f.chunkCount("live-1") != 1 {
		t.Fatalf("progress after the interrupt: dead-1=%d live-1=%d chunks, want 3 and 1", f.chunkCount("dead-1"), f.chunkCount("live-1"))
	}
	got := f.resumeDry()
	if got.ChainEventsResumed != 13 || got.ChainEventsWalked != 7 {
		t.Fatalf("resumed run resumed %d walked %d, want 13 and 7", got.ChainEventsResumed, got.ChainEventsWalked)
	}
	if !equalKeys(want, itemKeys(f.manifestItems(got))) {
		t.Fatalf("resumed manifest differs from the uninterrupted one:\nwant %v\ngot  %v", want, itemKeys(f.manifestItems(got)))
	}
	if got.ChainEvents != base.ChainEvents || got.Computers != base.Computers {
		t.Fatalf("report totals differ: %+v vs %+v", got, base)
	}
}

// R3: a chunk that fails a check restarts that computer's walk only.
func TestTapeSweepResumeDiscardsAnUnverifiedChunk(t *testing.T) {
	cases := map[string]string{
		"anchor":   `UPDATE tape_sweep_ref_chunks SET anchor_event_digest=repeat('a',64) WHERE computer_id='dead-1' AND to_sequence=8`,
		"checksum": `UPDATE tape_sweep_ref_chunks SET refs=decode('0000000000000000','hex') WHERE computer_id='dead-1' AND to_sequence=4`,
		"gap":      `DELETE FROM tape_sweep_ref_chunks WHERE computer_id='dead-1' AND to_sequence=4`,
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f, _, _ := resumeFixture(t)
			want := itemKeys(f.manifestItems(f.resumeDry()))
			f.interruptedRun(4)
			if _, err := f.store.db.Exec(mutate); err != nil {
				t.Fatal(err)
			}
			got := f.resumeDry()
			// Only live-1's 4 rows resume; dead-1 restarts from 0.
			if got.ChainEventsResumed != 4 || got.ChainEventsWalked != 16 {
				t.Fatalf("resumed %d walked %d, want 4 and 16", got.ChainEventsResumed, got.ChainEventsWalked)
			}
			if !equalKeys(want, itemKeys(f.manifestItems(got))) {
				t.Fatal("manifest differs after discarding the chunk")
			}
		})
	}
}

// R4
func TestTapeSweepRefChunkLoaderRefusals(t *testing.T) {
	t.Run("head below the stored progress", func(t *testing.T) {
		f, _, _ := resumeFixture(t)
		f.interruptedRun(3)
		chunks, err := f.svc.loadTapeRefChunks(f.ctx, "dead-1", 5, false)
		if err != nil || len(chunks) != 0 {
			t.Fatalf("chunks=%d err=%v for a head below the progress", len(chunks), err)
		}
		if f.chunkCount("dead-1") != 0 {
			t.Fatal("unusable progress was kept")
		}
	})
	t.Run("progress without event reads cannot serve an event-reading walk", func(t *testing.T) {
		f, _, _ := resumeFixture(t)
		f.interruptedRun(3)
		if _, err := f.store.db.Exec(`UPDATE tape_sweep_ref_chunks SET read_events=false WHERE computer_id='dead-1' AND to_sequence=8`); err != nil {
			t.Fatal(err)
		}
		chunks, err := f.svc.loadTapeRefChunks(f.ctx, "dead-1", 9, true)
		if err != nil || len(chunks) != 0 {
			t.Fatalf("chunks=%d err=%v", len(chunks), err)
		}
	})
	t.Run("progress without event reads serves a walk that does not read", func(t *testing.T) {
		f, _, _ := resumeFixture(t)
		f.interruptedRun(3)
		if _, err := f.store.db.Exec(`UPDATE tape_sweep_ref_chunks SET read_events=false WHERE computer_id='dead-1'`); err != nil {
			t.Fatal(err)
		}
		chunks, err := f.svc.loadTapeRefChunks(f.ctx, "dead-1", 9, false)
		if err != nil || len(chunks) != 3 {
			t.Fatalf("chunks=%d err=%v, want 3", len(chunks), err)
		}
	})
}

// R5
func TestTapeSweepResumeStillRefusesAnUnreadableEvent(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.svc.tapeSweep.refChunkEvents = 2
	live := f.chain("live-1", 6, tapeAged)
	f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
	if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, live.seqs[2].event))); err != nil {
		t.Fatal(err)
	}
	if base := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}}); base.Orphans[tapeNSPayload].Refused == "" {
		t.Fatal("uninterrupted run did not refuse the payload sweep")
	}
	interruptAfter(f, 3) // chunks to 2, 4 (holds the unreadable event) and 6
	_, err := f.svc.RunTapeSweep(f.ctx, TapeSweepRequest{Action: TapeSweepActionDryRun, Sweeps: []string{TapeSweepOrphans}})
	f.svc.tapeSweep.afterRefChunk = nil
	if !errors.Is(err, errTapeTestInterrupt) {
		t.Fatalf("err = %v", err)
	}
	got := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if got.Orphans[tapeNSPayload].Refused == "" {
		t.Fatalf("a resumed run swept payloads past an unreadable event: %+v", got.Orphans)
	}
	for _, it := range f.manifestItems(got) {
		if it.Namespace == tapeNSPayload {
			t.Fatalf("payload %s listed after a resume", it.Digest)
		}
	}
}

// R6
func TestTapeSweepDropsProgressWhenADryRunCompletes(t *testing.T) {
	f, _, _ := resumeFixture(t)
	f.interruptedRun(4)
	f.resumeDry()
	if f.chunkCount("dead-1")+f.chunkCount("live-1") != 0 {
		t.Fatal("a completed dry-run left progress behind")
	}
}

// R7
func TestTapeSweepApplyPurgesStoredProgress(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(destroyedReq(true))
	if _, err := f.store.db.Exec(`INSERT INTO tape_sweep_ref_chunks (computer_id,from_sequence,to_sequence,read_events,anchor_event_digest,refs,refs_sha256,created_at) VALUES ('dead-1',0,1,true,repeat('a',64),decode('','hex'),repeat('b',64),now()) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.apply(dry, destroyedReq(true)); err != nil {
		t.Fatal(err)
	}
	if f.chunkCount("dead-1") != 0 {
		t.Fatal("purge left the computer's reference progress")
	}
}
