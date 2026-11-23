package snapshot

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/filecas"
)

// Review round 3: the probe covers the partition chunks too, a candidate phase
// has a deadline, and a store that cannot probe says so once.

func TestRecordChunkDigestsIncludeThePartitions(t *testing.T) {
	rec := Record{}
	rec.Manifest.Files = []filecas.FileEntry{{Path: "state.db", Chunks: []string{"a", "b"}}}
	rec.Partitions = []PartitionEntry{{Path: "telemetry/x.db", Chunks: []string{"c", "d"}}}
	got := recordChunkDigests(rec)
	if len(got) != 4 || got[2] != "c" || got[3] != "d" {
		t.Fatalf("probe covers %v: partition chunks must be probed with the state file's", got)
	}
}

type downSource struct {
	*fakeTape
	mu      sync.Mutex
	fetches int
}

func (s *downSource) FetchPayload(context.Context, string, string) ([]byte, error) {
	s.mu.Lock()
	s.fetches++
	s.mu.Unlock()
	return nil, errors.New("host is down")
}

func TestInstallStopsStartingCandidatesPastItsBudget(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	src := &downSource{fakeTape: f.tape}
	cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	cfg.Source = src
	cfg.CandidateBudget = time.Nanosecond
	cands := []Candidate{{EventID: "a", Digest: strings.Repeat("a", 64)}, {EventID: "b", Digest: strings.Repeat("b", 64)}, {EventID: "c", Digest: strings.Repeat("c", 64)}}
	if _, err := Install(ctx, cfg, cands); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("Install = %v, want a refusal naming the candidate budget", err)
	}
	if src.fetches != 1 {
		t.Fatalf("%d candidates were tried after the budget was spent, want only the first", src.fetches)
	}
}

// noProbeChunks has no HasChunk.
type noProbeChunks struct{ m *memChunks }

func (p noProbeChunks) PutChunk(ctx context.Context, d string, b []byte) error {
	return p.m.PutChunk(ctx, d, b)
}

func (p noProbeChunks) GetChunk(ctx context.Context, d string) ([]byte, error) {
	return p.m.GetChunk(ctx, d)
}

