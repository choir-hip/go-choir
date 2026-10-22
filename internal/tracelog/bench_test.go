package tracelog

import (
	"fmt"
	"testing"
	"time"
)

// F16. Appends per second and allocations per append, on a real file with the
// 1 s timer syncing in the background (the production shape). The lines are
// 2 KiB, the middle of the 2 to 8 KB range the design guesses.
func BenchmarkAppend2KiB(b *testing.B) {
	dir := b.TempDir()
	l, err := Open(Options{Dir: dir, ComputerID: "computer-1", BootID: "boot-1"})
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	line := make([]byte, 2048)
	for i := range line {
		line[i] = 'a' + byte(i%26)
	}
	m := meta(1)
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Seq, m.StreamSeq = int64(i+1), int64(i+1)
		if err := l.Append(m, line); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAppend160B(b *testing.B) {
	dir := b.TempDir()
	l, err := Open(Options{Dir: dir, ComputerID: "computer-1", BootID: "boot-1"})
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	line := make([]byte, 160)
	m := meta(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Seq, m.StreamSeq = int64(i+1), int64(i+1)
		if err := l.Append(m, line); err != nil {
			b.Fatal(err)
		}
	}
}

// The barrier shape: one append and one explicit Sync, as before an
// acknowledged tape event.
func BenchmarkAppendThenSync(b *testing.B) {
	dir := b.TempDir()
	l, err := Open(Options{Dir: dir, ComputerID: "computer-1", BootID: "boot-1", SyncInterval: -1})
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	line := make([]byte, 2048)
	m := meta(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Seq, m.StreamSeq = int64(i+1), int64(i+1)
		if err := l.Append(m, line); err != nil {
			b.Fatal(err)
		}
		if err := l.Sync(); err != nil {
			b.Fatal(err)
		}
	}
}

// A run's page across 168 sealed segments (a week of hourly segments), the
// design's F16 bar of 50 ms.
func BenchmarkRunPageAcross168Segments(b *testing.B) {
	dir := b.TempDir()
	clk := newClock()
	l, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1, SealRecords: 1 << 30, Now: clk.Now})
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	line := make([]byte, 1024)
	n := 0
	for s := 0; s < 168; s++ {
		for i := 0; i < 1000; i++ { // 1,000 lines an hour, 40 runs
			n++
			m := meta(n)
			m.RunID = fmt.Sprintf("run-%d", i%40)
			m.Seq = int64(s*25 + i/40 + 1)
			if err := l.Append(m, line); err != nil {
				b.Fatal(err)
			}
		}
		clk.Advance(61 * time.Minute)
		if _, err := l.Maintain(); err != nil {
			b.Fatal(err)
		}
	}
	r := l.Reader()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recs, err := r.ByRun("owner-1", "run-7", 0, 200)
		if err != nil || len(recs) != 200 {
			b.Fatalf("%d records, %v", len(recs), err)
		}
	}
}