func TestInstallSaysOnceWhenTheChunkStoreCannotProbe(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	if _, err := TakeSnapshot(ctx, f.cfg); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	noProbeOnce = sync.Once{}
	for i := 0; i < 2; i++ {
		cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
		cfg.Chunks = noProbeChunks{f.chunks}
		if _, err := Restore(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(buf.String(), "cannot probe"); n != 1 {
		t.Fatalf("the skipped probe was logged %d times, want once: %q", n, buf.String())
	}
}

// Ways the restore's candidate selection could fail (lane/snapshot-by-replay-time,
// review round 2: the host keeps a chain of roots the newest-4 scan cannot reach):
//
//  a. the scan keeps only the newest four snapshot events at or below the
//     target, so with frequent snapshots they are all pruned rows and a live
//     root a few events lower is never tried;
//  b. a candidate the host no longer holds is downloaded chunk by chunk before
//     it fails, which on a 46 GB store is hours;
//  c. the bound: a snapshot older than target - MaxRecoveryTailEvents is
//     offered, or one above the target is;
//  d. a root the tape does not record is installed because the host holds its
//     chunks and its sealed record, or a record is installed under another
//     event's identity.

func TestScanAllReturnsEverySnapshotEventAndScanTheNewestFour(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	for i := 0; i < 7; i++ {
		if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, x'00', 'n')`, 2000+i); err != nil {
			t.Fatal(err)
		}
		if _, err := TakeSnapshot(ctx, f.cfg); err != nil {
			t.Fatal(err)
		}
	}
	all, err := ScanAll(ctx, f.tape, testComputer, nil)
	if err != nil || len(all) != 7 {
		t.Fatalf("ScanAll = %d candidates, %v; want 7", len(all), err)
	}
	newest, err := Scan(ctx, f.tape, testComputer, nil)
	if err != nil || len(newest) != MaxCandidates || newest[len(newest)-1] != all[len(all)-1] {
		t.Fatalf("Scan = %d candidates, %v; want the newest %d", len(newest), err, MaxCandidates)
	}
}

func TestPlanCandidatesBoundsByTargetAndByTheReplayBound(t *testing.T) {
	mk := func(seqs ...uint64) []Candidate {
		var out []Candidate
		for _, s := range seqs {
			out = append(out, Candidate{Sequence: s})
		}
		return out
	}
	seqs := func(c []Candidate) []uint64 {
		var out []uint64
		for _, x := range c {
			out = append(out, x.Sequence)
		}
		return out
	}
	got := PlanCandidates(mk(10, 100, 500, 900, 1200), 1000, 600)
	if want := []uint64{500, 900}; len(got) != 2 || got[0].Sequence != want[0] || got[1].Sequence != want[1] {
		t.Fatalf("got %v, want %v: above the target and older than target-bound are out, oldest first", seqs(got), want)
	}
	if got := PlanCandidates(mk(10, 100), 500, 600); len(got) != 2 {
		t.Fatalf("a target inside the bound keeps every snapshot below it: %v", seqs(got))
	}
	var many []Candidate
	for i := uint64(1); i <= MaxRestoreAttempts+50; i++ {
		many = append(many, Candidate{Sequence: i})
	}
	got = PlanCandidates(many, MaxRestoreAttempts+50, 1<<40)
	if len(got) != MaxRestoreAttempts || got[len(got)-1].Sequence != MaxRestoreAttempts+50 {
		t.Fatalf("%d candidates, newest %d: the attempts are capped at the newest %d", len(got), got[len(got)-1].Sequence, MaxRestoreAttempts)
	}
}

// countingChunks counts the chunk downloads.
type countingChunks struct {
	*memChunks
	mu   sync.Mutex
	gets int
}

func (c *countingChunks) GetChunk(ctx context.Context, digest string) ([]byte, error) {
	c.mu.Lock()
	c.gets++
	c.mu.Unlock()
	return c.memChunks.GetChunk(ctx, digest)
}

// A candidate whose chunks the host no longer holds is skipped by a probe,
// before a byte of it is downloaded; the older live one is installed.
func TestRestoreSkipsAPrunedCandidateWithoutDownloadingIt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 9<<20)
	first, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (5000, x'00', 'later')`); err != nil {
		t.Fatal(err)
	}
	second, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]bool{}
	for _, d := range first.Record.Manifest.Files[0].Chunks {
		held[d] = true
	}
	pruned := 0
	for _, d := range second.Record.Manifest.Files[0].Chunks {
		if !held[d] {
			delete(f.chunks.chunks, d) // the host swept what only the newer root named
			pruned++
		}
	}
	if pruned == 0 || len(second.Record.Manifest.Files[0].Chunks) < 3 {
		t.Fatalf("test premise: the second snapshot has %d unique chunks of %d", pruned, len(second.Record.Manifest.Files[0].Chunks))
	}
	counting := &countingChunks{memChunks: f.chunks}
	cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	cfg.Chunks = counting
	record, err := Restore(ctx, cfg)
	if err != nil || record.PageDigest != first.Record.PageDigest {
		t.Fatalf("Restore = %v, %v; want the older live snapshot", record.PageDigest, err)
	}
	if want := len(first.Record.Manifest.Files[0].Chunks); counting.gets != want {
		t.Fatalf("%d chunk downloads, want %d: the pruned candidate must be skipped by a probe", counting.gets, want)
	}
}

// hidingTape is a host that serves payloads and chunks but whose tape does
// not record the snapshot.
type hidingTape struct{ *fakeTape }

func (hidingTape) EventsPage(context.Context, string, uint64, int) ([]computerevent.DurableEvent, error) {
	return nil, nil
}

// A root the tape does not record is never installed, whatever the host holds;
// and a sealed record offered under another event's identity does not open.
func TestARootTheTapeDoesNotRecordIsRefused(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	cfg.Source = hidingTape{f.tape}
	if _, err := Restore(ctx, cfg); err == nil {
		t.Fatal("a snapshot the tape does not record was installed because the host holds it")
	}
	digest := ""
	for d := range f.tape.payloads {
		digest = d
	}
	cfg = f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	if _, err := Install(ctx, cfg, []Candidate{{EventID: "event-the-host-made-up", Digest: digest, Sequence: ref.Record.StoreHeadSequence}}); err == nil {
		t.Fatal("a sealed record was installed under an event identity it was not sealed for")
	}
}
